package gen

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMageListOutput(t *testing.T) {
	t.Run("standard output with multiple targets",
		func(t *testing.T) {
			output := `Targets:
  format    format codebase using gofmt and goimports
  lint      lint the code using go mod tidy, commitsar and golangci-lint
  test      run unit tests
  tidy      run go mod tidy
`
			targets, err := parseMageListOutput(output)
			require.NoError(t, err)

			require.Len(t, targets, 4)
			assert.Equal(t, "format", targets[0].name)
			assert.Equal(t, "format codebase using gofmt and goimports", targets[0].description)
			assert.Equal(t, "lint", targets[1].name)
			assert.Equal(t, "lint the code using go mod tidy, commitsar and golangci-lint", targets[1].description)
			assert.Equal(t, "test", targets[2].name)
			assert.Equal(t, "run unit tests", targets[2].description)
			assert.Equal(t, "tidy", targets[3].name)
			assert.Equal(t, "run go mod tidy", targets[3].description)
		},
	)

	t.Run("default target marked with asterisk is stripped",
		func(t *testing.T) {
			output := `Targets:
  build*    build the project (default)
  lint      lint the code
`
			targets, err := parseMageListOutput(output)
			require.NoError(t, err)

			require.Len(t, targets, 2)
			assert.Equal(t, "build", targets[0].name, "asterisk suffix should be stripped")
			assert.Equal(t, "build the project (default)", targets[0].description)
			assert.Equal(t, "lint", targets[1].name)
			assert.Equal(t, "lint the code", targets[1].description)
		},
	)

	t.Run("single target with no description",
		func(t *testing.T) {
			output := `Targets:
  nodesc
`
			targets, err := parseMageListOutput(output)
			require.NoError(t, err)

			require.Len(t, targets, 1)
			assert.Equal(t, "nodesc", targets[0].name)
			assert.Equal(t, "", targets[0].description)
		},
	)

	t.Run("multiple targets middle one with no description",
		func(t *testing.T) {
			output := `Targets:
  format      codebase using gofmt and goimports
  generate
  lint        the code using go mod tidy, commitsar and golangci-lint
`
			targets, err := parseMageListOutput(output)
			require.NoError(t, err)

			require.Len(t, targets, 3)
			assert.Equal(t, "format", targets[0].name)
			assert.Equal(t, "codebase using gofmt and goimports", targets[0].description)
			assert.Equal(t, "generate", targets[1].name)
			assert.Equal(t, "", targets[1].description)
			assert.Equal(t, "lint", targets[2].name)
			assert.Equal(t, "the code using go mod tidy, commitsar and golangci-lint", targets[2].description)
		},
	)

	t.Run("target names are lowercased",
		func(t *testing.T) {
			output := `Targets:
  BuildAll    build everything
`
			targets, err := parseMageListOutput(output)
			require.NoError(t, err)

			require.Len(t, targets, 1)
			assert.Equal(t, "buildall", targets[0].name)
		},
	)

	t.Run("empty output returns no targets",
		func(t *testing.T) {
			targets, err := parseMageListOutput("")
			require.NoError(t, err)
			assert.Empty(t, targets)
		},
	)

	t.Run("output with no Targets section returns no targets",
		func(t *testing.T) {
			output := `mage: no magefiles found in current directory
`
			targets, err := parseMageListOutput(output)
			require.NoError(t, err)
			assert.Empty(t, targets)
		},
	)

	t.Run("blank lines inside targets section are ignored",
		func(t *testing.T) {
			output := `Targets:
  format    format code

  lint      lint code
`
			targets, err := parseMageListOutput(output)
			require.NoError(t, err)

			require.Len(t, targets, 2)
			assert.Equal(t, "format", targets[0].name)
			assert.Equal(t, "lint", targets[1].name)
		},
	)
}

func TestIntrospectMageTasks(t *testing.T) {
	skipOnWindows(t)

	t.Run("returns targets from fakemage",
		func(t *testing.T) {
			gen := &Generator{magecmd: "testdata/fakemage.sh"}
			targets, err := gen.introspectMageTasks(t.Context())
			require.NoError(t, err)

			require.Len(t, targets, 4)
			assert.Equal(t, "build", targets[0].name)
			assert.Equal(t, "build the project (default)", targets[0].description)
			assert.Equal(t, "format", targets[1].name)
			assert.Equal(t, "lint", targets[2].name)
			assert.Equal(t, "test", targets[3].name)
		},
	)

	t.Run("returns error when mage command fails",
		func(t *testing.T) {
			gen := &Generator{magecmd: "/nonexistent/mage"}
			_, err := gen.introspectMageTasks(t.Context())
			require.Error(t, err)
		},
	)
}

// skipOnWindows skips a test that requires shell scripts (.sh files).
func skipOnWindows(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("test requires shell scripts (not supported on Windows)")
	}
}
