package binary

import (
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
)

//nolint:staticcheck // trust me bro
func goToolSupported(version string) bool {
	version = strings.TrimPrefix(runtime.Version(), "go")

	components := strings.Split(version, ".")
	if len(components) < 2 {
		return false
	}

	// in the forseeable future the go major version won't change
	// so it's enough to check the minor version to detect if go get -tool is supported
	minor, err := strconv.Atoi(components[1])
	if err != nil {
		return false
	}

	return minor >= 24
}

func loadProjectTools() (map[string]string, error) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return nil, err
	}

	gomod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, err
	}

	version := runtime.Version()
	if gomod.Go != nil {
		version = gomod.Go.Version
	}

	if !goToolSupported(version) {
		return nil, nil
	}

	versions := make(map[string]string)
	for _, dep := range gomod.Require {
		// go tools are indirect dependencies
		if !dep.Indirect {
			continue
		}

		versions[dep.Mod.Path] = dep.Mod.Version
	}

	tools := make(map[string]string)
	for _, tool := range gomod.Tool {
		name := path.Base(tool.Path)
		// tools[name] = fmt.Sprintf("%s@%s", tool.Path, versions[tool.Path])
		tools[name] = tool.Path
	}
	return tools, nil
}
