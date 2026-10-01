// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// countingReader is a partition door that counts the reads of each
// path, the missed ones included.
type countingReader struct {
	tree  fstest.MapFS
	count map[string]int
}

// Read counts the read and returns the file's bytes.
func (r *countingReader) Read(path string) ([]byte, error) {
	r.count[path]++
	return r.tree.ReadFile(path)
}

// The partition fixes what everything downstream keys on: the
// directory grain, the governing module, and the derived import
// path.
func TestPartition(t *testing.T) {
	t.Parallel()

	t.Run("declares the governing go.mod a shared input", func(t *testing.T) {
		t.Parallel()

		tree := fstest.MapFS{
			"go.mod":      {Data: []byte("module example.test/fix // the root\n")},
			"a/x.go":      {Data: []byte("package a\n")},
			"a/deep/y.go": {Data: []byte("package deep\n")},
			"nested/go.mod": {Data: []byte(
				"// nearer than the root\nmodule \"example.test/nested\"\n",
			)},
			"nested/z.go": {Data: []byte("package nested\n")},
		}
		f := frontend.New(nil)
		parts, err := f.Partition(context.Background(), []plugin.SourceRef{
			{Path: "a/deep/y.go"}, {Path: "a/x.go"}, {Path: "nested/z.go"},
		}, treeReader{tree})
		assert.NoError(t, err, "the tree partitions")
		assert.Length(t, parts, 3, "a unit per directory")

		shared := map[string]string{}
		for _, part := range parts {
			for _, ref := range part {
				assert.Length(t, ref.Shared, 1, ref.Path+" declares its module file")
				shared[ref.Path] = ref.Shared[0]
			}
		}
		assert.Equal(t, shared["a/x.go"], "go.mod", "the root module governs a/")
		assert.Equal(t, shared["a/deep/y.go"], "go.mod", "and everything under it")
		assert.Equal(t, shared["nested/go.mod"], "", "")
		assert.Equal(t, shared["nested/z.go"], "nested/go.mod",
			"the nearer module governs, its path read quoted and commented")
	})

	t.Run("derives import paths a resolver can meet", func(t *testing.T) {
		t.Parallel()

		tree := fstest.MapFS{
			"go.mod":  {Data: []byte("module example.test/fix\n")},
			"a/x.go":  {Data: []byte("package a\n")},
			"free.go": {Data: []byte("package free\n")},
		}
		f := frontend.New(nil)
		u := unitOf(t, f, tree, "a/x.go")
		assert.NoError(t, f.Parse(context.Background(), u), "the unit parses")
		assert.Equal(t, u.Graph().Packages()[0].ID.Package, "example.test/fix/a",
			"module path plus the directory under it")
	})

	t.Run("probes each directory of a shared ancestry once", func(t *testing.T) {
		t.Parallel()

		tree := fstest.MapFS{
			"go.mod":   {Data: []byte("module example.test/fix\n")},
			"a/x/x.go": {Data: []byte("package x\n")},
			"a/y/y.go": {Data: []byte("package y\n")},
			"a/z/z.go": {Data: []byte("package z\n")},
		}
		reads := &countingReader{tree: tree, count: map[string]int{}}
		f := frontend.New(nil)
		_, err := f.Partition(context.Background(), []plugin.SourceRef{
			{Path: "a/x/x.go"}, {Path: "a/y/y.go"}, {Path: "a/z/z.go"},
		}, reads)
		assert.NoError(t, err, "the tree partitions")
		assert.Equal(t, reads.count["go.mod"], 1, "the governing module file reads once")
		assert.Equal(t, reads.count["a/go.mod"], 1,
			"and the shared ancestor is probed once, the siblings' walks ending at its cached answer")
	})

	t.Run("keeps the workspace path without a module", func(t *testing.T) {
		t.Parallel()

		tree := fstest.MapFS{"f/one/x.go": {Data: []byte("package one\n")}}
		f := frontend.New(nil)
		u := unitOf(t, f, tree, "f/one/x.go")
		assert.NoError(t, f.Parse(context.Background(), u), "the unit parses")
		assert.Equal(t, u.Graph().Packages()[0].ID.Package, "f/one",
			"a moduleless tree loads under its directory paths")
	})

	t.Run("placeUnit", func(t *testing.T) {
		t.Parallel()

		t.Run("loads a standard library directory under its path in GOROOT", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, dependencyPackage(t, depWorkspace(), nil, inRoot(fmtFile)), fmtPackage,
				"the directory inside the store is the import path")
		})

		t.Run("loads a module cache directory under its module path", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, dependencyPackage(t, depWorkspace(), []string{appGoMod}, cached(libDir+"/pkg.go")),
				libPackage, "the module path and the directory under the module")
		})

		t.Run("loads a module's root directory under the module path", func(t *testing.T) {
			t.Parallel()

			stores := depStores()
			cache := depCache()
			cache[libTree+"/lib.go"] = &fstest.MapFile{Data: []byte("package lib\n")}
			stores[frontend.ModCacheStore] = cache
			got := dependencyPackageIn(t, depWorkspace(), stores, []string{appGoMod}, cached(libTree+"/lib.go"))
			assert.Equal(t, got, libModule, "the tree's root is the module's package")
		})

		t.Run("unescapes the module path of the cache directory", func(t *testing.T) {
			t.Parallel()

			const escaped, version = "github.com/!burnt!sushi/toml", "v1.6.0"
			stores := depStores()
			stores[frontend.ModCacheStore] = fstest.MapFS{
				escaped + "@" + version + "/toml.go": {Data: []byte("package toml\n")},
			}
			assert.Equal(t, dependencyPackageIn(t, depWorkspace(), stores, []string{appGoMod},
				cached(escaped+"@"+version+"/toml.go")), "github.com/BurntSushi/toml",
				"an exclamation mark escapes the upper-case letter after it")
		})

		t.Run("loads a replacement's directory under the original path", func(t *testing.T) {
			t.Parallel()

			const fork, forkVersion = "example.com/fork", "v1.3.0"
			tree := depWorkspace()
			tree[appGoMod] = &fstest.MapFile{Data: append(tree[appGoMod].Data,
				"replace "+libModule+" => "+fork+" "+forkVersion+"\n"...)}
			stores := depStores()
			stores[frontend.ModCacheStore] = fstest.MapFS{
				fork + "@" + forkVersion + "/pkg/pkg.go": {Data: []byte(libSource)},
			}
			assert.Equal(t, dependencyPackageIn(t, tree, stores, []string{appGoMod},
				cached(fork+"@"+forkVersion+"/pkg/pkg.go")), libPackage, "the import path the workspace spells")
		})

		t.Run("loads a vendored directory under its path under vendor", func(t *testing.T) {
			t.Parallel()

			tree := depWorkspace()
			tree["vendor/"+libPackage+"/pkg.go"] = &fstest.MapFile{Data: []byte(libSource)}
			assert.Equal(t, dependencyPackage(t, tree, []string{appGoMod}, "vendor/"+libPackage+"/pkg.go"),
				libPackage, "the path under the vendor directory beside the go.mod")
		})

		t.Run("stamps no module identity on a dependency package", func(t *testing.T) {
			t.Parallel()

			gb, err := dependencyUnit(depWorkspace(), depStores(), []string{appGoMod}, cached(libDir+"/pkg.go"))
			assert.NoError(t, err, "the unit parses")
			assert.Empty(t, gb.StampRecords(), "a dependency is governed by no workspace module")
		})

		errs := []struct {
			name   string
			member string
			tree   fstest.MapFS
			want   string
		}{
			{
				name:   "returns an error for a store the Go frontend does not read",
				member: plugin.StorePath("jdk", "java.base/java/lang/Object.class"),
				want:   "does not read",
			},
			{
				name:   "returns an error for a module cache directory that names no version",
				member: cached("example.com/unversioned/x.go"),
				want:   "names no module version",
			},
			{
				name:   "returns an error for a cache path that does not unescape",
				member: cached("example.com/Upper@v1.0.0/x.go"),
				want:   "example.com/Upper",
			},
			{
				name:   "returns an error for a cache version that does not unescape",
				member: cached("example.com/lib@V1.0.0/x.go"),
				want:   "V1.0.0",
			},
			{
				name:   "returns an error for a go.mod that does not parse",
				member: cached(libDir + "/pkg.go"),
				tree:   fstest.MapFS{appGoMod: {Data: []byte("module (\n")}},
				want:   appGoMod,
			},
		}
		for _, tt := range errs {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tree := tt.tree
				if tree == nil {
					tree = depWorkspace()
				}
				_, err := dependencyUnit(tree, depStores(), []string{appGoMod}, tt.member)
				assert.HasError(t, err, "the unit's place does not derive")
				assert.Contains(t, err.Error(), tt.want, "the error names what failed")
			})
		}
	})
}

