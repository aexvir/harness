package gen

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aexvir/harness"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Verbose() {
		harness.SetOutput(io.Discard)
	}
	code := m.Run()
	os.Exit(code)
}

func TestParseMageListOutput(t *testing.T) {
	t.Run("parses standard output",
		func(t *testing.T) {
			output := `Targets:
  format    codebase using gofmt and goimports
  lint      the code using go mod tidy, commitsar and golangci-lint
  test      run unit tests
  tidy      run go mod tidy
`
			targets := parseMageListOutput(output)

			require.Len(t, targets, 4)
			assert.Equal(t, "format", targets[0].name)
			assert.Equal(t, "codebase using gofmt and goimports", targets[0].description)
			assert.Equal(t, "lint", targets[1].name)
			assert.Equal(t, "test", targets[2].name)
			assert.Equal(t, "tidy", targets[3].name)
		},
	)

	t.Run("handles default target marker",
		func(t *testing.T) {
			output := `Targets:
  build*    build the project
  test      run tests
`
			targets := parseMageListOutput(output)

			require.Len(t, targets, 2)
			assert.Equal(t, "build", targets[0].name)
			assert.Equal(t, "build the project", targets[0].description)
		},
	)

	t.Run("handles targets without description",
		func(t *testing.T) {
			output := `Targets:
  build
  test
`
			targets := parseMageListOutput(output)

			require.Len(t, targets, 2)
			assert.Equal(t, "build", targets[0].name)
			assert.Equal(t, "", targets[0].description)
		},
	)

	t.Run("stops at non-target section",
		func(t *testing.T) {
			output := `Targets:
  format    format code
  lint      lint code

Aliases:
  f    format
`
			targets := parseMageListOutput(output)

			require.Len(t, targets, 2)
			assert.Equal(t, "format", targets[0].name)
			assert.Equal(t, "lint", targets[1].name)
		},
	)

	t.Run("returns empty for no targets",
		func(t *testing.T) {
			targets := parseMageListOutput("")
			assert.Empty(t, targets)
		},
	)
}

func TestBuildManagedSection(t *testing.T) {
	t.Run("single task",
		func(t *testing.T) {
			tasks := []map[string]any{
				{"label": "mage test", "command": "mage test"},
			}

			section, err := buildManagedSection(tasks)

			require.NoError(t, err)
			assert.Contains(t, section, harnessMarkerStart)
			assert.Contains(t, section, harnessMarkerEnd)
			assert.Contains(t, section, `"label": "mage test"`)
			assert.Contains(t, section, `"command": "mage test"`)
		},
	)

	t.Run("multiple tasks with comma separation",
		func(t *testing.T) {
			tasks := []map[string]any{
				{"label": "mage format", "command": "mage format"},
				{"label": "mage lint", "command": "mage lint"},
			}

			section, err := buildManagedSection(tasks)

			require.NoError(t, err)
			assert.Contains(t, section, `"mage format"`)
			assert.Contains(t, section, `"mage lint"`)
			// last task should not have trailing comma, but intermediate ones should
			lines := splitNonEmpty(section)
			foundComma := false
			for _, line := range lines {
				if line == "  }," {
					foundComma = true
				}
			}
			assert.True(t, foundComma, "expected comma between tasks")
		},
	)

	t.Run("task with optional fields",
		func(t *testing.T) {
			tasks := []map[string]any{
				{
					"label":   "custom",
					"command": "echo hello",
					"env":     map[string]any{"FOO": "bar"},
					"reveal":  "always",
				},
			}

			section, err := buildManagedSection(tasks)

			require.NoError(t, err)
			assert.Contains(t, section, `"env"`)
			assert.Contains(t, section, `"FOO": "bar"`)
			assert.Contains(t, section, `"reveal": "always"`)
		},
	)

	t.Run("preserves label and command ordering",
		func(t *testing.T) {
			tasks := []map[string]any{
				{
					"label":        "mage test",
					"command":      "mage test",
					"reveal":       "always",
					"show_summary": true,
				},
			}

			section, err := buildManagedSection(tasks)

			require.NoError(t, err)
			labelIdx := strings.Index(section, `"label"`)
			commandIdx := strings.Index(section, `"command"`)
			revealIdx := strings.Index(section, `"reveal"`)
			assert.Less(t, labelIdx, commandIdx, "label should come before command")
			assert.Less(t, commandIdx, revealIdx, "command should come before other fields")
		},
	)

	t.Run("empty tasks",
		func(t *testing.T) {
			section, err := buildManagedSection(nil)

			require.NoError(t, err)
			assert.Contains(t, section, harnessMarkerStart)
			assert.Contains(t, section, harnessMarkerEnd)
		},
	)
}

