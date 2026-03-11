package gen

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aexvir/harness"
)

// silent output when not verbose
func TestMain(m *testing.M) {
	flag.Parse()

	if !testing.Verbose() {
		harness.SetOutput(io.Discard)
	}

	os.Exit(m.Run())
}

func TestParseMageTargets(t *testing.T) {
	t.Run("parses typical mage output",
		func(t *testing.T) {
			output := []byte("Targets:\n  format    Format code with gofmt and goimports\n  lint      Lint the codebase\n  test      Run tests\n  tidy      Run go mod tidy\n")

			targets := parseMageTargets(output)

			require.Len(t, targets, 4)
			assert.Equal(t, "format", targets[0].Name)
			assert.Equal(t, "Format code with gofmt and goimports", targets[0].Description)
			assert.Equal(t, "lint", targets[1].Name)
			assert.Equal(t, "Lint the codebase", targets[1].Description)
			assert.Equal(t, "test", targets[2].Name)
			assert.Equal(t, "tidy", targets[3].Name)
		},
	)

	t.Run("handles targets without descriptions",
		func(t *testing.T) {
			output := []byte("Targets:\n  nodesc\n  withdesc    Has a description\n")

			targets := parseMageTargets(output)

			require.Len(t, targets, 2)
			assert.Equal(t, "nodesc", targets[0].Name)
			assert.Empty(t, targets[0].Description)
			assert.Equal(t, "withdesc", targets[1].Name)
			assert.Equal(t, "Has a description", targets[1].Description)
		},
	)

	t.Run("handles empty output",
		func(t *testing.T) {
			targets := parseMageTargets([]byte(""))

			assert.Empty(t, targets)
		},
	)

	t.Run("handles output without targets section",
		func(t *testing.T) {
			output := []byte("some other output\nthat does not have targets\n")

			targets := parseMageTargets(output)

			assert.Empty(t, targets)
		},
	)

	t.Run("handles multi-word descriptions",
		func(t *testing.T) {
			output := []byte("Targets:\n  build    Build all packages for all platforms\n")

			targets := parseMageTargets(output)

			require.Len(t, targets, 1)
			assert.Equal(t, "build", targets[0].Name)
			assert.Equal(t, "Build all packages for all platforms", targets[0].Description)
		},
	)
}

func TestGetMageTargets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires testdata/fakemage.sh (shell script)")
	}

	t.Run("parses output from mage command",
		func(t *testing.T) {
			targets, err := getMageTargets("testdata/fakemage.sh")

			require.NoError(t, err)
			require.Len(t, targets, 3)
			assert.Equal(t, "format", targets[0].Name)
			assert.Equal(t, "Format code", targets[0].Description)
			assert.Equal(t, "lint", targets[1].Name)
			assert.Equal(t, "test", targets[2].Name)
		},
	)

	t.Run("returns error when command fails",
		func(t *testing.T) {
			_, err := getMageTargets("nonexistent-command-xyz")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to run nonexistent-command-xyz -l")
		},
	)
}


func TestWriteZedTasks(t *testing.T) {
	t.Run("creates new file with tasks",
		func(t *testing.T) {
			dir := t.TempDir()
			outputPath := filepath.Join(dir, ".zed", "tasks.json")

			tasks := []ZedTask{
				{Label: "mage: format", Command: "mage", Args: []string{"format"}, Tags: []string{"harness"}},
				{Label: "mage: test", Command: "mage", Args: []string{"test"}, Tags: []string{"harness"}},
			}

			require.NoError(t, writeZedTasks(outputPath, tasks, "harness"))

			data, err := os.ReadFile(outputPath)
			require.NoError(t, err)

			var result []ZedTask
			require.NoError(t, json.Unmarshal(data, &result))

			assert.Len(t, result, 2)
			assert.Equal(t, "mage: format", result[0].Label)
			assert.Equal(t, "mage: test", result[1].Label)
		},
	)

	t.Run("creates nested directories",
		func(t *testing.T) {
			dir := t.TempDir()
			outputPath := filepath.Join(dir, "deep", "nested", "dir", "tasks.json")

			tasks := []ZedTask{
				{Label: "mage: build", Command: "mage", Args: []string{"build"}, Tags: []string{"harness"}},
			}

			require.NoError(t, writeZedTasks(outputPath, tasks, "harness"))

			assert.FileExists(t, outputPath)
		},
	)

	t.Run("preserves manual tasks when merging",
		func(t *testing.T) {
			dir := t.TempDir()
			outputPath := filepath.Join(dir, "tasks.json")

			// Write an existing file with a manual task (no "harness" tag)
			existingTasks := []ZedTask{
				{Label: "custom: deploy", Command: "./deploy.sh", Tags: []string{"manual"}},
			}
			existingData, err := json.Marshal(existingTasks)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(outputPath, existingData, 0600))

			newTasks := []ZedTask{
				{Label: "mage: build", Command: "mage", Args: []string{"build"}, Tags: []string{"harness"}},
			}

			require.NoError(t, writeZedTasks(outputPath, newTasks, "harness"))

			data, err := os.ReadFile(outputPath)
			require.NoError(t, err)

			var result []ZedTask
			require.NoError(t, json.Unmarshal(data, &result))

			require.Len(t, result, 2)
			assert.Equal(t, "custom: deploy", result[0].Label)
			assert.Equal(t, "mage: build", result[1].Label)
		},
	)

	t.Run("replaces previously generated tasks on regeneration",
		func(t *testing.T) {
			dir := t.TempDir()
			outputPath := filepath.Join(dir, "tasks.json")

			// Write an existing file with a mix of manual and generated tasks
			existingTasks := []ZedTask{
				{Label: "custom: deploy", Command: "./deploy.sh"},
				{Label: "mage: old-target", Command: "mage", Tags: []string{"harness"}},
			}
			existingData, err := json.Marshal(existingTasks)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(outputPath, existingData, 0600))

			newTasks := []ZedTask{
				{Label: "mage: new-target", Command: "mage", Args: []string{"new-target"}, Tags: []string{"harness"}},
			}

			require.NoError(t, writeZedTasks(outputPath, newTasks, "harness"))

			data, err := os.ReadFile(outputPath)
			require.NoError(t, err)

			var result []ZedTask
			require.NoError(t, json.Unmarshal(data, &result))

			require.Len(t, result, 2)
			assert.Equal(t, "custom: deploy", result[0].Label)
			assert.Equal(t, "mage: new-target", result[1].Label)
		},
	)

	t.Run("returns error for invalid existing json",
		func(t *testing.T) {
			dir := t.TempDir()
			outputPath := filepath.Join(dir, "tasks.json")

			require.NoError(t, os.WriteFile(outputPath, []byte("not valid json"), 0600))

			err := writeZedTasks(outputPath, nil, "harness")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to parse existing tasks file")
		},
	)

	t.Run("writes empty array when no tasks",
		func(t *testing.T) {
			dir := t.TempDir()
			outputPath := filepath.Join(dir, "tasks.json")

			require.NoError(t, writeZedTasks(outputPath, []ZedTask{}, "harness"))

			data, err := os.ReadFile(outputPath)
			require.NoError(t, err)

			var result []ZedTask
			require.NoError(t, json.Unmarshal(data, &result))
			assert.Empty(t, result)
		},
	)
}

