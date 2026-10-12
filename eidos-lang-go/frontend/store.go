// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go.dokimi.dev/eidos/sdk/plugin"
)

// The stores that the Go frontend reads dependency units from, under the
// names that a load lists them by.
const (
	// ModCacheStore is the module cache. The tree of a module is under
	// "<escaped path>@<escaped version>/". The hash that the go command
	// verified the tree with is at
	// "cache/download/<escaped path>/@v/<escaped version>.ziphash".
	ModCacheStore = "gomod"
	// GoRootStore is the src directory of GOROOT. It has the standard
	// library, and under "vendor/" the packages that the standard library
	// vendors.
	GoRootStore = "goroot"
)

// The variables of the go command that Stores reads, and the value of
// GOENV that turns the go env file off.
const (
	envGoEnv    = "GOENV"
	envModCache = "GOMODCACHE"
	envGoPath   = "GOPATH"
	envGoRoot   = "GOROOT"
	envPath     = "PATH"
	goEnvOff    = "off"
)

// The variables of the directories of the user, as os.UserConfigDir reads
// them on each system.
const (
	envHome      = "HOME"
	envPlan9Home = "home"
	envAppData   = "AppData"
	envXDGConfig = "XDG_CONFIG_HOME"
)

// The systems whose user directories differ from those of Unix.
const (
	goosWindows = "windows"
	goosDarwin  = "darwin"
	goosIOS     = "ios"
	goosPlan9   = "plan9"
)

// The slash paths under the roots that Stores resolves. The user
// configuration directory is under a home directory, differently on each
// system. The go env file is under the configuration directory, the
// default GOPATH is under the home directory, and the module cache is
// under an entry of GOPATH. The standard library and its runtime package
// are under GOROOT. The go binary has a suffix on Windows.
const (
	darwinConfig  = "Library/Application Support"
	plan9Config   = "lib"
	unixConfig    = ".config"
	envFile       = "go/env"
	goPathDefault = "go"
	modCacheDir   = "pkg/mod"
	srcDir        = "src"
	runtimeDir    = "src/runtime"
	goBinary      = "go"
	windowsExe    = ".exe"
)

// Stores returns the module cache and the standard library under
// [ModCacheStore] and [GoRootStore], as the stores that the load passes to
// the Go frontend. Each is rooted where the go command finds it. Stores
// reads the environment through getenv, which is os.Getenv outside a test,
// and runs no tool.
//
// The module cache is GOMODCACHE of the environment, then GOMODCACHE of
// the go env file, then pkg/mod under the first entry of GOPATH. GOPATH
// comes from the environment, then from the go env file, and is otherwise
// go under the home directory. GOROOT comes from the environment, then
// from the go env file. Otherwise GOROOT is the directory two levels above
// the first go binary on PATH where that directory contains src/runtime,
// and else the same directory above the binary with its symbolic links
// resolved. The go command finds its own root above its executable the
// same way. The go env file is GOENV, or go/env under the user
// configuration directory, and GOENV=off turns it off.
//
// Stores returns an error that names the variable to set for a root that
// it cannot resolve.
//
// # Allocation contract
//
// Stores allocates the map of stores, each root as a file system and the
// path of the standard library: five allocations where the environment
// names both roots. A go env file costs eight more: five to read the
// file, its text, and the map of its variables.
func Stores(getenv func(string) string) (map[string]fs.FS, error) {
	env := goEnv{getenv: getenv, file: readEnvFile(getenv)}
	modCache, err := env.modCache()
	if err != nil {
		return nil, err
	}
	goRoot, err := env.goRoot()
	if err != nil {
		return nil, err
	}
	return map[string]fs.FS{
		ModCacheStore: os.DirFS(modCache),
		GoRootStore:   os.DirFS(filepath.Join(goRoot, srcDir)),
	}, nil
}

// goEnv reads the variables of the go command as the go command does:
// from the environment first, then from the go env file.
type goEnv struct {
	getenv func(string) string
	file   map[string]string
}

// lookup returns the value of a variable from the environment, and from
// the go env file where the environment leaves it empty.
func (e goEnv) lookup(key string) string {
	if v := e.getenv(key); v != "" {
		return v
	}
	return e.file[key]
}

