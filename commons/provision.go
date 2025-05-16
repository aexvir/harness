package commons

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/aexvir/harness"
	"github.com/aexvir/harness/binary"
)

// Provision a list of binaries.
// Generates and executes a list of tasks where [Binary.Ensure] is called on each binary
// collecting and returning any errors encountered.
func Provision(binaries ...*binary.Binary) harness.Task {
	return func(ctx context.Context) (err error) {
		var errs []string
		start := time.Now()
		defer func() {
			elapsed := time.Since(start).Round(time.Millisecond)
			if err != nil {
				color.Red(" ✘ %s\n\n", elapsed)
				return
			}
			color.Green(" ✔ %s\n\n", elapsed)
		}()

		names := make([]string, 0, len(binaries))
		for _, bin := range binaries {
			names = append(names, bin.Name())
		}
		harness.LogStep(fmt.Sprintf("provisioning %d binaries: %s", len(binaries), strings.Join(names, ", ")))

		for _, bin := range binaries {
			if err := bin.Ensure(); err != nil {
				errs = append(errs, fmt.Sprintf("failed to provision %s: %s", bin.Name(), err))
			}
		}

		if len(errs) > 0 {
			for _, errmsg := range errs {
				color.Red(" • %s", errmsg)
			}
			return fmt.Errorf("provisioning failed")
		}

		return nil
	}
}
