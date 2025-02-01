package binary

import "fmt"

type Option func(b *Binary)

// WithGOOSMapping allows remapping the value of GOOS in the template
// before triggering the installation.
// This is useful for example in cases where a binary gets distributed as
// `binname-macos` and using the `binname-{{ GOOS }}` template with the default
// value would resolve to `binname-darwin` which doesn't exist.
// The key of the map is the GOOS value and the value is the wanted
// replacement; for the case mentioned earlier, pass {"darwin": "macos"}.
func WithGOOSMapping(mapping map[string]string) Option {
	return func(b *Binary) {
		if replacement, ok := mapping[b.template.GOOS]; ok {
			b.template.GOOS = replacement
		}
	}
}

// WithGOARCHMapping allows remapping the value of GOARCH in the template
// before triggering the installation.
// This is useful for example in cases where a binary gets distributed as
// `binname-aarch64` and using the `binname-{{ GOARCH }}` template with the default
// value would resolve to `binname-arm64` which doesn't exist.
// The key of the map is the GOARCH value and the value is the wanted
// replacement; for the case mentioned earlier, pass {"arm64": "aarch64"}.
func WithGOARCHMapping(mapping map[string]string) Option {
	return func(b *Binary) {
		if replacement, ok := mapping[b.template.GOARCH]; ok {
			b.template.GOARCH = replacement
		}
	}
}

func WithVersionCmd(format string) Option {
	return func(b *Binary) {
		if format == "" {
			b.versioncmd = ""
		} else {
			b.versioncmd = fmt.Sprintf(format, b.commandFullPath)
		}
	}
}
