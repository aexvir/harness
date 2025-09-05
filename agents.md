# Agents Context: Harness Project

## Project Overview

**Harness** is a Go package that provides a structured framework for building automation scripts using [Mage](https://magefile.org/). It eliminates the need to build CI/CD and development automation from scratch by providing reusable primitives, binary management, and common development tasks.

**Package URL**: `github.com/aexvir/harness`  
**Documentation**: [pkg.go.dev/github.com/aexvir/harness](https://pkg.go.dev/github.com/aexvir/harness)

## Key Concepts

### 1. Harness Framework
The core `Harness` struct provides:
- **Task Execution**: Sequential execution of tasks with consistent output formatting
- **Hooks**: Pre/post execution hooks for common setup/teardown
- **Timing**: Automatic timing and status indicators (✔/✘)
- **Error Handling**: Structured error collection and reporting

### 2. Tasks
Tasks are functions with signature `func(ctx context.Context) error` that represent individual automation steps (testing, linting, building, etc.).

### 3. Binary Management
The `binary` package handles automatic provisioning of external tools:
- **Origins**: Different sources for binaries (Go modules, remote archives, direct downloads)
- **Version Management**: Ensures correct versions are installed
- **Template System**: Dynamic configuration based on OS/architecture

### 4. Commons Package
Pre-built tasks for common development workflows:
- Go tooling (fmt, imports, test, mod tidy)
- Linting (golangci-lint, commitsar)
- Environment detection (CI vs local, OS-specific tasks)

## Project Structure

```
/
├── harness.go          # Core harness framework
├── runner.go           # Command execution utilities
├── doc.go             # Package documentation
├── binary/            # Binary management system
│   ├── binary.go      # Core binary provisioning
│   ├── origin.go      # Different binary sources
│   └── doc.go         # Binary package docs
├── commons/           # Pre-built common tasks
│   ├── doc.go         # Commons package docs
│   ├── envdetect.go   # Environment detection utilities
│   ├── provision.go   # Binary provisioning tasks
│   ├── go*.go         # Go-specific tasks (fmt, test, etc.)
│   ├── golangcilint.go # golangci-lint integration
│   └── commitsar.go   # Commit message linting
└── magefiles/         # Example usage
    └── main.go        # Magefile showing real usage
```

## Core Components

### Harness (`harness.go`)
- `New()`: Creates harness with optional hooks
- `Execute()`: Runs tasks sequentially with status reporting
- `LogStep()`: Consistent task step logging
- `WithPreExecFunc()`: Adds pre-execution hooks

### Task Runner (`runner.go`) 
- `Run()`: Simple command execution helper
- `Cmd()`: Advanced command builder with options
- Options: environment, arguments, directories, output handling

### Binary Management (`binary/`)
- `New()`: Creates binary specification
- `Ensure()`: Downloads/installs if needed
- Origins: `GoBinary()`, `RemoteBinaryDownload()`, `RemoteArchiveDownload()`

### Commons Tasks (`commons/`)
- `GoFmt()`, `GoImports()`, `GoTest()`, `GoModTidy()`
- `GolangCILint()`, `Commitsar()`
- `OnlyOnCI()`, `OnlyLocally()`, `OnlyOnGOOS()`
- `Provision()`: Bulk binary provisioning

## Usage Patterns

### Basic Harness Setup
```go
h := harness.New(
    harness.WithPreExecFunc(
        func(ctx context.Context) error {
            return harness.Run(ctx, "go", harness.WithArgs("mod", "download"))
        },
    ),
)
```

### Typical Task Function
```go
func Lint(ctx context.Context) error {
    return h.Execute(
        ctx,
        commons.GoModTidy(),
        commons.Commitsar(commons.WithCommitsarVersion("0.20.1")),
        commons.GolangCILint(
            commons.WithGolangCIVersion("v1.63.3"),
            commons.WithGolangCICodeClimate(commons.IsCIEnv()),
        ),
    )
}
```

### Binary Provisioning
```go
commitsar := binary.New(
    "commitsar",
    "0.20.1", 
    binary.RemoteArchiveDownload(
        "https://github.com/aevea/commitsar/releases/download/v{{.Version}}/commitsar_{{.Version}}_{{.GOOS}}_{{.GOARCH}}.tar.gz",
        map[string]string{"commitsar": "commitsar"},
    ),
)
```

## Development Workflow

### Available Mage Tasks
- `mage format`: Format code using gofmt and goimports
- `mage lint`: Lint code using go mod tidy, commitsar, and golangci-lint  
- `mage test`: Run unit tests
- `mage tidy`: Run go mod tidy

### Build and Test
```bash
go mod download
go build ./...
go test ./...
```

## Common Integration Patterns

### CI/CD Integration
- Use `commons.IsCIEnv()` to detect CI environment
- Use `OnlyOnCI()` and `OnlyLocally()` for conditional tasks
- Generate CI-friendly output (JUnit, Cobertura) when in CI

### Multi-Platform Support
- Binary templates include `{{.GOOS}}` and `{{.GOARCH}}` for platform-specific downloads
- Use `OnlyOnGOOS()` for OS-specific tasks
- Automatic Windows `.exe` extension handling

### Tool Versioning
- Pin specific versions of external tools (golangci-lint, commitsar)
- Use version checking to avoid unnecessary downloads
- Support for "latest" version when needed

## For LLM Agents

When working with this codebase:

1. **Focus on the `commons/` package** for most development task implementations
2. **Check `magefiles/main.go`** for real-world usage examples
3. **Binary provisioning** follows the pattern: define binary → ensure installed → use in tasks
4. **Task composition** is the key pattern - combine small tasks into larger workflows
5. **Environment detection** is crucial for CI vs local behavior differences
6. **Error handling** should be consistent with existing patterns (structured errors, colored output)

The project emphasizes **reusability**, **consistency**, and **ease of use** for Go development automation.