// dependencyUnit parses one dependency unit over a workspace and its
// stores at signature depth, its members declaring the shared inputs,
// and returns the unit's builder and the parse's error.
func dependencyUnit(
	tree fstest.MapFS, stores map[string]fs.FS, shared []string, members ...string,
) (*plugin.GraphBuilder, error) {
	refs := make([]plugin.SourceRef, len(members))
	for i, m := range members {
		refs[i] = plugin.SourceRef{Path: m, Shared: shared}
	}
	f := frontend.New(nil)
	u := plugin.NewSourceUnit(refs, storeTree{tree, stores}, plugin.DepthSignatures,
		f.Syntax(), brand, diag.NewSink(), f.Name())
	return u.Graph(), f.Parse(context.Background(), u)
}

// dependencyPackage parses one dependency unit over a workspace and the
// fixture stores and returns the path of the one package it declares.
func dependencyPackage(tb assert.TB, tree fstest.MapFS, shared []string, members ...string) string {
	tb.Helper()

	return dependencyPackageIn(tb, tree, depStores(), shared, members...)
}

// dependencyPackageIn parses one dependency unit over a workspace and
// the stores given and returns the path of the one package it declares.
func dependencyPackageIn(
	tb assert.TB, tree fstest.MapFS, stores map[string]fs.FS, shared []string, members ...string,
) string {
	tb.Helper()

	gb, err := dependencyUnit(tree, stores, shared, members...)
	assert.NoError(tb, err, "the unit parses")
	assert.Length(tb, gb.Packages(), 1, "the unit declares one package")
	return gb.Packages()[0].ID.Package
}
