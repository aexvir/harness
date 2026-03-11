package binary

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aexvir/harness/internal"
)

//go:generate go run testdata/gen/main.go

//go:embed testdata
var testdata embed.FS

func TestRemoteBinaryDownloadOrigin(t *testing.T) {
	t.Run("happy path",
		func(t *testing.T) {
			srv := setupTestServer(t)
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")

			require.NoError(t, RemoteBinaryDownload(srv.URL+"/util").Install(tmpl))

			info, err := os.Stat(tmpl.Cmd)
			require.NoError(t, err)
			if runtime.GOOS != "windows" {
				assert.NotZero(t, info.Mode().Perm()&0o111)
			}

			content, err := os.ReadFile(tmpl.Cmd)
			require.NoError(t, err)
			assert.NotEmpty(t, content)
		},
	)

	t.Run("template url",
		func(t *testing.T) {
			srv := setupTestServer(t)
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")

			require.NoError(t, RemoteBinaryDownload(srv.URL+"/{{.Name}}").Install(tmpl))
			assert.FileExists(t, tmpl.Cmd)
		},
	)

	t.Run("http error",
		func(t *testing.T) {
			srv := setupTestServer(t)
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")

			err := RemoteBinaryDownload(srv.URL + "/nonexistent").Install(tmpl)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unexpected response when downloading binary")
		},
	)

	t.Run("creates nested directory",
		func(t *testing.T) {
			srv := setupTestServer(t)
			dir := filepath.Join(t.TempDir(), "nested", "bin", "dir")
			tmpl := mktemplate(dir, "util", "1.2.3")

			require.NoError(t, RemoteBinaryDownload(srv.URL+"/util").Install(tmpl))
			assert.FileExists(t, tmpl.Cmd)
		},
	)

	t.Run("invalid template",
		func(t *testing.T) {
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")

			err := RemoteBinaryDownload("http://example.com/{{.Invalid").Install(tmpl)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to resolve URL")
		},
	)
}

func TestGoBinaryOrigin(t *testing.T) {
	t.Run("happy path",
		func(t *testing.T) {
			tmpl := mktemplate(t.TempDir(), "goimports", "latest")

			err := GoBinary("golang.org/x/tools/cmd/goimports").Install(tmpl)
			require.NoError(t, err)
			assert.FileExists(t, filepath.Join(tmpl.Directory, "goimports"))
		},
	)

	t.Run("renames binary when package base name differs from template name",
		func(t *testing.T) {
			// install goimports but give it a different name
			tmpl := mktemplate(t.TempDir(), "goimp", "latest")

			err := GoBinary("golang.org/x/tools/cmd/goimports").Install(tmpl)
			require.NoError(t, err)

			assert.FileExists(t, filepath.Join(tmpl.Directory, "goimp"))
			assert.NoFileExists(t, filepath.Join(tmpl.Directory, "goimports"))
		},
	)

	t.Run("go install failure",
		func(t *testing.T) {
			tmpl := mktemplate(t.TempDir(), "nonexistent", "latest")

			err := GoBinary("github.com/aexvir/harness/nonexistent/cmd/tool").Install(tmpl)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unable to install executable")
		},
	)

	t.Run("creates nested directory",
		func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "nested", "bin")
			tmpl := mktemplate(dir, "goimports", "latest")

			err := GoBinary("golang.org/x/tools/cmd/goimports").Install(tmpl)
			require.NoError(t, err)
			assert.FileExists(t, filepath.Join(dir, "goimports"))
		},
	)
}

