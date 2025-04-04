package js

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gabotechs/dep-tree/internal/utils"
)

// ResolvePath resolves an unresolved import based on the dir where the import was executed.
//
//nolint:gocyclo
func (l *Language) ResolvePath(unresolved string, dir string) (string, error) {
	absPath := ""
	var err error

	if len(unresolved) == 0 {
		return "", errors.New("import path cannot be empty")
	} else if len(unresolved) == 1 {
		return "", fmt.Errorf("invalid import path %s", unresolved)
	}

	// 1. If import is relative.
	if unresolved[0] == '.' && (unresolved[1] == '/' || unresolved[1] == '.') {
		absPath = getFileAbsPath(filepath.Join(dir, unresolved))
		if absPath == "" {
			return "", fmt.Errorf("could not perform relative import for '%s' because the file or dir was not found", unresolved)
		}
		return absPath, nil
	}

	// 2. If is imported from a workspace.
	if l.Cfg == nil || l.Cfg.Workspaces {
		workspaces, err := NewWorkspaces(dir)
		if err != nil {
			return "", err
		}
		absPath, err = workspaces.ResolveFromWorkspaces(unresolved)
		if absPath != "" || err != nil {
			return absPath, err
		}
	}

	// 3. If is imported from baseUrl.

	// 3.1 first load the appropriate tsconfig.json file
	packageJsonPath := findClosestPackageJsonPath(dir)
	if packageJsonPath == "" {
		return "", nil
	}
	tsConfigPath := filepath.Join(filepath.Dir(packageJsonPath), tsConfigFile)

	// 3.2 if there's no tsconfig file, then nothing else can be done.
	if !utils.FileExists(tsConfigPath) {
		return "", nil
	}
	tsConfig, err := ParseTsConfig(tsConfigPath)
	if err != nil {
		return "", fmt.Errorf("found TypeScript config file in %s but there was an error reading it: %w", tsConfigPath, err)
	}

	// 3.2 then use it for resolving the base url.
	resolved := tsConfig.ResolveFromBaseUrl(unresolved)
	absPath = getFileAbsPath(resolved)
	if absPath != "" {
		return absPath, nil
	}

	// 4. If imported from a path override.
	if l.Cfg == nil || l.Cfg.TsConfigPaths {
		candidates := tsConfig.ResolveFromPaths(unresolved)

		for _, candidate := range candidates {
			absPath = getFileAbsPath(candidate)
			if absPath != "" {
				break
			}
		}
		if absPath == "" && len(candidates) > 0 {
			return "", fmt.Errorf("import '%s' was matched to path '%s' in tscofing's paths option, but the resolved path did not match an existing file", unresolved, strings.Join(candidates, "', '"))
		}
		return absPath, nil
	}
	return "", nil
}

func retrieveWithExt(absPath string) string {
	for _, ext := range Extensions {
		if strings.HasSuffix(absPath, "."+ext) {
			absPath = absPath[0 : len(absPath)-len("."+ext)]
		}
	}
	for _, ext := range Extensions {
		withExtPath := absPath + "." + ext
		if utils.FileExists(withExtPath) {
			return withExtPath
		}
	}
	return ""
}

func getFileAbsPath(id string) string {
	absPath, err := filepath.Abs(id)
	switch {
	case err != nil:
		return ""
	case utils.DirExists(absPath):
		pckJson, err := readPackageJson(absPath)
		if err != nil || pckJson.Main == "" {
			return retrieveWithExt(filepath.Join(absPath, "index"))
		} else {
			return retrieveWithExt(filepath.Join(absPath, pckJson.Main))
		}
	default:
		return retrieveWithExt(absPath)
	}
}

// findPackageJson starts from a search path and goes up dir by dir
// until a package.json file is found. If one is found, it returns the
// dir where it was found and a parsed TsConfig object in case that there
// was also a tsconfig.json file.
func _findClosestPackageJsonPath(searchPath string) string {
	packageJsonPath := filepath.Join(searchPath, packageJsonFile)
	if utils.FileExists(packageJsonPath) {
		return packageJsonPath
	}
	nextSearchPath := filepath.Dir(searchPath)
	if nextSearchPath != searchPath {
		return _findClosestPackageJsonPath(nextSearchPath)
	} else {
		return ""
	}
}

var findClosestPackageJsonPath = utils.Cached1In1Out(_findClosestPackageJsonPath)
