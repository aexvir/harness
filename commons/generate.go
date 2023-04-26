package commons

import (
	"context"

	"github.com/kiwicom/harness"
)

// GoGenerate runs go generate recursively.
func GoGenerate() harness.Task {
	return func(ctx context.Context) error {
		return harness.Run(ctx, "go", harness.WithArgs("generate", "-x", "./..."))
	}
}
