package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// borders holds the unicode characters used to draw the box around
// command output.
type borders struct {
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
	Horizontal  string
	Vertical    string
}

var defaultBorderStyle = borders{
	TopLeft:     "╭",
	TopRight:    "╮",
	BottomLeft:  "╰",
	BottomRight: "╯",
	Horizontal:  "─",
	Vertical:    "│",
}

// ansiPattern matches ANSI escape sequences (CSI, OSC, etc.) so they can be
// excluded when computing the visual width of a line.
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07]*\x07`)

// BorderWriter wraps an io.Writer and decorates the output with a rounded box
// border that spans the terminal width minus one column on each side.
//
// When the underlying writer is not a terminal the BorderWriter acts as a
// transparent pass-through; this keeps CI logs and piped output clean.
//
// The top border is emitted lazily on the first Write, and the bottom border
// on Close. Commands that produce no output therefore don't render an empty
// card; the BorderWriter only decorates output that actually exists.
//
// Typical usage:
//
//	bw := internal.NewBorderWriter(os.Stdout)
//	defer bw.Close()
//	cmd.Stdout = bw
//	cmd.Stderr = bw
//	_ = cmd.Run()
type BorderWriter struct {
	out     io.Writer
	style   borders
	color   *color.Color
	width   int          // total terminal width
	content int          // inner width available for content; zero disables decoration
	started bool         // true once the top border has been emitted
	pending bytes.Buffer // current partial line not yet terminated by \n
}

// NewBorderWriter constructs a BorderWriter that draws a box around anything
// written to it before forwarding to w. If w is not a terminal, or the
// terminal is too narrow to hold any content, the writer becomes a transparent
// pass-through.
func NewBorderWriter(w io.Writer) *BorderWriter {
	bw := &BorderWriter{
		out:   w,
		style: defaultBorderStyle,
		color: color.New(color.FgHiBlack),
	}

	if !IsTerminalWriter(w) {
		return bw
	}

	bw.width = getTerminalWidth(w)
	// layout is "   │ <content> │"; left edge at column 4 (aligned with the
	// harness LogStep/LogCommand text), right edge at the second-to-last column.
	// reservation: 3 leading spaces + 1 vertical + 1 space + content + 1 space + 1 vertical + 1 trailing column = width
	if c := bw.width - 8; c > 0 {
		bw.content = c
	}

	return bw
}

// Wrap returns a writer that forwards output to b, while preserving any
// user-provided writer (other than os.Stdout/os.Stderr or b's own output sink,
// which would cause duplicate output).
func (b *BorderWriter) Wrap(existing io.Writer) io.Writer {
	if existing == nil || existing == os.Stdout || existing == os.Stderr || existing == b.out {
		return b
	}
	return io.MultiWriter(existing, b)
}

// leftIndent is the number of blank columns rendered before the left border
// character. Two of these align the box content with the harness step text;
// the remaining one matches the leading space used elsewhere in the output.
const leftIndent = "   "

// Write implements io.Writer. It buffers partial lines and emits each complete
// line wrapped with the box border characters.
func (b *BorderWriter) Write(text []byte) (int, error) {
	if b.content == 0 {
		return b.out.Write(text)
	}

	// lazy emit of the top border on the very first write
	if !b.started {
		line := b.style.TopLeft + strings.Repeat(b.style.Horizontal, b.width-6) + b.style.TopRight
		fmt.Fprintln(b.out, leftIndent+b.color.Sprint(line)) //nolint:errcheck
		b.started = true
	}

	written := 0
	for len(text) > 0 {
		idx := bytes.IndexByte(text, '\n')
		if idx < 0 {
			b.pending.Write(text)
			written += len(text)
			break
		}

		// accumulate the line up to (but not including) the newline
		b.pending.Write(text[:idx])
		b.emitLine(b.pending.Bytes())
		b.pending.Reset()

		written += idx + 1
		text = text[idx+1:]
	}

	return written, nil
}

// Close flushes any pending partial line and emits the bottom border. If no
// content was ever written the bottom border is skipped, so silent commands
// don't produce an empty card.
// Safe to call multiple times.
func (b *BorderWriter) Close() error {
	if b.content == 0 {
		return nil
	}

	// flush trailing partial line, if any
	if b.pending.Len() > 0 {
		b.emitLine(b.pending.Bytes())
		b.pending.Reset()
	}

	// disable further decoration so subsequent calls are no-ops
	width, started := b.width, b.started
	b.content = 0

	// nothing was ever written; skip the bottom border to avoid an empty card.
	if !started {
		return nil
	}

	line := b.style.BottomLeft + strings.Repeat(b.style.Horizontal, width-6) + b.style.BottomRight
	fmt.Fprintln(b.out, leftIndent+b.color.Sprint(line)) //nolint:errcheck
	return nil
}

// emitLine prints a single content line surrounded by the vertical border
// characters. Lines wider than the available content width are soft-wrapped
// across multiple rows; the active SGR style is carried over so colored
// output continues correctly across the wrap, and a reset is emitted before
// the right border so the border itself is never colored by the content.
func (b *BorderWriter) emitLine(line []byte) {
	left := leftIndent + b.color.Sprint(b.style.Vertical) + " "
	right := " " + b.color.Sprint(b.style.Vertical)

	for _, chunk := range b.wrapLine(line) {
		pad := b.content - chunk.width
		if pad < 0 {
			pad = 0
		}
		fmt.Fprintf(b.out, "%s%s%s%s%s\n", left, chunk.prefix, chunk.text, strings.Repeat(" ", pad), right) //nolint:errcheck
	}
}

// lineChunk is one row's worth of wrapped content.
type lineChunk struct {
	prefix string // active SGR style carried over from a previous row, "" for the first
	text   []byte // raw bytes (possibly containing ANSI sequences) to emit for this row
	width  int    // visible width in cells of text
}

// wrapLine splits line into chunks that each fit within b.content cells of
// visible width. ANSI escape sequences are passed through unchanged and
// excluded from the width count; SGR state from a previous row is re-emitted
// at the start of the next, and a reset is appended at the end of any row
// that did not already end with one.
func (b *BorderWriter) wrapLine(line []byte) []lineChunk {
	var (
		chunks     []lineChunk
		current    bytes.Buffer
		currentW   int
		activeSGR  string // SGR code in effect at end of last completed chunk; empty == reset
		nextPrefix string // SGR to re-apply at the start of the next chunk
	)

	flush := func() {
		if current.Len() == 0 && nextPrefix == "" {
			return
		}
		text := current.Bytes()
		if activeSGR != "" {
			text = append(text, []byte("\x1b[0m")...)
		}
		// copy because the buffer is reused
		chunks = append(chunks, lineChunk{
			prefix: nextPrefix,
			text:   append([]byte(nil), text...),
			width:  currentW,
		})
		current.Reset()
		currentW = 0
		// the next chunk starts with whatever SGR is currently active
		nextPrefix = activeSGR
	}

	for i := 0; i < len(line); {
		// detect ANSI escape sequences and pass them through without counting
		// their visible width; SGR sequences also update the active style.
		if loc := ansiPattern.FindIndex(line[i:]); loc != nil && loc[0] == 0 {
			esc := line[i : i+loc[1]]
			current.Write(esc)
			// SGR is the CSI subset ending in 'm'; it's the only state we carry
			// across a soft-wrap boundary.
			if esc[len(esc)-1] == 'm' {
				if s := string(esc); s == "\x1b[0m" || s == "\x1b[m" {
					activeSGR = ""
				} else {
					activeSGR = s
				}
			}
			i += loc[1]
			continue
		}

		// decode one rune and measure its display width
		r, size := utf8.DecodeRune(line[i:])
		w := runewidth.RuneWidth(r)
		if w == 0 {
			// zero-width rune (combining mark, ZWJ, etc.) — append without
			// flushing; it logically belongs to the previous grapheme.
			current.Write(line[i : i+size])
			i += size
			continue
		}

		// would this rune overflow the row? flush first.
		if currentW+w > b.content && currentW > 0 {
			flush()
		}

		current.Write(line[i : i+size])
		currentW += w
		i += size
	}

	flush()

	if len(chunks) == 0 {
		// preserve the empty-line case so we still render an empty bordered row
		chunks = append(chunks, lineChunk{})
	}

	return chunks
}

// getTerminalWidth returns the width in columns of the terminal backing w, or a
// reasonable fallback if it cannot be determined. Writers that implement
// Width() int can override the detection (used by tests).
func getTerminalWidth(w io.Writer) int {
	if sized, ok := w.(interface{ Width() int }); ok {
		if width := sized.Width(); width > 0 {
			return width
		}
	}
	if f, ok := w.(*os.File); ok {
		if width, _, err := term.GetSize(int(f.Fd())); err == nil && width > 0 {
			return width
		}
	}
	return 80
}
