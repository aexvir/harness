package commons

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"

	"github.com/kiwicom/harness"
	"github.com/kiwicom/harness/bintool"
)

// GolangCILint aggregates multiple linters that analyze go code.
// https://golangci-lint.run
func GolangCILint(opts ...GolangCILintOpt) harness.Task {
	conf := golangcilintconf{
		version:         "latest",
		codeclimatefile: "quality-report.json",
	}

	for _, opt := range opts {
		opt(&conf)
	}

	return func(ctx context.Context) error {
		gci, _ := bintool.NewGo(
			"github.com/golangci/golangci-lint/cmd/golangci-lint",
			conf.version,
		)

		if err := gci.Ensure(); err != nil {
			return fmt.Errorf("failed to provision golangci-lint binary: %w", err)
		}

		args := []string{
			"run",
			"--max-same-issues", "0",
			"--max-issues-per-linter", "0",
		}

		var err error

		if conf.codeclimate {
			args = append(args, "--out-format", fmt.Sprintf("code-climate:%s", conf.codeclimatefile))
			defer func() {
				if err != nil {
					// print found issues directly from the codeclimate file to avoid re-running golangci-lint with a different format
					output, _ := os.ReadFile(conf.codeclimatefile)

					var issues []linterissue
					if jsonerr := json.NewDecoder(bytes.NewBuffer(output)).Decode(&issues); jsonerr != nil {
						color.Red("failed to parse codeclimate output")
					}

					for _, issue := range issues {
						color.Red("  • %s:%d        %s", issue.Location.Path, issue.Location.Lines.Begin, issue.Description)
					}
				}
			}()
		}

		err = harness.Run(
			ctx,
			gci.BinPath(),
			harness.WithArgs(args...),
			harness.WithErrMsg("some linters found errors"),
		)

		return err
	}
}

type golangcilintconf struct {
	version string

	codeclimate     bool
	codeclimatefile string
}

type GolangCILintOpt func(c *golangcilintconf)

// WithGolangCIVersion allows specifying the golangci-lint version
// that should be used when running this task.
func WithGolangCIVersion(version string) GolangCILintOpt {
	return func(c *golangcilintconf) {
		c.version = version
	}
}

// WithGolangCICodeClimate controls if golangci-lint should generate a codeclimate report file
// instead of outputting everything to stdout or not.
// https://codeclimate.com
func WithGolangCICodeClimate(enabled bool) GolangCILintOpt {
	return func(c *golangcilintconf) {
		c.codeclimate = enabled
	}
}

// WithGolangCICodeClimateOutput specifies the filename for the codeclimate output.
func WithGolangCICodeClimateOutput(filename string) GolangCILintOpt {
	return func(c *golangcilintconf) {
		c.codeclimatefile = filename
	}
}

// basic codeclimate issue.
type linterissue struct {
	Description string `json:"description"`
	Location    struct {
		Path  string `json:"path"`
		Lines struct {
			Begin int `json:"begin"`
		} `json:"lines"`
	} `json:"location"`
}
