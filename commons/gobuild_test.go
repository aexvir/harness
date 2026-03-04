package commons

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithGoBuildTags(t *testing.T) {
	t.Run("sets build tags", func(t *testing.T) {
		var conf buildconf
		WithGoBuildTags("integration", "debug")(&conf)
		assert.Equal(t, []string{"integration", "debug"}, conf.tags)
	})

	t.Run("overwrites previous tags", func(t *testing.T) {
		var conf buildconf
		WithGoBuildTags("first")(&conf)
		WithGoBuildTags("second", "third")(&conf)
		assert.Equal(t, []string{"second", "third"}, conf.tags)
	})
}

func TestWithGoBuildLDFlags(t *testing.T) {
	t.Run("sets ldflags", func(t *testing.T) {
		var conf buildconf
		WithGoBuildLDFlags("main.version=1.0.0", "main.commit=abc123")(&conf)
		assert.Equal(t, []string{"main.version=1.0.0", "main.commit=abc123"}, conf.ldflags)
	})

	t.Run("overwrites previous ldflags", func(t *testing.T) {
		var conf buildconf
		WithGoBuildLDFlags("old=value")(&conf)
		WithGoBuildLDFlags("new=value")(&conf)
		assert.Equal(t, []string{"new=value"}, conf.ldflags)
	})
}

func TestBuildGoBuildArgs(t *testing.T) {
	t.Run("basic build without options", func(t *testing.T) {
		args := buildGoBuildArgs("./cmd/app", "bin/app", buildconf{})
		assert.Equal(t, []string{"build", "-o", "bin/app", "./cmd/app"}, args)
	})

	t.Run("with build tags", func(t *testing.T) {
		conf := buildconf{tags: []string{"integration", "debug"}}
		args := buildGoBuildArgs("./cmd/app", "bin/app", conf)
		assert.Equal(t, []string{
			"build", "-o", "bin/app",
			"-tags", "integration debug",
			"./cmd/app",
		}, args)
	})

	t.Run("with ldflags", func(t *testing.T) {
		conf := buildconf{ldflags: []string{"main.version=1.0.0", "main.commit=abc123"}}
		args := buildGoBuildArgs("./cmd/app", "bin/app", conf)
		assert.Equal(t, []string{
			"build", "-o", "bin/app",
			"-ldflags", "-X 'main.version=1.0.0' -X 'main.commit=abc123'",
			"./cmd/app",
		}, args)
	})

	t.Run("with both tags and ldflags", func(t *testing.T) {
		conf := buildconf{
			tags:    []string{"release"},
			ldflags: []string{"main.version=2.0.0"},
		}
		args := buildGoBuildArgs("./cmd/server", "bin/server", conf)
		assert.Equal(t, []string{
			"build", "-o", "bin/server",
			"-tags", "release",
			"-ldflags", "-X 'main.version=2.0.0'",
			"./cmd/server",
		}, args)
	})

	t.Run("single tag", func(t *testing.T) {
		conf := buildconf{tags: []string{"netgo"}}
		args := buildGoBuildArgs(".", "out", conf)
		assert.Contains(t, args, "-tags")
		assert.Contains(t, args, "netgo")
	})

	t.Run("single ldflag", func(t *testing.T) {
		conf := buildconf{ldflags: []string{"main.built=true"}}
		args := buildGoBuildArgs(".", "out", conf)
		assert.Contains(t, args, "-ldflags")
		assert.Contains(t, args, "-X 'main.built=true'")
	})
}
