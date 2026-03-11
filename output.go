package harness

import (
	"io"

	"github.com/aexvir/harness/internal"
)

// SetOutput sets where harness and binary logs are written.
func SetOutput(w io.Writer) {
	internal.SetOutput(w)
}
