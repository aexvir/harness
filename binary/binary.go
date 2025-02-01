package binary

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Binary struct {
	commandFullPath string
	directory       string
	version         string

	versioncmd string

	origin   Origin
	template Template
}

func New(command, version string, origin Origin, options ...Option) (*Binary, error) {
	binDir := filepath.FromSlash("./bin")

	if runtime.GOOS == "windows" {
		command += ".exe"
	}

	bin := Binary{
		commandFullPath: filepath.Join(binDir, command),
		directory:       binDir,
		version:         strings.TrimPrefix(version, "v"),

		versioncmd: fmt.Sprintf("%s --version", filepath.Join(binDir, command)),

		origin: origin,
	}

	bin.template = Template{
		GOOS:   runtime.GOOS,
		GOARCH: runtime.GOARCH,

		Directory: bin.directory,
		Name:      command,
		Cmd:       bin.commandFullPath,
		Version:   bin.version,
	}

	if runtime.GOOS == "windows" {
		bin.template.Extension = ".exe"
	}

	for _, opt := range options {
		opt(&bin)
	}

	return &bin, nil
}

func (b *Binary) BinPath() string {
	return b.template.Cmd
}

func (b *Binary) Ensure() error {
	if b.isInstalled() && b.isExpectedVersion() {
		return nil
	}
	logstep("downloading ")
	return b.Install()
}

func (b *Binary) Install() error {
	return b.origin.Install(b.template)
}

// isInstalled returns true if the binary is installed.
func (b *Binary) isInstalled() bool {
	_, err := os.Stat(b.template.Cmd)
	return err == nil
}

// isExpectedVersion returns true if the binary is installed and the version matches
// what was configured.
func (b *Binary) isExpectedVersion() bool {
	if b.version == "latest" || b.version == "" {
		return true
	}

	args := strings.Split(b.versioncmd, " ")
	logstep(fmt.Sprintf("running %v looking for %s", args, b.version))
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		logstep(fmt.Sprintf("error: %s", err))
		return false
	}

	if bytes.Contains(out, []byte(b.version)) {
		return true
	}

	logstep(fmt.Sprintf("no hit in %s", string(out)))
	return false
}
