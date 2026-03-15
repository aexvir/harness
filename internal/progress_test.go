package internal

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	taskTickInterval = 500 * time.Millisecond
)

func TestTaskProgressTracker(t *testing.T) {
	t.Run("single task emits indeterminate ticks and clears",
		func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				buf := installOutputCapture(t)

				ctx, cancel := context.WithCancel(t.Context())
				tracker := NewTaskProgressTracker(ctx, 1)
				synctest.Wait()

				assertOscEvents(t, buf)

				advance(t, taskTickInterval)
				assertOscEvents(t, buf,
					oscevent{state: 3, value: 1},
				)

				advance(t, taskTickInterval)
				assertOscEvents(t, buf,
					oscevent{state: 3, value: 1},
					oscevent{state: 3, value: 2},
				)

				cancel()
				synctest.Wait()
				buf.Reset()

				tracker.Clear()
				synctest.Wait()
				assertOscEvents(t, buf,
					oscevent{state: 0, value: 0},
				)
			})
		},
	)

	t.Run("two tasks transition to completion",
		func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				buf := installOutputCapture(t)

				tracker := NewTaskProgressTracker(t.Context(), 2)
				synctest.Wait()

				advance(t, taskTickInterval)
				tracker.TaskFinished(nil)

				advance(t, taskTickInterval)
				tracker.TaskFinished(nil)

				advance(t, taskTickInterval)
				assertOscEvents(t, buf,
					oscevent{state: 1, value: 1},
					oscevent{state: 1, value: 50},
					oscevent{state: 1, value: 100},
				)

				buf.Reset()
				tracker.Clear()
				synctest.Wait()
				assertOscEvents(t, buf,
					oscevent{state: 0, value: 0},
				)
			})
		},
	)

	t.Run("error state is sticky",
		func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				buf := installOutputCapture(t)

				tracker := NewTaskProgressTracker(t.Context(), 2)
				synctest.Wait()

				advance(t, taskTickInterval)
				tracker.TaskFinished(errors.New("failed"))

				advance(t, taskTickInterval)
				tracker.TaskFinished(nil)

				advance(t, taskTickInterval)
				assertOscEvents(t, buf,
					oscevent{state: 1, value: 1},
					oscevent{state: 2, value: 50},
					oscevent{state: 2, value: 100},
				)

				buf.Reset()
				tracker.Clear()
				synctest.Wait()
				assertOscEvents(t, buf,
					oscevent{state: 0, value: 0},
				)
			})
		},
	)

	t.Run("all tasks done before first tick emits completion",
		func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				buf := installOutputCapture(t)

				tracker := NewTaskProgressTracker(t.Context(), 2)
				synctest.Wait()

				tracker.TaskFinished(nil)
				tracker.TaskFinished(nil)

				advance(t, taskTickInterval)
				assertOscEvents(t, buf,
					oscevent{state: 1, value: 100},
				)

				buf.Reset()
				tracker.Clear()
				synctest.Wait()
				assertOscEvents(t, buf,
					oscevent{state: 0, value: 0},
				)
			})
		},
	)

	t.Run("value clamps at upper boundary",
		func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				buf := installOutputCapture(t)

				tracker := NewTaskProgressTracker(t.Context(), 2)
				synctest.Wait()

				for range 55 {
					advance(t, taskTickInterval)
				}

				events := readOscEvents(t, buf)
				require.Len(t, events, 55)

				for i := 1; i <= 49; i++ {
					assert.Equal(t, oscevent{state: 1, value: i}, events[i-1])
				}

				for i := 49; i < len(events); i++ {
					assert.Equal(t, oscevent{state: 1, value: 49}, events[i])
				}

				tracker.TaskFinished(nil)
				buf.Reset()

				advance(t, taskTickInterval)
				assertOscEvents(t, buf,
					oscevent{state: 1, value: 50},
				)

				tracker.TaskFinished(nil)
				advance(t, taskTickInterval)

				buf.Reset()
				tracker.Clear()
				synctest.Wait()
				assertOscEvents(t, buf,
					oscevent{state: 0, value: 0},
				)
			})
		},
	)

	t.Run("context cancellation stops keepalive",
		func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				buf := installOutputCapture(t)

				ctx, cancel := context.WithCancel(t.Context())
				tracker := NewTaskProgressTracker(ctx, 3)
				synctest.Wait()

				advance(t, taskTickInterval)
				tracker.TaskFinished(nil)

				advance(t, taskTickInterval)
				assertOscEvents(t, buf,
					oscevent{state: 1, value: 1},
					oscevent{state: 1, value: 33},
				)

				cancel()
				synctest.Wait()

				snapshot := readOscEvents(t, buf)
				advance(t, 2*time.Second)
				assert.Equal(t, snapshot, readOscEvents(t, buf), "no new output after cancellation")

				buf.Reset()
				tracker.Clear()
				synctest.Wait()
				assertOscEvents(t, buf,
					oscevent{state: 0, value: 0},
				)
			})
		},
	)

	t.Run("zero tasks is safe and clears",
		func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				buf := installOutputCapture(t)

				tracker := NewTaskProgressTracker(t.Context(), 0)
				synctest.Wait()

				advance(t, 3*taskTickInterval)
				assertOscEvents(t, buf)

				buf.Reset()
				tracker.Clear()
				synctest.Wait()
				assertOscEvents(t, buf,
					oscevent{state: 0, value: 0},
				)
			})
		},
	)
}

