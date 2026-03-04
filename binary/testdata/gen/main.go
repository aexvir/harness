//go:build ignore

// This program generates test fixtures for the binary package tests.
// Run it with: go run binary/testdata/gen/main.go
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const binaryContent = `#!/bin/sh
echo "util version 1.2.3"
`

func main() {
	// resolve output dir relative to this source file so the path is correct regardless of working directory
	_, self, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(self), "..")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fatal(err)
	}

	// 1. Plain binary
	if err := os.WriteFile(filepath.Join(dir, "util"), []byte(binaryContent), 0o755); err != nil {
		fatal(err)
	}
	fmt.Println("created util")

	// 2. tar.gz with util at root
	if err := createTarGz(filepath.Join(dir, "util.tar.gz"), map[string]string{
		"util": binaryContent,
	}); err != nil {
		fatal(err)
	}
	fmt.Println("created util.tar.gz")

	// 3. tar.gz with util nested at myapp-1.2.3/bin/util
	if err := createTarGz(filepath.Join(dir, "nested.tar.gz"), map[string]string{
		"myapp-1.2.3/bin/util": binaryContent,
	}); err != nil {
		fatal(err)
	}
	fmt.Println("created nested.tar.gz")

	// 4. zip with util at root
	if err := createZip(filepath.Join(dir, "util.zip"), map[string]string{
		"util": binaryContent,
	}); err != nil {
		fatal(err)
	}
	fmt.Println("created util.zip")

	// 5. tar.gz with multiple files (for selective extraction tests)
	if err := createTarGz(filepath.Join(dir, "multi.tar.gz"), map[string]string{
		"util":      binaryContent,
		"README.md": "# readme\n",
		"LICENSE":   "MIT\n",
	}); err != nil {
		fatal(err)
	}
	fmt.Println("created multi.tar.gz")

	fmt.Println("done")
}

func createTarGz(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// create directories first for nested paths
	dirs := map[string]bool{}
	for name := range files {
		dir := filepath.Dir(name)
		for dir != "." && !dirs[dir] {
			dirs[dir] = true
			if err := tw.WriteHeader(&tar.Header{
				Name:     dir + "/",
				Typeflag: tar.TypeDir,
				Mode:     0o755,
				ModTime:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			}); err != nil {
				return err
			}
			dir = filepath.Dir(dir)
		}
	}

	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0o755,
			Typeflag: tar.TypeReg,
			ModTime:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}); err != nil {
			return err
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return err
		}
	}

	return nil
}

func createZip(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for name, content := range files {
		header := &zip.FileHeader{
			Name:               name,
			Method:             zip.Deflate,
			Modified:           time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatorVersion:     20,
			ExternalAttrs:      0o755 << 16,
			UncompressedSize64: uint64(len(content)),
		}
		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			return err
		}
	}

	return nil
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
