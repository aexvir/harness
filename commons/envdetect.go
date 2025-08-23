package commons

import (
	"context"
	"os"
	"runtime"

	"github.com/aexvir/harness"
)

// OnlyOnCI returns the task specified as argument only in the case
// the current environment is a known ci system.
// Otherwise it returns a noop task.
func OnlyOnCI(task harness.Task) harness.Task {
	if !IsCIEnv() {
		return noop
	}

	return task
}

// OnlyLocally returns the task specified as argument only in the case
// the current environment is a dev machine.
// Otherwise it returns a noop task.
func OnlyLocally(task harness.Task) harness.Task {
	if IsCIEnv() {
		return noop
	}

	return task
}

// OnlyOnGOOS returns the task specified as argument only in the case
// the current OS is the specified GOOS.
// Otherwise it returns a noop task.
func OnlyOnGOOS(goos string, task harness.Task) harness.Task {
	if runtime.GOOS != goos {
		return noop
	}

	return task
}

// OnlyOnWindows returns the task specified as argument only in the case
// the current OS is Windows.
func OnlyOnWindows(task harness.Task) harness.Task {
	return OnlyOnGOOS("windows", task)
}

// OnlyOnLinux returns the task specified as argument only in the case
// the current OS is Linux.
func OnlyOnLinux(task harness.Task) harness.Task {
	return OnlyOnGOOS("linux", task)
}

// OnlyOnDarwin returns the task specified as argument only in the case
// the current OS is Darwin.
func OnlyOnDarwin(task harness.Task) harness.Task {
	return OnlyOnGOOS("darwin", task)
}

// IsCIEnv returns true if the current environment is a known ci system.
func IsCIEnv() bool {
	return os.Getenv("CI") != ""
}

func noop(ctx context.Context) error { return nil }
