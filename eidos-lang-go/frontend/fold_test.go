// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// parsedUnit lowers one package of several files and returns its
// files.
func parsedUnit(tb assert.TB, sources map[string]string) []*node.File {
	tb.Helper()

	tree := fstest.MapFS{}
	var refs []plugin.SourceRef
	for _, path := range []string{"p/a.go", "p/b.go"} {
		tree[path] = &fstest.MapFile{Data: []byte(sources[path])}
		refs = append(refs, plugin.SourceRef{Path: path})
	}
	f := frontend.New(nil)
	u := plugin.NewSourceUnit(refs, tree, plugin.DepthFull, f.Syntax(), diag.NewSink(), f.Name())
	assert.NoError(tb, f.Parse(context.Background(), u), "the unit parses")
	assert.Length(tb, u.Graph().Packages(), 1, "one package declared")
	return u.Graph().Packages()[0].Files
}

func TestFold(t *testing.T) {
	t.Parallel()

	t.Run("folds a package's methods onto their receiver's struct across files", func(t *testing.T) {
		t.Parallel()

		files := parsedUnit(t, map[string]string{
			"p/a.go": "package p\n\ntype S struct{}\n\nfunc (s S) First() {}\n",
			"p/b.go": "package p\n\nfunc (s *S) Second() {}\n\ntype Weight float64\n\nfunc (w Weight) String() string { return \"\" }\n",
		})
		var s *node.Struct
		var loose []string
		for _, f := range files {
			for _, decl := range f.Decls {
				switch d := decl.(type) {
				case *node.Struct:
					s = d
				case *node.Method:
					loose = append(loose, d.Receives.Spelling+"."+d.Name)
				}
			}
		}
		assert.NotNil(t, s, "the struct loads")
		assert.Length(t, s.Methods, 2, "both methods hang on it, whichever file spelled them")
		assert.Equal(t, s.Methods[0].Name, "First", "in file then declaration order")
		assert.Equal(t, s.Methods[1].Name, "Second", "the pointer receiver folds too")
		assert.Equal(t, loose, []string{"Weight.String"},
			"a method on a defined type over a builtin stays at file level, owned by its receiver's name")
	})
}
