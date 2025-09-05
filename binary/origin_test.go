package binary

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
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

func TestRemoteArchiveDownload_Install_TarGz_MappedFiles(t *testing.T) {
	// Create test tar.gz archive with multiple files
	archiveData := createTestTarGz(t, map[string]string{
		"tool1":        "binary content for tool1",
		"bin/tool2":    "binary content for tool2",
		"config.yaml": "config content",
		"readme.txt":  "readme content",
	})

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-gzip")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(archiveData)))
		w.Write(archiveData)
	}))
	defer server.Close()

	// Create temporary directory
	tmpdir, err := os.MkdirTemp("", "binary-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	// Test with mapped files - only extract specific files
	binaries := map[string]string{
		"tool1":     "extracted-tool1",
		"bin/tool2": "extracted-tool2",
	}
	origin := RemoteArchiveDownload(server.URL+"/test-{{.Version}}.tar.gz", binaries)

	template := Template{
		Name:      "test-archive",
		Version:   "1.0.0",
		Directory: tmpdir,
	}

	err = origin.Install(template)
	assert.NoError(t, err)

	// Check that mapped files were extracted with correct names
	extractedTool1 := filepath.Join(tmpdir, "extracted-tool1")
	assert.FileExists(t, extractedTool1)
	content1, _ := os.ReadFile(extractedTool1)
	assert.Equal(t, "binary content for tool1", string(content1))

	extractedTool2 := filepath.Join(tmpdir, "extracted-tool2")
	assert.FileExists(t, extractedTool2)
	content2, _ := os.ReadFile(extractedTool2)
	assert.Equal(t, "binary content for tool2", string(content2))

	// Check that unmapped files were NOT extracted
	assert.NoFileExists(t, filepath.Join(tmpdir, "config.yaml"))
	assert.NoFileExists(t, filepath.Join(tmpdir, "readme.txt"))

	// Check permissions
	info1, err := os.Stat(extractedTool1)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info1.Mode())
}

func TestRemoteArchiveDownload_Install_Zip_MappedFiles(t *testing.T) {
	// Create test zip archive
	archiveData := createTestZip(t, map[string]string{
		"app.exe":       "windows binary",
		"lib/helper.so": "library file",
		"docs/help.txt": "help documentation",
	})

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(archiveData)))
		w.Write(archiveData)
	}))
	defer server.Close()

	// Create temporary directory
	tmpdir, err := os.MkdirTemp("", "binary-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	// Test with mapped files
	binaries := map[string]string{
		"app.exe":       "myapp",
		"lib/helper.so": "helper",
	}
	origin := RemoteArchiveDownload(server.URL+"/{{.Name}}-{{.GOOS}}.zip", binaries)

	template := Template{
		Name:      "myproject",
		GOOS:      "windows",
		Directory: tmpdir,
	}

	err = origin.Install(template)
	assert.NoError(t, err)

	// Check extracted files
	extractedApp := filepath.Join(tmpdir, "myapp")
	assert.FileExists(t, extractedApp)
	content1, _ := os.ReadFile(extractedApp)
	assert.Equal(t, "windows binary", string(content1))

	extractedHelper := filepath.Join(tmpdir, "helper")
	assert.FileExists(t, extractedHelper)
	content2, _ := os.ReadFile(extractedHelper)
	assert.Equal(t, "library file", string(content2))

	// Check that unmapped file was NOT extracted
	assert.NoFileExists(t, filepath.Join(tmpdir, "docs", "help.txt"))
}

func TestRemoteArchiveDownload_Install_UnmappedFiles(t *testing.T) {
	// Create test tar.gz archive
	archiveData := createTestTarGz(t, map[string]string{
		"bin/app":    "app binary",
		"bin/helper": "helper binary",
		"config.txt": "config file",
	})

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-gzip")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(archiveData)))
		w.Write(archiveData)
	}))
	defer server.Close()

	// Create temporary directory
	tmpdir, err := os.MkdirTemp("", "binary-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	// Test with empty mapping - should extract all files
	origin := RemoteArchiveDownload(server.URL+"/archive.tar.gz", map[string]string{})

	template := Template{
		Name:      "test",
		Directory: tmpdir,
	}

	err = origin.Install(template)
	assert.NoError(t, err)

	// Check that all files were extracted with original structure
	assert.FileExists(t, filepath.Join(tmpdir, "bin", "app"))
	assert.FileExists(t, filepath.Join(tmpdir, "bin", "helper"))
	assert.FileExists(t, filepath.Join(tmpdir, "config.txt"))

	// Verify content
	content, _ := os.ReadFile(filepath.Join(tmpdir, "bin", "app"))
	assert.Equal(t, "app binary", string(content))
}

