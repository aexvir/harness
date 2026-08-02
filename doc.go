// Package harness provides a structured framework for building Go build automation
// scripts, primarily designed for use with Mage (https://magefile.org/).
//
// Harness eliminates the need to build CI/CD and development automation from scratch
// by providing reusable primitives, consistent task execution, binary provisioning,
// and pre-built common development tasks.
//
// Key features:
//
//	- Task Execution: Sequential execution of tasks with consistent output formatting,
//	  timing, and status indicators.
//	- Binary Management: Automatic provisioning and version checking of external tools
//	  via the binary package.
//	- Commons: Pre-built tasks for common Go workflows (formatting, linting, testing, etc.).
//	- Environment Detection: Utilities to run tasks conditionally based on CI environment
//	  or operating system.
//
// Harness is intended to be consumed as a library by other Go projects to create their
// build automation, typically within a magefile. It is NOT a standalone application.
//
// Basic Usage:
//
//	// Create a new harness instance
//	h := harness.New()
//
//	// Define a task function (compatible with mage)
//	func Format(ctx context.Context) error {
//		return h.Execute(ctx,
//			commons.GoFmt(),
//			commons.GoImports("my/module"),
//		)
//	}
//
// Advanced Usage with Hooks and Binary Provisioning:
//
//	// Define a binary using the binary package
//	tool := binary.New(
//		"mytool",
//		"1.0.0",
//		binary.RemoteBinaryDownload("https://example.com/mytool"),
//	)
//
//	// Ensure the tool is installed
//	h := harness.New(harness.WithPreExecFunc(func(ctx context.Context) error {
//		return tool.Ensure()
//	}))
//
//	// Execute tasks sequentially
//	return h.Execute(ctx,
//		func(ctx context.Context) error {
//			return harness.Run(ctx, tool.BinPath(), harness.WithArgs("--version"))
//		},
//		commons.GoTest(),
//	)
//
// See the binary and commons packages for additional functionality and configuration options.
package harness
