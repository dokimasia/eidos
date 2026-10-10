// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/frontend"
)

// The file in every fixture root, so a case tells which directory a
// store is rooted at by reading through the store.
const markerFile = "marker"

// The variables the cases set, as the go command spells them, and the
// variables of the home and the configuration directory on Windows.
const (
	varGoEnv       = "GOENV"
	varModCache    = "GOMODCACHE"
	varGoPath      = "GOPATH"
	varGoRoot      = "GOROOT"
	varPath        = "PATH"
	varHome        = "HOME"
	varXDG         = "XDG_CONFIG_HOME"
	varUserProfile = "USERPROFILE"
	varAppData     = "AppData"
	goEnvOff       = "off"
)

// goName is the file name of the go binary that Stores looks for on
// PATH. Windows runs a file with the suffix .exe alone.
var goName = func() string {
	if runtime.GOOS == "windows" {
		return "go.exe"
	}
	return "go"
}()

// The roots the allocation cases name. Stores reads no directory where
// the environment names both roots, so neither has to exist.
const (
	cacheRoot = "/cache"
	goRoot    = "/goroot"
)

// The allocations of the stores.
const (
	// storesAllocs is the stores of an environment that names both roots:
	// the map, each root as a file system, and the standard library's
	// path.
	storesAllocs = 2 + 2 + 1
	// envFileAllocs is the stores of a go env file that names both roots:
	// the open file and its path, its status and its contents, the file's
	// text and the map of its variables, and the stores.
	envFileAllocs = 2 + 1 + 1 + 1 + 3 + storesAllocs
)

