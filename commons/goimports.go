package commons

import (
	"context"
	"fmt"

	"github.com/aexvir/harness"
	"github.com/aexvir/harness/bintool"
)

// GoImports formats code sorting imports taking in account the
// local package supplied as argument.
func GoImports(localpkg string, opts ...GoImportsOpt) harness.Task {
	conf := goimportsconf{
		version: "latest",
	}

	for _, opt := range opts {
		opt(&conf)
	}

	return func(ctx context.Context) error {
		imp, _ := bintool.NewGo(
			"golang.org/x/tools/cmd/goimports",
			conf.version,
		)

		if err := imp.Ensure(); err != nil {
			return fmt.Errorf("failed to provision goimports: %w", err)
		}

		return harness.Run(ctx, imp.BinPath(), harness.WithArgs("-w", "-local", localpkg, "."))
	}
}

type goimportsconf struct {
	version string
}

type GoImportsOpt func(c *goimportsconf)

// WithGoImportsVersion allows specifying the goimports version
// that should be used when running this task.
func WithGoImportsVersion(version string) GoImportsOpt {
	return func(c *goimportsconf) {
		c.version = version
	}
}
