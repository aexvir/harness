package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// borderStyle holds the unicode characters used to draw the box around
// command output.
type borderStyle struct {
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
	Horizontal  string
	Vertical    string
}

var defaultBorderStyle = borderStyle{
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
	out      io.Writer
	style    borderStyle
	color    *color.Color
	width    int  // total terminal width
	content  int  // inner width available for content
	enabled  bool // false when output is not a terminal
	started  bool
	closed   bool
	pending  bytes.Buffer // current partial line not yet terminated by \n
}

// NewBorderWriter constructs a BorderWriter that draws a box around anything
// written to it before forwarding to w.
func NewBorderWriter(w io.Writer) *BorderWriter {
	width := 0
	if IsTerminalWriter(w) {
		width = terminalWidth(w)
	}
	return newBorderWriter(w, width)
}

// newBorderWriter is the constructor used internally and by tests. A width of
// zero (or a non-tty target) disables the border decoration.
func newBorderWriter(w io.Writer, width int) *BorderWriter {
	bw := &BorderWriter{
		out:     w,
		style:   defaultBorderStyle,
		color:   color.New(color.FgHiBlack),
		enabled: width > 0 && IsTerminalWriter(w),
	}

	if bw.enabled {
		bw.width = width
		// layout is "   │ <content> │"; left edge at column 4 (aligned with the
		// harness LogStep/LogCommand text), right edge at the second-to-last column.
		// reservation: 3 leading spaces + 1 vertical + 1 space + content + 1 space + 1 vertical + 1 trailing column = width
		bw.content = bw.width - 8
		if bw.content < 1 {
			// terminal too narrow; disable decoration to avoid garbled output
			bw.enabled = false
		}
	}

	return bw
}

// leftIndent is the number of blank columns rendered before the left border
// character. Two of these align the box content with the harness step text;
// the remaining one matches the leading space used elsewhere in the output.
const leftIndent = "   "

// emitTop writes the top border. It is invoked lazily on the first Write so
// commands that produce no output don't render an empty card.
func (b *BorderWriter) emitTop() {
	if !b.enabled || b.started {
		return
	}
	b.started = true

	line := b.style.TopLeft + repeat(b.style.Horizontal, b.width-6) + b.style.TopRight
	fmt.Fprintln(b.out, leftIndent+b.color.Sprint(line)) //nolint:errcheck
}

// Close flushes any pending partial line and emits the bottom border. If no
// content was ever written the bottom border is skipped, so silent commands
// don't produce an empty card.
// Safe to call multiple times.
func (b *BorderWriter) Close() error {
	if !b.enabled || b.closed {
		return nil
	}
	b.closed = true

	// flush trailing partial line, if any
	if b.pending.Len() > 0 {
		b.emitLine(b.pending.Bytes())
		b.pending.Reset()
	}

	// nothing was ever written; skip the bottom border to avoid an empty card.
	if !b.started {
		return nil
	}

	line := b.style.BottomLeft + repeat(b.style.Horizontal, b.width-6) + b.style.BottomRight
	fmt.Fprintln(b.out, leftIndent+b.color.Sprint(line)) //nolint:errcheck
	return nil
}

// Write implements io.Writer. It buffers partial lines and emits each complete
// line wrapped with the box border characters.
func (b *BorderWriter) Write(p []byte) (int, error) {
	if !b.enabled {
		return b.out.Write(p)
	}

	// lazy emit of the top border on the very first write
	if !b.started {
		b.emitTop()
	}

	written := 0
	for len(p) > 0 {
		idx := bytes.IndexByte(p, '\n')
		if idx < 0 {
			b.pending.Write(p)
			written += len(p)
			break
		}

		// accumulate the line up to (but not including) the newline
		b.pending.Write(p[:idx])
		b.emitLine(b.pending.Bytes())
		b.pending.Reset()

		written += idx + 1
		p = p[idx+1:]
	}

	return written, nil
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
		fmt.Fprintf(b.out, "%s%s%s%s%s\n", left, chunk.prefix, chunk.text, spaces(pad), right) //nolint:errcheck
	}
}

// lineChunk is one row's worth of wrapped content.
type lineChunk struct {
	prefix string // active SGR style carried over from a previous row, "" for the first
	text   []byte // raw bytes (possibly containing ANSI sequences) to emit for this row
	width  int    // visible width in cells of text
}

// sgrPattern matches a Select Graphic Rendition (SGR) escape sequence.
// SGR is the subset of CSI escapes used for colors and text attributes; it's
// the only state we carry across a soft-wrap boundary.
var sgrPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// wrapLine splits line into chunks that each fit within b.content cells of
// visible width. ANSI escape sequences are passed through unchanged and
// excluded from the width count; SGR state from a previous row is re-emitted
// at the start of the next, and a reset is appended at the end of any row
// that did not already end with one.
func (b *BorderWriter) wrapLine(line []byte) []lineChunk {
	var (
		chunks      []lineChunk
		current     bytes.Buffer
		currentW    int
		activeSGR   string // SGR code currently in effect at end of last completed chunk
		nextPrefix  string // SGR to re-apply at the start of the next chunk
		needsReset  bool   // current chunk has unreset SGR state and must end with \x1b[0m
	)

	flush := func() {
		if current.Len() == 0 && nextPrefix == "" {
			return
		}
		text := current.Bytes()
		if needsReset {
			text = append(text, []byte("\x1b[0m")...)
		}
		// copy because the buffer is reused
		chunk := lineChunk{
			prefix: nextPrefix,
			text:   append([]byte(nil), text...),
			width:  currentW,
		}
		chunks = append(chunks, chunk)
		current.Reset()
		currentW = 0
		needsReset = false
		// the next chunk starts with whatever SGR is currently active
		nextPrefix = activeSGR
	}

	for i := 0; i < len(line); {
		// detect ANSI escape sequences and pass them through without counting
		// their visible width; SGR sequences also update the active style.
		if loc := ansiPattern.FindIndex(line[i:]); loc != nil && loc[0] == 0 {
			esc := line[i : i+loc[1]]
			current.Write(esc)
			if sgrPattern.Match(esc) {
				if string(esc) == "\x1b[0m" || string(esc) == "\x1b[m" {
					activeSGR = ""
					needsReset = false
				} else {
					activeSGR = string(esc)
					needsReset = true
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

// terminalWidth returns the width in columns of the terminal backing w, or a
// reasonable fallback if it cannot be determined.
func terminalWidth(w io.Writer) int {
	if f, ok := w.(*os.File); ok {
		if width, _, err := term.GetSize(int(f.Fd())); err == nil && width > 0 {
			return width
		}
	}
	return 80
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	var buf bytes.Buffer
	buf.Grow(len(s) * n)
	for range n {
		buf.WriteString(s)
	}
	return buf.String()
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return string(bytes.Repeat([]byte{' '}, n))
}
