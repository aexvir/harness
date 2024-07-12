package commons

import (
	"context"
	"fmt"

	"github.com/aexvir/harness"
	"github.com/aexvir/harness/bintool"
)

// Commitsar lints commit messages and verifies conventional commits compliance.
//
// https://commitsar.aevea.ee
// https://conventionalcommits.org
func Commitsar(opts ...CommitsarOpt) harness.Task {
	conf := commitsarconf{
		version: "latest",
	}

	for _, opt := range opts {
		opt(&conf)
	}

	return func(ctx context.Context) error {
		// commitsar can't be used with NewGo at the moment, as it's version reporting is messed up and
		// it will always attempt to download the specified version
		cmsr, _ := bintool.New(
			"commitsar{{.BinExt}}",
			conf.version,
			"https://github.com/aevea/commitsar/releases/download/v{{.Version}}/commitsar_{{.Version}}_{{.GOOS}}_{{.GOARCH}}{{.ArchiveExt}}",
			bintool.WithVersionCmd("{{.FullCmd}} version"),
		)

		if err := cmsr.Ensure(); err != nil {
			return fmt.Errorf("failed to provision commitsar binary: %w", err)
		}

		return harness.Run(ctx, cmsr.BinPath())
	}
}

type commitsarconf struct {
	version string
}

type CommitsarOpt func(c *commitsarconf)

// WithCommitsarVersion allows specifying the commitsar version
// that should be used when running this task.
func WithCommitsarVersion(version string) CommitsarOpt {
	return func(c *commitsarconf) {
		c.version = version
	}
}
