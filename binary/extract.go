package binary

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// handles .tar.gz files
func untar(file io.Reader, destination string, processor func(path string) *string) (err error) {
	decompressor, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		if closerr := decompressor.Close(); closerr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close gzip reader: %w", closerr))
		}
	}()

	reader := tar.NewReader(decompressor)

	for {
		header, err := reader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		processed := processor(header.Name)
		if processed == nil {
			continue
		}
		target := filepath.Join(destination, *processed)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(target), err)
			}

			out, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", target, err)
			}
			defer func() {
				if closerr := out.Close(); closerr != nil {
					err = errors.Join(err, fmt.Errorf("failed to close file %s: %w", target, closerr))
				}
			}()

			if err := os.Chmod(target, 0o755); err != nil {
				return fmt.Errorf("failed to set permissions on %s: %w", target, err)
			}

			if _, err := io.Copy(out, reader); err != nil {
				return fmt.Errorf("failed to copy data to file %s: %w", target, err)
			}
		}
	}

	return nil
}

// handles .zip files
func unzip(file io.ReaderAt, size int64, destination string, processor func(path string) *string) (err error) {
	reader, err := zip.NewReader(file, size)
	if err != nil {
		return fmt.Errorf("failed to create zip reader: %w", err)
	}

	for _, file := range reader.File {
		processed := processor(file.Name)
		if processed == nil {
			continue
		}
		target := filepath.Join(destination, *processed)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(target), err)
		}

		out, err := os.Create(target)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", target, err)
		}
		defer func() {
			if closerr := out.Close(); closerr != nil {
				err = errors.Join(err, fmt.Errorf("failed to close file %s: %w", target, closerr))
			}
		}()

		if err := os.Chmod(target, 0o755); err != nil {
			return fmt.Errorf("failed to set permissions on %s: %w", target, err)
		}

		contents, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open compressed file %s: %w", file.Name, err)
		}
		defer func() {
			if closerr := contents.Close(); closerr != nil {
				err = errors.Join(err, fmt.Errorf("failed to close compressed file %s: %w", file.Name, closerr))
			}
		}()

		if _, err := io.Copy(out, contents); err != nil {
			return fmt.Errorf("failed to copy data to file %s: %w", target, err)
		}
	}

	return nil
}