func TestMergeContent(t *testing.T) {
	managedSection := func(t *testing.T) string {
		t.Helper()
		tasks := []map[string]any{
			{"label": "mage format", "command": "mage format"},
			{"label": "mage lint", "command": "mage lint"},
		}
		s, err := buildManagedSection(tasks)
		require.NoError(t, err)
		return s
	}

	t.Run("inserts into empty array",
		func(t *testing.T) {
			existing := "[\n]\n"
			section := managedSection(t)

			result, err := mergeContent(existing, section)

			require.NoError(t, err)
			assert.Contains(t, result, harnessMarkerStart)
			assert.Contains(t, result, harnessMarkerEnd)
			assert.Contains(t, result, `"mage format"`)
			assert.Contains(t, result, `"mage lint"`)
			assertValidJSONCArray(t, result)
		},
	)

	t.Run("inserts after existing tasks",
		func(t *testing.T) {
			existing := `[
  {
    "label": "custom task",
    "command": "echo hello"
  }
]
`
			section := managedSection(t)

			result, err := mergeContent(existing, section)

			require.NoError(t, err)
			assert.Contains(t, result, `"custom task"`)
			assert.Contains(t, result, `"mage format"`)
			assert.Contains(t, result, harnessMarkerStart)
			// the closing brace of the existing task should have a comma
			assert.Contains(t, result, "},")
			assertValidJSONCArray(t, result)
		},
	)

	t.Run("replaces existing managed section",
		func(t *testing.T) {
			existing := `[
  {
    "label": "custom task",
    "command": "echo hello"
  },
  // harness:start
  {
    "label": "mage old",
    "command": "mage old"
  }
  // harness:end
]
`
			section := managedSection(t)

			result, err := mergeContent(existing, section)

			require.NoError(t, err)
			assert.Contains(t, result, `"custom task"`)
			assert.Contains(t, result, `"mage format"`)
			assert.NotContains(t, result, `"mage old"`)
			assertValidJSONCArray(t, result)
		},
	)

	t.Run("replaces managed section when it is the only content",
		func(t *testing.T) {
			existing := `[
  // harness:start
  {
    "label": "mage old",
    "command": "mage old"
  }
  // harness:end
]
`
			section := managedSection(t)

			result, err := mergeContent(existing, section)

			require.NoError(t, err)
			assert.Contains(t, result, `"mage format"`)
			assert.NotContains(t, result, `"mage old"`)
			assertValidJSONCArray(t, result)
		},
	)

	t.Run("handles existing file with trailing comma",
		func(t *testing.T) {
			existing := `[
  {
    "label": "custom task",
    "command": "echo hello"
  },
]
`
			section := managedSection(t)

			result, err := mergeContent(existing, section)

			require.NoError(t, err)
			assert.Contains(t, result, `"custom task"`)
			assert.Contains(t, result, `"mage format"`)
			assertValidJSONCArray(t, result)
		},
	)

	t.Run("errors on invalid file",
		func(t *testing.T) {
			existing := `this is not json at all`

			_, err := mergeContent(existing, "  // harness:start\n  // harness:end")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing closing bracket")
		},
	)
}

