package commons

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aexvir/harness/binary"

	"github.com/fatih/color"

	"github.com/aexvir/harness"
)

// GoTest runs go test recursively.
//
//nolint:funlen,gocognit,gocyclo,cyclop,nestif // it's long but until usage patterns are clear it's better like this
func GoTest(opts ...TestOpt) harness.Task {
	conf := testconf{
		coberturafile: "test-coverage.xml",
		junitfile:     "test-results.xml",
		filedumpfile:  "test-output.txt",
	}

	for _, opt := range opts {
		opt(&conf)
	}

	return func(ctx context.Context) error {
		target := "./..."

		if conf.target != nil {
			target = fmt.Sprintf("./%s/...", *conf.target)
		}

		args := []string{"test", "-race", "-cover", target}
		var env []string

		if conf.integration {
			// replace this with if !conf.integration { args = append(args, "-skip", "^TestIntegration" }
			args = append(args, "-run", "^TestIntegration")
			env = append(env, "TEST_TARGET=integration")
		}

		output := io.Writer(os.Stdout)

		if conf.cifriendlyout || conf.junit {
			args = append(args, "-json")
			iobuf := new(bytes.Buffer)
			output = iobuf

			// write test output to file
			defer func() {
				jsonoutput := iobuf.Bytes()
				if conf.cifriendlyout {
					if err := gotestfmt(ctx, jsonoutput); err != nil {
						color.Red("failed to format test output: %s", err.Error())
					}
				}

				if conf.junit {
					if err := computeJunit(ctx, jsonoutput, conf.junitfile); err != nil {
						color.Red("failed to compute junit output: %s", err.Error())
					}
				}
			}()
		}

		if conf.filedump {
			iobuf := new(bytes.Buffer)
			output = iobuf

			defer func() {
				textoutput := iobuf.Bytes()
				if err := os.WriteFile(conf.filedumpfile, textoutput, 0o644); err != nil {
					color.Red("failed to write dump file: %s", err.Error())
				}
				fmt.Println(string(textoutput))
			}()
		}

		if conf.cobertura {
			gocoverfile := "coverage.out"
			args = append(args, "-coverprofile", gocoverfile)

			if conf.courtneycoverage {
				if err := computeCourtneyCoverage(ctx, gocoverfile); err != nil {
					color.Red("failed to apply coverage exclusions using courtney: %s", err.Error())
				}
			}

			defer func() {
				if err := computeCobertura(ctx, gocoverfile, conf.coberturafile); err != nil {
					color.Red("failed to compute cobertura output: %s")
				}
			}()
		}

		return harness.Run(ctx, "go",
			harness.WithArgs(args...),
			harness.WithEnv(env...),
			harness.WithStdOut(output),
		)
	}
}

// GoIntegrationTest runs only integration tests.
// It's a shortcut for GoTest(WithIntegrationTest()).
func GoIntegrationTest(opts ...TestOpt) harness.Task {
	return GoTest(append(opts, WithIntegrationTest())...)
}

func gotestfmt(ctx context.Context, testout []byte) error {
	fmt.Println("gotestfmt")
	gtf, _ := binary.New(
		"gotestfmt",
		"latest",
		binary.GoBinary("github.com/gotesttools/gotestfmt/v2/cmd/gotestfmt"),
	)
	if err := gtf.Ensure(); err != nil {
		return err
	}

	return harness.Run(ctx, gtf.BinPath(), harness.WithStdIn(bytes.NewReader(testout)))
}

// computeJunit translates the go test output to the junit format, so it can be parsed by
// tools like gitlab.
// https://docs.gitlab.com/ee/ci/testing/unit_test_reports.html
func computeJunit(ctx context.Context, testout []byte, junitfile string) error {
	gts, _ := binary.New(
		"gotestsum",
		"latest",
		binary.GoBinary("gotest.tools/gotestsum"),
	)
	if err := gts.Ensure(); err != nil {
		return err
	}

	return harness.Run(
		ctx,
		gts.BinPath(),
		harness.WithStdIn(bytes.NewReader(testout)),
		harness.WithStdOut(io.Discard),
		harness.WithArgs(
			fmt.Sprintf("--junitfile=%s", junitfile),
			"--hide-summary=all",
			"--raw-command",
			"--",
			"cat",
		),
	)
}

