//go:build mage

package main

import (
	"context"

	"github.com/aexvir/harness"
	"github.com/aexvir/harness/commons"
)

const (
	pkgName             = "github.com/aexvir/harness"
	commitsarVersion    = "0.20.1"
	golangcilintVersion = "v1.52.2"
)

var h = harness.New(
	harness.WithPreExecFunc(
		func(ctx context.Context) error { // ensure go mod download is run before any task
			return harness.Run(ctx, "go", harness.WithArgs("mod", "download"))
		},
	),
)

// format codebase using gofmt and goimports
func Format(ctx context.Context) error {
	return h.Execute(
		ctx,
		commons.GoFmt(),
		commons.GoImports(pkgName),
	)
}

// lint the code using go mod tidy, commitsar and golangci-lint
func Lint(ctx context.Context) error {
	return h.Execute(
		ctx,
		commons.GoModTidy(),
		commons.Commitsar(
			commons.WithCommitsarVersion(commitsarVersion),
		),
		commons.GolangCILint(
			commons.WithGolangCIVersion(golangcilintVersion),
			commons.WithGolangCICodeClimate(commons.IsCIEnv()),
		),
	)
}

// run unit tests
func Test(ctx context.Context) error {
	return h.Execute(
		ctx,
		commons.GoTest(
			commons.WithTestJunit(commons.IsCIEnv()),
			commons.WithTestCobertura(commons.IsCIEnv()),
			commons.WithTestCIFriendlyOutput(commons.IsCIEnv()),
		),
	)
}

// run go mod tidy
func Tidy(ctx context.Context) error {
	return h.Execute(
		ctx,
		commons.GoModTidy(),
	)
}
