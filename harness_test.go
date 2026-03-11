package harness

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// silent output when not verbose
func TestMain(m *testing.M) {
	flag.Parse()

	if !testing.Verbose() {
		SetOutput(io.Discard)
	}

	code := m.Run()
	os.Exit(code)
}

func TestHarnessExecute(t *testing.T) {
	t.Run("runs pre tasks and post in order",
		func(t *testing.T) {
			var order []string

			h := New(
				WithPreExecFunc(
					func(_ context.Context) error {
						order = append(order, "pre")
						return nil
					},
				),
				WithPostExecFunc(
					func(_ context.Context) error {
						order = append(order, "post")
						return nil
					},
				),
			)

			err := h.Execute(t.Context(),
				func(_ context.Context) error { order = append(order, "uno"); return nil },
				func(_ context.Context) error { order = append(order, "dos"); return nil },
				func(_ context.Context) error { order = append(order, "tres"); return nil },
			)

			require.NoError(t, err)
			assert.Equal(t, []string{"pre", "uno", "dos", "tres", "post"}, order)
		},
	)

	t.Run("fails when pre exec hook fails",
		func(t *testing.T) {
			h := New(
				WithPreExecFunc(
					func(_ context.Context) error {
						return errors.New("pre boom")
					},
				),
			)

			err := h.Execute(t.Context(), func(_ context.Context) error { return nil })

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to initialize ci harness")
			assert.Contains(t, err.Error(), "pre boom")
		},
	)

	t.Run("fails when post exec hook fails",
		func(t *testing.T) {
			h := New(
				WithPostExecFunc(
					func(_ context.Context) error {
						return errors.New("post boom")
					},
				),
			)

			err := h.Execute(t.Context(), func(_ context.Context) error { return nil })

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to run post exec hook")
			assert.Contains(t, err.Error(), "post boom")
		},
	)

	t.Run("collects task errors and returns final error",
		func(t *testing.T) {
			h := New()

			err := h.Execute(t.Context(),
				func(_ context.Context) error { return errors.New("first error") },
				func(_ context.Context) error { return nil },
				func(_ context.Context) error { return errors.New("second error") },
			)

			require.Error(t, err)
			assert.Equal(t, "task finished with errors", err.Error())
		},
	)

	t.Run("runs post hook even when tasks fail",
		func(t *testing.T) {
			called := false
			h := New(
				WithPostExecFunc(
					func(_ context.Context) error {
						called = true
						return nil
					},
				),
			)

			err := h.Execute(t.Context(),
				func(_ context.Context) error { return errors.New("task error") },
			)

			require.Error(t, err)
			assert.True(t, called)
		},
	)
}
