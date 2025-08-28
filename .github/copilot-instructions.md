# Harness - Go Build Automation Library

Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.

**For comprehensive project overview and architecture**: See [`agents.md`](../agents.md) for detailed information about the Harness framework, key concepts, project structure, and core components.

Harness is a Go library that provides primitives for building magefiles (automation scripts). It includes task runners, binary provisioning, and common Go development tasks. This is NOT a standalone application - it's a library consumed by other Go projects to create their build automation.

## Working Effectively

### Prerequisites
- Go 1.20 or later (check with `go version`)
- Install mage build tool: `go install github.com/magefile/mage@latest`

### Setup and Build Commands
- Download dependencies: `go mod download` - takes ~5 seconds
- Build the library: `go build ./...` - takes ~5 seconds, validates library compiles correctly
- Test library functionality: Create and run a test program using harness (see Validation section)

### Mage Build System Commands
The repository uses mage for automation. Access mage via: `~/go/bin/mage` or add `~/go/bin` to PATH.

**CRITICAL TIMEOUT REQUIREMENTS**: NEVER CANCEL the following commands. Set timeouts as specified.

- `~/go/bin/mage tidy` - runs go mod tidy - takes ~0.5 seconds. Set timeout to 2+ minutes.
- `~/go/bin/mage format` - formats code with gofmt and installs/runs goimports - takes ~5 seconds. Set timeout to 3+ minutes.
- `~/go/bin/mage lint` - runs go mod tidy, commitsar (git commit linting), and golangci-lint - takes ~65 seconds including binary downloads. Set timeout to 10+ minutes.
- `~/go/bin/mage test` - runs go test with coverage and junit output - takes ~27 seconds but currently FAILS due to Printf formatting issue. Set timeout to 5+ minutes.

**KNOWN ISSUE**: Tests and lint currently fail due to a Printf formatting directive issue in `binary/origin.go:286`. The error is:
```
binary/origin.go:286:5: (*github.com/fatih/color.Color).Sprint call has possible Printf formatting directive %s
```
This prevents `mage test` and `mage lint` from passing, but the library functionality works correctly.

### List Available Mage Targets
- `~/go/bin/mage -l` - shows all available build targets

## Validation

Always manually validate harness library functionality after making changes:

```go
package main

import (
	"context"
	"log"
	"github.com/aexvir/harness"
	"github.com/aexvir/harness/commons"
)

func main() {
	ctx := context.Background()
	h := harness.New()
	
	// Test basic command execution
	err := h.Execute(ctx, func(ctx context.Context) error {
		return harness.Run(ctx, "echo", harness.WithArgs("✓ Harness works!"))
	})
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}
	
	// Test go mod tidy task
	err = h.Execute(ctx, commons.GoModTidy())
	if err != nil {
		log.Fatalf("Go mod tidy failed: %v", err)
	}
	
	// Test multiple tasks
	err = h.Execute(ctx,
		commons.GoFmt(),
		func(ctx context.Context) error {
			return harness.Run(ctx, "go", harness.WithArgs("build", "./..."))
		},
	)
	if err != nil {
		log.Fatalf("Multiple tasks failed: %v", err)
	}
}
```

**REQUIRED VALIDATION STEPS**:
1. Save validation code to `/tmp/test_harness.go`
2. Run: `cd /path/to/harness && go run /tmp/test_harness.go`
3. Verify output shows green checkmarks and "all good" messages
4. Ensure no error messages appear

## Pre-commit Validation
Always run these commands before committing changes:
- `~/go/bin/mage tidy` - clean up go.mod
- `~/go/bin/mage format` - format code properly
- `go build ./...` - verify library builds (workaround for test failures)
- `go vet ./...` - will show the known Printf issue but proceed anyway
- Run manual validation test program (see Validation section)

## Library Structure

**For detailed project structure and core components**: See [`agents.md`](../agents.md) which provides comprehensive information about:
- Harness framework architecture
- Task execution model 
- Binary management system
- Commons package capabilities
- Complete project structure overview

### Key Files for Development
- `harness.go` - Core harness functionality, task execution
- `runner.go` - Command execution utilities  
- `magefiles/main.go` - Build automation definitions
- `commons/` - Ready-to-use common tasks for Go projects

### No Test Files
This library has no *_test.go files. Testing is done by consuming projects and manual validation programs.

## Common Tasks

### Understanding Library Usage
This library is consumed by other Go projects in their `magefiles/main.go`:

```go
//go:build mage

package main

import (
	"context"
	"github.com/aexvir/harness"
	"github.com/aexvir/harness/commons"
)

func Test(ctx context.Context) error {
	h := harness.New()
	return h.Execute(ctx, commons.GoTest())
}

func Format(ctx context.Context) error {
	h := harness.New()
	return h.Execute(ctx, commons.GoFmt(), commons.GoImports("pkg/path"))
}
```

### Generated Files (Automatically Ignored)
The following files are generated during mage operations and are excluded via .gitignore:
- `bin/` - Downloaded external binaries
- `coverage.out` - Go test coverage report  
- `quality-report.json` - Golangci-lint code climate report
- `test-coverage.xml` - Cobertura coverage report
- `test-results.xml` - JUnit test results

### Environment Detection
The commons package provides environment detection:
- `commons.IsCIEnv()` - detects CI environment via CI env var
- `commons.OnlyOnCI(task)` - run task only in CI
- `commons.OnlyLocally(task)` - run task only locally  
- `commons.OnlyOnLinux/Windows/Darwin(task)` - OS-specific tasks