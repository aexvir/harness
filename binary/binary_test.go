package binary

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// silent output when not verbose
func TestMain(m *testing.M) {
	flag.Parse()

	if !testing.Verbose() {
		SetOutput(io.Discard)
	}

	code := m.Run()
	os.Exit(code)
}

func TestNew(t *testing.T) {
	wantDir := filepath.FromSlash("./bin")

	var wantExt string
	if runtime.GOOS == "windows" {
		wantExt = ".exe"
	}

	wantCmd := filepath.Join(wantDir, "util") + wantExt

	t.Run("sets defaults",
		func(t *testing.T) {
			var origin *fakeorigin
			b := New("util", "1.0.0", origin)

			assert.Equal(t, "util", b.Name())
			assert.Equal(t, wantDir, b.directory)
			assert.Equal(t, wantCmd, b.BinPath())
			assert.Equal(t, "1.0.0", b.version)
			assert.Equal(t, runtime.GOOS, b.template.GOOS)
			assert.Equal(t, runtime.GOARCH, b.template.GOARCH)
			assert.Equal(t, ".tar.gz", b.template.ArchiveExtension)
			assert.Equal(t, wantCmd+" --version", b.versioncmd)
		},
	)

	t.Run("with all mapping options",
		func(t *testing.T) {
			var origin *fakeorigin
			b := New("util", "1.0.0", origin,
				WithGOOSMapping(map[string]string{runtime.GOOS: "customos"}),
				WithGOARCHMapping(map[string]string{runtime.GOARCH: "customarch"}),
				WithGOOSArchiveExtensionMapping(map[string]string{"customos": ".zip"}),
			)

			assert.Equal(t, "customos", b.template.GOOS)
			assert.Equal(t, "customarch", b.template.GOARCH)
			assert.Equal(t, ".zip", b.template.ArchiveExtension)
		},
	)

	t.Run("GOOS mapping without match keeps default",
		func(t *testing.T) {
			var origin *fakeorigin
			b := New("util", "1.0.0", origin,
				WithGOOSMapping(map[string]string{"someotheros": "replaced"}),
			)

			assert.Equal(t, runtime.GOOS, b.template.GOOS)
		},
	)

	t.Run("GOARCH mapping without match keeps default",
		func(t *testing.T) {
			var origin *fakeorigin
			b := New("util", "1.0.0", origin,
				WithGOARCHMapping(map[string]string{"mips": "mipsel"}),
			)

			assert.Equal(t, runtime.GOARCH, b.template.GOARCH)
		},
	)

	t.Run("archive extension mapping without match keeps default",
		func(t *testing.T) {
			var origin *fakeorigin
			b := New("util", "1.0.0", origin,
				WithGOOSArchiveExtensionMapping(map[string]string{"someotheros": ".zip"}),
			)

			assert.Equal(t, ".tar.gz", b.template.ArchiveExtension)
		},
	)

	t.Run("with custom version cmd",
		func(t *testing.T) {
			var origin *fakeorigin
			b := New("util", "1.0.0", origin,
				WithVersionCmd("%s version"),
			)

			wantVersionCmd := fmt.Sprintf("%s version", b.template.Cmd)
			assert.Equal(t, wantVersionCmd, b.versioncmd)
		},
	)

	t.Run("with skip version check",
		func(t *testing.T) {
			var origin *fakeorigin
			b := New("util", "1.0.0", origin,
				WithVersionCmd(SkipVersionCheck),
			)

			assert.Equal(t, SkipVersionCheck, b.versioncmd)
		},
	)
}

