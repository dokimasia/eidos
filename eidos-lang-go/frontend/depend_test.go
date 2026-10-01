// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"io/fs"
	"path"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The dependency fixture: a workspace module that requires one module
// at one version and imports a package of it, the hashes go.sum and
// the module cache record for the module's tree and its go.mod, and a
// hash no record states.
const (
	appModule  = "example.test/app"
	appGoMod   = "go.mod"
	appGoSum   = "go.sum"
	appFile    = "app/main.go"
	libModule  = "example.com/lib"
	libVersion = "v1.2.0"
	libPackage = "example.com/lib/pkg"
	libHash    = "h1:tree="
	libModHash = "h1:mod="
	otherHash  = "h1:other="
)

// The library's paths inside the module cache: its tree, the package
// the workspace imports, and the hash record and the unfinished-download
// mark beside its download.
const (
	libTree     = libModule + "@" + libVersion
	libDir      = libTree + "/pkg"
	libZipHash  = "cache/download/" + libModule + "/@v/" + libVersion + ".ziphash"
	libPartial  = "cache/download/" + libModule + "/@v/" + libVersion + ".partial"
	libSource   = "package pkg\n\n// Lib is the library's one type.\ntype Lib struct{}\n"
	libTestFile = libDir + "/pkg_test.go"
)

// The standard library fixture: a package that imports a package the
// standard library vendors, and the vendored package.
const (
	fmtPackage   = "fmt"
	fmtFile      = "fmt/print.go"
	vendoredPath = "golang.org/x/text/unicode"
	vendoredFile = "vendor/golang.org/x/text/unicode/unicode.go"
	fmtSource    = "package fmt\n\nimport \"" + vendoredPath + "\"\n\n" +
		"// Stringer is the fixture's type.\ntype Stringer interface{ String() unicode.Form }\n"
	vendoredSource = "package unicode\n\n// Form is the fixture's type.\ntype Form int\n"
)

// depWorkspace returns the workspace module: its go.mod requires the
// library, its go.sum records the library's hashes, and its one file
// imports the library's package.
func depWorkspace() fstest.MapFS {
	return fstest.MapFS{
		appGoMod: {Data: []byte("module " + appModule + "\n\ngo 1.27\n\nrequire " +
			libModule + " " + libVersion + "\n")},
		appGoSum: {Data: []byte(libModule + " " + libVersion + " " + libHash + "\n" +
			libModule + " " + libVersion + goModField + " " + libModHash + "\n")},
		appFile: {Data: []byte("package app\n\nimport \"" + libPackage + "\"\n\n" +
			"// App is the workspace's one type.\ntype App struct{ L pkg.Lib }\n")},
	}
}

// goModField ends the version field of the go.sum line that hashes a
// module's go.mod file alone.
const goModField = "/go.mod"

// depCache returns a module cache that has the library's tree, a test
// file beside the package's source, and the tree's hash record.
func depCache() fstest.MapFS {
	return fstest.MapFS{
		libDir + "/pkg.go": {Data: []byte(libSource)},
		libTestFile:        {Data: []byte("package pkg\n")},
		libZipHash:         {Data: []byte(libHash + "\n")},
	}
}

// goRootTree returns a GOROOT source tree with a package that imports
// a vendored package, and the vendored package.
func goRootTree() fstest.MapFS {
	return fstest.MapFS{
		fmtFile:      {Data: []byte(fmtSource)},
		vendoredFile: {Data: []byte(vendoredSource)},
	}
}

// depStores returns the module cache and the GOROOT tree under the
// names the Go frontend reads them by.
func depStores() map[string]fs.FS {
	return map[string]fs.FS{frontend.ModCacheStore: depCache(), frontend.GoRootStore: goRootTree()}
}

// storeTree is a test workspace with named stores beside it.
type storeTree struct {
	fstest.MapFS
	stores map[string]fs.FS
}

// Store returns one of the tree's stores.
func (t storeTree) Store(name string) (fs.FS, bool) {
	s, provided := t.stores[name]
	return s, provided
}

