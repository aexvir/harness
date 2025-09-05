package binary

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockOrigin is a testify mock implementation of Origin interface
type MockOrigin struct {
	mock.Mock
}

func (m *MockOrigin) Install(template Template) error {
	args := m.Called(template)
	return args.Error(0)
}

func TestNew(t *testing.T) {
	mockOrig := &MockOrigin{}
	
	binary := New("test-cmd", "v1.0.0", mockOrig)
	
	require.NotNil(t, binary)
	assert.Equal(t, "test-cmd", binary.command)
	assert.Equal(t, "v1.0.0", binary.version)
	assert.Equal(t, "./bin", binary.directory)
	assert.Equal(t, mockOrig, binary.origin)
	
	// Check template is properly initialized
	assert.Equal(t, runtime.GOOS, binary.template.GOOS)
	assert.Equal(t, runtime.GOARCH, binary.template.GOARCH)
	assert.Equal(t, "test-cmd", binary.template.Name)
	assert.Equal(t, "v1.0.0", binary.template.Version)
	
	// Check platform-specific extension
	if runtime.GOOS == "windows" {
		assert.Equal(t, ".exe", binary.template.Extension)
		assert.Contains(t, binary.template.Cmd, ".exe")
	} else {
		assert.Equal(t, "", binary.template.Extension)
		assert.NotContains(t, binary.template.Cmd, ".exe")
	}
}

func TestNewWithOptions(t *testing.T) {
	mockOrig := &MockOrigin{}
	
	binary := New("test-cmd", "v1.0.0", mockOrig, 
		WithVersionCmd("%s version"),
		WithGOOSMapping(map[string]string{"linux": "linux-gnu"}),
	)
	
	require.NotNil(t, binary)
	
	// Version command should be customized
	expectedCmd := filepath.Join("./bin", "test-cmd")
	if runtime.GOOS == "windows" {
		expectedCmd += ".exe"
	}
	assert.Contains(t, binary.versioncmd, "version")
	
	// GOOS mapping should be applied if applicable
	if runtime.GOOS == "linux" {
		assert.Equal(t, "linux-gnu", binary.template.GOOS)
	}
}

func TestBinary_Name(t *testing.T) {
	mockOrig := &MockOrigin{}
	binary := New("my-tool", "v1.0.0", mockOrig)
	
	assert.Equal(t, "my-tool", binary.Name())
}

func TestBinary_BinPath(t *testing.T) {
	mockOrig := &MockOrigin{}
	binary := New("my-tool", "v1.0.0", mockOrig)
	
	binPath := binary.BinPath()
	assert.Contains(t, binPath, "my-tool")
	assert.Contains(t, binPath, "bin")
	
	if runtime.GOOS == "windows" {
		assert.Contains(t, binPath, ".exe")
	}
}

func TestBinary_Ensure(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "binary-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	t.Run("error when version is empty", func(t *testing.T) {
		mockOrig := &MockOrigin{}
		binary := New("test-cmd", "", mockOrig)
		
		err := binary.Ensure()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "version must be set")
	})
	
	t.Run("calls install when binary not installed", func(t *testing.T) {
		mockOrig := &MockOrigin{}
		binary := New("test-cmd", "v1.0.0", mockOrig)
		// Change directory to tmpDir so binary won't be found
		binary.directory = tmpDir
		binary.template.Directory = tmpDir
		binary.template.Cmd = filepath.Join(tmpDir, "test-cmd")
		
		// Set up mock expectation
		mockOrig.On("Install", mock.AnythingOfType("Template")).Return(nil)
		
		err := binary.Ensure()
		assert.NoError(t, err)
		
		// Verify the mock was called as expected
		mockOrig.AssertExpectations(t)
	})
	
	t.Run("skips install when binary exists and version matches", func(t *testing.T) {
		mockOrig := &MockOrigin{}
		binary := New("test-cmd", "latest", mockOrig) // "latest" always matches
		
		// Create the binary file
		binary.directory = tmpDir
		binary.template.Directory = tmpDir
		binary.template.Cmd = filepath.Join(tmpDir, "test-cmd")
		
		file, err := os.Create(binary.template.Cmd)
		require.NoError(t, err)
		file.Close()
		
		err = binary.Ensure()
		assert.NoError(t, err)
		
		// Install should not have been called
		mockOrig.AssertNotCalled(t, "Install")
	})
}

func TestBinary_Install(t *testing.T) {
	mockOrig := &MockOrigin{}
	binary := New("test-cmd", "v1.0.0", mockOrig)
	
	// Set up mock expectation
	mockOrig.On("Install", mock.AnythingOfType("Template")).Return(nil)
	
	err := binary.Install()
	assert.NoError(t, err)
	
	// Verify the mock was called as expected
	mockOrig.AssertExpectations(t)
}

func TestBinary_isInstalled(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "binary-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	mockOrig := &MockOrigin{}
	binary := New("test-cmd", "v1.0.0", mockOrig)
	binary.template.Cmd = filepath.Join(tmpDir, "test-cmd")
	
	// Binary should not be installed initially
	assert.False(t, binary.isInstalled())
	
	// Create the binary file
	file, err := os.Create(binary.template.Cmd)
	require.NoError(t, err)
	file.Close()
	
	// Now it should be installed
	assert.True(t, binary.isInstalled())
}

func TestBinary_isExpectedVersion(t *testing.T) {
	mockOrig := &MockOrigin{}
	
	t.Run("returns true for latest version", func(t *testing.T) {
		binary := New("test-cmd", "latest", mockOrig)
		assert.True(t, binary.isExpectedVersion())
	})
	
	t.Run("returns true when version check is skipped", func(t *testing.T) {
		binary := New("test-cmd", "v1.0.0", mockOrig, WithVersionCmd(SkipVersionCheck))
		assert.True(t, binary.isExpectedVersion())
	})
	
	t.Run("returns false when command fails", func(t *testing.T) {
		binary := New("test-cmd", "v1.0.0", mockOrig)
		binary.versioncmd = "non-existent-command --version"
		assert.False(t, binary.isExpectedVersion())
	})
}