func TestWithIndeterminateProgressbar(t *testing.T) {
	t.Run("emits while running and clears on exit", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			buf := installOutputCapture(t)

			err := WithIndeterminateProgressbar(func() error {
				time.Sleep(2500 * time.Millisecond)
				return nil
			})
			synctest.Wait()

			require.NoError(t, err)
			assertOscEvents(t, buf,
				oscevent{state: 3, value: 0},
				oscevent{state: 3, value: 0},
				oscevent{state: 0, value: 0},
			)
		})
	})

	t.Run("propagates fn error", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			buf := installOutputCapture(t)

			sentinel := errors.New("fn failed")
			err := WithIndeterminateProgressbar(func() error {
				time.Sleep(500 * time.Millisecond)
				return sentinel
			})
			synctest.Wait()

			require.ErrorIs(t, err, sentinel)
			assertOscEvents(t, buf,
				oscevent{state: 0, value: 0},
			)
		})
	})

	t.Run("pauses active tracker then resumes", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			buf := installOutputCapture(t)

			tracker := NewTaskProgressTracker(t.Context(), 2)
			synctest.Wait()

			advance(t, taskTickInterval)
			assertOscEvents(t, buf,
				oscevent{state: 1, value: 1},
			)
			buf.Reset()

			err := WithIndeterminateProgressbar(
				func() error {
					time.Sleep(2100 * time.Millisecond)
					return nil
				},
			)
			synctest.Wait()

			require.NoError(t, err)
			assertOscEvents(t, buf,
				oscevent{state: 3, value: 0},
				oscevent{state: 3, value: 0},
				oscevent{state: 0, value: 0},
			)

			buf.Reset()
			advance(t, taskTickInterval)
			assertOscEvents(t, buf,
				oscevent{state: 1, value: 2},
			)

			tracker.TaskFinished(nil)
			tracker.TaskFinished(nil)
			advance(t, taskTickInterval)

			buf.Reset()
			tracker.Clear()
			synctest.Wait()
			assertOscEvents(t, buf,
				oscevent{state: 0, value: 0},
			)
		})
	})
}

type oscevent struct {
	state int
	value int
}

// syncbuffer is a thread-safe bytes.Buffer for capturing output in tests
// where multiple goroutines write concurrently.
type syncbuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncbuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncbuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *syncbuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

func installOutputCapture(t *testing.T) *syncbuffer {
	t.Helper()

	prev := Output
	buf := &syncbuffer{}
	Output = buf

	t.Cleanup(
		func() {
			Output = prev
		},
	)

	return buf
}

func advance(t *testing.T, d time.Duration) {
	t.Helper()
	time.Sleep(d)
	synctest.Wait()
}

func readOscEvents(t *testing.T, buf *syncbuffer) []oscevent {
	t.Helper()

	raw := buf.String()
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, "\x07")
	events := make([]oscevent, 0, len(parts))

	for _, part := range parts {
		if part == "" {
			continue
		}

		require.True(t, strings.HasPrefix(part, "\x1b]9;4;"), "unexpected OSC prefix: %q", part)

		fields := strings.Split(strings.TrimPrefix(part, "\x1b]9;4;"), ";")
		require.Len(t, fields, 2, "unexpected OSC payload: %q", part)

		state, err := strconv.Atoi(fields[0])
		require.NoError(t, err)

		value, err := strconv.Atoi(fields[1])
		require.NoError(t, err)

		events = append(events, oscevent{state: state, value: value})
	}

	return events
}

func assertOscEvents(t *testing.T, buf *syncbuffer, want ...oscevent) {
	t.Helper()
	assert.Equal(t, want, readOscEvents(t, buf))
}
