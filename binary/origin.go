package binary

import (
	"archive/tar"
	"compress/gzip"
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
	"github.com/mattn/go-isatty"
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
}

// RemoteBinaryDownload creates a new Origin that downloads a binary directly from a URL.
// The URL can contain template variables that will be resolved using the [Template] values
// during installation.
// e.g. "https://github.com/foo/bar/releases/download/v{{.Version}}/bin_{{.Version}}_{{.GOOS}}_{{.GOARCH}}{{.Extension}}",
func RemoteBinaryDownload(url string) Origin {
	return &remotebin{
		urlformat: url,
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

	logstep(fmt.Sprintf("downloading from %s", url))

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("received unexpected response when downloading binary: http%d", resp.StatusCode)
	}

	data, finish := progress(resp.Body, resp.ContentLength)
	defer finish()

	out, err := os.Create(template.Cmd)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", template.Cmd, err)
	}
	defer out.Close()

	if err := os.Chmod(template.Cmd, 0o755); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", template.Cmd, err)
	}

	_, err = io.Copy(out, data)
	return err
}

// remotearchive implements Origin for downloading and extracting archived binaries.
// It supports downloading compressed archives (tar.gz) containing multiple files
// and selectively extracting specific binaries from them.
type remotearchive struct {
	urlformat string
	binaries  map[string]string
}

// RemoteArchiveDownload creates a new Origin that downloads and extracts binaries from
// a compressed archive. The URL can contain template variables that will be resolved
// using the [Template] values during installation.
// e.g. "https://github.com/aevea/commitsar/releases/download/v{{.Version}}/commitsar_{{.Version}}_{{.GOOS}}_{{.GOARCH}}.tar.gz",
//
// The binaries parameter maps archive paths to the desired binary names in the
// installation directory. Only files specified in this map will be extracted.
func RemoteArchiveDownload(url string, binaries map[string]string) Origin {
	return &remotearchive{
		urlformat: url,
		binaries:  binaries,
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

	if err := download(url, filepath.Join(template.Directory, tmpname)); err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	return extract(
		filepath.Join(template.Directory, tmpname),
		template.Directory,
		func(path string) *string {
			// if there's no file override, extract the file as is
			if len(r.binaries) == 0 {
				return &path
			}

			// otherwise only extract files that are present in the map
			if replacement, ok := r.binaries[path]; ok {
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
	logstep(fmt.Sprintf("running %s", installcmd))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unable to install executable: %w", err)
	}

	// rename if name is different

	return nil
}

// download downloads a file from a URL to a local destination.
// If the destination file already exists, the download is skipped.
func download(url, destination string) (err error) {
	logstep(fmt.Sprintf("downloading %s to %s", url, destination))

	start := time.Now()
	defer func() {
		elapsed := time.Since(start).Round(time.Millisecond)
		if err != nil {
			color.Red("   ✘ %s", elapsed)
			return
		}
		color.Green("   ✔ %s", elapsed)
	}()

	if _, err := os.Stat(destination); err == nil {
		return nil
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", destination, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy data to file %s: %w", destination, err)
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
	logstep(fmt.Sprintf("extracting %s", compressed))

	start := time.Now()
	defer func() {
		elapsed := time.Since(start).Round(time.Millisecond)
		if err != nil {
			color.Red("   ✘ %s", elapsed)
			return
		}
		color.Green("   ✔ %s", elapsed)
	}()

	file, err := os.Open(compressed)
	if err != nil {
		return fmt.Errorf("failed to open compressed file: %w", err)
	}
	defer file.Close()

	decompressor, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer decompressor.Close()

	reader := tar.NewReader(decompressor)

	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	for {
		header, err := reader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("error while extracting %s: %w", compressed, err)
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
			defer out.Close()

			_ = os.Chmod(target, 0o755)

			if _, err := io.Copy(out, reader); err != nil {
				return fmt.Errorf("failed to copy data to file %s: %w", target, err)
			}
		}
	}

	return os.Remove(compressed)
}

// progress wraps an io.Reader to display a progress bar when running in a terminal.
// Returns the wrapped reader and a function to finalize the progress display.
// The progress bar shows transfer speed and completion percentage.
func progress(reader io.Reader, size int64) (io.Reader, func()) {
	if !isatty.IsTerminal(os.Stderr.Fd()) && !isatty.IsCygwinTerminal(os.Stderr.Fd()) {
		return reader, func() {}
	}

	bar := pb.
		New64(size).
		SetTemplate(
			pb.ProgressBarTemplate(
				`{{string . "prefix"}}{{counters . }}` +
					` {{bar . "[" "=" ">" " " "]" }} {{percent . }}` +
					` {{speed . "%s/s" }}{{string . "suffix"}}`,
			),
		).
		SetRefreshRate(time.Second / 60).
		SetMaxWidth(100).
		Start()

	return bar.NewProxyReader(reader), func() { bar.Finish() }
}

func logstep(text string) {
	fmt.Println(
		color.BlueString(" •"),
		color.New(color.FgHiBlack).Sprint(text),
	)
}
