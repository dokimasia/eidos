// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.dokimi.dev/eidos/lang/python/frontend/typeshed"
)

// The stores from which the Python frontend reads dependency units, under
// the names by which a load lists them.
const (
	// TypeshedStore is the embedded tree of typeshed: the stub of each
	// module of the standard library at stdlib/<module path>.pyi, the stub
	// of each package directory at stdlib/<package>/__init__.pyi, and the
	// table stdlib/VERSIONS.
	TypeshedStore = "typeshed"

	// VenvStore is the site-packages directory of the virtual environment
	// that VIRTUAL_ENV names: the modules of the installed distributions at
	// <module path>.py and .pyi, the modules of their package directories
	// at <package>/__init__.py and .pyi, and the packages of stubs alone at
	// <package>-stubs.
	VenvStore = "venv"
)

// The parts of a virtual environment: the variable that names it, its
// configuration and the keys of the version of Python in it, and the
// directories of its installed distributions below its root. The version
// names the directory lib/python<X.Y>, and an environment of Windows has
// Lib instead.
const (
	envVirtualEnv  = "VIRTUAL_ENV"
	venvConfig     = "pyvenv.cfg"
	versionKey     = "version"
	versionInfoKey = "version_info"
	libDir         = "lib"
	windowsLibDir  = "Lib"
	pythonPrefix   = "python"
	configSep      = "="
	versionSep     = "."
)

// Stores returns the stores that the load passes to the Python frontend:
// the embedded tree of typeshed under [TypeshedStore], and the
// site-packages directory of the virtual environment that VIRTUAL_ENV
// names under [VenvStore]. It reads the environment through getenv, which
// is os.Getenv outside a test, and runs no tool. The venv store is absent
// where VIRTUAL_ENV is unset. The directory is lib/python<X.Y>/site-packages
// below the environment, with the version of its pyvenv.cfg, and
// Lib/site-packages for an environment of Windows.
//
// Error modes: Stores returns an error that names the variable for an
// environment whose pyvenv.cfg does not read or states no version, and for
// an environment without either directory.
func Stores(getenv func(string) string) (map[string]fs.FS, error) {
	stores := map[string]fs.FS{TypeshedStore: typeshed.Files}
	env := getenv(envVirtualEnv)
	if env == "" {
		return stores, nil
	}
	cfg, err := os.ReadFile(filepath.Join(env, venvConfig))
	if err != nil {
		return nil, fmt.Errorf("frontend: set %s to a virtual environment: %w", envVirtualEnv, err)
	}
	version, stated := venvVersion(cfg)
	if !stated {
		return nil, fmt.Errorf("frontend: set %s to a virtual environment: %s states no version of Python",
			envVirtualEnv, filepath.Join(env, venvConfig))
	}
	candidates := []string{
		filepath.Join(env, libDir, pythonPrefix+version, sitePackages), filepath.Join(env, windowsLibDir, sitePackages),
	}
	for _, site := range candidates {
		dir := os.DirFS(site)
		if info, err := fs.Stat(dir, "."); err == nil && info.IsDir() {
			stores[VenvStore] = dir
			return stores, nil
		}
	}
	return nil, fmt.Errorf("frontend: set %s to a virtual environment: neither %s nor %s is a directory",
		envVirtualEnv, candidates[0], candidates[1])
}

// venvVersion returns the major and the minor version of the Python of a
// virtual environment, as 3.12, from the key version or version_info of its
// pyvenv.cfg. It reports false for a configuration that states neither.
func venvVersion(cfg []byte) (string, bool) {
	for line := range strings.Lines(string(cfg)) {
		key, value, assigned := strings.Cut(line, configSep)
		key = strings.TrimSpace(key)
		if !assigned || key != versionKey && key != versionInfoKey {
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(value), versionSep, 3)
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			return parts[0] + versionSep + parts[1], true
		}
	}
	return "", false
}
