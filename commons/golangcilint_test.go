package commons

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithGolangCIVersion(t *testing.T) {
	var conf golangcilintconf
	WithGolangCIVersion("v1.63.3")(&conf)
	assert.Equal(t, "v1.63.3", conf.version)
}

func TestWithGolangCICodeClimate(t *testing.T) {
	t.Run("enables codeclimate", func(t *testing.T) {
		var conf golangcilintconf
		WithGolangCICodeClimate(true)(&conf)
		assert.True(t, conf.codeclimate)
	})

	t.Run("disables codeclimate", func(t *testing.T) {
		conf := golangcilintconf{codeclimate: true}
		WithGolangCICodeClimate(false)(&conf)
		assert.False(t, conf.codeclimate)
	})
}

func TestWithGolangCICodeClimateOutput(t *testing.T) {
	var conf golangcilintconf
	WithGolangCICodeClimateOutput("custom-report.json")(&conf)
	assert.Equal(t, "custom-report.json", conf.codeclimatefile)
}

func TestGolangCILintDefaults(t *testing.T) {
	conf := golangcilintconf{
		version:         "latest",
		codeclimatefile: "quality-report.json",
	}
	assert.Equal(t, "latest", conf.version)
	assert.Equal(t, "quality-report.json", conf.codeclimatefile)
	assert.False(t, conf.codeclimate)
}

func TestBuildGolangCILintArgs(t *testing.T) {
	t.Run("default args without codeclimate", func(t *testing.T) {
		conf := golangcilintconf{}
		args := buildGolangCILintArgs(conf)
		assert.Equal(t, []string{
			"run",
			"--max-same-issues", "0",
			"--max-issues-per-linter", "0",
		}, args)
	})

	t.Run("with codeclimate enabled", func(t *testing.T) {
		conf := golangcilintconf{
			codeclimate:     true,
			codeclimatefile: "quality-report.json",
		}
		args := buildGolangCILintArgs(conf)
		assert.Equal(t, []string{
			"run",
			"--max-same-issues", "0",
			"--max-issues-per-linter", "0",
			"--out-format", "code-climate:quality-report.json",
		}, args)
	})

	t.Run("with custom codeclimate filename", func(t *testing.T) {
		conf := golangcilintconf{
			codeclimate:     true,
			codeclimatefile: "custom-output.json",
		}
		args := buildGolangCILintArgs(conf)
		assert.Contains(t, args, "--out-format")
		assert.Contains(t, args, "code-climate:custom-output.json")
	})

	t.Run("codeclimate disabled does not add out-format", func(t *testing.T) {
		conf := golangcilintconf{
			codeclimate:     false,
			codeclimatefile: "quality-report.json",
		}
		args := buildGolangCILintArgs(conf)
		assert.NotContains(t, args, "--out-format")
	})
}

func TestParseLinterIssues(t *testing.T) {
	t.Run("valid JSON with issues", func(t *testing.T) {
		data := []byte(`[
			{"description": "exported function Foo should have comment", "location": {"path": "main.go", "lines": {"begin": 10}}},
			{"description": "unused variable bar", "location": {"path": "pkg/util.go", "lines": {"begin": 42}}}
		]`)
		issues, err := parseLinterIssues(data)
		require.NoError(t, err)
		require.Len(t, issues, 2)

		assert.Equal(t, "exported function Foo should have comment", issues[0].Description)
		assert.Equal(t, "main.go", issues[0].Location.Path)
		assert.Equal(t, 10, issues[0].Location.Lines.Begin)

		assert.Equal(t, "unused variable bar", issues[1].Description)
		assert.Equal(t, "pkg/util.go", issues[1].Location.Path)
		assert.Equal(t, 42, issues[1].Location.Lines.Begin)
	})

	t.Run("empty array", func(t *testing.T) {
		data := []byte(`[]`)
		issues, err := parseLinterIssues(data)
		require.NoError(t, err)
		assert.Empty(t, issues)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		data := []byte(`{not valid json}`)
		_, err := parseLinterIssues(data)
		assert.Error(t, err)
	})

	t.Run("empty input returns error", func(t *testing.T) {
		_, err := parseLinterIssues([]byte{})
		assert.Error(t, err)
	})

	t.Run("null input returns error", func(t *testing.T) {
		_, err := parseLinterIssues(nil)
		assert.Error(t, err)
	})

	t.Run("single issue", func(t *testing.T) {
		data := []byte(`[{"description": "error check", "location": {"path": "x.go", "lines": {"begin": 1}}}]`)
		issues, err := parseLinterIssues(data)
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, "error check", issues[0].Description)
		assert.Equal(t, "x.go", issues[0].Location.Path)
		assert.Equal(t, 1, issues[0].Location.Lines.Begin)
	})
}