// computeCobertura translates the go coverage output to the cobertura format, so it can be parsed
// and ingested by tools like gitlab.
// https://docs.gitlab.com/ee/ci/testing/test_coverage_visualization.html
func computeCobertura(ctx context.Context, coverfile, coberturafile string) error {
	cbrt, _ := binary.New(
		"gocover-cobertura",
		"latest",
		binary.GoBinary("github.com/boumenot/gocover-cobertura"),
	)
	if err := cbrt.Ensure(); err != nil {
		return err
	}

	coverout, err := os.ReadFile(coverfile)
	if err != nil {
		return fmt.Errorf("error reading go coverage output: %w", err)
	}

	buf := new(bytes.Buffer)

	defer func() {
		err := os.WriteFile(coberturafile, buf.Bytes(), 0o644)
		if err != nil {
			color.Red("failed to write cobertura file: %s", err.Error())
		}
	}()

	return harness.Run(
		ctx,
		cbrt.BinPath(),
		harness.WithStdIn(bytes.NewReader(coverout)),
		harness.WithStdOut(buf),
	)
}

// computeCourtneyCoverage recomputes code coverage acknowledging for code intentionally excluded
// from the coverage calculation.
// https://github.com/dave/courtney
func computeCourtneyCoverage(ctx context.Context, coverfile string) error {
	ctny, _ := binary.New(
		"courtney",
		"latest",
		binary.GoBinary("github.com/dave/courtney"),
	)

	if err := ctny.Ensure(); err != nil {
		return err
	}

	return harness.Run(ctx, ctny.BinPath(), harness.WithArgs("-l", coverfile))
}

type testconf struct {
	target           *string
	integration      bool
	courtneycoverage bool
	filedump         bool
	filedumpfile     string

	cifriendlyout bool
	junit         bool
	junitfile     string
	cobertura     bool
	coberturafile string
}

type TestOpt func(c *testconf)

// WithTarget limits the tests to only a folder relative to the root path.
// Passing nil as target means running all tests, equivalent to ./...
func WithTarget(target *string) TestOpt {
	return func(c *testconf) {
		c.target = target
	}
}

func WithIntegrationTest() TestOpt {
	return func(c *testconf) {
		c.integration = true
	}
}

// WithTestCIFriendlyOutput formats the test output using gotestfmt, which has special handling
// of ci environments, grouping the output using the native uis available.
//
// https://github.com/GoTestTools/gotestfmt
func WithTestCIFriendlyOutput(enabled bool) TestOpt {
	return func(c *testconf) {
		c.cifriendlyout = enabled
	}
}

// WithTestFileDump controls if the test task should dump its output to a file.
func WithTestFileDump(enabled bool) TestOpt {
	return func(c *testconf) {
		c.filedump = enabled
	}
}

// WithTestFileDumpOutput specifies the filename for test output dump.
func WithTestFileDumpOutput(filename string) TestOpt {
	return func(c *testconf) {
		c.filedumpfile = filename
	}
}

// WithTestCobertura controls if the test task should generate a cobertura coverage file or not.
func WithTestCobertura(enabled bool) TestOpt {
	return func(c *testconf) {
		c.cobertura = enabled
	}
}

// WithTestCoberturaOutput specifies the filename for the cobertura output.
func WithTestCoberturaOutput(filename string) TestOpt {
	return func(c *testconf) {
		c.coberturafile = filename
	}
}

func WithTestCoverageExclusions() TestOpt {
	return func(c *testconf) {
		c.courtneycoverage = true
	}
}

// WithTestJunit controls if the test task should generate a junit report file or not.
func WithTestJunit(enabled bool) TestOpt {
	return func(c *testconf) {
		c.junit = enabled
	}
}

// WithTestJunitOutput specifies the filename for the junit output.
func WithTestJunitOutput(filename string) TestOpt {
	return func(c *testconf) {
		c.junitfile = filename
	}
}
