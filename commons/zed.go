package commons

import (
	"github.com/aexvir/harness"
	"github.com/aexvir/harness/gen"
)

// ZedTasks generates a .zed/tasks.json file from mage targets for Zed editor integration.
// This is a convenience function that wraps gen.ZedTasks with sensible defaults.
//
// Users can customize the generation with options:
// - gen.WithZedOutputPath() to change the output location
// - gen.WithZedMageCommand() to specify the mage command path
// - gen.WithZedExtraTasks() to add custom tasks
// - gen.WithZedTaskPrefix() to change the task label prefix
func ZedTasks(opts ...gen.ZedTasksOpt) harness.Task {
	return gen.ZedTasks(opts...)
}
