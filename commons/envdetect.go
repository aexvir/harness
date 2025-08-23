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

// OnlyOnWindows returns the task specified as argument only in the case
// the current OS Windows.
// Otherwise it returns a noop task.
func OnlyOnWindows(task harness.Task) harness.Task {
	if !IsWindows() {
		return noop
	}

	return task
}

// IsCIEnv returns true if the current environment is a known ci system.
func IsCIEnv() bool {
	return os.Getenv("CI") != ""
}

// IsWindows returns true if the current environment is Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func noop(ctx context.Context) error { return nil }
