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
	"go.dokimi.dev/eidos/sdk/symbol"
)

// parsedFile lowers one Go file at a depth and returns the unit's
// builder for inspection.
func parsedFile(
	tb assert.TB, opts *frontend.Options, depth plugin.Depth, src string,
) *plugin.GraphBuilder {
	tb.Helper()

	tree := fstest.MapFS{"p/a.go": {Data: []byte(src)}}
	f := frontend.New(opts)
	u := plugin.NewSourceUnit(
		[]plugin.SourceRef{{Path: "p/a.go"}}, tree, depth,
		f.Syntax(), diag.NewSink(), f.Name(),
	)
	assert.NoError(tb, f.Parse(context.Background(), u), "the file parses")
	return u.Graph()
}

// onlyFile returns the single parsed file.
func onlyFile(tb assert.TB, gb *plugin.GraphBuilder) *node.File {
	tb.Helper()

	assert.Length(tb, gb.Packages(), 1, "one package declared")
	files := gb.Packages()[0].Files
	assert.Length(tb, files, 1, "one file parsed")
	return files[0]
}

// The lowering turns Go's own shapes into the node model, so each
// mapping the corpus leans on is pinned at the unit level.
func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("tells an alias from a defined type", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype A = int\n\ntype D int\n"))
		a := file.Decls[0].(*node.Alias)
		d := file.Decls[1].(*node.Alias)
		assert.False(t, a.Defined, "= declares a transparent alias")
		assert.True(t, d.Defined, "a defined type is distinct")
		assert.Equal(t, d.Target.Spelling, "int", "the underlying spelling carries")
	})

	t.Run("lowers embeds beside fields", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype H struct {\n\tError\n\tn int `json:\"n\"`\n}\n\ntype Error struct{}\n"))
		h := file.Decls[0].(*node.Struct)
		assert.Length(t, h.Embeds, 1, "the unnamed field embeds")
		assert.Equal(t, h.Embeds[0].Ref.Spelling, "Error", "by its type")
		assert.Length(t, h.Fields, 1, "the named field stays a field")
		assert.Equal(t, h.Fields[0].Tag, `json:"n"`, "its tag stripped of delimiters")
	})

	t.Run("keeps value spellings unevaluated", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\nconst (\n\ta = iota * 2\n\tb\n)\n"))
		a := file.Decls[0].(*node.Constant)
		b := file.Decls[1].(*node.Constant)
		assert.Equal(t, a.Value, "iota * 2", "the expression verbatim")
		assert.Equal(t, b.Value, "", "an implicit carrier stays empty")
	})

	t.Run("lowers signatures whole", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\nfunc F[T any](a, b int, rest ...string) (n int, err error) { return }\n"))
		fn := file.Decls[0].(*node.Function)
		assert.Length(t, fn.TypeParams, 1, "the type parameter carries")
		assert.Equal(t, fn.TypeParams[0].Bounds[0].Spelling, "any", "with its bound")
		assert.Length(t, fn.Params, 3, "a shared type spelling still binds per name")
		assert.Equal(t, fn.Params[2].Variadic, symbol.VariadicPositional, "the tail is variadic")
		assert.Equal(t, fn.Params[2].Type.Spelling, "string", "over its element type")
		assert.Length(t, fn.Returns, 2, "named results carry")
		assert.Equal(t, fn.Returns[1].Name, "err", "by name")
	})

	t.Run("owns a method by its receiver's bare name", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype B[T any] struct{}\n\nfunc (b *B[T]) M() {}\n"))
		m := file.Decls[1].(*node.Method)
		assert.Equal(t, m.Receives.Spelling, "B",
			"pointer and instantiation unwrap: the owner is the name")
	})

	t.Run("splits carriers out of the documentation", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\n// T is a table.\n//+gen:table name=t\ntype T struct{}\n")
		file := onlyFile(t, gb)
		typ := file.Decls[0].(*node.Struct)
		assert.Equal(t, typ.Doc, []string{"T is a table."}, "the carrier is not documentation")
		assert.Length(t, gb.Attachments(), 1, "the carrier attached")
		assert.Equal(t, string(gb.Attachments()[0].Raw.Name), "gen:table", "under its spelling")
	})

	t.Run("keeps a constrained file's declarations out", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"//go:build exotic\n\npackage p\n\ntype Gone struct{}\n")
		file := onlyFile(t, gb)
		assert.Length(t, file.Decls, 0, "outside the tag set nothing declares")
		assert.Length(t, gb.StampRecords(), 1, "the constraint stamps instead")

		tagged := parsedFile(t, &frontend.Options{Tags: []string{"exotic"}}, plugin.DepthFull,
			"//go:build exotic\n\npackage p\n\ntype Gone struct{}\n")
		assert.Length(t, onlyFile(t, tagged).Decls, 1, "inside the set it declares")
	})

	t.Run("splits an external test package by path", func(t *testing.T) {
		t.Parallel()

		tree := fstest.MapFS{
			"p/a.go":       {Data: []byte("package p\n\ntype A struct{}\n")},
			"p/x_test.go":  {Data: []byte("package p_test\n\ntype X struct{}\n")},
			"p/in_test.go": {Data: []byte("package p\n\ntype In struct{}\n")},
		}
		f := frontend.New(nil)
		u := plugin.NewSourceUnit(
			[]plugin.SourceRef{{Path: "p/a.go"}, {Path: "p/in_test.go"}, {Path: "p/x_test.go"}},
			tree, plugin.DepthFull, f.Syntax(), diag.NewSink(), f.Name(),
		)
		assert.NoError(t, f.Parse(context.Background(), u), "the unit parses")
		pkgs := u.Graph().Packages()
		assert.Length(t, pkgs, 2, "one directory, two packages")
		assert.Equal(t, pkgs[0].ID.Package, "p", "the package under its path")
		assert.Equal(t, pkgs[1].ID.Package, "p_test", "the external tests beside it")
	})

	t.Run("loads shallow at signature depth", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthSignatures,
			"package p\n\ntype E struct {\n\tPub int\n\tsecret int\n}\n\nfunc hidden() {}\n"))
		assert.Length(t, file.Decls, 1, "the unexported function stays out")
		e := file.Decls[0].(*node.Struct)
		assert.Length(t, e.Fields, 1, "and so does the unexported field")
		assert.Equal(t, e.Fields[0].Name, "Pub", "the exported one loads")
	})
}
