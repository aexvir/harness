package commons

import (
	"bytes"
	"context"
	"errors"
	"os"

	"github.com/kiwicom/harness"
)

// GoModTidy runs go mod tidy and errors if the go.mod or go.sum files have changed.
func GoModTidy() harness.Task {
	return func(ctx context.Context) error {
		gomod, _ := os.ReadFile("go.mod")
		gosum, _ := os.ReadFile("go.sum")

		err := harness.Run(ctx, "go", harness.WithArgs("mod", "tidy", "-v"))
		if err != nil {
			return err
		}

		newmod, _ := os.ReadFile("go.mod")
		newsum, _ := os.ReadFile("go.sum")

		if !bytes.Equal(gomod, newmod) || !bytes.Equal(gosum, newsum) {
			return errors.New("differences found; fixed go module")
		}

		return nil
	}
}
