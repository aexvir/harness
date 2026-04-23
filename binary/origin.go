package binary

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/fatih/color"

	"github.com/aexvir/harness/internal"
)

// Origin defines the interface for provisioning binaries from different sources.
type Origin interface {
	// Install performs the installation of a binary.
	// The template contains information about the target environment and desired configuration.
	Install(template Template) error
}

// remotebin implements [Origin] for direct binary downloads from a URL.
// It supports downloading a single executable file from a remote location.
type remotebin struct {
	urlformat string
	config    origincfg
}

// RemoteBinaryDownload creates a new Origin that downloads a binary directly from a URL.
// The URL can contain template variables that will be resolved using the [Template] values
// during installation.
// e.g. "https://github.com/foo/bar/releases/download/v{{.Version}}/bin_{{.Version}}_{{.GOOS}}_{{.GOARCH}}{{.Extension}}",
//
// Pass [WithChecksums] to verify the downloaded file against a known hash.
func RemoteBinaryDownload(url string, options ...OriginOption) Origin {
	var cfg origincfg
	for _, opt := range options {
		opt(&cfg)
	}
	return &remotebin{
		urlformat: url,
		config:    cfg,
	}
}

func (r *remotebin) Install(template Template) error {
	if err := os.MkdirAll(template.Directory, 0o755); err != nil {
		return fmt.Errorf("failed to create destination folder %s: %w", template.Directory, err)
	}

	url, err := template.Resolve(r.urlformat)
	if err != nil {
		return fmt.Errorf("failed to resolve URL: %w", err)
	}

	internal.LogStep(fmt.Sprintf("downloading from %s", url))

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer func() {
		if closerr := resp.Body.Close(); closerr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close http response body: %w", closerr))
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("received unexpected response when downloading binary: http%d", resp.StatusCode)
	}

	data, finish := progress(resp.Body, resp.ContentLength)
	defer finish()

	var verify func() error
	if sum, ok := r.config.checksum(template); ok {
		verified, check, err := crcreader(data, sum)
		if err != nil {
			return err
		}
		data = verified
		verify = check
	}

	out, err := os.Create(template.Cmd)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", template.Cmd, err)
	}
	defer func() {
		if closerr := out.Close(); closerr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close file %s: %w", template.Cmd, closerr))
		}
	}()

	if err := os.Chmod(template.Cmd, 0o755); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", template.Cmd, err)
	}

	if _, err := io.Copy(out, data); err != nil {
		return err
	}

	if verify != nil {
		if err := verify(); err != nil {
			_ = os.Remove(template.Cmd)
			return err
		}
	}

	return nil
}

// remotearchive implements Origin for downloading and extracting archived binaries.
// It supports downloading compressed archives (tar.gz) containing multiple files
// and selectively extracting specific binaries from them.
type remotearchive struct {
	urlformat string
	binaries  map[string]string
	config    origincfg
}

// RemoteArchiveDownload creates a new Origin that downloads and extracts binaries from
// a compressed archive. The URL can contain template variables that will be resolved
// using the [Template] values during installation.
// e.g. "https://github.com/aevea/commitsar/releases/download/v{{.Version}}/commitsar_{{.Version}}_{{.GOOS}}_{{.GOARCH}}{{.ArchiveExtension}}",
//
// The binaries parameter maps archive paths to the desired binary names in the
// installation directory. Only files specified in this map will be extracted.
// Both archive paths and binary names can contain template variables that will be resolved
// using the [Template] values during installation.
//
// e.g. {"grafana-v{{.Version}}/bin/grafana-server": "grafana"} will resolve the path by replacing
// the version in the string and will extract the file under that path to a binary called simply
// "grafana" in the root of the bin directory.
//
// Pass [WithChecksums] to verify the downloaded archive against a known hash.
func RemoteArchiveDownload(url string, binaries map[string]string, options ...OriginOption) Origin {
	var cfg origincfg
	for _, opt := range options {
		opt(&cfg)
	}
	return &remotearchive{
		urlformat: url,
		binaries:  binaries,
		config:    cfg,
	}
}

