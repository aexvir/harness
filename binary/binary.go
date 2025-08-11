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
	// these fields are mostly used as metadata at the moment
	// helps with debugging
	command   string
	directory string
	version   string

	// command that will be run to obtain the version of the binary
	versioncmd string

	// origin that will be used to provision the binary
	origin Origin
	// template passed as argument to origins
	template Template
}

// New instantiates a new [Binary] given a command name, a version and it's [Origin].
// Origins determine where the binary is provisioned from, if it needs installation and how
// the installation process is handled.
func New(command, version string, origin Origin, options ...Option) *Binary {
	var extension string
	if runtime.GOOS == "windows" {
		extension = ".exe"
	}

	bindir := filepath.FromSlash("./bin")
	cmdQualifiedPath := filepath.Join(bindir, command) + extension

	bin := Binary{
		command:   command,
		directory: bindir,
		version:   version,

		versioncmd: fmt.Sprintf("%s --version", cmdQualifiedPath),

		origin: origin,
	}

	bin.template = Template{
		GOOS:   runtime.GOOS,
		GOARCH: runtime.GOARCH,

		Directory:        bin.directory,
		Name:             command,
		Cmd:              cmdQualifiedPath,
		Version:          bin.version,
		Extension:        extension,
		ArchiveExtension: ".tar.gz",
	}

	for _, opt := range options {
		opt(&bin)
	}

	return &bin
}

// Name returns the command name of the binary.
func (b *Binary) Name() string {
	return b.template.Name
}

// BinPath returns the qualified path to the binary.
// It's recommended to use this method to obtain the binary command string.
func (b *Binary) BinPath() string {
	return b.template.Cmd
}

// Ensure the binary is installed and it corresponds to the expected version.
func (b *Binary) Ensure() error {
	if b.version == "" {
		return fmt.Errorf("version must be set")
	}

	if b.isInstalled() && b.isExpectedVersion() {
		return nil
	}

	return b.Install()
}

// Install the binary.
func (b *Binary) Install() error {
	logstep(fmt.Sprintf("installing %s", b.template.Name))
	return b.origin.Install(b.template)
}

// isInstalled returns true if the binary is installed.
func (b *Binary) isInstalled() bool {
	_, err := os.Stat(b.template.Cmd)
	return err == nil
}

// isExpectedVersion returns true if binary version matches the expected version
// or latest version was requested.
// This check can be skipped by setting the version to SkipVersionCheck.
// If the version is "latest", there's no easy way to verify if the binary is actually
// the latest version, so it assumes it is, returning true.
func (b *Binary) isExpectedVersion() bool {
	if b.version == "latest" {
		return true
	}

	if b.versioncmd == SkipVersionCheck {
		return true
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
