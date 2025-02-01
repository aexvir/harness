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

const SkipVersionCheck = ""

type Binary struct {
	command   string
	directory string
	version   string

	versioncmd string

	origin   Origin
	template Template
}

func New(command, version string, origin Origin, options ...Option) (*Binary, error) {
	if version == "" {
		return nil, fmt.Errorf("version must be set")
	}

	bin := Binary{
		command:   command,
		directory: "./bin",
		version:   version,

		versioncmd: fmt.Sprintf("%s --version", command),

		origin: origin,
	}

	bin.directory = filepath.FromSlash(bin.directory)

	bin.template = Template{
		GOOS:   runtime.GOOS,
		GOARCH: runtime.GOARCH,

		Directory: bin.directory,
		Name:      command,
		Cmd:       filepath.Join(bin.directory, bin.command),
		Version:   version,
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

// isExpectedVersion returns true if binary version matches the expected version
// or latest version was requested. For the 'latest' use-case, we can't really
// check the binary version so we just return true.
func (b *Binary) isExpectedVersion() bool {
	if b.version == "latest" {
		return true
	}

	if b.versioncmd == SkipVersionCheck {
		return false
	}

	semver := strings.TrimPrefix(b.version, "v")
	args := strings.Split(b.versioncmd, " ")

	logstep(fmt.Sprintf("running %v looking for %s", args, semver))
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		return false
	}

	return bytes.Contains(out, []byte(semver))
}