func (r *remotearchive) Install(template Template) error {
	if err := os.MkdirAll(template.Directory, 0o755); err != nil {
		return fmt.Errorf("failed to create destination folder %s: %w", template.Directory, err)
	}

	url, err := template.Resolve(r.urlformat)
	if err != nil {
		return fmt.Errorf("failed to resolve URL: %w", err)
	}

	tmpname := filepath.Base(url)

	var sum *Checksum
	if expected, ok := r.config.checksum(template); ok {
		sum = &expected
	}

	if err := download(url, filepath.Join(template.Directory, tmpname), sum); err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	// resolve binary mapping templates
	mapping := make(map[string]string, len(r.binaries))
	for path, replacement := range r.binaries {
		mapping[template.MustResolve(path)] = template.MustResolve(replacement)
	}

	return extract(
		filepath.Join(template.Directory, tmpname),
		template.Directory,
		func(path string) *string {
			// if there's no file override, extract the file as is
			if len(mapping) == 0 {
				return &path
			}

			// otherwise only extract files that are present in the map
			if replacement, ok := mapping[path]; ok {
				internal.LogDetail(fmt.Sprintf("  resolved %s to %s", path, replacement))
				return &replacement
			}
			return nil
		},
	)
}

// gopkg implements Origin for installing binaries using Go's package management.
// It provisions binaries via 'go install'.
type gopkg struct {
	pkg string
}

// GoBinary creates a new Origin that installs a binary using 'go install'
// targetting the local bin directory.
// The pkg parameter should be a package installable using the go cli.
// e.g. golang.org/x/tools/cmd/goimports
func GoBinary(pkg string) Origin {
	return &gopkg{
		pkg: pkg,
	}
}

func (o *gopkg) Install(template Template) error {
	if err := os.MkdirAll(template.Directory, 0o755); err != nil {
		return fmt.Errorf("failed to create destination folder %s: %w", template.Directory, err)
	}

	path, err := filepath.Abs(template.Directory)
	if err != nil {
		return fmt.Errorf("failed to resolve dir %s: %w", template.Directory, err)
	}

	cmd := exec.Command("go", "install", o.pkg+"@"+template.Version)
	cmd.Env = append(os.Environ(), "GOBIN="+path)
	installcmd := fmt.Sprintf("GOBIN=%s go install %s@%s", path, o.pkg, template.Version)
	internal.LogDetail(fmt.Sprintf("running %s", installcmd))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unable to install executable: %w", err)
	}

	// rename if binary name is different from template
	if currentBinaryName := filepath.Base(o.pkg); currentBinaryName != template.Name {
		internal.LogDetail("renaming binary from " + currentBinaryName + " to " + template.Name)
		return os.Rename(
			fmt.Sprintf("%s/%s", path, currentBinaryName),
			fmt.Sprintf("%s/%s", path, template.Name),
		)
	}

	return nil
}

// download downloads a file from a URL to a local destination.
// If the destination file already exists, the download is skipped.
// When sum is non-nil, the downloaded (or cached) file is verified against it.
// A cached file that does not match is removed and re-downloaded.
func download(url, destination string, sum *Checksum) (err error) {
	internal.LogDetail(fmt.Sprintf("downloading %s to %s", url, destination))

	start := time.Now()
	defer func() {
		elapsed := time.Since(start).Round(time.Millisecond)
		internal.LogStatus(elapsed.String(), err)
	}()

	if _, err := os.Stat(destination); err == nil {
		if sum == nil {
			return nil
		}
		if verr := crcfile(destination, *sum); verr == nil {
			return nil
		}
		internal.LogDetail("cached file failed checksum verification, re-downloading")
		if rmerr := os.Remove(destination); rmerr != nil {
			return fmt.Errorf("failed to remove invalid cached file %s: %w", destination, rmerr)
		}
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer func() {
		if closerr := resp.Body.Close(); closerr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close http response body: %w", closerr))
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected response when downloading archive: http%d", resp.StatusCode)
	}

	data, finish := progress(resp.Body, resp.ContentLength)
	defer finish()

	var verify func() error
	if sum != nil {
		verified, check, err := crcreader(data, *sum)
		if err != nil {
			return err
		}
		data = verified
		verify = check
	}

	out, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", destination, err)
	}
	defer func() {
		if closerr := out.Close(); closerr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close file %s: %w", destination, closerr))
		}
	}()

	if _, err := io.Copy(out, data); err != nil {
		return fmt.Errorf("failed to copy data to file %s: %w", destination, err)
	}

	if verify != nil {
		if verr := verify(); verr != nil {
			_ = os.Remove(destination)
			return verr
		}
	}

	return nil
}

