// Example demonstrating OSC 9;4 progress reporting with Harness.
//
// OSC 9;4 is supported by terminals like Windows Terminal, iTerm2, and others
// that can display progress in taskbars, title bars, or other UI elements.
//
// To run: go run examples/osc-progress/main.go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aexvir/harness"
	"github.com/aexvir/harness/commons"
)

func main() {
	ctx := context.Background()

	fmt.Println("🚀 OSC 9;4 Progress Reporting Example")
	fmt.Println("====================================")
	fmt.Println("")

	// Example 1: Manual progress reporting
	fmt.Println("📊 Manual progress reporting...")
	harness.ShowIndeterminate()
	fmt.Println("Indeterminate progress for 2 seconds...")
	time.Sleep(2 * time.Second)

	for i := 0; i <= 100; i += 10 {
		harness.ShowProgress(i)
		fmt.Printf("Progress: %d%%\n", i)
		time.Sleep(300 * time.Millisecond)
	}

	harness.ClearProgress()
	fmt.Println("Progress cleared!")
	fmt.Println("")

	// Example 2: Harness integration
	fmt.Println("🔨 Harness integration with progress reporting...")
	h := harness.New(harness.WithProgressReporting(true))

	err := h.Execute(ctx,
		func(ctx context.Context) error {
			fmt.Println("Task 1: Checking Go version")
			return harness.Run(ctx, "go", harness.WithArgs("version"))
		},
		func(ctx context.Context) error {
			fmt.Println("Task 2: Running go mod tidy")
			return commons.GoModTidy()(ctx)
		},
		func(ctx context.Context) error {
			fmt.Println("Task 3: Building project")
			return harness.Run(ctx, "go", harness.WithArgs("build", "./..."))
		},
		func(ctx context.Context) error {
			fmt.Println("Task 4: Simulating work")
			time.Sleep(1 * time.Second)
			return nil
		},
	)

	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	fmt.Println("🎉 Example complete!")
	fmt.Println("")
	fmt.Println("ℹ️  OSC 9;4 sequences were sent to stderr.")
	fmt.Println("ℹ️  Compatible terminals may show progress in taskbar/title.")
	fmt.Println("")
	fmt.Println("Supported terminals:")
	fmt.Println("  • Windows Terminal")
	fmt.Println("  • iTerm2 (macOS)")
	fmt.Println("  • Some other modern terminals")
}
