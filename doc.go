// Package harness provides a set of primitives that can be used to build a magefile
// without having to set everything from the ground up.
//
// OSC 9;4 Progress Reporting
//
// Harness supports OSC 9;4 progress reporting for compatible terminals (Windows Terminal,
// iTerm2, etc.) which can display progress in taskbars or title bars. Progress reporting
// is automatically integrated into harness task execution and can be controlled via options.
//
// Basic usage:
//
//	h := harness.New(harness.WithProgressReporting(true))
//	h.Execute(ctx, task1, task2, task3) // Progress automatically reported
//
// Manual progress reporting:
//
//	harness.ShowIndeterminate()        // Show spinning progress
//	harness.ShowProgress(50)           // Show 50% progress
//	harness.ClearProgress()            // Clear progress indicator
//
// The OSC 9;4 escape sequence format is: \033]9;4;state;progress\033\\
// where state is 0 (clear), 1 (indeterminate), or 2 (normal), and progress is 0-100.
package harness