// extract extracts files from a compressed tar.gz archive.
// The processor function is called for each file in the archive and determines:
// - Which files to extract (by returning non-nil)
// - What name to give the extracted file (the returned string value)
// Files are extracted with executable permissions (0755).
// The source archive is removed after successful extraction.
func extract(compressed, destination string, processor func(path string) *string) (err error) {
	internal.LogDetail(fmt.Sprintf("extracting %s", compressed))

	start := time.Now()
	defer func() {
		elapsed := time.Since(start).Round(time.Millisecond)
		internal.LogStatus(elapsed.String(), err)
	}()

	file, err := os.Open(compressed)
	if err != nil {
		return fmt.Errorf("failed to open compressed file: %w", err)
	}
	defer func() {
		if closerr := file.Close(); closerr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close file %s: %w", compressed, closerr))
		}
		_ = os.Remove(compressed)
	}()

	// sniff mime header to determine file type
	header := make([]byte, 512)
	if _, err := file.Read(header); err != nil {
		return fmt.Errorf("failed to read file header: %w", err)
	}
	mime := http.DetectContentType(header)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	switch mime {
	case "application/x-gzip":
		return untar(file, destination, processor)
	case "application/zip":
		info, _ := file.Stat()
		return unzip(file, info.Size(), destination, processor)
	default:
		return fmt.Errorf("unsupported format: %s", mime)
	}
}

// progress wraps an io.Reader to display a progress bar when running in a terminal.
// Returns the wrapped reader and a function to finalize the progress display.
// The progress bar shows transfer speed and completion percentage.
func progress(reader io.Reader, size int64) (io.Reader, func()) {
	if !internal.IsTerminalWriter(internal.Output) {
		return reader, func() {}
	}

	bar := pb.
		New64(size).
		SetWriter(internal.Output).
		SetTemplate(
			pb.ProgressBarTemplate(
				color.New(color.FgHiBlack).Sprint(
					`   ` + internal.Symbols.Detail + ` {{string . "prefix"}}{{counters . }}` +
						` {{bar . "[" "=" ">" " " "]" }} {{percent . }}` +
						` {{speed . "%s/s" }}{{string . "suffix"}}`,
				),
			),
		).
		SetRefreshRate(time.Second / 60).
		SetMaxWidth(100).
		Start()

	return bar.NewProxyReader(reader), func() { bar.Finish() }
}

// OriginOption configures optional behavior for an [Origin].
type OriginOption func(*origincfg)

// origincfg accumulates optional configuration shared across origins.
type origincfg struct {
	checksums map[Platform]Checksum
}

// WithChecksums enables integrity verification of the downloaded file
// using known hashes keyed by platform.
//
// If the current platform is not present in the map, verification is
// skipped for that install. On a mismatch, the downloaded file is removed
// and an error is returned.
//
// example:
//
//	binary.RemoteBinaryDownload(
//		"https://example.com/bin_{{.GOOS}}_{{.GOARCH}}",
//		binary.WithChecksums(map[binary.Platform]binary.Checksum{
//			{OS: "darwin", Arch: "arm64"}: {Algorithm: crypto.SHA256, Value: "abc..."},
//			{OS: "linux",  Arch: "amd64"}: {Algorithm: crypto.SHA256, Value: "def..."},
//		}),
//	)
func WithChecksums(checksums map[Platform]Checksum) OriginOption {
	return func(c *origincfg) {
		c.checksums = checksums
	}
}

// checksum returns the checksum configured for the current template's
// platform, if any.
func (c origincfg) checksum(t Template) (Checksum, bool) {
	sum, ok := c.checksums[Platform{OS: t.GOOS, Arch: t.GOARCH}]
	return sum, ok
}
