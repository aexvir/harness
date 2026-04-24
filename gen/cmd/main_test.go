package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aexvir/harness/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRun invokes the CLI app with the given args (the binary name is prepended
// automatically). Help output is suppressed so tests stay clean.
func testRun(t *testing.T, args ...string) error {
	t.Helper()
	cmd := buildApp()
	cmd.Writer = io.Discard
	cmd.ErrWriter = io.Discard
	return cmd.Run(context.Background(), append([]string{"gen"}, args...))
}

func TestRun(t *testing.T) {
	t.Run("no arguments returns error",
		func(t *testing.T) {
			err := testRun(t)
			require.Error(t, err)
		},
	)

	t.Run("unknown generator returns error",
		func(t *testing.T) {
			err := testRun(t, "unknown", "task")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unknown generator")
		},
	)

	t.Run("zed with no action returns error",
		func(t *testing.T) {
			err := testRun(t, "zed")
			require.Error(t, err)
		},
	)

	t.Run("unknown zed action returns error",
		func(t *testing.T) {
			err := testRun(t, "zed", "unknown")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unknown zed action")
		},
	)
}

func TestZedTask(t *testing.T) {
	t.Run("missing --label returns error",
		func(t *testing.T) {
			err := testRun(t, "zed", "task", "--command=go test ./...")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "label")
		},
	)

	t.Run("missing --command returns error",
		func(t *testing.T) {
			err := testRun(t, "zed", "task", "--label=test")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "command")
		},
	)

	t.Run("unknown flag returns error",
		func(t *testing.T) {
			err := testRun(t, "zed", "task", "--label=test", "--command=go test", "--bogus=x")
			require.Error(t, err)
		},
	)

	t.Run("creates tasks.json with the given task",
		func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".zed", "tasks.json")

			err := testRun(t, "zed", "task",
				"--label=test",
				"--command=go test ./...",
				"--file="+path,
			)
			require.NoError(t, err)

			tasks := readTasks(t, path)
			require.Len(t, tasks, 1)
			assert.Equal(t, "test", tasks[0].Label)
			assert.Equal(t, "go test ./...", tasks[0].Command)
			assert.Empty(t, tasks[0].Args)
		},
	)

	t.Run("repeatable --args are collected in order",
		func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tasks.json")

			err := testRun(t, "zed", "task",
				"--label=docker up",
				"--command=docker",
				"--args=compose",
				"--args=up",
				"--args=-d",
				"--file="+path,
			)
			require.NoError(t, err)

			tasks := readTasks(t, path)
			require.Len(t, tasks, 1)
			assert.Equal(t, "docker", tasks[0].Command)
			assert.Equal(t, []string{"compose", "up", "-d"}, tasks[0].Args)
		},
	)

	t.Run("--reveal flag is written to the task",
		func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tasks.json")

			err := testRun(t, "zed", "task",
				"--label=test",
				"--command=go test ./...",
				"--reveal=always",
				"--file="+path,
			)
			require.NoError(t, err)

			tasks := readTasks(t, path)
			require.Len(t, tasks, 1)
			assert.Equal(t, "always", tasks[0].Reveal)
		},
	)

	t.Run("re-run updates command; user-added fields are preserved",
		func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tasks.json")

			// first run: create the task
			require.NoError(t, testRun(t, "zed", "task",
				"--label=test",
				"--command=go test ./...",
				"--file="+path,
			))

			// simulate user editing the file (add reveal)
			tasks := readTasks(t, path)
			tasks[0].Reveal = "focus"
			writeTasksToFile(t, path, tasks)

			// second run: update the command
			require.NoError(t, testRun(t, "zed", "task",
				"--label=test",
				"--command=gotestsum ./...",
				"--file="+path,
			))

			tasks = readTasks(t, path)
			require.Len(t, tasks, 1)
			assert.Equal(t, "gotestsum ./...", tasks[0].Command, "command must be refreshed")
			assert.Equal(t, "focus", tasks[0].Reveal, "user-set reveal must be preserved")
		},
	)

	t.Run("default file path is .zed/tasks.json in cwd",
		func(t *testing.T) {
			original, err := os.Getwd()
			require.NoError(t, err)
			dir := t.TempDir()
			require.NoError(t, os.Chdir(dir))
			t.Cleanup(func() { require.NoError(t, os.Chdir(original)) })

			err = testRun(t, "zed", "task", "--label=build", "--command=go build ./...")
			require.NoError(t, err)
			assert.FileExists(t, filepath.Join(dir, ".zed", "tasks.json"))
		},
	)
}

func readTasks(t *testing.T, path string) []gen.ZedTask {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var tasks []gen.ZedTask
	require.NoError(t, json.Unmarshal(data, &tasks))
	return tasks
}

func writeTasksToFile(t *testing.T, path string, tasks []gen.ZedTask) {
	t.Helper()
	data, err := json.MarshalIndent(tasks, "", "  ")
	require.NoError(t, err)
	data = append(data, '\n')
	require.NoError(t, os.WriteFile(path, data, 0o644))
}
