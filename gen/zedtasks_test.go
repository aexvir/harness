package gen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tailscale/hujson"
)

// readTasks is a test helper that decodes the tasks file into a []ZedTask
// slice for easy assertion. Comments and unknown fields are discarded.
func readTasks(t *testing.T, path string) []ZedTask {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	data, err = hujson.Standardize(data)
	require.NoError(t, err)
	var tasks []ZedTask
	require.NoError(t, json.Unmarshal(data, &tasks))
	return tasks
}

func TestUpsertZedTasks(t *testing.T) {
	t.Run("creates file and appends tasks when missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".zed", "tasks.json")

		require.NoError(t, New().UpsertZedTasks(
			WithZedTasksFile(path),
			WithExtraTasks(
				ZedTask{Label: "task1", Command: "cmd1"},
				ZedTask{Label: "task2", Command: "cmd2", Args: []string{"--flag"}},
			),
		)(t.Context()))

		tasks := readTasks(t, path)
		require.Len(t, tasks, 2)
		assert.Equal(t, "task1", tasks[0].Label)
		assert.Equal(t, "task2", tasks[1].Label)
		assert.Equal(t, []string{"--flag"}, tasks[1].Args)
	})

	t.Run("re-run refreshes command/args; preserves user fields and unknown JSON keys", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "tasks.json")

		// Seed file with a task that has a user-set field and a future Zed key
		// that is not in our ZedTask struct.
		seed := `[{"label":"my task","command":"old","args":["old"],"reveal":"always","zed_future":42}]`
		require.NoError(t, os.WriteFile(path, []byte(seed), 0o644))

		require.NoError(t, New().UpsertZedTasks(
			WithZedTasksFile(path),
			WithExtraTasks(ZedTask{Label: "my task", Command: "new", Args: []string{"new"}}),
		)(t.Context()))

		tasks := readTasks(t, path)
		require.Len(t, tasks, 1)
		assert.Equal(t, "new", tasks[0].Command, "command must be refreshed")
		assert.Equal(t, []string{"new"}, tasks[0].Args, "args must be refreshed")
		assert.Equal(t, "always", tasks[0].Reveal, "user-set field must be preserved")

		data, _ := os.ReadFile(path)
		assert.Contains(t, string(data), "zed_future", "unknown key must survive")
	})
}

func TestGenerateZedTasks(t *testing.T) {
	skipOnWindows(t)

	t.Run("generates entries for all mage targets", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "tasks.json")

		require.NoError(t, New(
			WithMageCommand("testdata/fakemage.sh"),
		).GenerateZedTasks(
			WithZedTasksFile(path),
			WithExtraTasks(ZedTask{Label: "extra", Command: "extra-cmd"}),
		)(t.Context()))

		tasks := readTasks(t, path)
		labels := make([]string, len(tasks))
		for i, task := range tasks {
			labels[i] = task.Label
		}

		assert.Contains(t, labels, "mage: build")
		assert.Contains(t, labels, "mage: format")
		assert.Contains(t, labels, "mage: lint")
		assert.Contains(t, labels, "mage: test")
		assert.Contains(t, labels, "extra")
	})

	t.Run("returns error when mage command is not found", func(t *testing.T) {
		err := New(
			WithMageCommand("/nonexistent/mage"),
		).GenerateZedTasks(
			WithZedTasksFile(filepath.Join(t.TempDir(), "tasks.json")),
		)(t.Context())
		require.Error(t, err)
	})
}

func TestCleanupZedTasks(t *testing.T) {
	skipOnWindows(t)

	t.Run("removes stale mage tasks; keeps active and non-mage; preserves comments", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "tasks.json")

		seed := `[
  // user comment
  {"label":"mage: build","command":"mage","args":["build"]},
  {"label":"mage: stale","command":"mage","args":["stale"]},
  {"label":"custom","command":"my-tool"}
]`
		require.NoError(t, os.WriteFile(path, []byte(seed), 0o644))

		require.NoError(t, New(
			WithMageCommand("testdata/fakemage.sh"),
		).CleanupZedTasks(
			WithZedTasksFile(path),
		)(t.Context()))

		tasks := readTasks(t, path)
		labels := make([]string, len(tasks))
		for i, task := range tasks {
			labels[i] = task.Label
		}

		assert.Contains(t, labels, "mage: build", "active mage task must be kept")
		assert.NotContains(t, labels, "mage: stale", "stale mage task must be removed")
		assert.Contains(t, labels, "custom", "non-mage task must be kept")

		data, _ := os.ReadFile(path)
		assert.Contains(t, string(data), "// user comment", "comments must survive")
	})
}
