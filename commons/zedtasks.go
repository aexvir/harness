package commons

import (
	"github.com/aexvir/harness"
	"github.com/aexvir/harness/gen"
)

// GenerateZedTasks returns a [harness.Task] that generates a .zed/tasks.json
// file by discovering mage targets and merging them with any additional tasks.
//
// This is a convenience wrapper around [gen.ZedTasksFile].
func GenerateZedTasks(opts ...gen.ZedOption) harness.Task {
	return gen.ZedTasksFile(opts...)
}
