package commons

import (
	"context"
	"fmt"

	"github.com/kiwicom/harness"
	"github.com/kiwicom/harness/bintool"
)

// GoImports formats code sorting imports taking in account the
// local package supplied as argument.
func GoImports(localpkg string) harness.Task {
	return func(ctx context.Context) error {
		imp, _ := bintool.NewGo(
			"golang.org/x/tools/cmd/goimports",
			"latest",
		)

		if err := imp.Ensure(); err != nil {
			return fmt.Errorf("failed to provision goimports: %w", err)
		}

		return harness.Run(ctx, imp.BinPath(), harness.WithArgs("-w", "-local", localpkg, "."))
	}
}
