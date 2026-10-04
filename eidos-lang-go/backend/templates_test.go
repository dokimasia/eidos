// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"io"
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Each stubbed builtin writes a marker, so a template's own bytes are
// distinguishable from the pass's.
const (
	bodyStub    = "\tbody()\n"
	importsStub = "IMPORTS\n"
	declsStub   = "DECLS\n"
	nestedStub  = "NESTED "
)

// templateMapAllocs is a map of kinds onto templates or reasons: the
// map and its one group.
const templateMapAllocs = 2

// Each kind template is pinned byte for byte over a declaration
// exercising its whole shape, member docblocks included.
func TestTemplates(t *testing.T) {
	t.Parallel()

	kind := func(k symbol.Kind) string { return backend.KindTemplates()[k] }

	t.Run("KindTemplates", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a template for every kind Go declares standalone", func(t *testing.T) {
			t.Parallel()

			kinds := backend.KindTemplates()
			for _, k := range []symbol.Kind{
				symbol.KindStruct, symbol.KindInterface, symbol.KindFunction,
				symbol.KindMethod, symbol.KindAlias, symbol.KindConstant,
				symbol.KindVariable,
			} {
				_, held := kinds[k]
				assert.True(t, held, "the inventory spells "+k.String())
			}
		})

		t.Run("writes a struct's fields under their docblocks", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Doc: []string{"Row is one record."}, Name: rowName}
			s.Fields.Append(
				&emit.Field{Doc: []string{"Key addresses the row."}, Name: "Key", Type: ref("string")},
				&emit.Field{Name: "N", Type: ref("int"), Tag: `json:"n"`, Comment: "counted"},
			)
			assert.Equal(t, executed(t, kind(symbol.KindStruct), s),
				"// Row is one record.\n"+
					"type Row struct {\n"+
					"\t// Key addresses the row.\n"+
					"\tKey string\n"+
					"\tN int `json:\"n\"` // counted\n"+
					"}\n",
				"a field's tag and trailing comment beside it")
		})

		t.Run("binds the import of a field's type", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: rowName}
			s.Fields.Append(&emit.Field{Name: "When", Type: imported(timePkg, timePkg, "Duration")})
			out, set := execute(t, kind(symbol.KindStruct), s)
			assert.Contains(t, out, "\tWhen time.Duration\n", "the qualified type")
			assert.Equal(t, set.Paths(), []string{timePkg}, "and its import")
		})

		t.Run("writes a struct's embeds like fields", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: rowName, Comment: "one per fetch"}
			s.Embeds = []*emit.Embed{{
				Doc: []string{"Base declares the shared fields."}, Ref: ref("Base"),
				Tag: `json:"-"`, Comment: "promoted",
				Annotations: symbol.Annotations{{Name: "go:fix", Args: []string{"inline"}}},
			}}
			assert.Equal(t, executed(t, kind(symbol.KindStruct), s),
				"type Row struct {\n"+
					"\t// Base declares the shared fields.\n"+
					"\t//go:fix inline\n"+
					"\tBase `json:\"-\"` // promoted\n"+
					"} // one per fetch\n",
				"the embed's docs, directives, tag and comment, then the type's comment")
		})

		t.Run("writes an interface method's comments as block comments", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store", Comment: "read side"}
			i.Methods.Append(&emit.Method{
				Name: "Get", Comment: "by key",
				Params:  []*emit.Param{{Name: "key", Type: ref("string"), Comment: "the row key"}},
				Returns: []*emit.Return{{Type: ref("string"), Comment: "the row"}},
			})
			assert.Equal(t, executed(t, kind(symbol.KindInterface), i),
				"type Store interface {\n"+
					"\tGet(key string /* the row key */) string /* the row */ // by key\n"+
					"} // read side\n",
				"the method's and the interface's comments close their lines")
		})

		t.Run("writes a function's trailing comment after its closing brace", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, executed(t, kind(symbol.KindFunction), &emit.Function{Name: "Sort", Comment: "stable"}),
				"func Sort() {\n"+bodyStub+"} // stable\n", "on the brace's line")
		})

		t.Run("writes an alias's trailing comment on its line", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t,
				executed(t, kind(symbol.KindAlias), &emit.Alias{Name: "ID", Target: ref("string"), Comment: "opaque"}),
				"type ID = string // opaque\n", "behind the target")
		})

		t.Run("writes a struct's methods after the type", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: rowName}
			s.Fields.Append(&emit.Field{Name: "Key", Type: ref("string")})
			s.Methods.Append(&emit.Method{Name: "Load"})
			assert.Equal(t, executed(t, kind(symbol.KindStruct), s),
				"type Row struct {\n"+
					"\tKey string\n"+
					"}\n"+
					"\n"+nestedStub+symbol.KindMethod.String()+"\n",
				"Go states methods at the package level")
		})

		t.Run("writes an interface's methods under their docblocks", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Doc: []string{"Store loads rows."}, Name: "Store"}
			i.Methods.Append(&emit.Method{
				Doc:     []string{"Load fetches one row."},
				Name:    "Load",
				Params:  []*emit.Param{{Name: "key", Type: ref("string")}},
				Returns: []*emit.Return{{Type: ref(rowName)}, {Type: ref("error")}},
			})
			assert.Equal(t, executed(t, kind(symbol.KindInterface), i),
				"// Store loads rows.\n"+
					"type Store interface {\n"+
					"\t// Load fetches one row.\n"+
					"\tLoad(key string) (Row, error)\n"+
					"}\n",
				"at member depth")
		})

		t.Run("writes an interface's embeds like a struct's", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "ReadCloser"}
			i.Embeds = []*emit.Embed{{
				Doc:         []string{"Reader is the stream side."},
				Ref:         imported("io", "io", "Reader"),
				Comment:     "stream side",
				Annotations: symbol.Annotations{{Name: "go:fix", Args: []string{"inline"}}},
			}}
			assert.Equal(t, executed(t, kind(symbol.KindInterface), i),
				"type ReadCloser interface {\n"+
					"\t// Reader is the stream side.\n"+
					"\t//go:fix inline\n"+
					"\tio.Reader // stream side\n"+
					"}\n",
				"the embed's docs, directives and comment")
		})

		t.Run("returns an error for a tag on an interface's embed", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "ReadCloser"}
			i.Embeds = []*emit.Embed{{Ref: ref("io.Reader"), Tag: `json:"r"`}}
			assert.HasError(t, parsed(t, kind(symbol.KindInterface), &render.ImportSet{}).Execute(io.Discard, i),
				"Go gives a struct field alone a tag")
		})

		t.Run("writes a function around its body", func(t *testing.T) {
			t.Parallel()

			f := &emit.Function{Name: "Load", Returns: []*emit.Return{{Type: ref("error")}}}
			assert.Equal(t, executed(t, kind(symbol.KindFunction), f),
				"func Load() error {\n"+bodyStub+"}\n", "the function's shape")
		})

		t.Run("writes a method around its body", func(t *testing.T) {
			t.Parallel()

			m := &emit.Method{Name: "Close", Receiver: &emit.Param{Name: "s", Type: ref("*Store")}}
			assert.Equal(t, executed(t, kind(symbol.KindMethod), m),
				"func (s *Store) Close() {\n"+bodyStub+"}\n", "the method's shape")
		})

		t.Run("writes a transparent alias with an equals sign", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, executed(t, kind(symbol.KindAlias), &emit.Alias{Name: "ID", Target: ref("string")}),
				"type ID = string\n", "the alias shape")
		})

		t.Run("writes a defined type without an equals sign", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t,
				executed(t, kind(symbol.KindAlias), &emit.Alias{Name: "phase", Defined: true, Target: ref("int")}),
				"type phase int\n", "the defined type's shape")
		})

		t.Run("writes a constant's type where it states one", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t,
				executed(t, kind(symbol.KindConstant), &emit.Constant{Name: "Max", Type: ref("int"), Value: "10"}),
				"const Max int = 10\n", "a typed constant")
			assert.Equal(t,
				executed(t, kind(symbol.KindConstant), &emit.Constant{Name: "Max", Value: "10"}),
				"const Max = 10\n", "an untyped one")
		})

		t.Run("writes a constant's trailing comment beside its value", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t,
				executed(t, kind(symbol.KindConstant), &emit.Constant{Name: "Max", Value: "10", Comment: "inclusive"}),
				"const Max = 10 // inclusive\n", "on the value's line")
		})

		t.Run("writes a variable's initializer behind an equals sign", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t,
				executed(t, kind(symbol.KindVariable), &emit.Variable{Name: "count", Type: ref("int")}),
				"var count int\n", "a variable without an initializer")
			assert.Equal(t,
				executed(t, kind(symbol.KindVariable), &emit.Variable{Name: "count", Type: ref("int"), Value: "0"}),
				"var count int = 0\n", "and one with")
		})

		t.Run("writes a struct's embeds before its fields", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{
				Name:   rowName,
				Embeds: []*emit.Embed{{Ref: ref("Base")}, {Ref: imported("sync", "sync", "Mutex")}},
			}
			s.Fields.Append(&emit.Field{Name: "Key", Type: ref("string")})
			assert.Equal(t, executed(t, kind(symbol.KindStruct), s),
				"type Row struct {\n\tBase\n\tsync.Mutex\n\tKey string\n}\n",
				"the way Go promotes")
		})

		t.Run("writes a nominal parent as an embed", func(t *testing.T) {
			t.Parallel()

			n := &emit.Struct{
				Name:       "Child",
				Extends:    []*emit.TypeRef{ref("Base")},
				Implements: []*emit.TypeRef{ref("Keyed")},
			}
			n.Fields.Append(&emit.Field{Name: "Key", Type: ref("string")})
			assert.Equal(t, executed(t, kind(symbol.KindStruct), n),
				"type Child struct {\n\tBase\n\tKey string\n}\n",
				"promotion without subtyping, and satisfaction is structural")
		})

		t.Run("writes an interface's embeds before its widened contracts", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store", Extends: []*emit.TypeRef{ref("Closer")}}
			i.Embeds = []*emit.Embed{{Ref: ref("Reader")}}
			i.Methods.Append(&emit.Method{Name: "Get", Returns: []*emit.Return{{Type: ref("string")}}})
			assert.Equal(t, executed(t, kind(symbol.KindInterface), i),
				"type Store interface {\n\tReader\n\tCloser\n\tGet() string\n}\n",
				"both as embedded lines")
		})

		t.Run("writes a generic declaration's parameters behind its name", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Box", TypeParams: []*emit.TypeParam{{Name: "T"}}}
			s.Fields.Append(&emit.Field{Name: "Item", Type: ref("T")})
			assert.Equal(t, executed(t, kind(symbol.KindStruct), s),
				"type Box[T any] struct {\n\tItem T\n}\n", "a struct's list")

			i := &emit.Interface{
				Name:       "Keyed",
				TypeParams: []*emit.TypeParam{{Name: "K", Bounds: []*emit.TypeRef{ref("Codec")}}},
			}
			i.Methods.Append(&emit.Method{
				Name:    "Pick",
				Params:  []*emit.Param{{Name: "key", Type: ref("K")}},
				Returns: []*emit.Return{{Type: ref("K")}},
			})
			assert.Equal(t, executed(t, kind(symbol.KindInterface), i),
				"type Keyed[K Codec] interface {\n\tPick(key K) K\n}\n", "an interface's list")

			f := &emit.Function{
				Name:       "Sort",
				TypeParams: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}}},
				Params:     []*emit.Param{{Name: "items", Type: ref("T")}},
				Returns:    []*emit.Return{{Type: ref("T")}},
			}
			assert.Equal(t, executed(t, kind(symbol.KindFunction), f),
				"func Sort[T Codec](items T) T {\n"+bodyStub+"}\n", "a function's list")

			a := &emit.Alias{
				Name:       "Match",
				TypeParams: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}}},
				Target:     &emit.TypeRef{Spelling: "Keyed", Args: []*emit.TypeRef{ref("T")}},
			}
			assert.Equal(t, executed(t, kind(symbol.KindAlias), a),
				"type Match[T Codec] = Keyed[T]\n", "an alias's list")
		})

		t.Run("writes a method's own parameters behind its name", func(t *testing.T) {
			t.Parallel()

			m := &emit.Method{
				Name:       "Fold",
				Receives:   &emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{ref("T")}},
				TypeParams: []*emit.TypeParam{{Name: "U", Bounds: []*emit.TypeRef{ref("Codec")}}},
				Params:     []*emit.Param{{Name: "item", Type: ref("U")}},
				Returns:    []*emit.Return{{Type: ref("U")}},
			}
			assert.Equal(t, executed(t, kind(symbol.KindMethod), m),
				"func (Box[T]) Fold[U Codec](item U) U {\n"+bodyStub+"}\n",
				"the receiver restates the host's argument, which Go spells since 1.27")
		})
	})

	t.Run("FileTemplate", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the package clause then the imports then the declarations", func(t *testing.T) {
			t.Parallel()

			got := executed(t, backend.FileTemplate, struct {
				Name string
				Pkg  symbol.Identity
			}{Name: "store_stub.go", Pkg: symbol.Identity{Package: "svc/api"}})
			assert.Equal(t, got, "package api\n\n"+importsStub+"\n"+declsStub,
				"the shape gofmt leaves")
		})
	})

	t.Run("RefusedKinds", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the sum kind alone", func(t *testing.T) {
			t.Parallel()

			refused := backend.RefusedKinds()
			assert.Length(t, refused, 1, "one kind Go cannot spell")
			assert.NotEqual(t, refused[symbol.KindSum], "", "the sum, with its reason")
		})

		t.Run("returns no kind the templates spell", func(t *testing.T) {
			t.Parallel()

			for k := range backend.RefusedKinds() {
				_, spelt := backend.KindTemplates()[k]
				assert.False(t, spelt, "a kind is spelt or refused: "+k.String())
			}
		})
	})
}

