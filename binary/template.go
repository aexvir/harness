package binary

import (
	"strings"
	"text/template"
)

type Template struct {
	GOOS   string
	GOARCH string

	Directory string
	Name      string
	Cmd       string
	Version   string
	Extension string
}

func (t Template) Resolve(format string) (string, error) {
	tmpl, err := template.New("bin").Parse(format)
	if err != nil {
		return "", err
	}

	var bld strings.Builder
	if err := tmpl.Execute(&bld, t); err != nil {
		return "", err
	}

	return bld.String(), nil
}