// The stores are the only door to bytes outside the workspace, and
// their roots follow the go command's own rules, so each rule is
// pinned over temporary directories.
func TestStore(t *testing.T) {
	t.Parallel()

	t.Run("Stores", func(t *testing.T) {
		t.Parallel()

		t.Run("roots the module cache at GOMODCACHE", func(t *testing.T) {
			t.Parallel()

			modCache := markedDir(t)
			stores, err := frontend.Stores(envOf(map[string]string{
				varModCache: modCache, varGoRoot: goRootDir(t), varGoEnv: goEnvOff,
			}))
			assert.NoError(t, err, "both roots resolve")
			assert.Equal(t, rootOf(t, stores, frontend.ModCacheStore), modCache, "the variable names the root")
		})

		t.Run("roots the module cache at the go env file's GOMODCACHE", func(t *testing.T) {
			t.Parallel()

			modCache := markedDir(t)
			stores, err := frontend.Stores(envOf(map[string]string{
				varGoEnv: envFileOf(t, varModCache+"="+modCache), varGoRoot: goRootDir(t),
			}))
			assert.NoError(t, err, "both roots resolve")
			assert.Equal(t, rootOf(t, stores, frontend.ModCacheStore), modCache, "the env file names the root")
		})

		t.Run("roots the module cache under the first GOPATH entry", func(t *testing.T) {
			t.Parallel()

			first := t.TempDir()
			modCache := markedDir(t)
			assert.NoError(t, os.MkdirAll(filepath.Join(first, "pkg"), 0o755), "pkg is created")
			assert.NoError(t, os.Symlink(modCache, filepath.Join(first, "pkg", "mod")), "pkg/mod links to the cache")
			stores, err := frontend.Stores(envOf(map[string]string{
				varGoPath: first + string(filepath.ListSeparator) + t.TempDir(), varGoRoot: goRootDir(t),
				varGoEnv: goEnvOff,
			}))
			assert.NoError(t, err, "both roots resolve")
			assert.Equal(t, rootOf(t, stores, frontend.ModCacheStore), modCache, "pkg/mod under the first entry")
		})

		t.Run("roots the module cache under go in the home directory", func(t *testing.T) {
			t.Parallel()

			home := t.TempDir()
			modCache := markedDir(t)
			assert.NoError(t, os.MkdirAll(filepath.Join(home, "go", "pkg"), 0o755), "go/pkg is created")
			assert.NoError(t, os.Symlink(modCache, filepath.Join(home, "go", "pkg", "mod")), "go/pkg/mod links")
			stores, err := frontend.Stores(envOf(map[string]string{
				varHome: home, varUserProfile: home, varGoRoot: goRootDir(t), varGoEnv: goEnvOff,
			}))
			assert.NoError(t, err, "both roots resolve")
			assert.Equal(t, rootOf(t, stores, frontend.ModCacheStore), modCache, "the default GOPATH is ~/go")
		})

		t.Run("returns an error naming GOMODCACHE without a home directory", func(t *testing.T) {
			t.Parallel()

			_, err := frontend.Stores(envOf(map[string]string{varGoRoot: goRootDir(t), varGoEnv: goEnvOff}))
			assert.HasError(t, err, "no rule roots the module cache")
			assert.Contains(t, err.Error(), varModCache, "the error names the variable to set")
		})

		t.Run("returns an error naming GOMODCACHE for an empty first GOPATH entry", func(t *testing.T) {
			t.Parallel()

			_, err := frontend.Stores(envOf(map[string]string{
				varGoPath: string(filepath.ListSeparator) + t.TempDir(), varGoRoot: goRootDir(t), varGoEnv: goEnvOff,
			}))
			assert.HasError(t, err, "the first entry roots nothing")
			assert.Contains(t, err.Error(), varModCache, "the error names the variable to set")
		})

		t.Run("roots the standard library at GOROOT's src", func(t *testing.T) {
			t.Parallel()

			root := goRootDir(t)
			stores, err := frontend.Stores(envOf(map[string]string{
				varModCache: markedDir(t), varGoRoot: root, varGoEnv: goEnvOff,
			}))
			assert.NoError(t, err, "both roots resolve")
			assert.Equal(t, rootOf(t, stores, frontend.GoRootStore), filepath.Join(root, "src"), "src under GOROOT")
		})

		t.Run("roots the standard library at the go env file's GOROOT", func(t *testing.T) {
			t.Parallel()

			root := goRootDir(t)
			stores, err := frontend.Stores(envOf(map[string]string{
				varModCache: markedDir(t), varGoEnv: envFileOf(t, varGoRoot+"="+root),
			}))
			assert.NoError(t, err, "both roots resolve")
			got := rootOf(t, stores, frontend.GoRootStore)
			assert.Equal(t, got, filepath.Join(root, "src"), "the env file names it")
		})

		t.Run("roots the standard library two levels above the go binary on PATH", func(t *testing.T) {
			t.Parallel()

			root := goRootDir(t)
			stores, err := frontend.Stores(envOf(map[string]string{
				varModCache: markedDir(t), varGoEnv: goEnvOff,
				varPath: string(filepath.ListSeparator) + t.TempDir() + string(filepath.ListSeparator) +
					filepath.Join(root, "bin"),
			}))
			assert.NoError(t, err, "both roots resolve")
			assert.Equal(t, rootOf(t, stores, frontend.GoRootStore), filepath.Join(root, "src"),
				"an empty entry and a directory without go are passed over")
		})

		t.Run("resolves a symbolic link to the go binary", func(t *testing.T) {
			t.Parallel()

			root := goRootDir(t)
			links := t.TempDir()
			assert.NoError(t, os.Symlink(filepath.Join(root, "bin", goName), filepath.Join(links, goName)),
				"the link is created")
			stores, err := frontend.Stores(envOf(map[string]string{
				varModCache: markedDir(t), varGoEnv: goEnvOff, varPath: links,
			}))
			assert.NoError(t, err, "both roots resolve")
			assert.Equal(t, rootOf(t, stores, frontend.GoRootStore), filepath.Join(root, "src"),
				"the root is above the binary the link names")
		})

		t.Run("roots the standard library above a go binary on PATH that links outside the root", func(t *testing.T) {
			t.Parallel()

			root := goRootDir(t)
			elsewhere := filepath.Join(t.TempDir(), goName)
			assert.NoError(t, os.WriteFile(elsewhere, nil, 0o755), "the binary outside the root is written")
			binary := filepath.Join(root, "bin", goName)
			assert.NoError(t, os.Remove(binary), "the binary of the root is removed")
			assert.NoError(t, os.Symlink(elsewhere, binary), "a link to the binary outside takes its place")
			stores, err := frontend.Stores(envOf(map[string]string{
				varModCache: markedDir(t), varGoEnv: goEnvOff, varPath: filepath.Join(root, "bin"),
			}))
			assert.NoError(t, err, "both roots resolve")
			assert.Equal(t, rootOf(t, stores, frontend.GoRootStore), filepath.Join(root, "src"),
				"the root is above the link, not above its target")
		})

		t.Run("passes over a directory named go on PATH", func(t *testing.T) {
			t.Parallel()

			root := goRootDir(t)
			decoy := t.TempDir()
			assert.NoError(t, os.Mkdir(filepath.Join(decoy, goName), 0o755), "the decoy directory is created")
			stores, err := frontend.Stores(envOf(map[string]string{
				varModCache: markedDir(t), varGoEnv: goEnvOff,
				varPath: decoy + string(filepath.ListSeparator) + filepath.Join(root, "bin"),
			}))
			assert.NoError(t, err, "both roots resolve")
			assert.Equal(t, rootOf(t, stores, frontend.GoRootStore), filepath.Join(root, "src"),
				"a directory is no binary")
		})

		t.Run("returns an error naming GOROOT without a go binary on PATH", func(t *testing.T) {
			t.Parallel()

			_, err := frontend.Stores(envOf(map[string]string{
				varModCache: markedDir(t), varGoEnv: goEnvOff, varPath: t.TempDir(),
			}))
			assert.HasError(t, err, "no rule roots the standard library")
			assert.Contains(t, err.Error(), varGoRoot, "the error names the variable to set")
		})

		t.Run("returns an error naming GOROOT for a go binary without src/runtime above it", func(t *testing.T) {
			t.Parallel()

			root := goRootDir(t)
			assert.NoError(t, os.RemoveAll(filepath.Join(root, "src", "runtime")), "src/runtime is removed")
			_, err := frontend.Stores(envOf(map[string]string{
				varModCache: markedDir(t), varGoEnv: goEnvOff, varPath: filepath.Join(root, "bin"),
			}))
			assert.HasError(t, err, "the binary is not a GOROOT's")
			assert.Contains(t, err.Error(), varGoRoot, "the error names the variable to set")
		})

		t.Run("returns an error naming GOROOT for a dangling link named go on PATH", func(t *testing.T) {
			t.Parallel()

			links := t.TempDir()
			assert.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "gone"), filepath.Join(links, goName)),
				"the dangling link is created")
			_, err := frontend.Stores(envOf(map[string]string{
				varModCache: markedDir(t), varGoEnv: goEnvOff, varPath: links,
			}))
			assert.HasError(t, err, "a dangling link names no binary")
			assert.Contains(t, err.Error(), varGoRoot, "the error names the variable to set")
		})

		t.Run("ignores the go env file for GOENV=off", func(t *testing.T) {
			t.Parallel()

			config := t.TempDir()
			assert.NoError(t, os.MkdirAll(filepath.Join(config, "go"), 0o755), "go is created")
			assert.NoError(t, os.WriteFile(filepath.Join(config, "go", "env"),
				[]byte(varModCache+"="+markedDir(t)+"\n"), 0o644), "the env file is written")
			_, err := frontend.Stores(envOf(map[string]string{
				varXDG: config, varGoEnv: goEnvOff, varGoRoot: goRootDir(t),
			}))
			assert.HasError(t, err, "the env file is off, and nothing else roots the module cache")
		})

		t.Run("ignores a go env file line without a value", func(t *testing.T) {
			t.Parallel()

			modCache := markedDir(t)
			stores, err := frontend.Stores(envOf(map[string]string{
				varGoEnv: envFileOf(t, varModCache, varModCache+"="+modCache), varGoRoot: goRootDir(t),
			}))
			assert.NoError(t, err, "both roots resolve")
			assert.Equal(t, rootOf(t, stores, frontend.ModCacheStore), modCache, "the line with = decides")
		})
	})
}

