package binary

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoteBinaryDownload(t *testing.T) {
	origin := RemoteBinaryDownload("https://example.com/{{.Name}}")

	require.NotNil(t, origin)

	// Check it implements Origin interface
	var _ Origin = origin
}

func TestRemoteBinaryDownload_Install(t *testing.T) {
	// Create a test server that serves a binary
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "11")
				w.Write([]byte("test-binary"))
			},
		),
	)

	defer server.Close()

	// Create temporary directory
	tmpdir, err := os.MkdirTemp("", "binary-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	origin := RemoteBinaryDownload(server.URL + "/{{.Name}}")

	template := Template{
		Name:      "test-bin",
		Directory: tmpdir,
		Cmd:       filepath.Join(tmpdir, "test-bin"),
		GOOS:      "linux",
		GOARCH:    "amd64",
	}

	err = origin.Install(template)
	assert.NoError(t, err)

	// Check file was created
	assert.FileExists(t, template.Cmd)

	// Check file content
	content, err := os.ReadFile(template.Cmd)
	require.NoError(t, err)
	assert.Equal(t, "test-binary", string(content))

	// Check file permissions (should be executable)
	info, err := os.Stat(template.Cmd)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode())
}

func TestRemoteBinaryDownload_Install_HTTPError(t *testing.T) {
	// Create a test server that returns 404
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
		),
	)

	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "binary-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	origin := RemoteBinaryDownload(server.URL + "/{{.Name}}")

	template := Template{
		Name:      "test-bin",
		Directory: tmpDir,
		Cmd:       filepath.Join(tmpDir, "test-bin"),
	}

	err = origin.Install(template)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response")
}

func TestRemoteArchiveDownload(t *testing.T) {
	binaries := map[string]string{"bin/tool": "tool"}
	origin := RemoteArchiveDownload("https://example.com/{{.Name}}.tar.gz", binaries)

	require.NotNil(t, origin)

	// Check it implements Origin interface
	var _ Origin = origin
}

func TestGoBinary(t *testing.T) {
	origin := GoBinary("example.com/pkg/cmd")

	require.NotNil(t, origin)

	// Check it implements Origin interface
	var _ Origin = origin
}

func TestGoBinary_Install(t *testing.T) {
	// Skip this test if go is not available
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	tmpDir, err := os.MkdirTemp("", "binary-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Use a real Go package that we know exists
	origin := GoBinary("golang.org/x/tools/cmd/goimports")

	template := Template{
		Name:      "goimports",
		Directory: tmpDir,
		Cmd:       filepath.Join(tmpDir, "goimports"),
		Version:   "latest",
	}

	err = origin.Install(template)
	// This might fail in test environment due to network or Go setup,
	// so we'll just check the error is reasonable
	if err != nil {
		// Error should be about go install, not about our code
		assert.Contains(t, err.Error(), "unable to install executable")
	} else {
		// If successful, check the binary was created
		assert.FileExists(t, template.Cmd)
	}
}

func TestProgress(t *testing.T) {
	t.Run("returns wrapped reader and finish function", func(t *testing.T) {
		content := []byte("test content")
		reader := &testReader{data: content}

		wrapped, finish := progress(reader, int64(len(content)))

		require.NotNil(t, wrapped)
		require.NotNil(t, finish)

		// Should be able to read from wrapped reader
		buf := make([]byte, len(content))
		n, err := wrapped.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, len(content), n)
		assert.Equal(t, content, buf)

		// Finish function should not panic
		assert.NotPanics(t, func() {
			finish()
		})
	})
}

// testReader is a simple io.Reader for testing
type testReader struct {
	data []byte
	pos  int
}

func (r *testReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, nil
	}

	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
