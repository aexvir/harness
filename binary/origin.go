package binary

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/princjef/mageutil/shellcmd"
)

type Origin interface {
	Install(template Template) error
}

type remotebin struct {
	urlformat string
	// could add auth, headers, sha verification, etc.
}

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
		return err
	}

	logstep(fmt.Sprintf("downloading from %s", url))

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("received unexpected response when downloading binary: http%d", resp.StatusCode)
	}

	data, finish := progress(resp.Body, resp.ContentLength)
	defer finish()

	out, err := os.Create(template.Cmd)
	if err != nil {
		return err
	}
	defer out.Close()

	if err := os.Chmod(template.Cmd, 0o755); err != nil {
		return err
	}

	_, err = io.Copy(out, data)
	return err
}

type remotearchive struct {
	urlformat string
	binaries  map[string]string
}

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
		return err
	}

	tmpname := filepath.Base(url)

	if err := download(url, filepath.Join(template.Directory, tmpname)); err != nil {
		return err
	}

	return extract(
		filepath.Join(template.Directory, tmpname),
		template.Directory,
		func(path string) *string {
			if replacement, ok := r.binaries[path]; ok {
				return &replacement
			}
			return nil
		},
	)
}

type gopkg struct {
	pkg string
}

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
		return err
	}

	installcmd := fmt.Sprintf("GOBIN=%s go install %s@%s", path, o.pkg, template.Version)
	if _, err := shellcmd.Command(installcmd).Output(); err != nil {
		return fmt.Errorf("unable to install executable: %w", err)
	}

	// rename if name is different

	return nil
}

func download(url, destination string) error {
	logstep(fmt.Sprintf("downloading %s to %s", url, destination))
	var err error

	start := time.Now()
	defer func() {
		elapsed := time.Since(start).Round(time.Millisecond)
		if err != nil {
			color.Red(" ✘ %s", elapsed)
			return
		}
		color.Green(" ✔ %s", elapsed)
	}()

	if _, err := os.Stat(destination); err == nil {
		return nil
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extract(compressed, destination string, processor func(path string) *string) error {
	logstep(fmt.Sprintf("extracting %s", compressed))
	var err error

	start := time.Now()
	defer func() {
		elapsed := time.Since(start).Round(time.Millisecond)
		if err != nil {
			color.Red(" ✘ %s", elapsed)
			return
		}
		color.Green(" ✔ %s", elapsed)
	}()

	file, err := os.Open(compressed)
	if err != nil {
		return err
	}
	defer file.Close()

	decompressor, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer decompressor.Close()

	reader := tar.NewReader(decompressor)

	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
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
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			defer out.Close()

			_ = os.Chmod(target, 0o755)

			if _, err := io.Copy(out, reader); err != nil {
				return err
			}
		}
	}

	return os.Remove(compressed)
}

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
		color.BlueString(">"),
		color.New(color.Bold).Sprint(text),
	)
}
