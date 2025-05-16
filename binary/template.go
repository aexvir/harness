package binary

import (
	"strings"
	"text/template"
)

// Template contains fields used to resolve specific metadata about the binary.
// It includes system architecture information, binary location details, and version information.
type Template struct {
	// GOOS is the operating system target (e.g., "linux", "darwin", "windows")
	GOOS string
	// GOARCH is the architecture target (e.g., "amd64", "arm64")
	GOARCH string

	// Directory where the binary is located
	Directory string
	// Name of the binary
	Name string
	// Cmd is the command to execute, typically the qualified path to the binary
	Cmd string
	// Version is the semantic version string
	Version string
	// Extension is the file extension for the binary.
	// Usually it's empty on unix systems and ".exe" on windows.
	Extension string
}

// Resolve executes the provided format string as a template with the Template's fields.
// It returns the resolved string and any error that occurred during template parsing or execution.
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

// Resolve executes the provided format string as a template with the Template's fields.
// Panics if the template can't be resolved correctly.
func (t Template) MustResolve(format string) string {
	resolved, err := t.Resolve(format)
	if err != nil {
		panic(err)
	}
	return resolved
}