func TestZedTasksFileIntegration(t *testing.T) {
	t.Run("creates new file with additional tasks when mage is unavailable",
		func(t *testing.T) {
			dir := t.TempDir()
			original, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(dir))
			t.Cleanup(func() { os.Chdir(original) })

			tasks := []map[string]any{
				{"label": "custom build", "command": "go build ./..."},
			}

			managedSection, err := buildManagedSection(tasks)
			require.NoError(t, err)

			require.NoError(t, os.MkdirAll(".zed", 0o755))
			content := "[\n" + managedSection + "\n]\n"
			require.NoError(t, os.WriteFile(zedTasksFilePath, []byte(content), 0o644))

			got, err := os.ReadFile(zedTasksFilePath)
			require.NoError(t, err)
			assert.Contains(t, string(got), `"custom build"`)
			assert.Contains(t, string(got), harnessMarkerStart)
			assert.Contains(t, string(got), harnessMarkerEnd)
		},
	)

	t.Run("merges with existing file",
		func(t *testing.T) {
			dir := t.TempDir()
			original, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(dir))
			t.Cleanup(func() { os.Chdir(original) })

			require.NoError(t, os.MkdirAll(".zed", 0o755))

			// write an existing file with a manual task
			existing := `[
  {
    "label": "my manual task",
    "command": "echo manual"
  }
]
`
			require.NoError(t, os.WriteFile(zedTasksFilePath, []byte(existing), 0o644))

			// now merge a managed section into it
			tasks := []map[string]any{
				{"label": "mage test", "command": "mage test"},
			}
			managedSection, err := buildManagedSection(tasks)
			require.NoError(t, err)

			content, err := mergeContent(existing, managedSection)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(zedTasksFilePath, []byte(content), 0o644))

			got, err := os.ReadFile(zedTasksFilePath)
			require.NoError(t, err)
			assert.Contains(t, string(got), `"my manual task"`)
			assert.Contains(t, string(got), `"mage test"`)

			// regenerate with different tasks should keep manual task
			tasks2 := []map[string]any{
				{"label": "mage build", "command": "mage build"},
			}
			managedSection2, err := buildManagedSection(tasks2)
			require.NoError(t, err)

			content2, err := mergeContent(string(got), managedSection2)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(zedTasksFilePath, []byte(content2), 0o644))

			got2, err := os.ReadFile(filepath.Join(dir, zedTasksFilePath))
			require.NoError(t, err)
			assert.Contains(t, string(got2), `"my manual task"`)
			assert.Contains(t, string(got2), `"mage build"`)
			assert.NotContains(t, string(got2), `"mage test"`)
		},
	)
}

func TestExtractManagedTasks(t *testing.T) {
	t.Run("extracts tasks from managed section",
		func(t *testing.T) {
			content := `[
  // harness:start
  {
    "label": "mage test",
    "command": "mage test",
    "reveal": "always"
  }
  // harness:end
]
`
			tasks := extractManagedTasks(content)

			require.Len(t, tasks, 1)
			assert.Equal(t, "mage test", tasks[0]["label"])
			assert.Equal(t, "mage test", tasks[0]["command"])
			assert.Equal(t, "always", tasks[0]["reveal"])
		},
	)

	t.Run("returns nil when no markers",
		func(t *testing.T) {
			content := `[
  {
    "label": "some task",
    "command": "echo hello"
  }
]
`
			tasks := extractManagedTasks(content)
			assert.Nil(t, tasks)
		},
	)

	t.Run("returns nil for empty content",
		func(t *testing.T) {
			tasks := extractManagedTasks("")
			assert.Nil(t, tasks)
		},
	)

	t.Run("extracts multiple tasks",
		func(t *testing.T) {
			content := `[
  // harness:start
  {
    "label": "mage format",
    "command": "mage format"
  },
  {
    "label": "mage test",
    "command": "mage test",
    "show_summary": true
  }
  // harness:end
]
`
			tasks := extractManagedTasks(content)

			require.Len(t, tasks, 2)
			assert.Equal(t, "mage format", tasks[0]["label"])
			assert.Equal(t, "mage test", tasks[1]["label"])
			assert.Equal(t, true, tasks[1]["show_summary"])
		},
	)
}

