package binary

import (
	"fmt"
	"io"
	"os"

	"github.com/aexvir/harness"
	"github.com/fatih/color"
)

var output io.Writer = os.Stdout

func SetOutput(w io.Writer) {
	output = w
}

func logstep(text string) {
	fmt.Fprintln(
		output,
		color.BlueString(" %s", harness.Symbols.Dot),
		color.New(color.FgHiBlack).Sprint(text),
	)
}

func logdetail(text string) {
	fmt.Fprintln(
		output,
		color.New(color.FgHiBlack).Sprintf("   %s", harness.Symbols.Detail),
		color.New(color.FgHiBlack).Sprint(text),
	)
}

func logstatus(text string, err error) {
	if err != nil {
		color.New(color.FgRed).Fprintf(output, "     %s %s", harness.Symbols.Error, text)
		return
	}

	color.New(color.FgGreen).Fprintf(output, "     %s %s", harness.Symbols.Success, text)
}
