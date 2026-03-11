package harness

import (
	"bytes"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCmd(t *testing.T) {
	t.Run("executable names are used as-is", // they'll be looked up in $PATH by os/exec
		func(t *testing.T) {
			r, err := Cmd(t.Context(), "go")
			require.NoError(t, err)

			assert.Equal(t, "go", r.Executable)
			assert.Equal(t, []string{"go"}, r.cmd.Args)
		},
	)

	t.Run("relative paths are made absolute",
		func(t *testing.T) {
			want, err := filepath.Abs("tmp/tool")
			require.NoError(t, err)

			r, err := Cmd(t.Context(), "tmp/tool")
			require.NoError(t, err)

			assert.Equal(t, want, r.Executable)
			assert.Equal(t, []string{want}, r.cmd.Args)
		},
	)

	t.Run("applies options",
		func(t *testing.T) {
			dir := t.TempDir()

			// run "go version" inside a temp dir with FOO=bar in the env
			r, err := Cmd(t.Context(), "go",
				WithArgs("version"),
				WithDir(dir),
				WithEnv("FOO=bar"),
			)
			require.NoError(t, err)

			assert.Equal(t, "go", r.Executable)
			assert.Equal(t, []string{"version"}, r.Arguments)

			assert.Equal(t, []string{"go", "version"}, r.cmd.Args)
			assert.Equal(t, dir, r.cmd.Dir)
			assert.Contains(t, r.cmd.Env, "FOO=bar")
		},
	)

	t.Run("returns error for invalid env format",
		func(t *testing.T) {
			_, err := Cmd(t.Context(), "go", WithEnv("INVALID"))

			require.Error(t, err)
			assert.Contains(t, err.Error(), "doesn't match NAME=value expectation")
		},
	)
}

func TestTaskRunnerExec(t *testing.T) {
	skipOnWindows(t)

	t.Run("success",
		func(t *testing.T) {
			var out bytes.Buffer

			r, err := Cmd(t.Context(), "testdata/util.sh", WithArgs("success"), WithStdOut(&out))
			require.NoError(t, err)

			require.NoError(t, r.Exec())
			assert.Equal(t, "ok", strings.TrimSpace(out.String()))
		},
	)

	t.Run("failure",
		func(t *testing.T) {
			r, err := Cmd(t.Context(), "testdata/util.sh", WithArgs("fail"), WithStdOut(io.Discard))
			require.NoError(t, err)

			err = r.Exec()
			require.Error(t, err)
			assert.Contains(t, err.Error(), r.Executable)
		},
	)

	t.Run("allow errors",
		func(t *testing.T) {
			r, err := Cmd(t.Context(), "testdata/util.sh", WithArgs("fail"), WithAllowErrors(), WithStdOut(io.Discard))
			require.NoError(t, err)

			require.NoError(t, r.Exec())
		},
	)

	t.Run("stdin and stdout",
		func(t *testing.T) {
			var out bytes.Buffer

			r, err := Cmd(t.Context(), "testdata/util.sh", WithArgs("print"), WithStdIn(strings.NewReader("payload")), WithStdOut(&out))
			require.NoError(t, err)

			require.NoError(t, r.Exec())
			assert.Equal(t, "payload", out.String())
		},
	)

	t.Run("executes in provided directory",
		func(t *testing.T) {
			var out bytes.Buffer
			dir := t.TempDir()

			r, err := Cmd(t.Context(), "testdata/util.sh", WithArgs("pwd"), WithDir(dir), WithStdOut(&out))
			require.NoError(t, err)

			require.NoError(t, r.Exec())

			// note: EvalSymlinks is needed because macos has /var symlinked to /private/var
			// t.TempDir will return "/var/<someid>" and the util script will print "/private/var/<someid>"

			want, err := filepath.EvalSymlinks(dir)
			require.NoError(t, err)

			got, err := filepath.EvalSymlinks(strings.TrimSpace(out.String()))
			require.NoError(t, err)

			assert.Equal(t, want, got)
		},
	)
}

func TestRun(t *testing.T) {
	skipOnWindows(t)

	t.Run("fails to build command",
		func(t *testing.T) {
			err := Run(t.Context(), "testdata/util.sh", WithEnv("INVALID_ENV"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "doesn't match NAME=value expectation")
		},
	)

	t.Run("success",
		func(t *testing.T) {
			err := Run(t.Context(), "testdata/util.sh", WithArgs("success"), WithStdOut(io.Discard))
			require.NoError(t, err)
		},
	)

	t.Run("failure",
		func(t *testing.T) {
			err := Run(t.Context(), "testdata/util.sh", WithArgs("fail"), WithStdOut(io.Discard))
			require.Error(t, err)
		},
	)
}

func skipOnWindows(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("test requires testdata/util.sh (shell script)")
	}
}