// Each map allocates itself. The ordinary run, which runs no benchmark,
// checks those ceilings here.
func TestTemplatesAllocs(t *testing.T) {
	checkAllocs(t, templatesCalls())
}

// BenchmarkTemplates measures the maps the backend reads once per
// build.
func BenchmarkTemplates(b *testing.B) {
	benchCalls(b, templatesCalls())
}

// templatesCalls returns a call of KindTemplates and of RefusedKinds.
func templatesCalls() []allocCall {
	var kinds map[symbol.Kind]string
	return []allocCall{
		{
			name: "KindTemplates", allocs: templateMapAllocs,
			call:  func() { kinds = backend.KindTemplates() },
			check: func(tb assert.TB) { assert.Length(tb, kinds, 7, "KindTemplates returns seven templates") },
		},
		{
			name: "RefusedKinds", allocs: templateMapAllocs,
			call:  func() { kinds = backend.RefusedKinds() },
			check: func(tb assert.TB) { assert.Length(tb, kinds, 1, "RefusedKinds returns the sum") },
		},
	}
}

// execute runs one template over one declaration the way the render
// pass does, the builtins stubbed to markers, and returns the output
// beside the file's import set.
func execute(t *testing.T, src string, data any) (string, *render.ImportSet) {
	t.Helper()

	set := &render.ImportSet{}
	var b strings.Builder
	assert.NoError(t, parsed(t, src, set).Execute(&b, data), "the template executes")
	return b.String(), set
}

// executed returns what [execute] writes.
func executed(t *testing.T, src string, data any) string {
	t.Helper()

	out, _ := execute(t, src, data)
	return out
}

// parsed parses one template against the backend's vocabulary bound to
// set, the builtins stubbed.
func parsed(t *testing.T, src string, set *render.ImportSet) *template.Template {
	t.Helper()

	tmpl, err := template.New("kind").
		Funcs(backend.Funcs(set)).
		Funcs(template.FuncMap{
			render.BuiltinBody:    func(any) string { return bodyStub },
			render.BuiltinUse:     func(string) string { return "" },
			render.BuiltinImports: func() string { return importsStub },
			render.BuiltinDecls:   func() string { return declsStub },
			render.BuiltinSlots:   func() string { return "" },
			render.BuiltinSlot:    func(string) string { return "" },
			render.BuiltinNested: func(indent string, s symbol.Symbol) string {
				return indent + nestedStub + s.Kind().String()
			},
		}).
		Parse(src)
	assert.NoError(t, err, "the template parses")
	return tmpl
}