func TestEnsure(t *testing.T) {
	t.Run("missing version",
		func(t *testing.T) {
			origin := new(fakeorigin)
			withTempDir(t)

			bin := New("util", "", origin)

			err := bin.Ensure()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "version must be set")
		},
	)

	t.Run("not installed",
		func(t *testing.T) {
			origin := new(fakeorigin)
			withTempDir(t)

			bin := New("util", "1.0.0", origin,
				WithVersionCmd(SkipVersionCheck),
			)
			require.NoError(t, bin.Ensure())
			assert.True(t, origin.installed, "install should have been called")
		},
	)

	t.Run("not installed and download fails",
		func(t *testing.T) {
			origin := &fakeorigin{err: fmt.Errorf("download failed")}
			withTempDir(t)

			bin := New("util", "1.0.0", origin,
				WithVersionCmd(SkipVersionCheck),
			)

			err := bin.Ensure()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "download failed")
		},
	)

	t.Run("already installed and version check skipped",
		func(t *testing.T) {
			origin := new(fakeorigin)
			withTempDir(t)

			bin := New("util", "1.0.0", origin,
				WithVersionCmd(SkipVersionCheck),
			)

			// pre-create the binary so it appears installed
			dir := filepath.FromSlash("./bin")
			require.NoError(t, os.MkdirAll(dir, 0o755))
			require.NoError(t, os.WriteFile(bin.BinPath(), []byte("existing"), 0o755))

			require.NoError(t, bin.Ensure())
			assert.False(t, origin.installed, "install shouldn't have been called for an existing binary without version check")
		},
	)

	t.Run("already installed and version is latest",
		func(t *testing.T) {
			origin := new(fakeorigin)
			withTempDir(t)

			bin := New("util", "latest", origin,
				WithVersionCmd(SkipVersionCheck),
			)

			// pre-create the binary so it appears installed
			dir := filepath.FromSlash("./bin")
			require.NoError(t, os.MkdirAll(dir, 0o755))
			require.NoError(t, os.WriteFile(bin.BinPath(), []byte("existing"), 0o755))

			require.NoError(t, bin.Ensure())
			assert.False(t, origin.installed, "install shouldn't have been called for an existing binary with 'latest' as version")
		},
	)

	t.Run("already installed and version matches",
		func(t *testing.T) {
			origin := new(fakeorigin)
			withTempDir(t)

			bin := New("util", "2.5.0", origin)

			// pre-create the binary so it appears installed
			dir := filepath.FromSlash("./bin")
			require.NoError(t, os.MkdirAll(dir, 0o755))
			require.NoError(t, os.WriteFile(bin.BinPath(), []byte("#!/bin/sh\necho 'util version 2.5.0'"), 0o755))

			require.NoError(t, bin.Ensure())
			assert.False(t, origin.installed, "install shouldn't have been called since version matches")
		},
	)

	t.Run("already installed but version doesn't match",
		func(t *testing.T) {
			origin := new(fakeorigin)
			withTempDir(t)

			bin := New("util", "2.5.0", origin)

			// pre-create the binary so it appears installed
			dir := filepath.FromSlash("./bin")
			require.NoError(t, os.MkdirAll(dir, 0o755))
			require.NoError(t, os.WriteFile(bin.BinPath(), []byte("#!/bin/sh\necho 'util version 2.3.0'"), 0o755))

			require.NoError(t, bin.Ensure())
			assert.True(t, origin.installed, "install should have been called since the installed bin is older")
		},
	)

	t.Run("already installed and version matches but has v in front",
		func(t *testing.T) {
			origin := new(fakeorigin)
			withTempDir(t)

			bin := New("util", "v2.5.0", origin)

			// pre-create the binary so it appears installed
			dir := filepath.FromSlash("./bin")
			require.NoError(t, os.MkdirAll(dir, 0o755))
			require.NoError(t, os.WriteFile(bin.BinPath(), []byte("#!/bin/sh\necho 'util version 2.5.0'"), 0o755))

			require.NoError(t, bin.Ensure())
			assert.False(t, origin.installed, "install shouldn't have been called since the 'v' prefix is stripped")
		},
	)
}

func TestRemoteBinaryDownload(t *testing.T) {
	srv := setupTestServer(t)
	withTempDir(t)

	origin := RemoteBinaryDownload(srv.URL + "/util")
	bin := New("util", "1.2.3", origin)

	require.NoError(t, bin.Ensure())
	assert.FileExists(t, bin.BinPath())

	// calling Ensure again should be a no-op (binary exists and version matches)
	origin2 := &fakeorigin{}
	b2 := New("util", "1.2.3", origin2)
	require.NoError(t, b2.Ensure())
	assert.False(t, origin2.installed, "second Ensure() should not have triggered install")
}

func TestRemoteArchiveDownload(t *testing.T) {
	srv := setupTestServer(t)
	withTempDir(t)

	origin := RemoteArchiveDownload(
		srv.URL+"/util.tar.gz",
		map[string]string{"util": "util"},
	)
	bin := New("util", "1.2.3", origin)

	require.NoError(t, bin.Ensure())
	assert.FileExists(t, bin.BinPath())
}

// fakeorigin is a mock Origin that records whether Install was called
// and optionally returns an error.
type fakeorigin struct {
	installed bool
	err       error
}

func (f *fakeorigin) Install(tmpl Template) error {
	f.installed = true
	if f.err != nil {
		return f.err
	}
	// create the binary file so isInstalled() returns true on subsequent checks
	if err := os.MkdirAll(tmpl.Directory, 0o755); err != nil {
		return err
	}
	return os.WriteFile(tmpl.Cmd, []byte("fake"), 0o755)
}

// withTempDir changes the working directory to a temp dir for the test
// and restores it afterward. Returns the temp dir path.
func withTempDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	current, err := os.Getwd()

	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))

	t.Cleanup(func() { os.Chdir(current) })

	return dir
}