// roundReader is a dependency round's door over a test workspace and
// its stores.
type roundReader struct {
	tree storeTree
}

// Read returns one file's bytes, from the workspace or a store.
func (r roundReader) Read(p string) ([]byte, error) { return plugin.ReadFile(r.tree, p) }

// ReadDir returns one directory's entries, from the workspace or a
// store.
func (r roundReader) ReadDir(p string) ([]fs.DirEntry, error) { return plugin.ReadDir(r.tree, p) }

// runRound runs the Go frontend's first dependency round over a
// workspace and its stores for the needs given, with every go.mod of
// the workspace as the round's shared inputs.
func runRound(
	tree fstest.MapFS, stores map[string]fs.FS, needs ...string,
) ([][]plugin.SourceRef, error) {
	var goMods []string
	for p := range tree {
		if path.Base(p) == appGoMod {
			goMods = append(goMods, p)
		}
	}
	slices.Sort(goMods)
	round := plugin.DependencyRound{Number: 1, Shared: goMods}
	for _, need := range needs {
		round.Needs = append(round.Needs, plugin.Need{Path: need})
	}
	dependent, is := frontend.New(nil).(plugin.Dependent)
	if !is {
		panic("the Go frontend is in the dependent role")
	}
	return dependent.Dependencies(context.Background(), round, roundReader{storeTree{tree, stores}})
}

// placed runs a round that the case states succeeds and returns the
// member paths of each unit.
func placed(tb assert.TB, tree fstest.MapFS, stores map[string]fs.FS, needs ...string) [][]string {
	tb.Helper()

	units, err := runRound(tree, stores, needs...)
	assert.NoError(tb, err, "the round places its needs")
	out := make([][]string, len(units))
	for i, unit := range units {
		for _, ref := range unit {
			out[i] = append(out[i], ref.Path)
		}
	}
	return out
}

// refused runs a round that the case states fails and returns its
// error's text.
func refused(tb assert.TB, tree fstest.MapFS, stores map[string]fs.FS, needs ...string) string {
	tb.Helper()

	_, err := runRound(tree, stores, needs...)
	assert.HasError(tb, err, "the round fails")
	return err.Error()
}

// cached returns the qualified path of a module cache file.
func cached(p string) string { return plugin.StorePath(frontend.ModCacheStore, p) }

// inRoot returns the qualified path of a GOROOT source file.
func inRoot(p string) string { return plugin.StorePath(frontend.GoRootStore, p) }

