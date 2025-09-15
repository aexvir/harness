package harness

import (
	"fmt"
	"io"
	"os"
)

// ProgressState represents the different states for OSC 9;4 progress reporting.
type ProgressState int

const (
	// ProgressClear clears the progress indicator (state 0).
	ProgressClear ProgressState = 0
	// ProgressIndeterminate shows indeterminate progress (state 1).
	ProgressIndeterminate ProgressState = 1
	// ProgressNormal shows normal progress with percentage (state 2).
	ProgressNormal ProgressState = 2
)

// ProgressReporter handles OSC 9;4 progress reporting to compatible terminals.
// OSC 9;4 allows applications to report progress that can be displayed in
// terminal title bars, taskbars, or other progress indicators.
type ProgressReporter struct {
	writer  io.Writer
	enabled bool
}

// NewProgressReporter creates a new progress reporter that outputs to the given writer.
// If writer is nil, it defaults to os.Stderr.
func NewProgressReporter(writer io.Writer) *ProgressReporter {
	if writer == nil {
		writer = os.Stderr
	}

	return &ProgressReporter{
		writer:  writer,
		enabled: true,
	}
}

// SetEnabled enables or disables progress reporting.
func (p *ProgressReporter) SetEnabled(enabled bool) {
	p.enabled = enabled
}

// IsEnabled returns whether progress reporting is currently enabled.
func (p *ProgressReporter) IsEnabled() bool {
	return p.enabled
}

// Report sends an OSC 9;4 progress report with the specified state and progress value.
// For ProgressClear and ProgressIndeterminate states, the progress value is ignored.
// For ProgressNormal state, progress should be between 0 and 100.
func (p *ProgressReporter) Report(state ProgressState, progress int) {
	if !p.enabled {
		return
	}

	// Clamp progress to valid range
	if progress < 0 {
		progress = 0
	} else if progress > 100 {
		progress = 100
	}

	// Format: \033]9;4;state;progress\033\\
	fmt.Fprintf(p.writer, "\033]9;4;%d;%d\033\\", int(state), progress)
}

// Clear clears the progress indicator (equivalent to Report(ProgressClear, 0)).
func (p *ProgressReporter) Clear() {
	p.Report(ProgressClear, 0)
}

// ShowIndeterminate shows indeterminate progress (equivalent to Report(ProgressIndeterminate, 0)).
func (p *ProgressReporter) ShowIndeterminate() {
	p.Report(ProgressIndeterminate, 0)
}

// ShowProgress shows normal progress with the specified percentage.
// Progress is clamped to the range 0-100.
func (p *ProgressReporter) ShowProgress(progress int) {
	p.Report(ProgressNormal, progress)
}

// DefaultProgressReporter is a global progress reporter instance that outputs to os.Stderr.
var DefaultProgressReporter = NewProgressReporter(nil)

// Progress convenience functions that use the DefaultProgressReporter

// ShowProgress shows normal progress with the specified percentage using the default reporter.
func ShowProgress(progress int) {
	DefaultProgressReporter.ShowProgress(progress)
}

// ShowIndeterminate shows indeterminate progress using the default reporter.
func ShowIndeterminate() {
	DefaultProgressReporter.ShowIndeterminate()
}

// ClearProgress clears the progress indicator using the default reporter.
func ClearProgress() {
	DefaultProgressReporter.Clear()
}

// SetProgressEnabled enables or disables the default progress reporter.
func SetProgressEnabled(enabled bool) {
	DefaultProgressReporter.SetEnabled(enabled)
}

// IsProgressEnabled returns whether the default progress reporter is enabled.
func IsProgressEnabled() bool {
	return DefaultProgressReporter.IsEnabled()
}