func TestMergeTaskCustomizations(t *testing.T) {
	t.Run("preserves user-added fields",
		func(t *testing.T) {
			existing := []map[string]any{
				{
					"label":        "mage test",
					"command":      "mage test",
					"reveal":       "always",
					"show_summary": true,
				},
			}

			newTasks := []ZedTask{
				{Label: "mage test", Command: "mage test"},
			}

			merged, err := mergeTaskCustomizations(newTasks, existing)

			require.NoError(t, err)
			require.Len(t, merged, 1)
			assert.Equal(t, "mage test", merged[0]["label"])
			assert.Equal(t, "mage test", merged[0]["command"])
			assert.Equal(t, "always", merged[0]["reveal"])
			assert.Equal(t, true, merged[0]["show_summary"])
		},
	)

	t.Run("adds new task when no existing match",
		func(t *testing.T) {
			existing := []map[string]any{
				{"label": "mage format", "command": "mage format"},
			}

			newTasks := []ZedTask{
				{Label: "mage format", Command: "mage format"},
				{Label: "mage lint", Command: "mage lint"},
			}

			merged, err := mergeTaskCustomizations(newTasks, existing)

			require.NoError(t, err)
			require.Len(t, merged, 2)
			assert.Equal(t, "mage format", merged[0]["label"])
			assert.Equal(t, "mage lint", merged[1]["label"])
		},
	)

	t.Run("handles empty existing",
		func(t *testing.T) {
			newTasks := []ZedTask{
				{Label: "mage test", Command: "mage test"},
			}

			merged, err := mergeTaskCustomizations(newTasks, nil)

			require.NoError(t, err)
			require.Len(t, merged, 1)
			assert.Equal(t, "mage test", merged[0]["label"])
		},
	)

	t.Run("updates command while preserving customizations",
		func(t *testing.T) {
			existing := []map[string]any{
				{
					"label":   "docker build",
					"command": "docker build -t old .",
					"reveal":  "always",
				},
			}

			newTasks := []ZedTask{
				{Label: "docker build", Command: "docker build -t new ."},
			}

			merged, err := mergeTaskCustomizations(newTasks, existing)

			require.NoError(t, err)
			require.Len(t, merged, 1)
			assert.Equal(t, "docker build -t new .", merged[0]["command"])
			assert.Equal(t, "always", merged[0]["reveal"])
		},
	)

	t.Run("preserves show_summary false",
		func(t *testing.T) {
			existing := []map[string]any{
				{
					"label":        "mage format",
					"command":      "mage format",
					"show_summary": false,
				},
			}

			newTasks := []ZedTask{
				{Label: "mage format", Command: "mage format"},
			}

			merged, err := mergeTaskCustomizations(newTasks, existing)

			require.NoError(t, err)
			require.Len(t, merged, 1)
			assert.Equal(t, false, merged[0]["show_summary"])
		},
	)

	t.Run("drops tasks removed from new set",
		func(t *testing.T) {
			existing := []map[string]any{
				{"label": "mage old", "command": "mage old"},
			}

			newTasks := []ZedTask{
				{Label: "mage new", Command: "mage new"},
			}

			merged, err := mergeTaskCustomizations(newTasks, existing)

			require.NoError(t, err)
			require.Len(t, merged, 1)
			assert.Equal(t, "mage new", merged[0]["label"])
		},
	)
}