// The dependency round places what the workspace imports from outside
// it, so each source, each refusal and the placements that yield
// nothing are pinned over fixture stores.
func TestDepend(t *testing.T) {
	t.Parallel()

	t.Run("dependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one unit per placed need", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, placed(t, depWorkspace(), depStores(), fmtPackage, libPackage),
				[][]string{{inRoot(fmtFile)}, {cached(libDir + "/pkg.go")}}, "a unit per package, in need order")
		})

		t.Run("returns the context's error for a done context", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			dependent, _ := frontend.New(nil).(plugin.Dependent)
			_, err := dependent.Dependencies(ctx, plugin.DependencyRound{
				Number: 1, Needs: []plugin.Need{{Path: libPackage}}, Shared: []string{appGoMod},
			}, roundReader{storeTree{depWorkspace(), depStores()}})
			assert.ErrorIs(t, err, context.Canceled, "a done context places nothing")
		})
	})

	t.Run("place", func(t *testing.T) {
		t.Parallel()

		t.Run("places a standard library need in GOROOT", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, placed(t, depWorkspace(), depStores(), fmtPackage), [][]string{{inRoot(fmtFile)}},
				"a path whose first element has no dot is the standard library's")
		})

		t.Run("places a vendored standard library path under GOROOT's vendor tree", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, placed(t, depWorkspace(), depStores(), path.Dir(vendoredFile)),
				[][]string{{inRoot(vendoredFile)}}, "a vendor/ path is the standard library's copy")
		})

		t.Run("places nothing for an import of a workspace module", func(t *testing.T) {
			t.Parallel()

			const appVersion = "v1.0.0"
			tree := depWorkspace()
			tree[svcGoMod] = &fstest.MapFile{Data: []byte("module example.test/svc\n\nrequire " +
				appModule + " " + appVersion + "\n")}
			tree[svcGoSum] = &fstest.MapFile{Data: []byte(appModule + " " + appVersion + " " + libHash + "\n")}
			stores := depStores()
			cache := depCache()
			cache[appModule+"@"+appVersion+"/other/other.go"] = &fstest.MapFile{Data: []byte("package other\n")}
			cache["cache/download/"+appModule+"/@v/"+appVersion+".ziphash"] = &fstest.MapFile{Data: []byte(libHash)}
			stores[frontend.ModCacheStore] = cache
			assert.Empty(t, placed(t, tree, stores, appModule+"/other"),
				"the workspace's own units load its packages, whatever another module requires")
		})

		t.Run("places nothing for an import no required module provides", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, placed(t, depWorkspace(), depStores(), "example.org/unrequired/pkg"),
				"the go command refuses it as provided by no required module")
		})

		t.Run("places nothing for a module a directory inside the workspace replaces", func(t *testing.T) {
			t.Parallel()

			tree := depWorkspace()
			tree[appGoMod] = &fstest.MapFile{Data: append(tree[appGoMod].Data,
				"replace "+libModule+" => ./lib\n"...)}
			assert.Empty(t, placed(t, tree, depStores(), libPackage), "the replacement is a workspace module")
		})

		t.Run("returns an error for a module a directory outside the workspace replaces", func(t *testing.T) {
			t.Parallel()

			tree := depWorkspace()
			tree[appGoMod] = &fstest.MapFile{Data: append(tree[appGoMod].Data,
				"replace "+libModule+" => ../lib\n"...)}
			assert.Contains(t, refused(t, tree, depStores(), libPackage), "outside the workspace",
				"no store provides the directory")
		})

		t.Run("returns an error naming go mod download for a module no source provides", func(t *testing.T) {
			t.Parallel()

			stores := depStores()
			stores[frontend.ModCacheStore] = fstest.MapFS{}
			assert.Contains(t, refused(t, depWorkspace(), stores, libPackage),
				"run go mod download "+libTree, "the go command that fetches the module")
		})

		t.Run("places nothing for a module when the load provides no module cache", func(t *testing.T) {
			t.Parallel()

			stores := depStores()
			delete(stores, frontend.ModCacheStore)
			assert.Empty(t, placed(t, depWorkspace(), stores, libPackage), "the source is off")
		})
	})

	t.Run("fromCache", func(t *testing.T) {
		t.Parallel()

		t.Run("places a need in the module cache at the selected version", func(t *testing.T) {
			t.Parallel()

			units, err := runRound(depWorkspace(), depStores(), libPackage)
			assert.NoError(t, err, "the round places the need")
			want := [][]plugin.SourceRef{{{Path: cached(libDir + "/pkg.go"), Shared: []string{appGoMod}}}}
			assert.Equal(t, units, want, "the package's source, which declares the selecting go.mod")
		})

		t.Run("returns an error for a module no go.sum records a hash of", func(t *testing.T) {
			t.Parallel()

			tree := depWorkspace()
			delete(tree, appGoSum)
			assert.Contains(t, refused(t, tree, depStores(), libPackage), "run go mod download "+libModule,
				"the go command that records the hash")
		})

		t.Run("returns an error for a cached hash go.sum does not record", func(t *testing.T) {
			t.Parallel()

			stores := depStores()
			cache := depCache()
			cache[libZipHash] = &fstest.MapFile{Data: []byte(otherHash + "\n")}
			stores[frontend.ModCacheStore] = cache
			msg := refused(t, depWorkspace(), stores, libPackage)
			assert.Contains(t, msg, otherHash, "the error names the cache's hash")
			assert.Contains(t, msg, libHash, "and go.sum's")
		})

		t.Run("reads a module with an unfinished download as missing", func(t *testing.T) {
			t.Parallel()

			stores := depStores()
			cache := depCache()
			cache[libPartial] = &fstest.MapFile{}
			stores[frontend.ModCacheStore] = cache
			assert.Contains(t, refused(t, depWorkspace(), stores, libPackage), "run go mod download",
				"the go command finishes the download")
		})

		t.Run("reads a module without its tree as missing", func(t *testing.T) {
			t.Parallel()

			stores := depStores()
			stores[frontend.ModCacheStore] = fstest.MapFS{libZipHash: {Data: []byte(libHash + "\n")}}
			assert.Contains(t, refused(t, depWorkspace(), stores, libPackage), "run go mod download",
				"a hash record alone is no module")
		})

		t.Run("places nothing for a package the cached module lacks", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, placed(t, depWorkspace(), depStores(), libModule+"/absent"),
				"the module has no such directory")
		})

		t.Run("places a need at the module's root directory", func(t *testing.T) {
			t.Parallel()

			stores := depStores()
			cache := depCache()
			cache[libTree+"/lib.go"] = &fstest.MapFile{Data: []byte("package lib\n")}
			stores[frontend.ModCacheStore] = cache
			assert.Equal(t, placed(t, depWorkspace(), stores, libModule), [][]string{{cached(libTree + "/lib.go")}},
				"the module path names the tree's root")
		})

		t.Run("reads a replacement by module path from the replacement's tree", func(t *testing.T) {
			t.Parallel()

			const fork, forkVersion = "example.com/fork", "v1.3.0"
			tree := depWorkspace()
			tree[appGoMod] = &fstest.MapFile{Data: append(tree[appGoMod].Data,
				"replace "+libModule+" => "+fork+" "+forkVersion+"\n"...)}
			tree[appGoSum] = &fstest.MapFile{Data: []byte(fork + " " + forkVersion + " " + libHash + "\n")}
			stores := depStores()
			stores[frontend.ModCacheStore] = fstest.MapFS{
				fork + "@" + forkVersion + "/pkg/pkg.go":                     {Data: []byte(libSource)},
				"cache/download/" + fork + "/@v/" + forkVersion + ".ziphash": {Data: []byte(libHash)},
			}
			assert.Equal(t, placed(t, tree, stores, libPackage),
				[][]string{{cached(fork + "@" + forkVersion + "/pkg/pkg.go")}}, "the fork's tree")
		})
	})

	t.Run("fromVendor", func(t *testing.T) {
		t.Parallel()

		vendored := func() fstest.MapFS {
			tree := depWorkspace()
			tree["vendor/modules.txt"] = &fstest.MapFile{Data: []byte(
				"# " + libModule + " " + libVersion + "\n## explicit; go 1.27\n" + libPackage + "\n")}
			tree["vendor/"+libPackage+"/pkg.go"] = &fstest.MapFile{Data: []byte(libSource)}
			return tree
		}
		noCache := func() map[string]fs.FS {
			stores := depStores()
			stores[frontend.ModCacheStore] = fstest.MapFS{}
			return stores
		}

		t.Run("places a need in the vendor tree when the module cache lacks the module", func(t *testing.T) {
			t.Parallel()

			units, err := runRound(vendored(), noCache(), libPackage)
			assert.NoError(t, err, "the round places the need")
			want := [][]plugin.SourceRef{{{Path: "vendor/" + libPackage + "/pkg.go", Shared: []string{appGoMod}}}}
			assert.Equal(t, units, want, "the vendored copy, which declares the go.mod beside the vendor tree")
		})

		t.Run("places a need in the vendor tree when the load provides no module cache", func(t *testing.T) {
			t.Parallel()

			stores := depStores()
			delete(stores, frontend.ModCacheStore)
			assert.Equal(t, placed(t, vendored(), stores, libPackage), [][]string{{"vendor/" + libPackage + "/pkg.go"}},
				"the vendor tree is a source of its own")
		})

		t.Run("returns an error for a vendor tree its go.mod contradicts", func(t *testing.T) {
			t.Parallel()

			tree := vendored()
			tree["vendor/modules.txt"] = &fstest.MapFile{Data: []byte(
				"# " + libModule + " " + libVersion + "\n" + libPackage + "\n")}
			assert.Contains(t, refused(t, tree, noCache(), libPackage), "inconsistent vendoring",
				"the go command refuses the tree")
		})

		t.Run("returns a vendor list read's own error", func(t *testing.T) {
			t.Parallel()

			tree := depWorkspace()
			tree["vendor/modules.txt"] = &fstest.MapFile{Mode: fs.ModeDir}
			_, err := runRound(tree, noCache(), libPackage)
			assert.HasError(t, err, "a list that exists and does not read fails the round")
		})

		t.Run("passes over a vendor tree of another version", func(t *testing.T) {
			t.Parallel()

			tree := vendored()
			tree["vendor/modules.txt"] = &fstest.MapFile{Data: []byte(
				"# " + libModule + " v1.1.0\n## explicit\n" + libPackage + "\n")}
			assert.Contains(t, refused(t, tree, noCache(), libPackage), "run go mod download",
				"the vendored version is not the selected one")
		})
	})

	t.Run("packageUnit", func(t *testing.T) {
		t.Parallel()

		t.Run("leaves out the package's test files", func(t *testing.T) {
			t.Parallel()

			assert.False(t, slices.Contains(placed(t, depWorkspace(), depStores(), libPackage)[0], cached(libTestFile)),
				"a dependency's tests are not part of its API")
		})

		t.Run("leaves out the files the go command ignores", func(t *testing.T) {
			t.Parallel()

			stores := depStores()
			cache := depCache()
			cache[libDir+"/_ignored.go"] = &fstest.MapFile{Data: []byte("package pkg\n")}
			cache[libDir+"/.hidden.go"] = &fstest.MapFile{Data: []byte("package pkg\n")}
			cache[libDir+"/notes.txt"] = &fstest.MapFile{Data: []byte("notes\n")}
			cache[libDir+"/sub.go/sub.go"] = &fstest.MapFile{Data: []byte("package sub\n")}
			stores[frontend.ModCacheStore] = cache
			assert.Equal(t, placed(t, depWorkspace(), stores, libPackage), [][]string{{cached(libDir + "/pkg.go")}},
				"an underscore or dot name, another extension and a subdirectory are no members")
		})

		t.Run("returns a listing's own error", func(t *testing.T) {
			t.Parallel()

			stores := depStores()
			stores[frontend.GoRootStore] = fstest.MapFS{fmtPackage: {Data: []byte("not a directory\n")}}
			_, err := runRound(depWorkspace(), stores, fmtPackage)
			assert.HasError(t, err, "a package path that names a file fails the round")
		})

		t.Run("places nothing for a standard library path GOROOT does not contain", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, placed(t, depWorkspace(), depStores(), "C"), "cgo's pseudo-package is no directory")
		})

		t.Run("places nothing when the load provides no GOROOT", func(t *testing.T) {
			t.Parallel()

			stores := depStores()
			delete(stores, frontend.GoRootStore)
			assert.Empty(t, placed(t, depWorkspace(), stores, fmtPackage), "the source is off")
		})
	})

	t.Run("standard", func(t *testing.T) {
		t.Parallel()

		t.Run("reads a path whose first element has a dot as a module's", func(t *testing.T) {
			t.Parallel()

			stores := depStores()
			root := goRootTree()
			root[libPackage+"/decoy.go"] = &fstest.MapFile{Data: []byte("package pkg\n")}
			stores[frontend.GoRootStore] = root
			assert.Equal(t, placed(t, depWorkspace(), stores, libPackage), [][]string{{cached(libDir + "/pkg.go")}},
				"GOROOT's directory of the same path is not the standard library's")
		})
	})
}
