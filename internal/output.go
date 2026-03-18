package internal

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

var Output io.Writer = os.Stdout

func SetOutput(w io.Writer) {
	Output = w
}

func IsTerminalWriter(w io.Writer) bool {
	// IsTTY is implemented by the testing syncbuffer.
	type tty interface{ IsTTY() bool }
	if t, ok := w.(tty); ok {
		return t.IsTTY()
	}

	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := file.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// LogBlank writes an empty line to the output.
func LogBlank() {
	fmt.Fprintln(Output) //nolint:errcheck
}

// LogSeparator writes a dim horizontal rule.
func LogSeparator() {
	color.New(color.FgHiBlack).Fprintf(Output, "------------------------\n\n") //nolint:errcheck
}

// LogCommand writes a top-level command heading using the command symbol.
// This is the most prominent log level, used for task names.
func LogCommand(text string) {
	fmt.Fprintln( //nolint:errcheck
		Output,
		color.MagentaString(" %s", Symbols.Command),
		color.New(color.Bold).Sprint(text),
	)
}

// LogStep writes a secondary step line using the dot symbol.
// Used for provisioning and sub-task progress.
func LogStep(text string) {
	fmt.Fprintln( //nolint:errcheck
		Output,
		color.BlueString(" %s", Symbols.Dot),
		color.New(color.FgHiBlack).Sprint(text),
	)
}

// LogDetail writes an indented detail line using the detail symbol.
func LogDetail(text string) {
	fmt.Fprintln( //nolint:errcheck
		Output,
		color.New(color.FgHiBlack).Sprintf("   %s", Symbols.Detail),
		color.New(color.FgHiBlack).Sprint(text),
	)
}

// LogSuccess writes a green success line with the success symbol.
func LogSuccess(text string) {
	color.New(color.FgGreen).Fprintf(Output, " %s %s\n", Symbols.Success, text) //nolint:errcheck
}

// LogError writes a red error line with the error symbol.
func LogError(text string) {
	color.New(color.FgRed).Fprintf(Output, " %s %s\n", Symbols.Error, text) //nolint:errcheck
}

// LogErrorItem writes an indented red error bullet using the dot symbol.
func LogErrorItem(text string) {
	color.New(color.FgRed).Fprintf(Output, "   %s %s\n", Symbols.Dot, text) //nolint:errcheck
}

// LogStatus writes an indented status indicator based on whether err is nil.
// Unlike LogSuccess/LogError, this does not append a newline, allowing the
// caller to control line termination.
func LogStatus(text string, err error) {
	if err != nil {
		color.New(color.FgRed).Fprintf(Output, "     %s %s", Symbols.Error, text) //nolint:errcheck
		return
	}

	color.New(color.FgGreen).Fprintf(Output, "     %s %s", Symbols.Success, text) //nolint:errcheck
}

// LogMessage writes a line in the specified color without any symbol prefix.
func LogMessage(attr color.Attribute, text string) {
	color.New(attr).Fprintln(Output, text) //nolint:errcheck
}
