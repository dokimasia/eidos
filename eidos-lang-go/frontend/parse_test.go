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
// builder for inspection; a trailing path overrides the default,
// for the filename-implied cases.
func parsedFile(
	tb assert.TB, opts *frontend.Options, depth plugin.Depth, src string, at ...string,
) *plugin.GraphBuilder {
	tb.Helper()

	filePath := "p/a.go"
	if len(at) > 0 {
		filePath = at[0]
	}
	tree := fstest.MapFS{filePath: {Data: []byte(src)}}
	f := frontend.New(opts)
	u := plugin.NewSourceUnit(
		[]plugin.SourceRef{{Path: filePath}}, tree, depth,
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

	t.Run("keeps every declaration a broken file still parses", func(t *testing.T) {
		t.Parallel()

		tree := fstest.MapFS{"p/a.go": {Data: []byte(
			"package p\n\ntype Kept struct{}\n\nvar x = \n\ntype AlsoKept struct{}\n",
		)}}
		f := frontend.New(nil)
		sink := diag.NewSink()
		u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: "p/a.go"}}, tree,
			plugin.DepthFull, f.Syntax(), sink, f.Name())
		assert.NoError(t, f.Parse(context.Background(), u), "the error is the source's")

		names := map[string]bool{}
		for _, d := range u.Graph().Packages()[0].Files[0].Decls {
			if s, is := d.(*node.Struct); is {
				names[s.Name] = true
			}
		}
		assert.True(t, names["Kept"] && names["AlsoKept"],
			"one bad token does not erase the file")
		findings := 0
		for range sink.All() {
			findings++
		}
		assert.True(t, findings > 0, "and the error still reports")
	})

	t.Run("lowers a receiverless method as a function", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\nfunc () M() {}\n"))
		_, is := file.Decls[0].(*node.Function)
		assert.True(t, is, "an empty receiver list owns nothing")
	})

	t.Run("keeps an alias of an inline shape transparent", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype A = struct{ X int }\n\ntype B = interface{ M() }\n"))
		a, aliased := file.Decls[0].(*node.Alias)
		assert.True(t, aliased, "= struct{...} states an alias, not a defined struct")
		assert.False(t, a.Defined, "transparent")
		_, aliased = file.Decls[1].(*node.Alias)
		assert.True(t, aliased, "= interface{...} likewise")
	})

	t.Run("attaches carriers on members and trailing comments", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype T struct {\n\t//+gen:col name=a\n\tA int\n}\n\n"+
				"type I interface {\n\t//+gen:op name=m\n\tM()\n}\n\n"+
				"const limit = 1 //+gen:mark\n")
		subjects := map[string]bool{}
		for _, a := range gb.Attachments() {
			switch s := a.Subject.(type) {
			case *node.Field:
				subjects["field:"+s.Name] = true
			case *node.Method:
				subjects["method:"+s.Name] = true
			case *node.Constant:
				subjects["const:"+s.Name] = true
				assert.Equal(t, s.Comment, "", "the carrier is not comment text")
			}
		}
		assert.True(t, subjects["field:A"], "a field carries its directive")
		assert.True(t, subjects["method:M"], "an interface method carries its directive")
		assert.True(t, subjects["const:limit"], "a trailing comment is a carrier position")
	})

	t.Run("unions group carriers with a spec's own doc", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\n//+gen:group name=g\nconst (\n\t// Own doc.\n\ta = 1\n)\n")
		file := onlyFile(t, gb)
		a := file.Decls[0].(*node.Constant)
		assert.Equal(t, a.Doc, []string{"Own doc."}, "the nearer doc text stands")
		assert.Length(t, gb.Attachments(), 1, "the group's carrier still applies")
	})

	t.Run("lowers tool directives as annotations", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\n// V is data.\n//go:embed a.txt b.txt\n//nolint:all\nvar V string\n"))
		v := file.Decls[0].(*node.Variable)
		assert.Equal(t, v.Doc, []string{"V is data."}, "directives are not documentation")
		assert.Length(t, v.Annotations, 2, "both directives carry")
		assert.Equal(t, v.Annotations[0].Name, "go:embed", "named without the marker")
		assert.Equal(t, v.Annotations[0].Args, []string{"a.txt", "b.txt"}, "arguments split")
		assert.Equal(t, v.Annotations[1].Name, "nolint:all", "the nolint kin too")
	})

	t.Run("reads neither legacy build lines nor go:build as data", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\n// D doc.\n// +build linux\nvar D int\n")
		file := onlyFile(t, gb)
		d := file.Decls[0].(*node.Variable)
		assert.Equal(t, d.Doc, []string{"D doc."}, "a legacy build line is not documentation")
		assert.Length(t, d.Annotations, 0, "and not an annotation")
		assert.Length(t, gb.Attachments(), 0, "and never a carrier named build")
	})

	t.Run("hoists the package doc and attaches its carriers to the package", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"// Package p holds fixtures.\n//+gen:module name=fix\npackage p\n")
		pkg := gb.Packages()[0]
		assert.Equal(t, pkg.Doc, []string{"Package p holds fixtures."},
			"the clause doc belongs to the package")
		assert.Length(t, gb.Attachments(), 1, "its carrier attaches")
		assert.True(t, gb.Attachments()[0].Subject == symbol.Symbol(pkg),
			"to the package, not the file")
	})

	t.Run("classifies generated and cgo files", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"// Code generated by protoc. DO NOT EDIT.\n\npackage p\n\nimport \"C\"\n")
		keys := map[string]bool{}
		for _, s := range gb.StampRecords() {
			keys[string(s.Stamp.Key)] = true
		}
		assert.True(t, keys["golang.generated"], "the marker classifies")
		assert.True(t, keys["golang.cgo"], "and so does the C import")
	})

	t.Run("applies filename-implied constraints", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull, "package p\n\ntype Gone struct{}\n", "p/x_windows_amd64.go")
		assert.Length(t, onlyFile(t, gb).Decls, 0, "outside the tag set nothing declares")
		assert.Length(t, gb.StampRecords(), 1, "the implied constraint stamps")
		assert.Equal(t, gb.StampRecords()[0].Stamp.Value.(string), "windows && amd64",
			"spelled from the suffixes")

		tagged := parsedFile(t, &frontend.Options{Tags: []string{"windows", "amd64"}},
			plugin.DepthFull, "package p\n\ntype Gone struct{}\n", "p/x_windows_amd64.go")
		assert.Length(t, onlyFile(t, tagged).Decls, 1, "inside the set it declares")
	})

	t.Run("pairs a two-part filename the way the go tool does", func(t *testing.T) {
		t.Parallel()

		archOnly := parsedFile(t, &frontend.Options{Tags: []string{"amd64"}},
			plugin.DepthFull, "package p\n\ntype Gone struct{}\n", "p/linux_amd64.go")
		assert.Length(t, onlyFile(t, archOnly).Decls, 0,
			"linux_amd64.go implies both suffixes, so the arch alone stays out")
		assert.Equal(t, archOnly.StampRecords()[0].Stamp.Value.(string), "linux && amd64",
			"the pair spells whole")

		both := parsedFile(t, &frontend.Options{Tags: []string{"linux", "amd64"}},
			plugin.DepthFull, "package p\n\ntype Gone struct{}\n", "p/linux_amd64.go")
		assert.Length(t, onlyFile(t, both).Decls, 1, "both tags admit it")
	})

	t.Run("ignores a go:build spelling inside a block comment", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"/*\ngo:build exotic\n*/\n\npackage p\n\ntype Kept struct{}\n"))
		assert.Length(t, file.Decls, 1, "a block comment is not a constraint")
	})

	t.Run("refuses carriers the model cannot address", func(t *testing.T) {
		t.Parallel()

		tree := fstest.MapFS{"p/a.go": {Data: []byte(
			"package p\n\ntype H struct {\n\t//+gen:x\n\tError\n}\n\n" +
				"func F(\n\t//+gen:y\n\tn int,\n) {\n}\n\ntype Error struct{}\n",
		)}}
		f := frontend.New(nil)
		sink := diag.NewSink()
		u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: "p/a.go"}}, tree,
			plugin.DepthFull, f.Syntax(), sink, f.Name())
		assert.NoError(t, f.Parse(context.Background(), u), "the file parses")

		refused := 0
		for d := range sink.All() {
			if d.Code == frontend.UnaddressedCarrier {
				refused++
			}
		}
		assert.Equal(t, refused, 2, "an embed and a parameter each refuse, positioned")
		assert.Length(t, u.Graph().Attachments(), 0, "and nothing attaches in silence")
	})

	t.Run("stamps constraint elements as the interface's type set", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype N interface {\n\t~int | ~float64\n\tM()\n\t_()\n}\n")
		n := onlyFile(t, gb).Decls[0].(*node.Interface)
		assert.Length(t, n.Embeds, 0, "a type-set element is not an embed")
		assert.Length(t, n.Methods, 1, "the method survives and the blank one binds nothing")
		stamps := map[string]any{}
		for _, s := range gb.StampRecords() {
			stamps[string(s.Stamp.Key)] = s.Stamp.Value
		}
		assert.Equal(t, stamps["golang.typeSet"].([]string),
			[]string{"~int | ~float64"}, "the terms stamp as written")
		assert.Equal(t, stamps["golang.constraintInterface"].(bool), true,
			"and the interface marks as a bound")
	})

	t.Run("shares one initializer call across its names", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\nvar a, b = pair()\n\nfunc pair() (int, int) { return 1, 2 }\n"))
		a := file.Decls[0].(*node.Variable)
		b := file.Decls[1].(*node.Variable)
		assert.Equal(t, a.Value, "pair()", "the call is a's initializer")
		assert.Equal(t, b.Value, "pair()", "and b's, because one call binds both")
	})

	t.Run("keeps a blank import's underscore", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\nimport _ \"example.test/side\"\n"))
		assert.Equal(t, file.Imports[0].Alias, "_",
			"a side-effect import round-trips as one")
	})

	t.Run("unwraps a parenthesized receiver", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype T struct{}\n\nfunc (p (T)) M() {}\n"))
		m := file.Decls[1].(*node.Method)
		assert.Equal(t, m.Receives.Spelling, "T", "punctuation owns nothing")
	})

	t.Run("positions every bound name at itself", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\nfunc F(a, b int) (_ int, err error) { return }\n"))
		fn := file.Decls[0].(*node.Function)
		assert.True(t, fn.Params[0].Pos.Col < fn.Params[1].Pos.Col,
			"each parameter sits at its own name")
		assert.Equal(t, fn.Returns[0].Name, "", "a blank result binds no name")
		assert.Equal(t, fn.Returns[1].Name, "err", "a named one keeps it")
	})

	t.Run("stamps what the parse alone can see", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype T struct{}\n\nfunc (t *T) M() {}\n\n"+
				"func Walk() iter.Seq[int] { return nil }\n\n"+
				"func Pairs() iter.Seq2[int, string] { return nil }\n\n"+
				"type Any interface{}\n\ntype Handle uintptr\n")
		keys := map[string]int{}
		for _, s := range gb.StampRecords() {
			keys[string(s.Stamp.Key)]++
		}
		assert.Equal(t, keys["golang.receiverIsPointer"], 1, "the pointer receiver marks")
		assert.Equal(t, keys["golang.iterSeq"], 1, "the one-arity iterator marks")
		assert.Equal(t, keys["golang.iterSeq2"], 1, "and the two-arity one")
		assert.Equal(t, keys["golang.emptyInterface"], 1, "the memberless interface marks")
		assert.Equal(t, keys["golang.underlyingKind"], 1, "the defined type carries its shape")
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
