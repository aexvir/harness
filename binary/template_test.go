package binary

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplate_Resolve(t *testing.T) {
	template := Template{
		GOOS:             "linux",
		GOARCH:           "amd64",
		Directory:        "/tmp/bin",
		Name:             "example",
		Cmd:              "/tmp/bin/example",
		Version:          "v1.0.0",
		Extension:        "",
		ArchiveExtension: ".tar.gz",
	}

	tests := []struct {
		name     string
		format   string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple GOOS template",
			format:   "{{.GOOS}}",
			expected: "linux",
		},
		{
			name:     "simple GOARCH template",
			format:   "{{.GOARCH}}",
			expected: "amd64",
		},
		{
			name:     "version template",
			format:   "{{.Version}}",
			expected: "v1.0.0",
		},
		{
			name:     "complex URL template",
			format:   "https://github.com/foo/bar/releases/download/{{.Version}}/bin_{{.Version}}_{{.GOOS}}_{{.GOARCH}}{{.Extension}}",
			expected: "https://github.com/foo/bar/releases/download/v1.0.0/bin_v1.0.0_linux_amd64",
		},
		{
			name:     "archive URL template",
			format:   "https://example.com/{{.Name}}_{{.Version}}_{{.GOOS}}_{{.GOARCH}}{{.ArchiveExtension}}",
			expected: "https://example.com/example_v1.0.0_linux_amd64.tar.gz",
		},
		{
			name:     "invalid template",
			format:   "{{.InvalidField}}",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "malformed template",
			format:   "{{.GOOS",
			expected: "",
			wantErr:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name,
			func(t *testing.T) {
				result, err := template.Resolve(test.format)

				if test.wantErr {
					assert.Error(t, err)
					return
				}

				require.NoError(t, err)
				assert.Equal(t, test.expected, result)
			},
		)
	}
}

func TestTemplate_MustResolve(t *testing.T) {
	template := Template{
		GOOS:   "windows",
		GOARCH: "amd64",
		Name:   "test",
	}

	t.Run("successful resolution",
		func(t *testing.T) {
			result := template.MustResolve("{{.GOOS}}-{{.GOARCH}}")
			assert.Equal(t, "windows-amd64", result)
		},
	)

	t.Run("panic on invalid template",
		func(t *testing.T) {
			assert.Panics(t, func() { template.MustResolve("{{.InvalidField}}") })
		},
	)
}
