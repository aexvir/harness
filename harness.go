package harness

import (
	"context"
	"fmt"
	"time"

	"github.com/aexvir/harness/internal"
)

// Harness is a support structure that runs tasks, the harness can be customized with
// pre- and post- execution hook functions, where common functionality to all tasks
// can be defined.
type Harness struct {
	PreExecHook  Task
	PostExecHook Task
}

// New constructs a harness.
func New(opts ...Option) *Harness {
	h := Harness{
		PreExecHook:  func(_ context.Context) error { return nil },
		PostExecHook: func(_ context.Context) error { return nil },
	}

	for _, opt := range opts {
		opt(&h)
	}

	return &h
}

// Execute a list of tasks inside the harness.
// Every task inside the harness is run sequentially, showing a consistent output where
// the task status and timing info are clearly visible.
func (h *Harness) Execute(ctx context.Context, tasks ...Task) error {
	var errs []string
	start := time.Now()

	internal.LogBlank()

	if err := h.PreExecHook(ctx); err != nil {
		return fmt.Errorf("failed to initialize ci harness: %s", err.Error())
	}

	for i := range tasks {
		task := tasks[i]
		if err := task(ctx); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if err := h.PostExecHook(ctx); err != nil {
		return fmt.Errorf("failed to run post exec hook: %s", err.Error())
	}

	elapsed := time.Since(start).Round(time.Millisecond)
	internal.LogSeparator()

	if len(errs) > 0 {
		internal.LogError(fmt.Sprintf("finished with errors after %s", elapsed))
		for _, errmsg := range errs {
			internal.LogErrorItem(errmsg)
		}
		internal.LogBlank()
		return fmt.Errorf("task finished with errors")
	}

	internal.LogSuccess(fmt.Sprintf("all good after %s", elapsed))
	internal.LogBlank()
	return nil
}

// Logs the name of a task step.
// Harness.Run automatically uses this function to print what is running,
// this is mainly useful for adding additional info when defining ad-hoc tasks inside
// a Harness.Exec block.
func LogStep(text string) {
	internal.LogCommand(text)
}

// Task defines the basic function that the harness executes.
// Additional configuration and tweaks can be done by using clojures which return
// Tasks.
type Task func(ctx context.Context) error

type Option func(h *Harness)

// WithPreExecFunc allows specifying a [Task] that will be run every execution, **before** the
// specific execution tasks are run.
func WithPreExecFunc(hook Task) Option {
	return func(h *Harness) {
		h.PreExecHook = hook
	}
}

// WithPostExecFunc allows specifying a [Task] that will be run every execution, **after** the
// specific execution tasks are run.
func WithPostExecFunc(hook Task) Option {
	return func(h *Harness) {
		h.PostExecHook = hook
	}
}
