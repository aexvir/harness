package binary

import (
	"crypto"
	_ "crypto/sha256" // register sha224, sha256
	_ "crypto/sha512" // register sha384, sha512
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// Platform identifies a specific OS and architecture pair, matching the
// values Go uses for GOOS and GOARCH.
type Platform struct {
	OS   string
	Arch string
}

// Checksum pairs a hash algorithm with its expected hex-encoded value.
type Checksum struct {
	Algorithm crypto.Hash
	Value     string
}

// crcreader wraps r so bytes are fed into a hasher as they're read.
// The returned check function validates the accumulated hash against sum.
func crcreader(reader io.Reader, sum Checksum) (io.Reader, func() error, error) {
	if !sum.Algorithm.Available() {
		return nil, nil, fmt.Errorf("hash algorithm %s is not available", sum.Algorithm)
	}

	hasher := sum.Algorithm.New()
	check := func() error {
		got := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(got, sum.Value) {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", sum.Value, got)
		}
		return nil
	}

	return io.TeeReader(reader, hasher), check, nil
}

// crcfile hashes a file on disk and compares it against sum.
func crcfile(path string, sum Checksum) (err error) {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open %s for verification: %w", path, err)
	}

	defer func() {
		if closerr := file.Close(); closerr != nil {
			err = fmt.Errorf("failed to close %s: %w", path, closerr)
		}
	}()

	reader, check, err := crcreader(file, sum)
	if err != nil {
		return err
	}

	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("failed to read %s for verification: %w", path, err)
	}

	return check()
}
