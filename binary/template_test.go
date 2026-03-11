package binary

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateResolve(t *testing.T) {
	tmpl := Template{
		GOOS:             "linux",
		GOARCH:           "amd64",
		Directory:        "bin",
		Name:             "util",
		Cmd:              "bin/util",
		Version:          "1.2.3",
		ArchiveExtension: ".tar.gz",
	}

	tests := map[string]struct {
		format       string
		wantResolved string
		wantErr      bool
	}{
		"simple version": {
			format:       "v{{.Version}}",
			wantResolved: "v1.2.3",
		},
		"full url template": {
			format:       "https://example.com/releases/v{{.Version}}/{{.Name}}_{{.GOOS}}_{{.GOARCH}}{{.ArchiveExtension}}",
			wantResolved: "https://example.com/releases/v1.2.3/util_linux_amd64.tar.gz",
		},
		"binary path with extension": {
			format:       "{{.Cmd}}{{.Extension}}",
			wantResolved: "bin/util",
		},
		"just a plain string": {
			format:       "no-templates-here",
			wantResolved: "no-templates-here",
		},
		"directory": {
			format:       "{{.Directory}}/subdir",
			wantResolved: "bin/subdir",
		},
		"invalid template syntax": {
			format:  "{{.Invalid",
			wantErr: true,
		},
		"unknown field": {
			format:  "{{.NonExistent}}",
			wantErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name,
			func(t *testing.T) {
				gotResolved, err := tmpl.Resolve(test.format)
				if test.wantErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, test.wantResolved, gotResolved)
			},
		)
	}
}

func TestTemplateResolveWithWindowsExtension(t *testing.T) {
	tmpl := Template{
		GOOS:      "windows",
		GOARCH:    "amd64",
		Name:      "util",
		Cmd:       "bin\\util.exe",
		Version:   "2.0.0",
		Extension: ".exe",
	}

	got, err := tmpl.Resolve("{{.Name}}_{{.Version}}_{{.GOOS}}_{{.GOARCH}}{{.Extension}}")

	require.NoError(t, err)
	assert.Equal(t, "util_2.0.0_windows_amd64.exe", got)
}

func TestTemplateMustResolve(t *testing.T) {
	t.Run("valid template",
		func(t *testing.T) {
			tmpl := Template{
				Version: "3.0.0",
			}

			got := tmpl.MustResolve("v{{.Version}}")
			assert.Equal(t, "v3.0.0", got)
		},
	)

	t.Run("panics on invalid template",
		func(t *testing.T) {
			tmpl := Template{}
			assert.Panics(t,
				func() {
					tmpl.MustResolve("{{.NonExistent}}")
				},
			)
		},
	)
}
