package harness

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCmd verifies the Cmd constructor behavior.
func TestCmd(t *testing.T) {
	ctx := context.Background()

	t.Run("simple executable", func(t *testing.T) {
		r, err := Cmd(ctx, "echo")
		require.NoError(t, err)
		assert.Equal(t, "echo", r.Executable)
		assert.Empty(t, r.Arguments)
	})

	t.Run("relative path is resolved to absolute", func(t *testing.T) {
		r, err := Cmd(ctx, "./some/binary")
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(r.Executable), "expected absolute path, got: %s", r.Executable)
		assert.True(t, strings.HasSuffix(r.Executable, "some/binary"))
	})

	t.Run("absolute path stays unchanged", func(t *testing.T) {
		r, err := Cmd(ctx, "/usr/bin/echo")
		require.NoError(t, err)
		assert.Equal(t, "/usr/bin/echo", r.Executable)
	})

	t.Run("no-slash executable unchanged", func(t *testing.T) {
		r, err := Cmd(ctx, "go")
		require.NoError(t, err)
		assert.Equal(t, "go", r.Executable)
	})

	t.Run("options are applied", func(t *testing.T) {
		r, err := Cmd(ctx, "echo", WithArgs("a", "b"))
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, r.Arguments)
	})

	t.Run("option error propagates", func(t *testing.T) {
		_, err := Cmd(ctx, "echo", WithEnv("INVALID"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid env format")
	})

	t.Run("cmd.Args assembled correctly", func(t *testing.T) {
		r, err := Cmd(ctx, "echo", WithArgs("hello", "world"))
		require.NoError(t, err)
		assert.Equal(t, []string{"echo", "hello", "world"}, r.cmd.Args)
	})

	t.Run("default stdout stderr stdin", func(t *testing.T) {
		r, err := Cmd(ctx, "echo")
		require.NoError(t, err)
		assert.Equal(t, os.Stdout, r.cmd.Stdout)
		assert.Equal(t, os.Stderr, r.cmd.Stderr)
		assert.Equal(t, os.Stdin, r.cmd.Stdin)
	})
}

// TestExec verifies TaskRunner.Exec behavior.
func TestExec(t *testing.T) {
	ctx := context.Background()

	t.Run("successful command", func(t *testing.T) {
		r, err := Cmd(ctx, "echo", WithArgs("hello"), WithoutNoise())
		require.NoError(t, err)
		assert.NoError(t, r.Exec())
	})

	t.Run("failing command returns error", func(t *testing.T) {
		r, err := Cmd(ctx, "false", WithoutNoise())
		require.NoError(t, err)

		execErr := r.Exec()
		require.Error(t, execErr)
		assert.Contains(t, execErr.Error(), "false")
	})

	t.Run("failing command with errmsg", func(t *testing.T) {
		r, err := Cmd(ctx, "false", WithErrMsg("something broke"), WithoutNoise())
		require.NoError(t, err)

		execErr := r.Exec()
		require.Error(t, execErr)
		assert.Contains(t, execErr.Error(), "false")
	})

	t.Run("successful command with okmsg", func(t *testing.T) {
		r, err := Cmd(ctx, "echo", WithArgs("hi"), WithOKMsg("all good"), WithoutNoise())
		require.NoError(t, err)
		assert.NoError(t, r.Exec())
	})

	t.Run("quiet mode sets fields correctly", func(t *testing.T) {
		r, err := Cmd(ctx, "echo", WithoutNoise())
		require.NoError(t, err)
		assert.True(t, r.quiet)
		assert.Nil(t, r.cmd.Stdout)
		assert.Nil(t, r.cmd.Stderr)
	})

	t.Run("allowerr swallows error", func(t *testing.T) {
		r, err := Cmd(ctx, "false", WithAllowErrors(), WithoutNoise())
		require.NoError(t, err)
		assert.NoError(t, r.Exec())
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel() // cancel immediately

		r, err := Cmd(ctx, "sleep", WithArgs("10"), WithoutNoise())
		require.NoError(t, err)

		execErr := r.Exec()
		require.Error(t, execErr)
	})
}

// TestRun verifies the Run helper function.
func TestRun(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := Run(ctx, "echo", WithArgs("hi"), WithoutNoise())
		assert.NoError(t, err)
	})

	t.Run("command failure", func(t *testing.T) {
		err := Run(ctx, "false", WithoutNoise())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "false")
	})

	t.Run("option error prevents execution", func(t *testing.T) {
		err := Run(ctx, "echo", WithEnv("BAD"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid env format")
	})
}

// TestRunnerOpts verifies each RunnerOpt function individually.
func TestRunnerOpts(t *testing.T) {
	ctx := context.Background()

	t.Run("WithEnv valid", func(t *testing.T) {
		r, err := Cmd(ctx, "echo", WithEnv("FOO=bar", "BAZ=qux"))
		require.NoError(t, err)

		// should contain os.Environ() + our vars
		assert.Contains(t, r.cmd.Env, "FOO=bar")
		assert.Contains(t, r.cmd.Env, "BAZ=qux")
		assert.True(t, len(r.cmd.Env) > 2, "should include inherited env vars")
	})

	t.Run("WithEnv invalid format no equals", func(t *testing.T) {
		_, err := Cmd(ctx, "echo", WithEnv("NOEQUALSSIGN"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid env format")
	})

	t.Run("WithEnv with equals in value", func(t *testing.T) {
		r, err := Cmd(ctx, "echo", WithEnv("DATABASE_URL=postgres://host?opt=val"))
		require.NoError(t, err)
		assert.Contains(t, r.cmd.Env, "DATABASE_URL=postgres://host?opt=val")
	})

	t.Run("WithArgs", func(t *testing.T) {
		r, err := Cmd(ctx, "echo", WithArgs("a", "b", "c"))
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, r.Arguments)
	})

	t.Run("WithOKMsg", func(t *testing.T) {
		r, err := Cmd(ctx, "echo", WithOKMsg("success message"))
		require.NoError(t, err)
		assert.Equal(t, "success message", r.okmsg)
	})

	t.Run("WithErrMsg", func(t *testing.T) {
		r, err := Cmd(ctx, "echo", WithErrMsg("error message"))
		require.NoError(t, err)
		assert.Equal(t, "error message", r.errmsg)
	})

	t.Run("WithDir", func(t *testing.T) {
		dir := t.TempDir()
		r, err := Cmd(ctx, "echo", WithDir(dir))
		require.NoError(t, err)
		assert.Equal(t, dir, r.cmd.Dir)
	})

	t.Run("WithoutNoise", func(t *testing.T) {
		r, err := Cmd(ctx, "echo", WithoutNoise())
		require.NoError(t, err)
		assert.True(t, r.quiet)
		assert.Nil(t, r.cmd.Stdout)
		assert.Nil(t, r.cmd.Stderr)
	})

	t.Run("WithStdOut", func(t *testing.T) {
		var buf bytes.Buffer
		r, err := Cmd(ctx, "echo", WithStdOut(&buf))
		require.NoError(t, err)
		assert.Equal(t, &buf, r.cmd.Stdout)
	})

	t.Run("WithStdIn", func(t *testing.T) {
		reader := strings.NewReader("input data")
		r, err := Cmd(ctx, "echo", WithStdIn(reader))
		require.NoError(t, err)
		assert.Equal(t, reader, r.cmd.Stdin)
	})

	t.Run("WithAllowErrors", func(t *testing.T) {
		r, err := Cmd(ctx, "echo", WithAllowErrors())
		require.NoError(t, err)
		assert.True(t, r.allowerr)
	})
}

// TestRunnerEdgeCases covers edge case scenarios.
func TestRunnerEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("nonexistent executable", func(t *testing.T) {
		err := Run(ctx, "nonexistent-binary-xyz-12345", WithoutNoise())
		require.Error(t, err)
	})

	t.Run("WithDir with nonexistent dir", func(t *testing.T) {
		err := Run(ctx, "echo", WithArgs("hi"), WithDir("/nonexistent/dir/xyz"), WithoutNoise())
		require.Error(t, err)
	})

	t.Run("WithStdOut captures output", func(t *testing.T) {
		var buf bytes.Buffer
		r, err := Cmd(ctx, "echo", WithArgs("captured"), WithStdOut(&buf), WithoutNoise())
		require.NoError(t, err)

		// WithoutNoise sets stdout to nil, so set it back after
		r.cmd.Stdout = &buf

		err = r.Exec()
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "captured")
	})

	t.Run("multiple options compose correctly", func(t *testing.T) {
		dir := t.TempDir()
		var buf bytes.Buffer

		r, err := Cmd(ctx, "echo",
			WithArgs("hello"),
			WithDir(dir),
			WithStdOut(&buf),
			WithEnv("MY_VAR=test"),
			WithOKMsg("done"),
			WithErrMsg("failed"),
		)
		require.NoError(t, err)
		assert.Equal(t, []string{"hello"}, r.Arguments)
		assert.Equal(t, dir, r.cmd.Dir)
		assert.Equal(t, &buf, r.cmd.Stdout)
		assert.Contains(t, r.cmd.Env, "MY_VAR=test")
		assert.Equal(t, "done", r.okmsg)
		assert.Equal(t, "failed", r.errmsg)
	})
}