// The go command reads its env file under os.UserConfigDir, whose rule
// differs per platform. Stores reads the file where os.UserConfigDir
// finds it for each setting of XDG_CONFIG_HOME. Where os.UserConfigDir
// does not find a directory, Stores does not read an env file. A decoy
// env file under .config in the home directory contains the module cache
// that a wrong fallback would read. The cases set the environment of the
// process, so the test runs alone.
func TestStoreEnv(t *testing.T) {
	t.Run("Stores", func(t *testing.T) {
		tests := []struct {
			name string
			// give returns XDG_CONFIG_HOME for a home directory.
			give func(home string) string
		}{
			{
				name: "reads the go env file that os.UserConfigDir finds for an absolute XDG_CONFIG_HOME",
				give: func(home string) string { return filepath.Join(home, "config") },
			},
			{
				name: "reads the go env file that os.UserConfigDir finds without XDG_CONFIG_HOME",
				give: func(string) string { return "" },
			},
			{
				name: "reads the go env file that os.UserConfigDir finds for a relative XDG_CONFIG_HOME",
				give: func(string) string { return "relative" },
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				home := t.TempDir()
				for _, v := range []string{varGoEnv, varModCache, varGoPath} {
					t.Setenv(v, "")
				}
				t.Setenv(varHome, home)
				t.Setenv(varUserProfile, home)
				t.Setenv(varAppData, filepath.Join(home, "AppData"))
				t.Setenv(varXDG, tt.give(home))
				t.Setenv(varGoRoot, goRootDir(t))
				decoy := filepath.Join(home, ".config", "go")
				assert.NoError(t, os.MkdirAll(decoy, 0o755), ".config/go is created")
				assert.NoError(t, os.WriteFile(filepath.Join(decoy, "env"),
					[]byte(varModCache+"="+markedDir(t)+"\n"), 0o644), "a decoy env file is written")
				want := filepath.Join(home, "go", "pkg", "mod")
				assert.NoError(t, os.MkdirAll(want, 0o755), "the default module cache is created")
				assert.NoError(t, os.WriteFile(filepath.Join(want, markerFile), []byte(want), 0o644),
					"the marker of the default module cache is written")
				if config, err := os.UserConfigDir(); err == nil {
					want = markedDir(t)
					assert.NoError(t, os.MkdirAll(filepath.Join(config, "go"), 0o755), "go is created")
					assert.NoError(t, os.WriteFile(filepath.Join(config, "go", "env"),
						[]byte(varModCache+"="+want+"\n"), 0o644), "the env file is written")
				}
				stores, err := frontend.Stores(os.Getenv)
				assert.NoError(t, err, "both roots resolve")
				assert.Equal(t, rootOf(t, stores, frontend.ModCacheStore), want,
					"Stores follows os.UserConfigDir to the env file")
			})
		}
	})
}

