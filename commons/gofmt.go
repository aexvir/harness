package commons

import (
	"context"

	"github.com/kiwicom/harness"
)

// GoFmt runs gofmt and formats code in place.
func GoFmt() harness.Task {
	return func(ctx context.Context) error {
		return harness.Run(
			ctx,
			"gofmt",
			harness.WithArgs("-w", "-s", "."),
			harness.WithErrMsg("failed to format code"),
		)
	}
}