func TestRemoteArchiveDownloadOrigin(t *testing.T) {
	t.Run("tar.gz",
		func(t *testing.T) {
			srv := setupTestServer(t)
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")

			require.NoError(t, RemoteArchiveDownload(srv.URL+"/util.tar.gz", map[string]string{"util": "util"}).Install(tmpl))

			info, err := os.Stat(filepath.Join(tmpl.Directory, "util"))
			require.NoError(t, err)
			if runtime.GOOS != "windows" {
				assert.NotZero(t, info.Mode().Perm()&0o111)
			}
			assert.NoFileExists(t, filepath.Join(tmpl.Directory, "util.tar.gz"))
		},
	)

	t.Run("zip",
		func(t *testing.T) {
			srv := setupTestServer(t)
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")
			tmpl.ArchiveExtension = ".zip"

			require.NoError(t, RemoteArchiveDownload(srv.URL+"/util.zip", map[string]string{"util": "util"}).Install(tmpl))
			assert.FileExists(t, filepath.Join(tmpl.Directory, "util"))
		},
	)

	t.Run("nested path with mapping",
		func(t *testing.T) {
			srv := setupTestServer(t)
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")

			require.NoError(t, RemoteArchiveDownload(srv.URL+"/nested.tar.gz", map[string]string{"myapp-1.2.3/bin/util": "util"}).Install(tmpl))
			assert.FileExists(t, filepath.Join(tmpl.Directory, "util"))
		},
	)

	t.Run("template variable in mapping",
		func(t *testing.T) {
			srv := setupTestServer(t)
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")

			require.NoError(t, RemoteArchiveDownload(srv.URL+"/nested.tar.gz", map[string]string{"myapp-{{.Version}}/bin/util": "util"}).Install(tmpl))
			assert.FileExists(t, filepath.Join(tmpl.Directory, "util"))
		},
	)

	t.Run("selective extraction",
		func(t *testing.T) {
			srv := setupTestServer(t)
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")

			require.NoError(t, RemoteArchiveDownload(srv.URL+"/multi.tar.gz", map[string]string{"util": "util"}).Install(tmpl))

			assert.FileExists(t, filepath.Join(tmpl.Directory, "util"))
			assert.NoFileExists(t, filepath.Join(tmpl.Directory, "README.md"))
			assert.NoFileExists(t, filepath.Join(tmpl.Directory, "LICENSE"))
		},
	)

	t.Run("empty mapping extracts everything",
		func(t *testing.T) {
			srv := setupTestServer(t)
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")

			require.NoError(t, RemoteArchiveDownload(srv.URL+"/multi.tar.gz", map[string]string{}).Install(tmpl))

			for _, name := range []string{"util", "README.md", "LICENSE"} {
				assert.FileExists(t, filepath.Join(tmpl.Directory, name))
			}
		},
	)

	t.Run("http error",
		func(t *testing.T) {
			srv := setupTestServer(t)
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")

			err := RemoteArchiveDownload(srv.URL+"/nonexistent.tar.gz", map[string]string{"util": "util"}).Install(tmpl)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unexpected response when downloading archive")
		},
	)

	t.Run("unsupported content type",
		func(t *testing.T) {
			srv := setupTestServer(t)
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")

			// serve a plain text file; sniffed mime type will not be a supported archive format
			err := RemoteArchiveDownload(srv.URL+"/util", map[string]string{"util": "util"}).Install(tmpl)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported format")
		},
	)

	t.Run("connection refused",
		func(t *testing.T) {
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")

			err := RemoteArchiveDownload("http://127.0.0.1:1/util.tar.gz", map[string]string{"util": "util"}).Install(tmpl)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to download file")
		},
	)

	t.Run("invalid template",
		func(t *testing.T) {
			tmpl := mktemplate(t.TempDir(), "util", "1.2.3")

			err := RemoteArchiveDownload("http://example.com/{{.Invalid", map[string]string{"util": "util"}).Install(tmpl)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to resolve URL")
		},
	)

	t.Run("skips existing download",
		func(t *testing.T) {
			dir := t.TempDir()
			tmpl := mktemplate(dir, "util", "1.2.3")

			data, err := testdata.ReadFile("testdata/util.tar.gz")
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "util.tar.gz"), data, 0o644))

			require.NoError(t, RemoteArchiveDownload("http://127.0.0.1:1/util.tar.gz", map[string]string{"util": "util"}).Install(tmpl))
			assert.FileExists(t, filepath.Join(dir, "util"))
		},
	)

	t.Run("corrupted archive",
		func(t *testing.T) {
			dir := t.TempDir()
			tmpl := mktemplate(dir, "util", "1.2.3")

			// pre-place a corrupt tar.gz so the download step is skipped and extract is attempted
			require.NoError(t, os.WriteFile(filepath.Join(dir, "util.tar.gz"), []byte("this is not a valid archive"), 0o644))

			err := RemoteArchiveDownload("http://127.0.0.1:1/util.tar.gz", map[string]string{"util": "util"}).Install(tmpl)
			require.Error(t, err)
		},
	)

	t.Run("creates nested directory",
		func(t *testing.T) {
			srv := setupTestServer(t)
			dir := filepath.Join(t.TempDir(), "deep", "nested", "dir")
			tmpl := mktemplate(dir, "util", "1.2.3")

			require.NoError(t, RemoteArchiveDownload(srv.URL+"/util.tar.gz", map[string]string{"util": "util"}).Install(tmpl))
			assert.FileExists(t, filepath.Join(dir, "util"))
		},
	)

	t.Run("renames extracted binary",
		func(t *testing.T) {
			srv := setupTestServer(t)
			dir := t.TempDir()
			tmpl := mktemplate(dir, "renamed", "1.2.3")

			require.NoError(t, RemoteArchiveDownload(srv.URL+"/util.tar.gz", map[string]string{"util": "renamed"}).Install(tmpl))

			assert.FileExists(t, filepath.Join(dir, "renamed"))
			assert.NoFileExists(t, filepath.Join(dir, "util"))
		},
	)
}

func TestProgressDisablesOnNonTerminalOutput(t *testing.T) {
	out := internal.Output
	t.Cleanup(func() { SetOutput(out) })

	t.Run("io.Discard",
		func(t *testing.T) {
			SetOutput(io.Discard)

			src := bytes.NewBufferString("payload")
			got, finish := progress(src, int64(src.Len()))
			defer finish()

			assert.True(t, got == src, "expected progress to be disabled for non-terminal output")
		},
	)

	t.Run("bytes.Buffer",
		func(t *testing.T) {
			buf := new(bytes.Buffer)
			SetOutput(buf)

			src := bytes.NewBufferString("payload")
			got, finish := progress(src, int64(src.Len()))
			defer finish()

			assert.True(t, got == src, "expected progress to be disabled for non-terminal output")
		},
	)
}

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	sub, err := fs.Sub(testdata, "testdata")
	require.NoError(t, err)
	srv := httptest.NewServer(http.FileServer(http.FS(sub)))
	t.Cleanup(srv.Close)
	return srv
}

func mktemplate(dir, name, version string) Template {
	var ext string
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return Template{
		GOOS:             runtime.GOOS,
		GOARCH:           runtime.GOARCH,
		Directory:        dir,
		Name:             name,
		Cmd:              filepath.Join(dir, name) + ext,
		Version:          version,
		Extension:        ext,
		ArchiveExtension: ".tar.gz",
	}
}