func TestAddZedTask(t *testing.T) {
	t.Run("creates file with single task",
		func(t *testing.T) {
			dir := t.TempDir()
			original, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(dir))
			t.Cleanup(func() { os.Chdir(original) })

			err = AddZedTask(ZedTask{Label: "build", Command: "go build ./..."})
			require.NoError(t, err)

			got, err := os.ReadFile(zedTasksFilePath)
			require.NoError(t, err)
			assert.Contains(t, string(got), `"label": "build"`)
			assert.Contains(t, string(got), `"command": "go build ./..."`)
			assert.Contains(t, string(got), harnessMarkerStart)
			assert.Contains(t, string(got), harnessMarkerEnd)
			assertValidJSONCArray(t, string(got))
		},
	)

	t.Run("appends task to existing managed section",
		func(t *testing.T) {
			dir := t.TempDir()
			original, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(dir))
			t.Cleanup(func() { os.Chdir(original) })

			// add first task
			err = AddZedTask(ZedTask{Label: "build", Command: "go build ./..."})
			require.NoError(t, err)

			// add second task
			err = AddZedTask(ZedTask{Label: "test", Command: "go test ./..."})
			require.NoError(t, err)

			got, err := os.ReadFile(zedTasksFilePath)
			require.NoError(t, err)
			assert.Contains(t, string(got), `"label": "build"`)
			assert.Contains(t, string(got), `"label": "test"`)
			assertValidJSONCArray(t, string(got))
		},
	)

	t.Run("updates existing task by label",
		func(t *testing.T) {
			dir := t.TempDir()
			original, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(dir))
			t.Cleanup(func() { os.Chdir(original) })

			// add a task
			err = AddZedTask(ZedTask{Label: "build", Command: "go build ./..."})
			require.NoError(t, err)

			// update the same task with new command
			err = AddZedTask(ZedTask{Label: "build", Command: "go build -o app ./cmd/app"})
			require.NoError(t, err)

			got, err := os.ReadFile(zedTasksFilePath)
			require.NoError(t, err)
			assert.Contains(t, string(got), `"command": "go build -o app ./cmd/app"`)
			assert.NotContains(t, string(got), `"command": "go build ./..."`)
			assertValidJSONCArray(t, string(got))
		},
	)

	t.Run("preserves user customizations on update",
		func(t *testing.T) {
			dir := t.TempDir()
			original, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(dir))
			t.Cleanup(func() { os.Chdir(original) })

			// add initial task
			err = AddZedTask(ZedTask{Label: "build", Command: "go build ./..."})
			require.NoError(t, err)

			// simulate user customization by editing the file
			got, err := os.ReadFile(zedTasksFilePath)
			require.NoError(t, err)
			customized := strings.Replace(string(got),
				`"command": "go build ./..."`,
				`"command": "go build ./...",`+"\n"+`    "reveal": "always",`+"\n"+`    "show_summary": true`,
				1)
			require.NoError(t, os.WriteFile(zedTasksFilePath, []byte(customized), 0o644))

			// re-add the same task (simulating re-generation)
			err = AddZedTask(ZedTask{Label: "build", Command: "go build ./..."})
			require.NoError(t, err)

			got, err = os.ReadFile(zedTasksFilePath)
			require.NoError(t, err)
			assert.Contains(t, string(got), `"reveal": "always"`)
			assert.Contains(t, string(got), `"show_summary": true`)
			assertValidJSONCArray(t, string(got))
		},
	)

	t.Run("preserves tasks outside managed section",
		func(t *testing.T) {
			dir := t.TempDir()
			original, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(dir))
			t.Cleanup(func() { os.Chdir(original) })

			require.NoError(t, os.MkdirAll(".zed", 0o755))

			existing := `[
  {
    "label": "my manual task",
    "command": "echo manual"
  }
]
`
			require.NoError(t, os.WriteFile(zedTasksFilePath, []byte(existing), 0o644))

			err = AddZedTask(ZedTask{Label: "build", Command: "go build ./..."})
			require.NoError(t, err)

			got, err := os.ReadFile(zedTasksFilePath)
			require.NoError(t, err)
			assert.Contains(t, string(got), `"my manual task"`)
			assert.Contains(t, string(got), `"label": "build"`)
			assertValidJSONCArray(t, string(got))
		},
	)
}

// helpers

func splitNonEmpty(s string) []string {
	var result []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// assertValidJSONCArray checks that the content is a valid JSON array
// after stripping JSONC comments.
func assertValidJSONCArray(t *testing.T, content string) {
	t.Helper()

	var jsonLines []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		jsonLines = append(jsonLines, line)
	}
	jsonContent := strings.Join(jsonLines, "\n")

	var arr []any
	err := json.Unmarshal([]byte(jsonContent), &arr)
	assert.NoError(t, err, "content should be valid JSON after stripping comments:\n%s", jsonContent)
}
