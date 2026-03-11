package commons

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeTestSummaryFromJSON(t *testing.T) {
	fixture := filepath.Join("testdata", "gotest-summary.jsonl")
	data, err := os.ReadFile(fixture)
	require.NoError(t, err)

	tests, passed, skipped, failed, err := computeTestSummaryFromJSON(data)
	require.NoError(t, err)

	assert.Equal(t, 4, tests)
	assert.Equal(t, 2, passed)
	assert.Equal(t, 1, skipped)
	assert.Equal(t, 1, failed)
}
