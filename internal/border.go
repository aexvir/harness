package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"

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
// characters. Lines wider than the available content width are emitted without
// a right border to avoid breaking ANSI escape sequences mid-stream; the
// terminal will visually wrap them.
func (b *BorderWriter) emitLine(line []byte) {
	left := leftIndent + b.color.Sprint(b.style.Vertical) + " "

	// strip ansi codes for width measurement only
	visible := ansiPattern.ReplaceAll(line, nil)
	w := runewidth.StringWidth(string(visible))

	if w > b.content {
		fmt.Fprintf(b.out, "%s%s\n", left, line) //nolint:errcheck
		return
	}

	pad := b.content - w
	right := " " + b.color.Sprint(b.style.Vertical)
	fmt.Fprintf(b.out, "%s%s%s%s\n", left, line, spaces(pad), right) //nolint:errcheck
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
