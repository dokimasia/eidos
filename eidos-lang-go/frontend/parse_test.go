// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// brand is the brand every fixture unit reads its carriers under.
const brand = string(frontendtest.Brand)

// parsedFile lowers one Go file at a depth and returns the unit's
// builder for inspection; a trailing path overrides the default,
// for the filename-implied cases.
func parsedFile(
	tb assert.TB, opts *frontend.Options, depth plugin.Depth, src string, at ...string,
) *plugin.GraphBuilder {
	tb.Helper()

	gb, _ := parsedFindings(tb, opts, depth, src, at...)
	return gb
}

// parsedFindings lowers one Go file as [parsedFile] does and returns
// the findings the parse reported beside the builder.
func parsedFindings(
	tb assert.TB, opts *frontend.Options, depth plugin.Depth, src string, at ...string,
) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	filePath := "p/a.go"
	if len(at) > 0 {
		filePath = at[0]
	}
	tree := fstest.MapFS{filePath: {Data: []byte(src)}}
	f := frontend.New(opts)
	sink := diag.NewSink()
	u := plugin.NewSourceUnit(
		[]plugin.SourceRef{{Path: filePath}}, tree, depth,
		f.Syntax(), brand, sink, f.Name(),
	)
	assert.NoError(tb, f.Parse(context.Background(), u), "the file parses")
	return u.Graph(), slices.Collect(sink.All())
}

