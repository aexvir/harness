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
	bw.Start()
	_, err := bw.Write([]byte("hello\nworld\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	assert.Equal(t, "hello\nworld\n", buf.String(), "non-tty output must be untouched")
}

func TestBorderWriter_RendersTopAndBottomBorder(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20) // content width = 14
	bw.Start()
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 2)
	assert.Equal(t, " ╭"+strings.Repeat("─", 16)+"╮", lines[0])
	assert.Equal(t, " ╰"+strings.Repeat("─", 16)+"╯", lines[1])
}

func TestBorderWriter_WrapsCompleteLines(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20) // content width = 14
	bw.Start()
	_, err := bw.Write([]byte("hello\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 3)
	assert.Equal(t, " │ hello"+strings.Repeat(" ", 10)+"│", lines[1])
}

func TestBorderWriter_BuffersPartialLinesAcrossWrites(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20)
	bw.Start()
	_, err := bw.Write([]byte("hel"))
	require.NoError(t, err)
	// after a partial write only the top border should be present
	assert.Equal(t, 1, strings.Count(buf.String(), "\n"), "partial line must not be flushed yet")

	_, err = bw.Write([]byte("lo\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 3)
	assert.Equal(t, " │ hello"+strings.Repeat(" ", 10)+"│", lines[1])
}

func TestBorderWriter_FlushesPendingPartialLineOnClose(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20)
	bw.Start()
	_, err := bw.Write([]byte("trailing"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 3)
	assert.Equal(t, " │ trailing"+strings.Repeat(" ", 7)+"│", lines[1])
}

func TestBorderWriter_HandlesMultipleLinesInSingleWrite(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20)
	bw.Start()
	_, err := bw.Write([]byte("one\ntwo\nthree\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 5)
	assert.Equal(t, " │ one"+strings.Repeat(" ", 12)+"│", lines[1])
	assert.Equal(t, " │ two"+strings.Repeat(" ", 12)+"│", lines[2])
	assert.Equal(t, " │ three"+strings.Repeat(" ", 10)+"│", lines[3])
}

func TestBorderWriter_LongLineSkipsRightBorder(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20) // content width = 14
	bw.Start()
	long := strings.Repeat("x", 30)
	_, err := bw.Write([]byte(long + "\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 3)
	assert.Equal(t, " │ "+long, lines[1], "overflowing line must skip right border")
}

func TestBorderWriter_StripsAnsiForWidth(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20) // content width = 14
	bw.Start()
	// "hello" is 5 visible chars, surrounded by ANSI escapes
	_, err := bw.Write([]byte("\x1b[31mhello\x1b[0m\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 3)
	// padding must be computed from the visible width (5), not the raw byte length
	assert.Equal(t, " │ \x1b[31mhello\x1b[0m"+strings.Repeat(" ", 10)+"│", lines[1])
}

func TestBorderWriter_LazyStartOnFirstWrite(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20)
	// no explicit Start
	_, err := bw.Write([]byte("hi\n"))
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	lines := splitLines(buf.String())
	require.Len(t, lines, 3, "Start should have been triggered automatically on first write")
	assert.Contains(t, lines[0], "╭")
	assert.Contains(t, lines[2], "╰")
}

func TestBorderWriter_StartAndCloseAreIdempotent(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 20)
	bw.Start()
	bw.Start()
	require.NoError(t, bw.Close())
	require.NoError(t, bw.Close())

	assert.Equal(t, 1, strings.Count(buf.String(), "╭"))
	assert.Equal(t, 1, strings.Count(buf.String(), "╰"))
}

func TestBorderWriter_DisabledWhenWidthTooSmall(t *testing.T) {
	withNoColor(t)
	buf := &borderBuffer{}

	bw := newBorderWriter(buf, 5) // content would be -1
	bw.Start()
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
