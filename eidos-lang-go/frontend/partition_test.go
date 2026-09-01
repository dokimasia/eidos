// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

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
				assert.Length(t, ref.Shared, 1, ref.Path+" carries its module file")
				shared[ref.Path] = ref.Shared[0]
			}
		}
		assert.Equal(t, shared["a/x.go"], "go.mod", "the root module governs a/")
		assert.Equal(t, shared["a/deep/y.go"], "go.mod", "and everything under it")
		assert.Equal(t, shared["nested/go.mod"], "", "")
		assert.Equal(t, shared["nested/z.go"], "nested/go.mod",
			"the nearer module wins, its path read quoted and commented")
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

	t.Run("keeps the workspace path without a module", func(t *testing.T) {
		t.Parallel()

		tree := fstest.MapFS{"f/one/x.go": {Data: []byte("package one\n")}}
		f := frontend.New(nil)
		u := unitOf(t, f, tree, "f/one/x.go")
		assert.NoError(t, f.Parse(context.Background(), u), "the unit parses")
		assert.Equal(t, u.Graph().Packages()[0].ID.Package, "f/one",
			"a moduleless tree loads under its directory paths")
	})
}