// stampValues returns the values stamped under one key, in stamp
// order.
func stampValues(gb *plugin.GraphBuilder, key meta.KeyName) []any {
	var out []any
	for _, s := range gb.StampRecords() {
		if s.Stamp.Key == key {
			out = append(out, s.Stamp.Value)
		}
	}
	return out
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
// mapping the corpus depends on is pinned at the unit level.
func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("records a standard library file's import of a vendored package under vendor", func(t *testing.T) {
			t.Parallel()

			gb, err := dependencyUnit(depWorkspace(), depStores(), nil, inRoot(fmtFile))
			assert.NoError(t, err, "the unit parses")
			assert.Equal(t, importPaths(onlyFile(t, gb)), []string{"vendor/" + vendoredPath},
				"the go command reads the standard library's module imports from its vendor tree")
		})

		t.Run("tells an alias from a defined type", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\ntype A = int\n\ntype D int\n"))
			a := file.Decls[0].(*node.Alias)
			d := file.Decls[1].(*node.Alias)
			assert.False(t, a.Defined, "= declares a transparent alias")
			assert.True(t, d.Defined, "a defined type is distinct")
			assert.Equal(t, d.Target.Spelling, "int", "the underlying spelling is kept")
		})

		t.Run("lowers embeds beside fields", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\ntype H struct {\n\tError\n\tn int `json:\"n\"`\n}\n\ntype Error struct{}\n"))
			h := file.Decls[0].(*node.Struct)
			assert.Length(t, h.Embeds, 1, "the unnamed field embeds")
			assert.Equal(t, h.Embeds[0].Ref.Spelling, "Error", "by its type")
			assert.Length(t, h.Fields, 1, "the named field remains a field")
			assert.Equal(t, h.Fields[0].Tag, `json:"n"`, "its tag stripped of delimiters")
		})

		t.Run("keeps value spellings unevaluated", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nconst (\n\ta = iota * 2\n\tb\n)\n"))
			a := file.Decls[0].(*node.Constant)
			b := file.Decls[1].(*node.Constant)
			assert.Equal(t, a.Value, "iota * 2", "the expression verbatim")
			assert.Equal(t, b.Value, "", "an implicit row keeps an empty value")
		})

		t.Run("lowers signatures whole", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nfunc F[T any](a, b int, rest ...string) (n int, err error) { return }\n"))
			fn := file.Decls[0].(*node.Function)
			assert.Length(t, fn.TypeParams, 1, "the type parameter is kept")
			assert.Equal(t, fn.TypeParams[0].Bounds[0].Spelling, "any", "with its bound")
			assert.Length(t, fn.Params, 3, "a shared type spelling still binds per name")
			assert.Equal(t, fn.Params[2].Variadic, symbol.VariadicPositional, "the tail is variadic")
			assert.Equal(t, fn.Params[2].Type.Spelling, "string", "over its element type")
			assert.Length(t, fn.Returns, 2, "named results are kept")
			assert.Equal(t, fn.Returns[1].Name, "err", "by name")
		})

		t.Run("attaches a method to its receiver's bare name", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\ntype B[T any] struct{}\n\nfunc (b *B[T]) M() {}\n"))
			assert.Length(t, file.Decls, 1, "the method folds onto its receiver's struct")
			m := file.Decls[0].(*node.Struct).Methods[0]
			assert.Equal(t, m.Receives.Spelling, "B",
				"pointer and instantiation unwrap: the owner is the name")
		})

		t.Run("splits carriers out of the documentation", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\n// T is a table.\n//+fixture:gen:table name=t\ntype T struct{}\n")
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

		t.Run("records the imports of a file the build excludes", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"//go:build exotic\n\npackage p\n\nimport \""+signatureImport+"\"\n\ntype Gone struct{}\n"))
			assert.Equal(t, importPaths(file), []string{signatureImport}, "the header parses")
		})

		t.Run("reports no syntax error after the imports of a file the build excludes", func(t *testing.T) {
			t.Parallel()

			_, findings := parsedFindings(t, nil, plugin.DepthFull,
				"//go:build exotic\n\npackage p\n\nimport \""+signatureImport+"\"\n\nfunc F() { var = }\n")
			assert.Empty(t, findings, "the go command reads only the header of an excluded file")
		})

		t.Run("reports a syntax error in the imports of a file the build excludes", func(t *testing.T) {
			t.Parallel()

			_, findings := parsedFindings(t, nil, plugin.DepthFull,
				"//go:build exotic\n\npackage p\n\nimport (\n\t\""+signatureImport+"\"\n\tvar\n)\n")
			assert.NotEmpty(t, findings, "the header's own error reports")
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
				tree, plugin.DepthFull, f.Syntax(), brand, diag.NewSink(), f.Name(),
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
				plugin.DepthFull, f.Syntax(), brand, sink, f.Name())
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
			assert.True(t, findings > 0, "the error still reports")
		})

		t.Run("declares nothing for a file without a package clause", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{"p/a.go": {Data: []byte("func F() {}\n")}}
			f := frontend.New(nil)
			sink := diag.NewSink()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: "p/a.go"}}, tree,
				plugin.DepthFull, f.Syntax(), brand, sink, f.Name())
			assert.NoError(t, f.Parse(context.Background(), u), "the missing clause is the source's error")
			assert.Empty(t, u.Graph().Packages(), "the file declares nothing")
			findings := 0
			for range sink.All() {
				findings++
			}
			assert.True(t, findings > 0, "the syntax error reports")
		})

		t.Run("leaves out an embed the parser synthesized past the end of a file", func(t *testing.T) {
			t.Parallel()

			st := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\ntype S struct {\n\tA\n\t*")).Decls[0].(*node.Struct)
			assert.Length(t, st.Embeds, 1, "the embed the source spells is kept")
			assert.Equal(t, st.Embeds[0].Ref.Spelling, "A", "the synthesized one is left out")

			it := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\ntype I interface {\n\tJ\n\t*")).Decls[0].(*node.Interface)
			assert.Length(t, it.Embeds, 1, "an interface keeps its spelled embed")
			assert.Equal(t, it.Embeds[0].Ref.Spelling, "J", "an interface leaves the synthesized one out")
		})

		t.Run("names the package from the files inside the build", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				"p/a.go": {
					Data: []byte("//go:build exotic\n\n// Package other is outside.\n" +
						"//+fixture:gen:table name=t\npackage other\n"),
				},
				"p/b.go": {Data: []byte("// Package p is inside.\npackage p\n\ntype T struct{}\n")},
			}
			f := frontend.New(nil)
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: "p/a.go"}, {Path: "p/b.go"}}, tree,
				plugin.DepthFull, f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.NoError(t, f.Parse(context.Background(), u), "the unit parses")
			pkg := u.Graph().Packages()[0]
			assert.Equal(t, pkg.Name, "p", "the file inside the build names the package")
			assert.Equal(t, pkg.Doc, []string{"Package p is inside."}, "the file inside the build documents it")
			assert.Empty(t, u.Graph().Attachments(), "the excluded file's carrier attaches nowhere")
		})

		t.Run("reports MixedPackage for a second package name in one directory", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				"p/a.go": {Data: []byte("package p\n")},
				"p/b.go": {Data: []byte("package q\n")},
			}
			f := frontend.New(nil)
			sink := diag.NewSink()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: "p/a.go"}, {Path: "p/b.go"}}, tree,
				plugin.DepthFull, f.Syntax(), brand, sink, f.Name())
			assert.NoError(t, f.Parse(context.Background(), u), "the unit parses")
			assert.Equal(t, u.Graph().Packages()[0].Name, "p", "the first name is kept")
			var codes []diag.Code
			for d := range sink.All() {
				codes = append(codes, d.Code)
			}
			assert.Equal(t, codes, []diag.Code{frontend.MixedPackage}, "the second name reports")
		})

		t.Run("lowers a receiverless method as a function", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nfunc () M() {}\n"))
			_, is := file.Decls[0].(*node.Function)
			assert.True(t, is, "an empty receiver list names no owner")
		})

		t.Run("keeps an alias of an inline shape transparent", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\ntype A = struct{ X int }\n\ntype B = interface{ M() }\n"))
			a, aliased := file.Decls[0].(*node.Alias)
			assert.True(t, aliased, "= struct{...} states an alias, not a defined struct")
			assert.False(t, a.Defined, "transparent")
			_, aliased = file.Decls[1].(*node.Alias)
			assert.True(t, aliased, "= interface{...} states one too")
		})

		t.Run("attaches a carrier above a member to the member", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\ntype T struct {\n\t//+fixture:gen:col name=a\n\tA int\n}\n\n"+
					"type I interface {\n\t//+fixture:gen:op name=m\n\tM()\n}\n")
			subjects := map[string]bool{}
			for _, a := range gb.Attachments() {
				switch s := a.Subject.(type) {
				case *node.Field:
					subjects["field:"+s.Name] = true
				case *node.Method:
					subjects["method:"+s.Name] = true
				}
			}
			assert.True(t, subjects["field:A"], "a field takes its directive")
			assert.True(t, subjects["method:M"], "an interface method takes its directive")
		})

		t.Run("attaches a carrier in a trailing comment to its declaration", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull, "package p\n\nconst limit = 1 //+fixture:gen:mark\n")
			assert.Length(t, gb.Attachments(), 1, "the trailing comment is a carrier position")
			limit, is := gb.Attachments()[0].Subject.(*node.Constant)
			assert.True(t, is, "the constant takes the directive")
			assert.Equal(t, limit.Comment, "", "the carrier is not comment text")
		})

		t.Run("unions group carriers with a spec's own doc", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\n//+fixture:gen:group name=g\nconst (\n\t// Own doc.\n\ta = 1\n)\n")
			file := onlyFile(t, gb)
			a := file.Decls[0].(*node.Constant)
			assert.Equal(t, a.Doc, []string{"Own doc."}, "the nearer doc text is kept")
			assert.Length(t, gb.Attachments(), 1, "the group's carrier still applies")
		})

		t.Run("lowers tool directives as annotations", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\n// V is data.\n//go:embed a.txt b.txt\n//nolint:all\nvar V string\n"))
			v := file.Decls[0].(*node.Variable)
			assert.Equal(t, v.Doc, []string{"V is data."}, "directives are not documentation")
			assert.Length(t, v.Annotations, 2, "both directives lower")
			assert.Equal(t, v.Annotations[0].Name, "go:embed", "named without the marker")
			assert.Equal(t, v.Annotations[0].Args, []string{"a.txt", "b.txt"}, "arguments split")
			assert.Equal(t, v.Annotations[1].Name, "nolint:all", "the nolint directive lowers too")
		})

		t.Run("reads a legacy build line in a doc comment as documentation", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\n// D doc.\n// +build linux\nvar D int\n")
			file := onlyFile(t, gb)
			d := file.Decls[0].(*node.Variable)
			assert.Equal(t, d.Doc, []string{"D doc.", "+build linux"},
				"the line is documentation, as go/ast reads it")
			assert.Length(t, d.Annotations, 0, "the line is no annotation")
			assert.Length(t, gb.Attachments(), 0, "the line opens no carrier")
		})

		t.Run("attaches the package doc's carriers to the package", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"// Package p contains fixtures.\n//+fixture:gen:module name=fix\npackage p\n")
			pkg := gb.Packages()[0]
			assert.Equal(t, pkg.Doc, []string{"Package p contains fixtures."},
				"the clause doc belongs to the package")
			assert.Length(t, gb.Attachments(), 1, "its carrier attaches")
			assert.True(t, gb.Attachments()[0].Subject == symbol.Symbol(pkg),
				"to the package, not the file")
		})

		t.Run("classifies a generated file", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"// Code generated by protoc. DO NOT EDIT.\n\npackage p\n")
			assert.Length(t, stampValues(gb, golang.GeneratedKey), 1, "the marker line is stamped")
		})

		t.Run("classifies a cgo file", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull, "package p\n\nimport \"C\"\n")
			assert.Length(t, stampValues(gb, golang.CgoKey), 1, "the C import is stamped")
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

		t.Run("reads a filename's constraint the way the go tool does", func(t *testing.T) {
			t.Parallel()

			const src = "package p\n\ntype Kept struct{}\n"
			pair := parsedFile(t, &frontend.Options{Tags: []string{"amd64"}},
				plugin.DepthFull, src, "p/x_linux_amd64.go")
			assert.Length(t, onlyFile(t, pair).Decls, 0,
				"x_linux_amd64.go implies both suffixes, so the arch alone keeps it out")
			assert.Equal(t, pair.StampRecords()[0].Stamp.Value.(string), "linux && amd64",
				"the pair spells whole")
			both := parsedFile(t, &frontend.Options{Tags: []string{"linux", "amd64"}},
				plugin.DepthFull, src, "p/x_linux_amd64.go")
			assert.Length(t, onlyFile(t, both).Decls, 1, "both tags admit it")

			leading := parsedFile(t, &frontend.Options{Tags: []string{"amd64"}},
				plugin.DepthFull, src, "p/linux_amd64.go")
			assert.Length(t, onlyFile(t, leading).Decls, 1,
				"the element before the first underscore is ignored, so linux_amd64.go implies amd64 alone")

			dotted := parsedFile(t, &frontend.Options{Tags: []string{"linux"}},
				plugin.DepthFull, src, "p/x_windows.pb.go")
			assert.Length(t, onlyFile(t, dotted).Decls, 0,
				"the name is cut at its first dot, so x_windows.pb.go implies windows")

			test := parsedFile(t, &frontend.Options{Tags: []string{"linux"}},
				plugin.DepthFull, src, "p/x_windows_test.go")
			assert.Length(t, onlyFile(t, test).Decls, 0, "a final test element drops before the suffix reads")

			bare := parsedFile(t, nil, plugin.DepthFull, src, "p/windows.go")
			assert.Length(t, onlyFile(t, bare).Decls, 1, "a name with no underscore implies nothing")
		})

		t.Run("ignores a go:build spelling inside a block comment", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"/*\ngo:build exotic\n*/\n\npackage p\n\ntype Kept struct{}\n"))
			assert.Length(t, file.Decls, 1, "a block comment is not a constraint")
		})

		t.Run("reports UnaddressedCarrier for each carrier the model cannot address", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{"p/a.go": {Data: []byte(
				"package p\n\nimport \"io\" //+fixture:gen:z\n\n" +
					"func F(\n\t//+fixture:gen:y\n\tn int,\n) (\n\t//+fixture:gen:w\n\tout int,\n) {\n\treturn n\n}\n\n" +
					"var _ io.Reader\n",
			)}}
			f := frontend.New(nil)
			sink := diag.NewSink()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: "p/a.go"}}, tree,
				plugin.DepthFull, f.Syntax(), brand, sink, f.Name())
			assert.NoError(t, f.Parse(context.Background(), u), "the file parses")

			refused := 0
			for d := range sink.All() {
				if d.Code == frontend.UnaddressedCarrier {
					refused++
				}
			}
			assert.Equal(t, refused, 3, "an import, a parameter and a result each refuse, positioned")
			assert.Length(t, u.Graph().Attachments(), 0, "nothing attaches in silence")
		})

		t.Run("reports BadCarrier for a carrier outside the kernel grammar", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedFindings(t, nil, plugin.DepthFull,
				"package p\n\n//+fixture:gen:table name=\nfunc F() {}\n")
			assert.Length(t, found, 1, "the malformed carrier reports once")
			assert.Equal(t, found[0].Code, frontend.BadCarrier, "under the grammar refusal's code")
			assert.Equal(t, found[0].Pos.Line, 3, "at the carrier's own line")
			assert.Empty(t, gb.Attachments(), "the function it documents takes no directive")
		})

		t.Run("reports ContinuedCarrier for a continued carrier gofmt moves", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedFindings(t, nil, plugin.DepthFull,
				"package p\n\n// T is a table.\n//fixture:gen:table \\\n// name=t\ntype T struct{}\n")
			assert.Length(t, found, 1, "the continued carrier reports once")
			assert.Equal(t, found[0].Code, plugin.ContinuedCarrier, "under the kit's code")
			assert.Empty(t, gb.Attachments(), "the refused carrier attaches nothing")
		})

		t.Run("attaches the carrier gofmt moved under a refused one", func(t *testing.T) {
			t.Parallel()

			// The file as gofmt leaves it: the continuation remains in the
			// text, and both directive-shaped carriers move to the end.
			gb, _ := parsedFindings(t, nil, plugin.DepthFull,
				"package p\n\n// T is a table.\n// name=t\n//\n"+
					"//fixture:gen:table \\\n//fixture:gen:index fields=[b]\ntype T struct{}\n")
			attached := gb.Attachments()
			assert.Length(t, attached, 1, "the moved carrier attaches alone")
			assert.Equal(t, string(attached[0].Raw.Name), "gen:index", "whole, and not folded into the refused one")
		})

		t.Run("lowers an embedded field as a declaration of its own", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\ntype H struct {\n"+
					"\t// Base has the shared fields.\n\t//go:fix inline\n\t//+fixture:gen:x\n"+
					"\t*Base `json:\"base\"` // promoted\n}\n\ntype Base struct{}\n")
			h := onlyFile(t, gb).Decls[0].(*node.Struct)
			assert.Length(t, h.Embeds, 1, "the embed lowers")
			e := h.Embeds[0]
			assert.Equal(t, e.Doc, []string{"Base has the shared fields."}, "with its doc")
			assert.Equal(t, e.Comment, "promoted", "its trailing comment")
			assert.Equal(t, e.Tag, `json:"base"`, "its tag")
			assert.Equal(t, e.Annotations, symbol.Annotations{{Name: "go:fix", Args: []string{"inline"}}},
				"its tool directive")
			assert.Equal(t, e.Ref.Form, symbol.FormOptional, "the pointer embed is an optional form")
			attached := gb.Attachments()
			assert.Length(t, attached, 1, "the carrier attaches to the embed")
			assert.True(t, attached[0].Subject == e, "itself, not its host")
		})

		t.Run("keeps trailing comments on every kind that ends a line", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\n"+
					"import \"io\" // for Reader\n\n"+
					"type S struct{} // the struct\n\n"+
					"type I interface {\n\tRead() // the method\n} // the interface\n\n"+
					"type A = int // the alias\n\n"+
					"type E int // the enum\n\nconst EOne E = 1\n\n"+
					"func F(\n\ta int, // the param\n) (\n\tout int, // the result\n) {\n\treturn a\n} // the function\n\n"+
					"func (S) M() {} // the method\n\nvar _ io.Reader\n")
			file := onlyFile(t, gb)
			assert.Equal(t, file.Imports[0].Comment, "for Reader", "an import's comment")
			byName := map[string]string{}
			for _, d := range file.Decls {
				switch x := d.(type) {
				case *node.Struct:
					byName["S"] = x.Comment
					byName["S.M"] = x.Methods[0].Comment
				case *node.Interface:
					byName["I"] = x.Comment
					byName["I.Read"] = x.Methods[0].Comment
				case *node.Alias:
					byName[x.Name] = x.Comment
				case *node.Enum:
					byName[x.Name] = x.Comment
				case *node.Function:
					byName["F"] = x.Comment
					byName["F.a"] = x.Params[0].Comment
					byName["F.out"] = x.Returns[0].Comment
				}
			}
			assert.Equal(t, byName, map[string]string{
				"S": "the struct", "I": "the interface", "I.Read": "the method",
				"A": "the alias", "E": "the enum",
				"F": "the function", "F.a": "the param", "F.out": "the result",
				"S.M": "the method",
			}, "each declaration keeps the comment it ends its line with")
		})

		t.Run("gives the file the tool directives outside every declaration", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"//go:build !exotic\n\n//go:generate stringer -type=E\n\n"+
					"package p\n\n//go:noinline\n\n// Free prose drops.\n\ntype E int\n")
			file := onlyFile(t, gb)
			assert.Equal(t, file.Annotations, symbol.Annotations{
				{Name: "go:generate", Args: []string{"stringer", "-type=E"}},
				{Name: "go:noinline"},
			}, "the file has them in source order, and a build constraint is "+
				"configuration the split reads as no annotation")
		})

		t.Run("stamps constraint elements as the interface's type set", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\ntype N interface {\n\t~int | ~float64\n\tM()\n\t_()\n}\n")
			n := onlyFile(t, gb).Decls[0].(*node.Interface)
			assert.Length(t, n.Embeds, 0, "a type-set element is not an embed")
			assert.Length(t, n.Methods, 1, "the method survives, and the blank one binds nothing")
			assert.Equal(t, stampValues(gb, golang.TypeSetKey), []any{[]string{"~int | ~float64"}},
				"the terms stamp as written")
			assert.Equal(t, stampValues(gb, golang.ConstraintInterfaceKey), []any{true},
				"the interface marks as a bound")
		})

		t.Run("shares one initializer call across its names", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nvar a, b = pair()\n\nfunc pair() (int, int) { return 1, 2 }\n"))
			a := file.Decls[0].(*node.Variable)
			b := file.Decls[1].(*node.Variable)
			assert.Equal(t, a.Value, "pair()", "the call is a's initializer")
			assert.Equal(t, b.Value, "pair()", "the call is b's too, because one call binds both")
		})

		t.Run("leaves out the declarations no code can address", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nfunc init() {}\n\nfunc init() {}\n\nfunc _() {}\n\nfunc _() {}\n\n"+
					"type T struct {\n\t_ [4]byte\n\tA, _ int\n\t_ [2]byte\n}\n\n"+
					"func (T) _() {}\n\nfunc (T) init() {}\n"))
			assert.Length(t, file.Decls, 1,
				"two inits and two blank functions spell no declaration, so no identity repeats")
			st := file.Decls[0].(*node.Struct)
			assert.Length(t, st.Fields, 1, "the padding fields and the blank name beside A bind nothing")
			assert.Equal(t, st.Fields[0].Name, "A", "A is kept")
			assert.Length(t, st.Methods, 1, "a blank method binds nothing")
			assert.Equal(t, st.Methods[0].Name, "init",
				"a method named init is kept, because a value can call it")
		})

		t.Run("keeps a blank import's underscore", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nimport _ \"example.test/side\"\n"))
			assert.Equal(t, file.Imports[0].Alias, "_",
				"a side-effect import round-trips as one")
		})

		t.Run("records a dot import as a wildcard", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nimport . \"example.test/dsl\"\n"))
			assert.True(t, file.Imports[0].Wildcard, "every exported name enters the file's scope")
		})

		t.Run("keeps an aliased import's alias", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nimport ren \"example.test/other\"\n"))
			assert.Equal(t, file.Imports[0].Alias, "ren", "the alias as written")
		})

		t.Run("unwraps a parenthesized receiver", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\ntype T struct{}\n\nfunc (p (T)) M() {}\n"))
			m := file.Decls[0].(*node.Struct).Methods[0]
			assert.Equal(t, m.Receives.Spelling, "T", "punctuation names no owner")
		})

		t.Run("positions every bound name at itself", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nfunc F(a, b int) (_ int, err error) { return }\n"))
			fn := file.Decls[0].(*node.Function)
			assert.True(t, fn.Params[0].Pos.Col < fn.Params[1].Pos.Col,
				"each parameter is positioned at its own name")
			assert.Equal(t, fn.Returns[0].Name, "_",
				"a blank result keeps its underscore, because a mixed list "+
					"re-renders only fully named")
			assert.Equal(t, fn.Returns[1].Name, "err", "a named one keeps it")
		})

		t.Run("stamps a pointer receiver", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull, "package p\n\ntype T struct{}\n\nfunc (t *T) M() {}\n")
			assert.Equal(t, stampValues(gb, golang.ReceiverPointerKey), []any{true}, "the method marks")
		})

		t.Run("stamps an iter.Seq result", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nimport \"iter\"\n\nfunc Walk() iter.Seq[int] { return nil }\n")
			assert.Equal(t, stampValues(gb, golang.IterSeqKey), []any{true}, "the one-arity iterator marks")
		})

		t.Run("stamps an iter.Seq2 result", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nimport \"iter\"\n\nfunc Pairs() iter.Seq2[int, string] { return nil }\n")
			assert.Equal(t, stampValues(gb, golang.IterSeq2Key), []any{true}, "the two-arity iterator marks")
		})

		t.Run("stamps an iter.Seq result through an aliased import", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nimport it \"iter\"\n\nfunc Walk() it.Seq[int] { return nil }\n")
			assert.Equal(t, stampValues(gb, golang.IterSeqKey), []any{true},
				"the package the import names decides, not the qualifier")
		})

		t.Run("stamps no iterator for a Seq another package named iter declares", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nimport \"example.test/iter\"\n\nfunc Walk() iter.Seq[int] { return nil }\n")
			assert.Empty(t, stampValues(gb, golang.IterSeqKey), "the qualifier binds another import path")
		})

		t.Run("stamps a memberless interface", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull, "package p\n\ntype Any interface{}\n")
			assert.Equal(t, stampValues(gb, golang.EmptyInterfaceKey), []any{true}, "the interface marks")
		})

		t.Run("stamps a defined type's underlying shape", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull, "package p\n\ntype Handle uintptr\n")
			assert.Length(t, stampValues(gb, golang.UnderlyingKey), 1, "the defined type is stamped with its shape")
		})

		t.Run("stamps the package's module identity", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				"go.mod":      {Data: []byte("module example.test/fix\n")},
				"a/x.go":      {Data: []byte("package a\n\ntype A int\n")},
				"a/x_test.go": {Data: []byte("package a_test\n\ntype Probe int\n")},
			}
			f := frontend.New(nil)
			u := unitOf(t, f, tree, "a/x.go")
			assert.NoError(t, f.Parse(context.Background(), u), "the unit parses")

			stamped := map[string]map[string]string{}
			for _, s := range u.Graph().StampRecords() {
				pkg, is := s.Subject.(*node.Package)
				if !is {
					continue
				}
				if stamped[pkg.ID.Package] == nil {
					stamped[pkg.ID.Package] = map[string]string{}
				}
				value, _ := s.Stamp.Value.(string)
				stamped[pkg.ID.Package][string(s.Stamp.Key)] = value
			}
			assert.Equal(t, stamped["example.test/fix/a"], map[string]string{
				string(meta.ModuleKey):     "example.test/fix",
				string(meta.ModuleRootKey): ".",
			}, "the package has the neutral module facts")
			assert.Equal(t, stamped["example.test/fix/a_test"][string(meta.ModuleKey)],
				"example.test/fix", "the external test package beside it has them too")

			free := unitOf(t, f, fstest.MapFS{"f/one/x.go": {Data: []byte("package one\n")}}, "f/one/x.go")
			assert.NoError(t, f.Parse(context.Background(), free), "a moduleless unit parses")
			for _, s := range free.Graph().StampRecords() {
				assert.False(t, s.Stamp.Key == meta.ModuleKey || s.Stamp.Key == meta.ModuleRootKey,
					"a moduleless unit stamps no module identity: absence is the negative")
			}
		})

		t.Run("loads shallow at signature depth", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthSignatures,
				"package p\n\ntype E struct {\n\tPub int\n\tsecret int\n}\n\nfunc hidden() {}\n"))
			assert.Length(t, file.Decls, 1, "the unexported function is left out")
			e := file.Decls[0].(*node.Struct)
			assert.Length(t, e.Fields, 1, "the unexported field is left out")
			assert.Equal(t, e.Fields[0].Name, "Pub", "the exported one loads")
		})

		t.Run("consumes the comments of a declaration signature depth leaves out", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedFindings(t, nil, plugin.DepthSignatures,
				"package p\n\n// +fixture:gen:table name=t\n//go:noinline\nfunc hidden() {}\n\n"+
					"// +fixture:gen:x\ntype inner struct{}\n\nfunc Visible() {}\n")
			assert.Empty(t, found, "a left-out declaration's carriers refuse nothing")
			assert.Empty(t, onlyFile(t, gb).Annotations, "its tool directives leave with it")
		})

		t.Run("passes over the comments inside function bodies", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedFindings(t, nil, plugin.DepthFull,
				"package p\n\nfunc F() int {\n\t// +Inf passes through\n\t//nolint:errcheck\n\treturn 1\n}\n\n"+
					"var G = func() {\n\t// +fixture:gen:inside a literal\n}\n")
			assert.Empty(t, found, "a body's comments refuse nothing")
			assert.Empty(t, onlyFile(t, gb).Annotations, "a body gives the file no tool directive")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a function no code can name", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedFindings(t, nil, plugin.DepthFull,
				"package p\n\n// +fixture:gen:x\n//go:noinline\nfunc init() {}\n")
			assert.Length(t, found, 1, "the carrier refuses once")
			assert.Equal(t, found[0].Code, frontend.UnaddressedCarrier, "as a carrier nothing addresses")
			assert.Contains(t, found[0].Msg, "no code can name", "naming why")
			assert.Contains(t, found[0].Msg, `"+fixture:gen:x"`, "quoting the carrier as the author wrote it")
			assert.Empty(t, onlyFile(t, gb).Annotations, "the tool directive leaves with the function")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a member of an inline body", func(t *testing.T) {
			t.Parallel()

			_, found := parsedFindings(t, nil, plugin.DepthFull,
				"package p\n\ntype H struct {\n\ta struct {\n\t\t// +fixture:gen:x\n\t\tX int\n\t}\n}\n")
			assert.Length(t, found, 1, "the carrier refuses once")
			assert.Equal(t, found[0].Code, frontend.UnaddressedCarrier, "no identity names the member")
		})

		t.Run("gives an import declaration's doc to the file", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedFindings(t, nil, plugin.DepthFull,
				"package p\n\n// +fixture:gen:table name=t\n//go:generate stringer\n"+
					"import \"fmt\"\n\nvar _ = fmt.Sprint\n")
			assert.Length(t, found, 1, "the carrier on the import refuses")
			assert.Contains(t, found[0].Msg, "an import", "naming the import")
			assert.Equal(t, onlyFile(t, gb).Annotations,
				symbol.Annotations{{Name: "go:generate", Args: []string{"stringer"}}},
				"the tool directive is the file's")
		})

		t.Run("stamps a single-type constraint as a type set", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\ntype Int interface{ int }\n\ntype Bytes interface{ []byte }\n\n"+
					"type Wide interface{ Stringer }\n\ntype Stringer interface{ String() string }\n")
			decls := onlyFile(t, gb).Decls
			assert.Length(t, decls[0].(*node.Interface).Embeds, 0, "a basic type is a term, not an embed")
			assert.Length(t, decls[1].(*node.Interface).Embeds, 0, "a type literal is a term")
			assert.Length(t, decls[2].(*node.Interface).Embeds, 1,
				"a named type remains an embed, because only its declaration shows an interface")
			assert.Equal(t, stampValues(gb, golang.TypeSetKey), []any{[]string{"int"}, []string{"[]byte"}},
				"each term stamps as written")
		})

		t.Run("reads an array length in any integer form", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\ntype T struct {\n\tA [0x20]byte\n\tB [1_024]byte\n\tC [n]byte\n}\n\nconst n = 3\n"))
			st := file.Decls[0].(*node.Struct)
			assert.Equal(t, []int{st.Fields[0].Type.Length, st.Fields[1].Type.Length, st.Fields[2].Type.Length},
				[]int{32, 1024, 0}, "a literal length reads in any base, and an expression reads 0")
		})

		t.Run("stamps a defined type over a predeclared interface as named", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull, "package p\n\ntype Cause error\n\ntype Size int\n")
			assert.Equal(t, stampValues(gb, golang.UnderlyingKey), []any{"named", "basic"},
				"error is an interface, and int is basic")
		})

		t.Run("shares a parameter's trailing comment across its names", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\nfunc F(\n\ta, b int, // operands\n) {}\n"))
			fn := file.Decls[0].(*node.Function)
			assert.Equal(t, []string{fn.Params[0].Comment, fn.Params[1].Comment}, []string{"operands", "operands"},
				"every name the field binds keeps the comment, as a result's names do")
		})
	})
}