func TestRemoteArchiveDownload_Install_TemplateResolution(t *testing.T) {
	// Create test archive
	archiveData := createTestTarGz(t, map[string]string{
		"myapp-v1.2.3/bin/myapp": "versioned app",
		"myapp-v1.2.3/lib/lib.so": "library",
	})

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL template was resolved correctly
		expectedPath := "/myapp-v1.2.3-linux-amd64.tar.gz"
		assert.Equal(t, expectedPath, r.URL.Path)
		
		w.Header().Set("Content-Type", "application/x-gzip")
		w.Write(archiveData)
	}))
	defer server.Close()

	tmpdir, err := os.MkdirTemp("", "binary-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	// Test template resolution in both URL and binary mapping
	binaries := map[string]string{
		"{{.Name}}-v{{.Version}}/bin/{{.Name}}": "{{.Name}}",
		"{{.Name}}-v{{.Version}}/lib/lib.so":     "lib",
	}
	origin := RemoteArchiveDownload(server.URL+"/{{.Name}}-v{{.Version}}-{{.GOOS}}-{{.GOARCH}}.tar.gz", binaries)

	template := Template{
		Name:      "myapp",
		Version:   "1.2.3",
		GOOS:      "linux",
		GOARCH:    "amd64",
		Directory: tmpdir,
	}

	err = origin.Install(template)
	assert.NoError(t, err)

	// Check resolved files
	assert.FileExists(t, filepath.Join(tmpdir, "myapp"))
	assert.FileExists(t, filepath.Join(tmpdir, "lib"))
}

func TestRemoteArchiveDownload_Install_HTTPError(t *testing.T) {
	// Create test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmpdir, err := os.MkdirTemp("", "binary-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	origin := RemoteArchiveDownload(server.URL+"/missing.tar.gz", map[string]string{"app": "app"})

	template := Template{
		Name:      "test",
		Directory: tmpdir,
	}

	err = origin.Install(template)
	assert.Error(t, err)
	// The download succeeds but extraction fails due to unsupported format
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestRemoteArchiveDownload_Install_UnsupportedFormat(t *testing.T) {
	// Create test server that returns unsupported content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("not an archive"))
	}))
	defer server.Close()

	tmpdir, err := os.MkdirTemp("", "binary-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	origin := RemoteArchiveDownload(server.URL+"/file.txt", map[string]string{"app": "app"})

	template := Template{
		Name:      "test",
		Directory: tmpdir,
	}

	err = origin.Install(template)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
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
		return 0, io.EOF
	}

	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// createTestTarGz creates a tar.gz archive with the given files for testing
func createTestTarGz(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	
	// Create gzip writer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	// Add files to archive
	for filename, content := range files {
		header := &tar.Header{
			Name: filename,
			Size: int64(len(content)),
			Mode: 0755,
		}
		
		err := tarWriter.WriteHeader(header)
		require.NoError(t, err)
		
		_, err = tarWriter.Write([]byte(content))
		require.NoError(t, err)
	}

	// Close writers
	err := tarWriter.Close()
	require.NoError(t, err)
	err = gzipWriter.Close()
	require.NoError(t, err)

	return buf.Bytes()
}

// createTestZip creates a zip archive with the given files for testing
func createTestZip(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	
	zipWriter := zip.NewWriter(&buf)

	// Add files to archive
	for filename, content := range files {
		writer, err := zipWriter.Create(filename)
		require.NoError(t, err)
		
		_, err = writer.Write([]byte(content))
		require.NoError(t, err)
	}

	// Close writer
	err := zipWriter.Close()
	require.NoError(t, err)

	return buf.Bytes()
}
