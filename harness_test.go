package harness

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func succeedingTask() Task {
	return func(_ context.Context) error { return nil }
}

func failingTask(msg string) Task {
	return func(_ context.Context) error { return errors.New(msg) }
}

func recordingTask(called *bool) Task {
	return func(_ context.Context) error {
		*called = true
		return nil
	}
}

func orderTracker(order *[]string, name string) Task {
	return func(_ context.Context) error {
		*order = append(*order, name)
		return nil
	}
}

// TestNew verifies the Harness constructor and options.
func TestNew(t *testing.T) {
	t.Run("defaults are non-nil no-ops", func(t *testing.T) {
		h := New()
		require.NotNil(t, h)
		require.NotNil(t, h.PreExecHook)
		require.NotNil(t, h.PostExecHook)

		assert.NoError(t, h.PreExecHook(context.Background()))
		assert.NoError(t, h.PostExecHook(context.Background()))
	})

	t.Run("WithPreExecFunc sets the hook", func(t *testing.T) {
		called := false
		h := New(WithPreExecFunc(func(_ context.Context) error {
			called = true
			return nil
		}))

		err := h.PreExecHook(context.Background())
		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("post-exec hook can be set directly", func(t *testing.T) {
		called := false
		h := New()
		h.PostExecHook = func(_ context.Context) error {
			called = true
			return nil
		}

		err := h.PostExecHook(context.Background())
		assert.NoError(t, err)
		assert.True(t, called)
	})
}

// TestExecute verifies task execution behavior.
func TestExecute(t *testing.T) {
	ctx := context.Background()

	t.Run("no tasks succeeds", func(t *testing.T) {
		h := New()
		err := h.Execute(ctx)
		assert.NoError(t, err)
	})

	t.Run("single succeeding task", func(t *testing.T) {
		h := New()
		err := h.Execute(ctx, succeedingTask())
		assert.NoError(t, err)
	})

	t.Run("single failing task", func(t *testing.T) {
		h := New()
		err := h.Execute(ctx, failingTask("boom"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task finished with errors")
	})

	t.Run("multiple tasks all succeed", func(t *testing.T) {
		h := New()
		err := h.Execute(ctx, succeedingTask(), succeedingTask(), succeedingTask())
		assert.NoError(t, err)
	})

	t.Run("multiple tasks one fails but all still run", func(t *testing.T) {
		called1 := false
		called2 := false
		called3 := false

		h := New()
		err := h.Execute(ctx,
			recordingTask(&called1),
			failingTask("task2 failed"),
			recordingTask(&called2),
			failingTask("task4 failed"),
			recordingTask(&called3),
		)

		require.Error(t, err)
		assert.True(t, called1, "task1 should have run")
		assert.True(t, called2, "task3 should have run despite task2 failing")
		assert.True(t, called3, "task5 should have run despite task4 failing")
	})

	t.Run("pre-exec hook failure prevents tasks from running", func(t *testing.T) {
		taskCalled := false
		h := New(WithPreExecFunc(failingTask("pre-exec failed")))

		err := h.Execute(ctx, recordingTask(&taskCalled))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to initialize ci harness")
		assert.Contains(t, err.Error(), "pre-exec failed")
		assert.False(t, taskCalled, "tasks should not run when pre-exec hook fails")
	})

	t.Run("post-exec hook failure returns error", func(t *testing.T) {
		h := New()
		h.PostExecHook = failingTask("post-exec failed")

		err := h.Execute(ctx, succeedingTask())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to run post exec hook")
		assert.Contains(t, err.Error(), "post-exec failed")
	})

	t.Run("execution order is pre-hook then tasks then post-hook", func(t *testing.T) {
		var order []string

		h := New(WithPreExecFunc(orderTracker(&order, "pre")))
		h.PostExecHook = orderTracker(&order, "post")

		err := h.Execute(ctx,
			orderTracker(&order, "task1"),
			orderTracker(&order, "task2"),
			orderTracker(&order, "task3"),
		)

		assert.NoError(t, err)
		assert.Equal(t, []string{"pre", "task1", "task2", "task3", "post"}, order)
	})

	t.Run("post-exec hook runs even when tasks fail", func(t *testing.T) {
		postCalled := false
		h := New()
		h.PostExecHook = recordingTask(&postCalled)

		err := h.Execute(ctx, failingTask("boom"))

		require.Error(t, err)
		assert.True(t, postCalled, "post-exec hook should run even when tasks fail")
	})

	t.Run("context values are passed through to tasks", func(t *testing.T) {
		type ctxKey string
		key := ctxKey("testkey")
		ctx := context.WithValue(context.Background(), key, "testvalue")

		var receivedValue interface{}
		task := func(ctx context.Context) error {
			receivedValue = ctx.Value(key)
			return nil
		}

		h := New()
		err := h.Execute(ctx, task)

		assert.NoError(t, err)
		assert.Equal(t, "testvalue", receivedValue)
	})

	t.Run("post-exec hook error takes precedence over task errors", func(t *testing.T) {
		h := New()
		h.PostExecHook = failingTask("post failed")

		err := h.Execute(ctx, failingTask("task failed"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to run post exec hook")
	})
}

// TestLogStep verifies step logging output.
func TestLogStep(t *testing.T) {
	t.Run("prints step text to stdout", func(t *testing.T) {
		oldStdout := os.Stdout
		r, w, err := os.Pipe()
		require.NoError(t, err)

		os.Stdout = w
		LogStep("my test step")
		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, err = buf.ReadFrom(r)
		require.NoError(t, err)

		assert.Contains(t, buf.String(), "my test step")
	})
}
