package harness

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWithDirRelativePath(t *testing.T) {
	// Create a temporary directory structure to test with
	tempDir, err := os.MkdirTemp("", "harness-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create subdirectories
	binDir := filepath.Join(tempDir, "bin")
	frontendDir := filepath.Join(tempDir, "frontend")
	
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("Failed to create bin dir: %v", err)
	}
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	// Create a test script in bin directory
	testScript := filepath.Join(binDir, "test-script")
	content := "#!/bin/bash\necho 'success'\n"
	if err := os.WriteFile(testScript, []byte(content), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	// Change to temp directory to make bin/test-script a relative path
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current dir: %v", err)
	}
	defer os.Chdir(oldWd)
	
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	// Test 1: This should work - relative path should be resolved relative to current dir, not the WithDir target
	ctx := context.Background()
	
	// Create command with relative path and WithDir - this currently fails
	runner, err := Cmd(ctx, "bin/test-script", WithDir("frontend"))
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// The command should use the absolute path to bin/test-script
	// and run in the frontend directory
	expectedPath := filepath.Join(tempDir, "bin", "test-script")
	if runner.Executable != expectedPath {
		t.Errorf("Expected executable path %q, got %q", expectedPath, runner.Executable)
	}
	
	// The cmd.Path should also be absolute
	if runner.cmd.Path != expectedPath {
		t.Errorf("Expected cmd.Path %q, got %q", expectedPath, runner.cmd.Path)
	}

	// The working directory should be set correctly
	expectedDir := filepath.Join(tempDir, "frontend")
	if runner.cmd.Dir != expectedDir {
		t.Errorf("Expected cmd.Dir %q, got %q", expectedDir, runner.cmd.Dir)
	}
}

func TestWithDirAbsolutePath(t *testing.T) {
	// Test that absolute paths are not modified
	ctx := context.Background()
	
	runner, err := Cmd(ctx, "/bin/echo", WithDir("/tmp"))
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Absolute paths should remain unchanged
	if runner.Executable != "/bin/echo" {
		t.Errorf("Expected executable path %q, got %q", "/bin/echo", runner.Executable)
	}
	
	if runner.cmd.Dir != "/tmp" {
		t.Errorf("Expected cmd.Dir %q, got %q", "/tmp", runner.cmd.Dir)
	}
}

func TestWithDirNoDir(t *testing.T) {
	// Test that behavior without WithDir is unchanged
	ctx := context.Background()
	
	runner, err := Cmd(ctx, "echo")
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// For relative path without WithDir, should resolve to absolute
	if !filepath.IsAbs(runner.Executable) {
		t.Errorf("Expected absolute executable path, got relative: %q", runner.Executable)
	}
	
	// Dir should be empty (current directory)
	if runner.cmd.Dir != "" {
		t.Errorf("Expected empty cmd.Dir, got %q", runner.cmd.Dir)
	}
}

func TestWithDirExecutionIntegration(t *testing.T) {
	// Create a temporary directory structure to test with
	tempDir, err := os.MkdirTemp("", "harness-integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create subdirectories
	binDir := filepath.Join(tempDir, "bin")
	workDir := filepath.Join(tempDir, "work")
	
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("Failed to create bin dir: %v", err)
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("Failed to create work dir: %v", err)
	}

	// Create a test script that outputs the current working directory
	testScript := filepath.Join(binDir, "pwd-script")
	content := "#!/bin/bash\npwd\n"
	if err := os.WriteFile(testScript, []byte(content), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	// Change to temp directory to make bin/pwd-script a relative path
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current dir: %v", err)
	}
	defer os.Chdir(oldWd)
	
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	// This is the key test: run a relative executable in a different directory
	// The executable should be found relative to the current directory (tempDir)
	// but executed in the work directory
	ctx := context.Background()
	
	err = Run(ctx, "bin/pwd-script", WithDir("work"), WithoutNoise())
	if err != nil {
		t.Errorf("Failed to run command with relative path and WithDir: %v", err)
	}
	
	// If we got here without error, the test passed - the script was found and executed
	// The fix ensures that "bin/pwd-script" is resolved relative to tempDir before 
	// changing the working directory to "work"
}