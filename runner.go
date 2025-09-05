package harness

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
)

// TaskRunner holds the metadata for a specific command.
type TaskRunner struct {
	Executable string
	Arguments  []string

	cmd      *exec.Cmd
	okmsg    string
	errmsg   string
	quiet    bool
	allowerr bool
}

// Cmd builds a command runner for a specific Executable.
func Cmd(ctx context.Context, executable string, opts ...RunnerOpt) (*TaskRunner, error) {
	// Resolve relative executable paths to absolute paths to ensure they work correctly
	// when the working directory is changed via WithDir
	resolvedExecutable := executable
	if !filepath.IsAbs(executable) {
		abs, err := filepath.Abs(executable)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve executable path %q: %w", executable, err)
		}
		resolvedExecutable = abs
	}

	cmd := exec.CommandContext(ctx, resolvedExecutable)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	r := TaskRunner{
		Executable: resolvedExecutable,
		cmd:        cmd,
	}

	for _, opt := range opts {
		err := opt(&r)
		if err != nil {
			return nil, err
		}
	}

	cmd.Args = append([]string{resolvedExecutable}, r.Arguments...)

	return &r, nil
}

// Exec a command returning its error and pretty printing the ok and error messages.
func (r *TaskRunner) Exec() error {
	var err error

	start := time.Now()
	defer func() {
		elapsed := time.Since(start).Round(time.Millisecond)
		if err != nil {
			color.Red(" ✘ %s\n\n", elapsed)
			return
		}
		color.Green(" ✔ %s\n\n", elapsed)
	}()

	if !r.quiet {
		LogStep(fmt.Sprint(r.Executable, " ", strings.Join(r.Arguments, " ")))
	}

	err = r.cmd.Run()

	if !r.allowerr && err != nil {
		if !r.quiet && r.errmsg != "" {
			color.Red(r.errmsg)
		}
		return fmt.Errorf("%s: %w", r.Executable, err)
	}

	if !r.quiet && r.okmsg != "" {
		color.Green(r.okmsg)
	}

	return nil
}

// Run is a helper function to avoid repetition while gracefully handling errors.
func Run(ctx context.Context, program string, opts ...RunnerOpt) error {
	rnr, err := Cmd(ctx, program, opts...)
	if err != nil {
		return err
	}

	return rnr.Exec()
}

// RunnerOpt allows customizing the behavior of the command runner.
type RunnerOpt func(r *TaskRunner) error

// WithEnv sets up environment variables for the command.
func WithEnv(vars ...string) RunnerOpt {
	return func(r *TaskRunner) error {
		r.cmd.Env = os.Environ()
		for _, vrb := range vars {
			items := strings.Split(vrb, "=")
			if len(items) != 2 {
				return fmt.Errorf("invalid env format; %s doesn't match NAME=value expectation", vrb)
			}
			r.cmd.Env = append(r.cmd.Env, vrb)
		}
		return nil
	}
}

// WithArgs command arguments.
func WithArgs(args ...string) RunnerOpt {
	return func(r *TaskRunner) error {
		r.Arguments = args
		return nil
	}
}

// WithOKMsg sets a message to be printed when the command finishes successfully.
func WithOKMsg(msg string) RunnerOpt {
	return func(r *TaskRunner) error {
		r.okmsg = msg
		return nil
	}
}

// WithErrMsg sets a message to be printed when the command fails.
func WithErrMsg(msg string) RunnerOpt {
	return func(r *TaskRunner) error {
		r.errmsg = msg
		return nil
	}
}

// WithDir sets the directory where the command should be run inside.
func WithDir(dir string) RunnerOpt {
	return func(r *TaskRunner) error {
		// Resolve relative directory paths to absolute paths
		resolvedDir := dir
		if !filepath.IsAbs(dir) {
			abs, err := filepath.Abs(dir)
			if err != nil {
				return fmt.Errorf("failed to resolve directory path %q: %w", dir, err)
			}
			resolvedDir = abs
		}
		r.cmd.Dir = resolvedDir
		return nil
	}
}

// WithoutNoise silences all output for the command; useful when handling that on the caller side.
func WithoutNoise() RunnerOpt {
	return func(r *TaskRunner) error {
		r.quiet = true
		r.cmd.Stdout = nil
		r.cmd.Stderr = nil

		return nil
	}
}

// WithStdOut set up stdout writer.
func WithStdOut(w io.Writer) RunnerOpt {
	return func(r *TaskRunner) error {
		r.cmd.Stdout = w
		return nil
	}
}

// WithStdIn set up stdin reader.
func WithStdIn(read io.Reader) RunnerOpt {
	return func(r *TaskRunner) error {
		r.cmd.Stdin = read
		return nil
	}
}

// WithAllowErrors allow errors in the command.
func WithAllowErrors() RunnerOpt {
	return func(r *TaskRunner) error {
		r.allowerr = true
		return nil
	}
}
