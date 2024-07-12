package commons

import (
	"context"
	"fmt"
	"strings"

	"github.com/aexvir/harness"
)

// GoBuild builds a go binary, from the package specified as argument, outputting it on the relative path
// supplied as argument.
// The go build command can be customized with build tags and ldflags via GoBuildOpt arguments.
func GoBuild(pkg, out string, opts ...GoBuildOpt) harness.Task {
	var conf buildconf

	for _, opt := range opts {
		opt(&conf)
	}

	return func(ctx context.Context) error {
		args := []string{"build", "-o", out}

		if len(conf.tags) > 0 {
			args = append(args, "-tags", strings.Join(conf.tags, " "))
		}

		if len(conf.ldflags) > 0 {
			flags := make([]string, 0, len(conf.ldflags))
			for _, flag := range conf.ldflags {
				flags = append(flags, fmt.Sprintf("-X '%s'", flag))
			}
			args = append(args, "-ldflags", strings.Join(flags, " "))
		}

		args = append(args, pkg)

		return harness.Run(ctx, "go", harness.WithArgs(args...))
	}
}

type buildconf struct {
	tags    []string
	ldflags []string
}

type GoBuildOpt func(c *buildconf)

// WithGoBuildTags allows specifying build tags for the go build command.
func WithGoBuildTags(tags ...string) GoBuildOpt {
	return func(c *buildconf) {
		c.tags = tags
	}
}

// WithGoBuildLDFlags allows specifying ldflags for the go build command.
func WithGoBuildLDFlags(flags ...string) GoBuildOpt {
	return func(c *buildconf) {
		c.ldflags = flags
	}
}
