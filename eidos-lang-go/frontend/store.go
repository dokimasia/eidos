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
)

// The stores the Go frontend reads dependency units from, under the
// names a load lists them by.
const (
	// ModCacheStore is the module cache. A module's tree is under
	// "<escaped path>@<escaped version>/", and the hash the go command
	// verified the tree with is at
	// "cache/download/<escaped path>/@v/<escaped version>.ziphash".
	ModCacheStore = "gomod"
	// GoRootStore is the src directory of GOROOT: the standard
	// library, and under "vendor/" the packages it vendors.
	GoRootStore = "goroot"
)

// The go command's variables Stores reads, and the value of GOENV that
// turns the go env file off.
const (
	envGoEnv    = "GOENV"
	envModCache = "GOMODCACHE"
	envGoPath   = "GOPATH"
	envGoRoot   = "GOROOT"
	envPath     = "PATH"
	goEnvOff    = "off"
)

// The variables of the user's directories, as os.UserConfigDir and
// os.UserHomeDir read them per platform.
const (
	envHome        = "HOME"
	envPlan9Home   = "home"
	envUserProfile = "USERPROFILE"
	envAppData     = "AppData"
	envXDGConfig   = "XDG_CONFIG_HOME"
)

// The platforms whose user directories differ from Unix's.
const (
	goosWindows = "windows"
	goosDarwin  = "darwin"
	goosIOS     = "ios"
	goosPlan9   = "plan9"
)

// The slash paths under the roots Stores resolves: the user
// configuration directory under a home directory per platform, the go
// env file under the configuration directory, the default GOPATH under
// the home directory, the module cache under a GOPATH entry, the
// standard library and its runtime package under GOROOT, and the go
// binary and its suffix on Windows.
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

// Stores returns the module cache and the standard library as the
// stores the load passes to the Go frontend, under [ModCacheStore] and
// [GoRootStore], rooted where the go command finds them. It reads the
// environment through getenv, which is os.Getenv outside a test, and
// runs no tool.
//
// The module cache is GOMODCACHE, then the go env file's GOMODCACHE,
// then pkg/mod under the first entry of GOPATH. GOPATH is the
// environment's, then the go env file's, then go under the home
// directory. GOROOT is the environment's, then the go env file's,
// then the directory two levels above the first go binary on PATH,
// symbolic links resolved, where that directory contains src/runtime.
// The go command finds its own root above its executable the same
// way. The go env file is GOENV, or go/env under the user
// configuration directory, and GOENV=off turns it off.
//
// Stores returns an error naming the variable to set for a root it
// cannot resolve.
//
// # Allocation contract
//
// Stores allocates the map of stores, each root as a file system and
// the standard library's path: five allocations where the environment
// names both roots. A go env file allocates eight more: five to read
// the file, its text, and the map of its variables.
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

// goEnv reads the go command's variables the way the go command does:
// the environment first, then the go env file.
type goEnv struct {
	getenv func(string) string
	file   map[string]string
}

// lookup returns a variable's value from the environment, and from
// the go env file where the environment leaves it empty.
func (e goEnv) lookup(key string) string {
	if v := e.getenv(key); v != "" {
		return v
	}
	return e.file[key]
}

// modCache returns the module cache's root.
func (e goEnv) modCache() (string, error) {
	if dir := e.lookup(envModCache); dir != "" {
		return dir, nil
	}
	goPath := e.lookup(envGoPath)
	if goPath == "" {
		home := homeDir(e.getenv)
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
	resolved, err := filepath.EvalSymlinks(binary)
	if err != nil {
		return "", fmt.Errorf("frontend: set GOROOT, because the go binary on PATH does not resolve: %w", err)
	}
	root := filepath.Dir(filepath.Dir(resolved))
	if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(runtimeDir))); err != nil || !info.IsDir() {
		return "", fmt.Errorf("frontend: set GOROOT, because %s, two levels above the go binary on PATH, "+
			"has no %s", root, runtimeDir)
	}
	return root, nil
}

// onPath returns the first regular file of a name in the directories a
// PATH value lists, skipping the empty entries, which name the working
// directory, and reports whether one exists.
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

// readEnvFile reads the go env file into its variables, and nothing
// for a file GOENV turns off or that does not read. A line is a
// variable's name, "=" and its value, as the go command writes it, and
// a line without "=" is left out.
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

// configDir returns the user configuration directory the go command
// finds its env file under, by os.UserConfigDir's rule read through
// getenv: AppData on Windows, Library/Application Support under HOME
// on Darwin, lib under home on Plan 9, and elsewhere XDG_CONFIG_HOME
// where it is absolute, or .config under HOME where it is unset.
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

// homeDir returns the home directory by os.UserHomeDir's rule read
// through getenv: USERPROFILE on Windows, home on Plan 9, and HOME
// elsewhere.
func homeDir(getenv func(string) string) string {
	switch runtime.GOOS {
	case goosWindows:
		return getenv(envUserProfile)
	case goosPlan9:
		return getenv(envPlan9Home)
	}
	return getenv(envHome)
}

// under returns a slash path below a directory, and nothing for an
// unset directory.
func under(dir, rel string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, filepath.FromSlash(rel))
}

// exeSuffix returns the suffix of an executable's name: .exe on
// Windows, and nothing elsewhere.
func exeSuffix() string {
	if runtime.GOOS == goosWindows {
		return windowsExe
	}
	return ""
}
