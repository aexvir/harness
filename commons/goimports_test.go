package commons

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithGoImportsVersion(t *testing.T) {
	t.Run("sets the version", func(t *testing.T) {
		var conf goimportsconf
		WithGoImportsVersion("v0.17.0")(&conf)
		assert.Equal(t, "v0.17.0", conf.version)
	})
}

func TestGoImportsDefaults(t *testing.T) {
	t.Run("default version is latest", func(t *testing.T) {
		conf := goimportsconf{version: "latest"}
		assert.Equal(t, "latest", conf.version)
	})
}
