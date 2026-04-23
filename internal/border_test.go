package internal

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// borderBuffer is a minimal io.Writer that reports as a TTY so the
// BorderWriter enables decoration during tests.
type borderBuffer struct {
	bytes.Buffer
}

func (*borderBuffer) IsTTY() bool { return true }

// withNoColor disables fatih/color's escape sequences for the duration of the
// test so assertions can match plain strings.
func withNoColor(t *testing.T) {
	t.Helper()
	prev := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = prev })
}

func TestBorderWriter_PassThroughWhenNotTTY(t *testing.T) {
	withNoColor(t)
	var buf bytes.Buffer

	bw := NewBorderWriter(&buf) // bytes.Buffer is not a TTY
	_, err := bw.Write([]byte("hello\nworld\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	assert.Equal(t, "hello\nworld\n", buf.String(), "non-tty output must be untouched")
}

// expected layout helpers — keep tests resilient to padding tweaks while still
// asserting on the visible structure.
const (
	indent       = "   " // matches leftIndent in border.go
	contentExtra = 8     // chars used by indent + borders + padding (width - contentExtra = content width)
)

// padded returns the inner content (line + right padding) for a given visible
// length and total terminal width.
func padded(line string, visibleLen, width int) string {
	pad := (width - contentExtra) - visibleLen
	if pad < 0 {
		pad = 0
	}
	return line + strings.Repeat(" ", pad)
}

func TestBorderWriter_NoOutputProducesNoBorder(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20)
	require.NoError(t, bw.Close())

	assert.Empty(t, buf.String(), "writer with no content must not render an empty card")
}

func TestBorderWriter_WrapsCompleteLines(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20)
	_, err := bw.Write([]byte("hello\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 3)
	assert.Equal(t, indent+"╭"+strings.Repeat("─", 20-6)+"╮", lines[0])
	assert.Equal(t, indent+"│ "+padded("hello", 5, 20)+" │", lines[1])
	assert.Equal(t, indent+"╰"+strings.Repeat("─", 20-6)+"╯", lines[2])
}

func TestBorderWriter_BuffersPartialLinesAcrossWrites(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20)
	_, err := bw.Write([]byte("hel"))
	require.NoError(t, err)
	// the partial write triggered the top border but not the line itself
	assert.Equal(t, 1, strings.Count(buf.String(), "\n"), "partial line must not be flushed yet")

	_, err = bw.Write([]byte("lo\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 3)
	assert.Equal(t, indent+"│ "+padded("hello", 5, 20)+" │", lines[1])
}

func TestBorderWriter_FlushesPendingPartialLineOnClose(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20)
	_, err := bw.Write([]byte("trailing"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 3)
	assert.Equal(t, indent+"│ "+padded("trailing", 8, 20)+" │", lines[1])
}

func TestBorderWriter_HandlesMultipleLinesInSingleWrite(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20)
	_, err := bw.Write([]byte("one\ntwo\nthree\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 5)
	assert.Equal(t, indent+"│ "+padded("one", 3, 20)+" │", lines[1])
	assert.Equal(t, indent+"│ "+padded("two", 3, 20)+" │", lines[2])
	assert.Equal(t, indent+"│ "+padded("three", 5, 20)+" │", lines[3])
}

func TestBorderWriter_SoftWrapsLongLines(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20) // content width = 12
	// 30 'x's split across rows of 12, 12, 6
	long := strings.Repeat("x", 30)
	_, err := bw.Write([]byte(long + "\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 5, "expected top border + 3 wrapped rows + bottom border")
	assert.Equal(t, indent+"│ "+padded(strings.Repeat("x", 12), 12, 20)+" │", lines[1])
	assert.Equal(t, indent+"│ "+padded(strings.Repeat("x", 12), 12, 20)+" │", lines[2])
	assert.Equal(t, indent+"│ "+padded(strings.Repeat("x", 6), 6, 20)+" │", lines[3])
}

func TestBorderWriter_SoftWrapCarriesSGRStateAcrossRows(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20) // content width = 12
	// 20 visible chars all wrapped in red, no explicit reset until the end
	red := strings.Repeat("x", 20)
	_, err := bw.Write([]byte("\x1b[31m" + red + "\x1b[0m\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 4, "expected top border + 2 wrapped rows + bottom border")

	// first row: opens SGR, fills 12 cells, ends with reset before right border
	assert.Equal(
		t,
		indent+"│ "+"\x1b[31m"+strings.Repeat("x", 12)+"\x1b[0m"+padded("", 12, 20)+" │",
		lines[1],
	)
	// second row: re-applies the SGR carried over, has the explicit reset from the input
	assert.Equal(
		t,
		indent+"│ "+"\x1b[31m"+strings.Repeat("x", 8)+"\x1b[0m"+padded("", 8, 20)+" │",
		lines[2],
	)
}

func TestBorderWriter_SoftWrapStripsAnsiForWidth(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20) // content width = 12
	// "hello" wrapped in red is 5 visible chars; should fit on a single row
	_, err := bw.Write([]byte("\x1b[31mhello\x1b[0m\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 3)
	assert.Equal(t, indent+"│ "+padded("\x1b[31mhello\x1b[0m", 5, 20)+" │", lines[1])
}

func TestBorderWriter_CloseIsIdempotent(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20)
	_, err := bw.Write([]byte("hi\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())
	require.NoError(t, bw.Close())

	assert.Equal(t, 1, strings.Count(buf.String(), "╭"))
	assert.Equal(t, 1, strings.Count(buf.String(), "╰"))
}

func TestBorderWriter_DisabledWhenWidthTooSmall(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 5) // content would be negative
	_, err := bw.Write([]byte("data\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	assert.Equal(t, "data\n", buf.String(), "narrow terminal should fall back to pass-through")
}

// splitLines splits on '\n' and drops the trailing empty element produced when
// the input ends with a newline, which makes per-line assertions cleaner.
func splitLines(s string) []string {
	parts := strings.Split(s, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
