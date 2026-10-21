// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"errors"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The roots of the libraries that the library cases select: the default
// library of ES2025, the library of ES2024, the library of ES5, and the
// library of a target of ES2015.
const (
	defaultRoot = "lib.es2025.full.d.ts"
	es2024Root  = "lib.es2024.d.ts"
	es5Root     = "lib.es5.d.ts"
	es6Root     = "lib.es6.d.ts"
	domRoot     = "lib.dom.d.ts"
)

// The files of the fake library of the default key: its root, which refers
// to ES5 and to the DOM, the two files that it refers to, and the
// declarations of each. ES5 declares Error twice with the property message,
// so its two declarations merge into one.
const (
	fakeRoot = "/// <reference lib=\"es5\" />\n/// <reference lib=\"dom\" />\n"
	fakeES5  = "interface Error {\n  message: string;\n}\n\n" +
		"declare namespace Intl {\n  interface Locale {\n    tag: string;\n  }\n}\n\n" +
		"interface Error {\n  message: string;\n  stack?: string;\n}\n"
	fakeDOM = "interface Event {\n  type: string;\n}\n"
)

// The store paths of the files of the fake library of the default key, and
// the package of its declarations.
var (
	defaultInput   = plugin.StorePath(typescript.LibraryStore, defaultRoot+"/"+defaultRoot)
	defaultES5     = plugin.StorePath(typescript.LibraryStore, defaultRoot+"/"+es5Root)
	defaultDOM     = plugin.StorePath(typescript.LibraryStore, defaultRoot+"/"+domRoot)
	defaultPackage = plugin.StorePath(typescript.LibraryStore, defaultRoot)
)

// errBroken is the error of the failing path of a failing file system.
var errBroken = errors.New("frontend_test: the file system is broken")

// storeTree is a workspace tree with named stores beside it, as the load
// passes it to a unit.
type storeTree struct {
	fstest.MapFS
	stores map[string]fs.FS
}

// Store returns one named store, and false for a name that the tree does
// not provide.
func (t storeTree) Store(name string) (fs.FS, bool) {
	s, provided := t.stores[name]
	return s, provided
}

// roundReader is the recorded door of a partition and of a dependency round
// over a test workspace and its stores.
type roundReader struct {
	tree fs.FS
}

// Read returns the bytes of one file from the workspace or a store.
func (r roundReader) Read(p string) ([]byte, error) { return plugin.ReadFile(r.tree, p) }

// ReadDir returns the entries of one directory from the workspace or a
// store.
func (r roundReader) ReadDir(p string) ([]fs.DirEntry, error) { return plugin.ReadDir(r.tree, p) }

// countingStoreReader is the door of a partition and of a dependency round
// over a tree with stores, which counts the reads of each path.
type countingStoreReader struct {
	tree  fs.FS
	reads map[string]int
}

// Read counts the read of path and returns the bytes of the file of the
// workspace or of a store.
func (r countingStoreReader) Read(path string) ([]byte, error) {
	r.reads[path]++
	return plugin.ReadFile(r.tree, path)
}

// ReadDir returns the entries of one directory from the workspace or a
// store, and counts no read.
func (r countingStoreReader) ReadDir(p string) ([]fs.DirEntry, error) {
	return plugin.ReadDir(r.tree, p)
}

// failingFS is a file system in which one path fails to open with
// errBroken, an error that differs from the error of a missing file.
type failingFS struct {
	fs.FS
	fail string
}

// Open opens a path of the file system, and returns errBroken for the
// failing path.
func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.fail {
		return nil, errBroken
	}
	return f.FS.Open(name)
}

// fakeLibrary returns the fake library store of the default key: its root,
// ES5 and the DOM, each at <key>/<file>.
func fakeLibrary() fstest.MapFS {
	return fstest.MapFS{
		defaultRoot + "/" + defaultRoot: {Data: []byte(fakeRoot)},
		defaultRoot + "/" + es5Root:     {Data: []byte(fakeES5)},
		defaultRoot + "/" + domRoot:     {Data: []byte(fakeDOM)},
	}
}

// withLibrary returns a tree with the fake library store of the default key
// beside a workspace.
func withLibrary(workspace fstest.MapFS) storeTree {
	return storeTree{workspace, map[string]fs.FS{typescript.LibraryStore: fakeLibrary()}}
}

// realStores returns the stores of the frontend, the embedded library among
// them.
func realStores(tb assert.TB) map[string]fs.FS {
	tb.Helper()

	stores, err := frontend.Stores(nil)
	assert.NoError(tb, err, "the frontend locates its stores")
	return stores
}

// partitionedThrough partitions every claimed file through the frontend,
// with a door over a tree and its stores.
func partitionedThrough(tb testing.TB, tree storeTree, claimed ...string) [][]plugin.SourceRef {
	tb.Helper()

	refs := make([]plugin.SourceRef, 0, len(claimed))
	for _, p := range claimed {
		refs = append(refs, plugin.SourceRef{Path: p})
	}
	units, err := frontend.New().Partition(tb.Context(), refs, roundReader{tree})
	assert.NoError(tb, err, "the tree partitions")
	return units
}

// runRound runs a round through a TypeScript frontend over the door r of a
// workspace and its stores, under the context of the test, and returns the
// units of the round.
func runRound(tb assert.TB, r plugin.StoreReader, round *plugin.DependencyRound) ([][]plugin.SourceRef, error) {
	tb.Helper()

	dependent, depends := frontend.New().(plugin.Dependent)
	assert.True(tb, depends, "the TypeScript frontend is a dependent")
	return dependent.Dependencies(assert.Context(tb), round, r)
}

// membersOf returns the member paths of each unit.
func membersOf(units [][]plugin.SourceRef) [][]string {
	out := make([][]string, 0, len(units))
	for _, unit := range units {
		members := make([]string, 0, len(unit))
		for _, ref := range unit {
			members = append(members, ref.Path)
		}
		out = append(out, members)
	}
	return out
}

// parsedWithLibrary partitions src/a.ts of a workspace beside the embedded
// library and parses its unit at full depth. It returns the builder of the
// unit and the findings of the parse.
func parsedWithLibrary(tb testing.TB, workspace fstest.MapFS) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	tree := storeTree{workspace, realStores(tb)}
	units := partitionedThrough(tb, tree, aFile)
	f := frontend.New()
	sink := diag.NewSink()
	u := plugin.NewSourceUnit(units[0], tree, plugin.DepthFull, f.Syntax(), brand, sink, f.Name())
	assert.NoError(tb, f.Parse(tb.Context(), u), "the unit parses")
	return u.Graph(), slices.Collect(sink.All())
}