// modCache returns the root of the module cache.
func (e goEnv) modCache() (string, error) {
	if dir := e.lookup(envModCache); dir != "" {
		return dir, nil
	}
	goPath := e.lookup(envGoPath)
	if goPath == "" {
		home := e.getenv(plugin.UserHomeVar())
		if home == "" {
			return "", errors.New("frontend: set GOMODCACHE or GOPATH, because the home directory " +
				"the default GOPATH is under is unset")
		}
		goPath = filepath.Join(home, goPathDefault)
	}
	first := filepath.SplitList(goPath)[0]
	if first == "" {
		return "", errors.New("frontend: set GOMODCACHE, because the first entry of GOPATH is empty")
	}
	return filepath.Join(first, filepath.FromSlash(modCacheDir)), nil
}

// goRoot returns GOROOT.
func (e goEnv) goRoot() (string, error) {
	if dir := e.lookup(envGoRoot); dir != "" {
		return dir, nil
	}
	binary, found := onPath(e.getenv(envPath), goBinary+exeSuffix())
	if !found {
		return "", errors.New("frontend: set GOROOT, because no go binary is on PATH")
	}
	if root := filepath.Dir(filepath.Dir(binary)); hasRuntime(root) {
		return root, nil
	}
	resolved, err := filepath.EvalSymlinks(binary)
	if err != nil {
		return "", fmt.Errorf("frontend: set GOROOT, because the go binary on PATH does not resolve: %w", err)
	}
	root := filepath.Dir(filepath.Dir(resolved))
	if !hasRuntime(root) {
		return "", fmt.Errorf("frontend: set GOROOT, because %s, two levels above the go binary on PATH, "+
			"has no %s", root, runtimeDir)
	}
	return root, nil
}

// hasRuntime reports whether root contains the directory src/runtime, as
// a GOROOT does.
func hasRuntime(root string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(runtimeDir)))
	return err == nil && info.IsDir()
}

// onPath returns the first regular file named name in the directories
// that a PATH value lists, and reports whether one exists. It skips the
// empty entries, which refer to the working directory.
func onPath(pathList, name string) (string, bool) {
	for _, dir := range filepath.SplitList(pathList) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate, true
		}
	}
	return "", false
}

// readEnvFile returns the variables of the go env file. It returns nothing
// for a file that GOENV turns off or that does not read. A line is the
// name of a variable, "=" and its value, as the go command writes it, and
// readEnvFile leaves out a line without "=".
func readEnvFile(getenv func(string) string) map[string]string {
	name := getenv(envGoEnv)
	if name == goEnvOff {
		return nil
	}
	if name == "" {
		name = under(configDir(getenv), envFile)
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return nil
	}
	vars := map[string]string{}
	for line := range strings.Lines(string(data)) {
		if key, value, found := strings.Cut(strings.TrimSuffix(line, "\n"), "="); found {
			vars[key] = value
		}
	}
	return vars
}

// configDir returns the user configuration directory that the go command
// finds its env file under. It applies the rule of os.UserConfigDir
// through getenv: AppData on Windows, Library/Application Support under
// HOME on Darwin and lib under home on Plan 9. On other systems it returns
// XDG_CONFIG_HOME where it is absolute, or .config under HOME where it is
// unset.
func configDir(getenv func(string) string) string {
	switch runtime.GOOS {
	case goosWindows:
		return getenv(envAppData)
	case goosDarwin, goosIOS:
		return under(getenv(envHome), darwinConfig)
	case goosPlan9:
		return under(getenv(envPlan9Home), plan9Config)
	}
	dir := getenv(envXDGConfig)
	if dir == "" {
		return under(getenv(envHome), unixConfig)
	}
	if !filepath.IsAbs(dir) {
		return ""
	}
	return dir
}

// under returns a slash path below a directory, and the empty string for
// an unset directory.
func under(dir, rel string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, filepath.FromSlash(rel))
}

// exeSuffix returns the suffix of the name of an executable: .exe on
// Windows, and the empty string on other systems.
func exeSuffix() string {
	if runtime.GOOS == goosWindows {
		return windowsExe
	}
	return ""
}
