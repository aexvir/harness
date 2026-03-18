package gen

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const defaultMageCmd = "mage"

// Generator holds shared settings for all code generators that rely on mage
// target introspection. Create one with [New] and call its methods to produce
// editor or tooling configuration files.
type Generator struct {
	magecmd string
	workdir string
}

// Option configures a [Generator].
type Option func(*Generator)

// WithMageCommand overrides the mage binary used to discover targets.
// Defaults to "mage" (resolved from PATH).
func WithMageCommand(cmd string) Option {
	return func(g *Generator) { g.magecmd = cmd }
}

// WithWorkDir sets the working directory used when running mage -l.
// Defaults to the current working directory.
func WithWorkDir(dir string) Option {
	return func(g *Generator) { g.workdir = dir }
}

// New returns a [Generator] with sensible defaults, modified by opts.
func New(opts ...Option) *Generator {
	gen := &Generator{magecmd: defaultMageCmd}
	for _, opt := range opts {
		opt(gen)
	}
	return gen
}

// target represents a single build target.
type target struct {
	name        string
	description string
}

// introspectMageTasks runs mage -l using the given binary and working directory,
// and returns the list of available targets.
func (g *Generator) introspectMageTasks(ctx context.Context) ([]target, error) {
	cmd := exec.CommandContext(ctx, g.magecmd, "-l")
	if g.workdir != "" {
		cmd.Dir = g.workdir
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s -l: %w", g.magecmd, err)
	}

	return parseMageListOutput(stdout.String())
}

// parseMageListOutput parses the output of mage -l and returns the list of targets.
//
// The expected format is:
//
//	Targets:
//	  format    format codebase using gofmt and goimports
//	  lint*     lint the code (default)
//	  test      run unit tests
func parseMageListOutput(output string) ([]target, error) {
	var targets []target

	inTargets := false
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "Targets:") {
			inTargets = true
			continue
		}

		if !inTargets {
			continue
		}

		// target lines are indented with at least one space
		if len(line) == 0 || line[0] != ' ' {
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}

		// strip the * suffix mage adds to the default target
		name := strings.TrimSuffix(fields[0], "*")

		// mage always lower-cases target names in -l output, but normalise
		// defensively so callers can rely on a consistent casing
		name = strings.ToLower(name)

		// everything after the target name (and surrounding whitespace) is the description
		var desc string
		if len(fields) > 1 {
			rest := strings.TrimPrefix(trimmed, fields[0])
			desc = strings.TrimSpace(rest)
		}

		targets = append(targets, target{name: name, description: desc})
	}

	return targets, nil
}