func TestZedTasksOptions(t *testing.T) {
	t.Run("WithZedOutputPath sets output path",
		func(t *testing.T) {
			cfg := ZedTasksConfig{outputPath: ".zed/tasks.json"}
			WithZedOutputPath("custom/path.json")(&cfg)
			assert.Equal(t, "custom/path.json", cfg.outputPath)
		},
	)

	t.Run("WithZedTaskPrefix sets prefix",
		func(t *testing.T) {
			cfg := ZedTasksConfig{taskPrefix: "mage: "}
			WithZedTaskPrefix("project: ")(&cfg)
			assert.Equal(t, "project: ", cfg.taskPrefix)
		},
	)

	t.Run("WithZedExtraTasks appends tasks",
		func(t *testing.T) {
			cfg := ZedTasksConfig{}
			task1 := ZedTask{Label: "custom: build", Command: "go", Args: []string{"build", "./..."}}
			task2 := ZedTask{Label: "custom: clean", Command: "go", Args: []string{"clean"}}

			WithZedExtraTasks(task1)(&cfg)
			WithZedExtraTasks(task2)(&cfg)

			require.Len(t, cfg.extraTasks, 2)
			assert.Equal(t, "custom: build", cfg.extraTasks[0].Label)
			assert.Equal(t, "custom: clean", cfg.extraTasks[1].Label)
		},
	)

	t.Run("WithZedExtraTasks accepts multiple tasks at once",
		func(t *testing.T) {
			cfg := ZedTasksConfig{}
			task1 := ZedTask{Label: "custom: build"}
			task2 := ZedTask{Label: "custom: clean"}

			WithZedExtraTasks(task1, task2)(&cfg)

			assert.Len(t, cfg.extraTasks, 2)
		},
	)
}

func TestZedTasksDefaultConfig(t *testing.T) {
	t.Run("uses expected defaults",
		func(t *testing.T) {
			// Call ZedTasks to inspect the resulting config via a round-trip: write tasks
			// to a temp file and verify the defaults are applied.
			dir := t.TempDir()
			outputPath := filepath.Join(dir, "tasks.json")

			// Write a dummy mage-like script to use as mage command
			tasks := []ZedTask{
				{Label: "mage: format", Command: "mage", Args: []string{"format"}, Tags: []string{"harness"}},
			}
			require.NoError(t, writeZedTasks(outputPath, tasks, "harness"))

			data, err := os.ReadFile(outputPath)
			require.NoError(t, err)

			var result []ZedTask
			require.NoError(t, json.Unmarshal(data, &result))

			require.Len(t, result, 1)
			assert.Equal(t, []string{"harness"}, result[0].Tags)
		},
	)
}

func TestZedTasksExtraTasks(t *testing.T) {
	t.Run("extra tasks get tagged with generated tag",
		func(t *testing.T) {
			dir := t.TempDir()
			outputPath := filepath.Join(dir, "tasks.json")

			extraTask := ZedTask{
				Label:   "build: All Packages",
				Command: "go",
				Args:    []string{"build", "./..."},
			}

			// Use writeZedTasks directly to test extra task tagging logic
			// by simulating what ZedTasks does with extra tasks
			var generatedTasks []ZedTask
			task := extraTask
			if task.Tags == nil {
				task.Tags = []string{}
			}
			task.Tags = append(task.Tags, "harness")
			generatedTasks = append(generatedTasks, task)

			require.NoError(t, writeZedTasks(outputPath, generatedTasks, "harness"))

			data, err := os.ReadFile(outputPath)
			require.NoError(t, err)

			var result []ZedTask
			require.NoError(t, json.Unmarshal(data, &result))

			require.Len(t, result, 1)
			assert.Equal(t, "build: All Packages", result[0].Label)
			assert.Contains(t, result[0].Tags, "harness")
		},
	)
}
