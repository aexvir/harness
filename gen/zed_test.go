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
			tasks := []ZedTask{
				{Label: "mage test", Command: "mage test"},
			}

			section, err := buildManagedSection(tasks)

			require.NoError(t, err)
			assert.Contains(t, section, harnessMarkerStart)
			assert.Contains(t, section, harnessMarkerEnd)
			assert.Contains(t, section, `"label": "mage test"`)
			assert.Contains(t, section, `"command": "mage test"`)
			assert.NotContains(t, section, `"args"`)
		},
	)

	t.Run("multiple tasks with comma separation",
		func(t *testing.T) {
			tasks := []ZedTask{
				{Label: "mage format", Command: "mage format"},
				{Label: "mage lint", Command: "mage lint"},
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
			tasks := []ZedTask{
				{
					Label:   "custom",
					Command: "echo hello",
					Env:     map[string]string{"FOO": "bar"},
					Reveal:  "always",
				},
			}

			section, err := buildManagedSection(tasks)

			require.NoError(t, err)
			assert.Contains(t, section, `"env"`)
			assert.Contains(t, section, `"FOO": "bar"`)
			assert.Contains(t, section, `"reveal": "always"`)
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
		tasks := []ZedTask{
			{Label: "mage format", Command: "mage format"},
			{Label: "mage lint", Command: "mage lint"},
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
			original, _ := os.Getwd()
			require.NoError(t, os.Chdir(dir))
			t.Cleanup(func() { os.Chdir(original) })

			tasks := []ZedTask{
				{Label: "custom build", Command: "go build ./..."},
			}

			// build the managed section directly (skipping mage discovery)
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
			original, _ := os.Getwd()
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
			tasks := []ZedTask{
				{Label: "mage test", Command: "mage test"},
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
			tasks2 := []ZedTask{
				{Label: "mage build", Command: "mage build"},
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