// The stores allocate the map and its two roots, and a go env file
// allocates its read. The ordinary run, which runs no benchmark, checks
// those ceilings here. Darwin and Windows allocate more in their file
// system calls and path functions, so the test runs on Linux alone.
func TestStoreAllocs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the ceilings count the allocations of Linux's file system calls")
	}
	checkAllocs(t, storeCalls(t))
}

// BenchmarkStore measures the resolution a load makes once, before it
// reads any dependency unit.
func BenchmarkStore(b *testing.B) {
	benchCalls(b, storeCalls(b))
}

// storeCalls returns a call of Stores over an environment that names
// both roots, and over a go env file that names them.
func storeCalls(tb testing.TB) []allocCall {
	tb.Helper()

	env := envOf(map[string]string{varModCache: cacheRoot, varGoRoot: goRoot, varGoEnv: goEnvOff})
	fileEnv := envOf(map[string]string{
		varGoEnv: envFileOf(tb, varModCache+"="+cacheRoot, varGoRoot+"="+goRoot),
	})
	var (
		stores map[string]fs.FS
		err    error
	)
	check := func(tb testing.TB) {
		tb.Helper()

		assert.NoError(tb, err, "Stores resolves both roots")
		assert.Length(tb, stores, 2, "Stores returns the module cache and the standard library")
	}
	return []allocCall{
		{
			name: "Stores", caseName: "the environment's variables", allocs: storesAllocs,
			call:  func() { stores, err = frontend.Stores(env) },
			check: check,
		},
		{
			name: "Stores", caseName: "the go env file", allocs: envFileAllocs,
			call:  func() { stores, err = frontend.Stores(fileEnv) },
			check: check,
		},
	}
}

// envOf returns a getenv over a fixed set of variables.
func envOf(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

// markedDir creates a directory under a test's temporary directory with
// the marker file in it, the marker's content naming the directory.
func markedDir(tb testing.TB, parts ...string) string {
	tb.Helper()

	dir := filepath.Join(append([]string{tb.TempDir()}, parts...)...)
	assert.NoError(tb, os.MkdirAll(dir, 0o755), "the fixture directory is created")
	assert.NoError(tb, os.WriteFile(filepath.Join(dir, markerFile), []byte(dir), 0o644), "the marker is written")
	return dir
}

// goRootDir creates a GOROOT under a test's temporary directory. The
// GOROOT has a file named goName in bin, the directory src/runtime, and
// the marker in src.
func goRootDir(tb testing.TB) string {
	tb.Helper()

	root := tb.TempDir()
	src := filepath.Join(root, "src")
	assert.NoError(tb, os.MkdirAll(filepath.Join(src, "runtime"), 0o755), "src/runtime is created")
	assert.NoError(tb, os.WriteFile(filepath.Join(src, markerFile), []byte(src), 0o644), "the marker is written")
	assert.NoError(tb, os.MkdirAll(filepath.Join(root, "bin"), 0o755), "bin is created")
	assert.NoError(tb, os.WriteFile(filepath.Join(root, "bin", goName), nil, 0o755), "the go binary is created")
	return root
}

// envFileOf writes a go env file under a test's temporary directory and
// returns its path.
func envFileOf(tb testing.TB, lines ...string) string {
	tb.Helper()

	file := filepath.Join(tb.TempDir(), "env")
	assert.NoError(tb, os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o644), "the env file is written")
	return file
}

// rootOf returns the directory a store is rooted at, read off the
// marker the fixture wrote there.
func rootOf(tb testing.TB, stores map[string]fs.FS, name string) string {
	tb.Helper()

	b, err := fs.ReadFile(stores[name], markerFile)
	assert.NoError(tb, err, "the store is rooted at a marked directory")
	return string(b)
}
