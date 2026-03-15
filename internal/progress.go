package internal

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// indeterminate tracks whether the progress bar is in indeterminate mode.
var indeterminate atomic.Bool

type oscProgressState uint8

const (
	oscProgressStateInactive      oscProgressState = 0
	oscProgressStateInProgress    oscProgressState = 1
	oscProgressStateError         oscProgressState = 2
	oscProgressStateIndeterminate oscProgressState = 3
)

type TaskProgressTracker struct {
	total     int
	completed int
	state     oscProgressState

	mtx  sync.RWMutex
	done chan bool
}

func NewTaskProgressTracker(ctx context.Context, amount int) *TaskProgressTracker {
	indeterminate.Store(false)

	tracker := &TaskProgressTracker{
		total:     amount,
		completed: 0,
		state:     oscProgressStateInProgress,
		done:      make(chan bool),
	}

	if amount <= 0 {
		// no task means no keepalive loop; Clear can still wait on done safely
		tracker.state = oscProgressStateInactive
		close(tracker.done)
		return tracker
	}

	if amount == 1 {
		tracker.state = oscProgressStateIndeterminate
	}

	go tracker.keepalive(ctx, 500*time.Millisecond)

	return tracker
}

// TaskFinished signals the task progress tracker that a task has finished and
// with what error. This will update the progress bar to show the right value and
// error status.
func (tracker *TaskProgressTracker) TaskFinished(err error) {
	tracker.mtx.Lock()
	defer tracker.mtx.Unlock()

	tracker.completed++

	if err != nil {
		tracker.state = oscProgressStateError
	}
}

// Clear the progress bar.
func (tracker *TaskProgressTracker) Clear() {
	<-tracker.done
	// intentional delay to allow the user to see the full progress bar
	// this should be a good balance between overhead and user experience
	time.Sleep(500 * time.Millisecond)
	emitOscCode(oscProgressStateInactive, 0)
}

// WithIndeterminateProgressbar sets the progress bar status to indeterminate.
// If there's an active task progress tracker, its progress bar will be paused for
// the duration of this function call.
func WithIndeterminateProgressbar(fn func() error) error {
	indeterminate.Store(true)
	defer indeterminate.Store(false)

	done := make(chan bool)
	defer close(done)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				emitOscCode(oscProgressStateIndeterminate, 0)
			case <-done:
				emitOscCode(oscProgressStateInactive, 0)
				return
			}
		}
	}()

	return fn()
}

// keepalive periodically emits osc 9;4 codes based on the current state of the tracker
// to avoid terminals clearing this when there's no enough updates on long running tasks
// https://ghostty.org/docs/vt/osc/conemu#change-progress-state-(osc-94)
func (tracker *TaskProgressTracker) keepalive(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer close(tracker.done)

	value := 0
	for {
		select {
		case <-ticker.C:
			tracker.mtx.RLock()
			completed, total := tracker.completed, tracker.total
			status := tracker.state
			tracker.mtx.RUnlock()

			// while there's an indeterminate progress bar running there should be no
			// updates for the task based progress tracking
			if indeterminate.Load() {
				continue
			}

			// calculate boundaries depending on the amount of completed tasks out of the total
			// e.g. for 3 completed out of 10, lower bound is 30 and upper bound is 39
			lower := completed * 100 / total
			upper := (completed+1)*100/total - 1

			// increase progress 1% every tick to provide some feedback that something is happening
			// waiting right below the upper boundary until the task is completed
			value = min(max(value+1, lower), upper)

			emitOscCode(status, value)

			if completed >= total {
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func emitOscCode(state oscProgressState, value int) {
	fmt.Fprintf(Output, "\x1b]9;4;%d;%d\x07", state, value) //nolint:errcheck
}
