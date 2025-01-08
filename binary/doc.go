// Package binary provides utilities to provision external binaries
// that can be used as part of automation scripts.
//
// At the core, a [binary] is a specification indicating the binary name
// the desired version and an origin pointing at where to obtain the
// binary from.
//
// Origins implement the logic needed to provision the binary and ensure
// the version matches expectations. Currently there are only three origins
// implemented:
// - [GoBinary]: provisions binaries by running `go install`
// - [RemoteBinaryDownload]: for binaries that can be downloaded directly from a url
// - [RemoteArchiveDownload]: for binaries contained in archives that can be downloaded from a url
// If any other source is needed, a new origin can be implemented by just fulfilling the [Origin] interface.
//
// Each origin defines its own inputs that are required in order to work.
// Additionally, the template passed as argument to the Install function will contain all the
// information regarding the environment this code is running in, to tailor the installation process.
// e.g. using the GOOS value to point to the correct binary built for the platform.
//
// example usage
//
//	// define binary
//	commitsar, err := binary.New(
//		"commitsar",                  // name the binary will have after installation
//		"0.20.1",                     // version that will be installed
//		binary.RemoteArchiveDownload( // the origin is a remote archive for this binary
//			"https://github.com/aevea/commitsar/releases/download/v{{.Version}}/commitsar_{{.Version}}_{{.GOOS}}_{{.GOARCH}}.tar.gz",
//			// an archive can contain mutiple files, and here only the binary is needed
//			// which already has the correct name
//			map[string]string{"commitsar": "commitsar"},
//		),
//		logging.WithLevel(slog.LevelDebug),
//	)
//
//	// ensure the binary is present
//	// this will download or update the binary if necessary
//	if err := commitsar.Ensure(); err != nil {
//		return fmt.Errorf("failed to provision commitsar binary: %w", err)
//	}
//
//	// use via harness
//	harness.Run(ctx, commitsar.BinPath(), harness.WithArgs("--help"))
//
//	// or via os/exec
//	exec.Command(commitsar.BinPath(), "--help").Run()
package binary
