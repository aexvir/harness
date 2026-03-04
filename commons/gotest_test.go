package commons

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoTestDefaultConfig(t *testing.T) {
	conf := testconf{
		race:          true,
		coberturafile: "test-coverage.xml",
		junitfile:     "test-results.xml",
		filedumpfile:  "test-output.txt",
	}

	assert.True(t, conf.race, "race detection should be enabled by default")
	assert.Equal(t, "test-coverage.xml", conf.coberturafile)
	assert.Equal(t, "test-results.xml", conf.junitfile)
	assert.Equal(t, "test-output.txt", conf.filedumpfile)
	assert.False(t, conf.integration)
	assert.False(t, conf.cifriendlyout)
	assert.False(t, conf.junit)
	assert.False(t, conf.cobertura)
	assert.False(t, conf.filedump)
	assert.False(t, conf.courtneycoverage)
	assert.Nil(t, conf.target)
}

func TestWithTarget(t *testing.T) {
	t.Run("sets target pointer", func(t *testing.T) {
		var conf testconf
		target := "cmd/app"
		WithTarget(&target)(&conf)
		assert.Equal(t, &target, conf.target)
		assert.Equal(t, "cmd/app", *conf.target)
	})

	t.Run("accepts nil to reset", func(t *testing.T) {
		var conf testconf
		WithTarget(nil)(&conf)
		assert.Nil(t, conf.target)
	})
}

func TestWithRace(t *testing.T) {
	t.Run("enables race detection", func(t *testing.T) {
		var conf testconf
		WithRace(true)(&conf)
		assert.True(t, conf.race)
	})

	t.Run("disables race detection", func(t *testing.T) {
		conf := testconf{race: true}
		WithRace(false)(&conf)
		assert.False(t, conf.race)
	})
}

func TestWithIntegrationTest(t *testing.T) {
	var conf testconf
	WithIntegrationTest()(&conf)
	assert.True(t, conf.integration)
}

func TestWithTestCIFriendlyOutput(t *testing.T) {
	t.Run("enables CI-friendly output", func(t *testing.T) {
		var conf testconf
		WithTestCIFriendlyOutput(true)(&conf)
		assert.True(t, conf.cifriendlyout)
	})

	t.Run("disables CI-friendly output", func(t *testing.T) {
		conf := testconf{cifriendlyout: true}
		WithTestCIFriendlyOutput(false)(&conf)
		assert.False(t, conf.cifriendlyout)
	})
}

func TestWithTestFileDump(t *testing.T) {
	var conf testconf
	WithTestFileDump(true)(&conf)
	assert.True(t, conf.filedump)
}

func TestWithTestFileDumpOutput(t *testing.T) {
	var conf testconf
	WithTestFileDumpOutput("custom-output.txt")(&conf)
	assert.Equal(t, "custom-output.txt", conf.filedumpfile)
}

func TestWithTestCobertura(t *testing.T) {
	var conf testconf
	WithTestCobertura(true)(&conf)
	assert.True(t, conf.cobertura)
}

func TestWithTestCoberturaOutput(t *testing.T) {
	var conf testconf
	WithTestCoberturaOutput("custom-coverage.xml")(&conf)
	assert.Equal(t, "custom-coverage.xml", conf.coberturafile)
}

func TestWithTestCoverageExclusions(t *testing.T) {
	var conf testconf
	WithTestCoverageExclusions()(&conf)
	assert.True(t, conf.courtneycoverage)
}

func TestWithTestJunit(t *testing.T) {
	var conf testconf
	WithTestJunit(true)(&conf)
	assert.True(t, conf.junit)
}

func TestWithTestJunitOutput(t *testing.T) {
	var conf testconf
	WithTestJunitOutput("custom-results.xml")(&conf)
	assert.Equal(t, "custom-results.xml", conf.junitfile)
}

func TestBuildGoTestArgs(t *testing.T) {
	t.Run("defaults with race enabled", func(t *testing.T) {
		conf := testconf{race: true}
		args, env := buildGoTestArgs(conf)
		assert.Equal(t, []string{"test", "-cover", "./...", "-race"}, args)
		assert.Empty(t, env)
	})

	t.Run("race disabled", func(t *testing.T) {
		conf := testconf{race: false}
		args, _ := buildGoTestArgs(conf)
		assert.Equal(t, []string{"test", "-cover", "./..."}, args)
		assert.NotContains(t, args, "-race")
	})

	t.Run("with target", func(t *testing.T) {
		target := "cmd/app"
		conf := testconf{target: &target}
		args, _ := buildGoTestArgs(conf)
		assert.Equal(t, []string{"test", "-cover", "./cmd/app/..."}, args)
	})

	t.Run("nil target uses ./...", func(t *testing.T) {
		conf := testconf{}
		args, _ := buildGoTestArgs(conf)
		assert.Contains(t, args, "./...")
	})

	t.Run("integration mode", func(t *testing.T) {
		conf := testconf{integration: true}
		args, env := buildGoTestArgs(conf)
		assert.Contains(t, args, "-run")
		assert.Contains(t, args, "^TestIntegration")
		assert.Equal(t, []string{"TEST_TARGET=integration"}, env)
	})

	t.Run("CI-friendly output adds json flag", func(t *testing.T) {
		conf := testconf{cifriendlyout: true}
		args, _ := buildGoTestArgs(conf)
		assert.Contains(t, args, "-json")
	})

	t.Run("junit adds json flag", func(t *testing.T) {
		conf := testconf{junit: true}
		args, _ := buildGoTestArgs(conf)
		assert.Contains(t, args, "-json")
	})

	t.Run("json flag not duplicated when both ci-friendly and junit", func(t *testing.T) {
		conf := testconf{cifriendlyout: true, junit: true}
		args, _ := buildGoTestArgs(conf)
		count := 0
		for _, a := range args {
			if a == "-json" {
				count++
			}
		}
		assert.Equal(t, 1, count, "expected exactly one -json flag")
	})

	t.Run("cobertura adds coverprofile", func(t *testing.T) {
		conf := testconf{cobertura: true}
		args, _ := buildGoTestArgs(conf)
		assert.Contains(t, args, "-coverprofile")
		assert.Contains(t, args, "coverage.out")
	})

	t.Run("no coverprofile without cobertura", func(t *testing.T) {
		conf := testconf{}
		args, _ := buildGoTestArgs(conf)
		assert.NotContains(t, args, "-coverprofile")
	})

	t.Run("combined flags", func(t *testing.T) {
		target := "pkg/core"
		conf := testconf{
			target:        &target,
			race:          true,
			integration:   true,
			cifriendlyout: true,
			cobertura:     true,
		}
		args, env := buildGoTestArgs(conf)

		assert.Contains(t, args, "./pkg/core/...")
		assert.Contains(t, args, "-race")
		assert.Contains(t, args, "-run")
		assert.Contains(t, args, "^TestIntegration")
		assert.Contains(t, args, "-json")
		assert.Contains(t, args, "-coverprofile")
		assert.Contains(t, args, "coverage.out")
		assert.Equal(t, []string{"TEST_TARGET=integration"}, env)
	})
}
