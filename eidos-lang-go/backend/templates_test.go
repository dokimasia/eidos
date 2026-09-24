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
	"go.dokimi.dev/eidos/sdk/symbol"
)

// execute runs one template over one declaration the way the
// render pass does, with the body builtin stubbed to a marker, so
// the output is the template's own bytes and nothing else's.
func execute(t *testing.T, src string, data any) string {
	t.Helper()

	var b strings.Builder
	assert.NoError(t, parsed(t, src).Execute(&b, data), "the template executes")
	return b.String()
}

// parsed parses one template against the backend's vocabulary and
// the builtins stubbed.
func parsed(t *testing.T, src string) *template.Template {
	t.Helper()

	tmpl, err := template.New("kind").
		Funcs(backend.Funcs()).
		Funcs(template.FuncMap{
			"body":    func(any) string { return "\tbody()\n" },
			"use":     func(string) string { return "" },
			"imports": func() string { return "IMPORTS\n" },
			"decls":   func() string { return "DECLS\n" },
			"slots":   func() string { return "" },
			"slot":    func(string) string { return "" },
			"nested": func(indent string, s symbol.Symbol) string {
				return indent + "NESTED " + s.Kind().String()
			},
		}).
		Parse(src)
	assert.NoError(t, err, "the template parses")
	return tmpl
}

// Each kind template is pinned byte for byte over a declaration
// exercising its whole shape, member docblocks included.
func TestTemplates(t *testing.T) {
	t.Parallel()

	t.Run("every declared kind has a template", func(t *testing.T) {
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

	t.Run("struct", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Doc: []string{"Row is one record."}, Name: "Row"}
		s.Fields.Append(
			&emit.Field{Doc: []string{"Key addresses the row."}, Name: "Key", Type: ref("string")},
			&emit.Field{
				Name: "N", Type: ref("int"),
				Tag: `json:"n"`, Comment: "counted",
			},
		)
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"// Row is one record.\n"+
				"type Row struct {\n"+
				"\t// Key addresses the row.\n"+
				"\tKey string\n"+
				"\tN int `json:\"n\"` // counted\n"+
				"}\n",
			"fields under their own docblocks, tag and trailing comment beside")
	})

	t.Run("embeds keep their docs, directives, tag and comment", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Name: "Row", Comment: "one per fetch"}
		s.Embeds = []*emit.Embed{{
			Doc: []string{"Base carries the shared fields."}, Ref: ref("Base"),
			Tag: `json:"-"`, Comment: "promoted",
			Annotations: symbol.Annotations{{Name: "go:fix", Args: []string{"inline"}}},
		}}
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"type Row struct {\n"+
				"\t// Base carries the shared fields.\n"+
				"\t//go:fix inline\n"+
				"\tBase `json:\"-\"` // promoted\n"+
				"} // one per fetch\n",
			"an embedded field renders like a field, and the type's own comment closes the brace")
	})

	t.Run("trailing comments close every kind that ends a line", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Name: "Store", Comment: "read side"}
		i.Methods.Append(&emit.Method{
			Name: "Get", Comment: "by key",
			Params:  []*emit.Param{{Name: "key", Type: ref("string"), Comment: "the row key"}},
			Returns: []*emit.Return{{Type: ref("string"), Comment: "the row"}},
		})
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"type Store interface {\n"+
				"\tGet(key string /* the row key */) string /* the row */ // by key\n"+
				"} // read side\n",
			"a signature's comments spell as block comments, the method's and the interface's close their lines")
		assert.Equal(t,
			execute(t, backend.FunctionTemplate, &emit.Function{Name: "Sort", Comment: "stable"}),
			"func Sort() {\n\tbody()\n} // stable\n", "a function's comment follows its closing brace")
		assert.Equal(t,
			execute(t, backend.AliasTemplate, &emit.Alias{Name: "ID", Target: ref("string"), Comment: "opaque"}),
			"type ID = string // opaque\n", "an alias's comment closes its line")
	})

	t.Run("struct methods follow the type", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Name: "Row"}
		s.Fields.Append(&emit.Field{Name: "Key", Type: ref("string")})
		s.Methods.Append(&emit.Method{Name: "Load"})
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"type Row struct {\n"+
				"\tKey string\n"+
				"}\n"+
				"\nNESTED Method\n",
			"a member method renders after its type through the kind template, "+
				"because Go states methods at the package level")
	})

	t.Run("interface", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Doc: []string{"Store loads rows."}, Name: "Store"}
		i.Methods.Append(&emit.Method{
			Doc:     []string{"Load fetches one row."},
			Name:    "Load",
			Params:  []*emit.Param{{Name: "key", Type: ref("string")}},
			Returns: []*emit.Return{{Type: ref("Row")}, {Type: ref("error")}},
		})
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"// Store loads rows.\n"+
				"type Store interface {\n"+
				"\t// Load fetches one row.\n"+
				"\tLoad(key string) (Row, error)\n"+
				"}\n",
			"methods under their own docblocks, at member depth")
	})

	t.Run("interface embeds keep their docs, directives and comment", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Name: "ReadCloser"}
		i.Embeds = []*emit.Embed{{
			Doc: []string{"Reader is the stream side."}, Ref: ref("io.Reader"), Comment: "stream side",
			Annotations: symbol.Annotations{{Name: "go:fix", Args: []string{"inline"}}},
		}}
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"type ReadCloser interface {\n"+
				"\t// Reader is the stream side.\n"+
				"\t//go:fix inline\n"+
				"\tio.Reader // stream side\n"+
				"}\n",
			"an interface's embed renders like a struct's embed")

		tagged := &emit.Interface{Name: "ReadCloser"}
		tagged.Embeds = []*emit.Embed{{Ref: ref("io.Reader"), Tag: `json:"r"`}}
		assert.HasError(t, parsed(t, backend.InterfaceTemplate).Execute(io.Discard, tagged),
			"a tag on an interface's embed refuses, because Go gives a struct field alone one")
	})

	t.Run("function and method place the body", func(t *testing.T) {
		t.Parallel()

		f := &emit.Function{Name: "Load", Returns: []*emit.Return{{Type: ref("error")}}}
		assert.Equal(t, execute(t, backend.FunctionTemplate, f),
			"func Load() error {\n\tbody()\n}\n", "the function's shape")

		m := &emit.Method{
			Name:     "Close",
			Receiver: &emit.Param{Name: "s", Type: ref("*Store")},
		}
		assert.Equal(t, execute(t, backend.MethodTemplate, m),
			"func (s *Store) Close() {\n\tbody()\n}\n", "the method's shape")
	})

	t.Run("alias, constant and variable", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t,
			execute(t, backend.AliasTemplate, &emit.Alias{Name: "ID", Target: ref("string")}),
			"type ID = string\n", "the alias shape")
		assert.Equal(t,
			execute(t, backend.AliasTemplate,
				&emit.Alias{Name: "phase", Defined: true, Target: ref("int")}),
			"type phase int\n", "a defined type drops the equals sign")
		assert.Equal(t,
			execute(t, backend.ConstantTemplate,
				&emit.Constant{Name: "Max", Type: ref("int"), Value: "10"}),
			"const Max int = 10\n", "a typed constant")
		assert.Equal(t,
			execute(t, backend.ConstantTemplate, &emit.Constant{Name: "Max", Value: "10"}),
			"const Max = 10\n", "an untyped one")
		assert.Equal(t,
			execute(t, backend.ConstantTemplate,
				&emit.Constant{Name: "Max", Value: "10", Comment: "inclusive"}),
			"const Max = 10 // inclusive\n", "the trailing comment beside the value")
		assert.Equal(t,
			execute(t, backend.VariableTemplate,
				&emit.Variable{Name: "count", Type: ref("int")}),
			"var count int\n", "the variable shape")
		assert.Equal(t,
			execute(t, backend.VariableTemplate,
				&emit.Variable{Name: "count", Type: ref("int"), Value: "0"}),
			"var count int = 0\n", "the initializer behind the equals sign")
	})

	t.Run("supertypes", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{
			Name:   "Row",
			Embeds: []*emit.Embed{{Ref: ref("Base")}, {Ref: ref("sync.Mutex")}},
		}
		s.Fields.Append(&emit.Field{Name: "Key", Type: ref("string")})
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"type Row struct {\n\tBase\n\tsync.Mutex\n\tKey string\n}\n",
			"embedded types before the fields, the way Go promotes")

		n := &emit.Struct{
			Name:       "Child",
			Extends:    []*emit.TypeRef{ref("Base")},
			Implements: []*emit.TypeRef{ref("Keyed")},
		}
		n.Fields.Append(&emit.Field{Name: "Key", Type: ref("string")})
		assert.Equal(t, execute(t, backend.StructTemplate, n),
			"type Child struct {\n\tBase\n\tKey string\n}\n",
			"a nominal parent spells as embedding, promotion without "+
				"subtyping, and implements spells nothing at all")

		i := &emit.Interface{
			Name:    "Store",
			Extends: []*emit.TypeRef{ref("Closer")},
		}
		i.Embeds = []*emit.Embed{{Ref: ref("Reader")}}
		i.Methods.Append(&emit.Method{
			Name:    "Get",
			Returns: []*emit.Return{{Type: ref("string")}},
		})
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"type Store interface {\n\tReader\n\tCloser\n\tGet() string\n}\n",
			"embeds then the widened contracts, both as embedded lines")
	})

	t.Run("generics", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Name: "Box", TypeParams: []*emit.TypeParam{{Name: "T"}}}
		s.Fields.Append(&emit.Field{Name: "Item", Type: ref("T")})
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"type Box[T any] struct {\n\tItem T\n}\n",
			"the struct's parameter list behind its name")

		i := &emit.Interface{
			Name:       "Keyed",
			TypeParams: []*emit.TypeParam{{Name: "K", Bounds: []*emit.TypeRef{ref("Codec")}}},
		}
		i.Methods.Append(&emit.Method{
			Name:    "Pick",
			Params:  []*emit.Param{{Name: "key", Type: ref("K")}},
			Returns: []*emit.Return{{Type: ref("K")}},
		})
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"type Keyed[K Codec] interface {\n\tPick(key K) K\n}\n",
			"the bound behind the parameter, members referencing it")

		f := &emit.Function{
			Name:       "Sort",
			TypeParams: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}}},
			Params:     []*emit.Param{{Name: "items", Type: ref("T")}},
			Returns:    []*emit.Return{{Type: ref("T")}},
		}
		assert.Equal(t, execute(t, backend.FunctionTemplate, f),
			"func Sort[T Codec](items T) T {\n\tbody()\n}\n",
			"the function's parameter list behind its name")

		m := &emit.Method{
			Name:       "Fold",
			Receives:   &emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{ref("T")}},
			TypeParams: []*emit.TypeParam{{Name: "U", Bounds: []*emit.TypeRef{ref("Codec")}}},
			Params:     []*emit.Param{{Name: "item", Type: ref("U")}},
			Returns:    []*emit.Return{{Type: ref("U")}},
		}
		assert.Equal(t, execute(t, backend.MethodTemplate, m),
			"func (Box[T]) Fold[U Codec](item U) U {\n\tbody()\n}\n",
			"the receiver restates the host's argument, and the method "+
				"declares its own, which Go spells since 1.27")

		a := &emit.Alias{
			Name:       "Match",
			TypeParams: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}}},
			Target:     &emit.TypeRef{Spelling: "Keyed", Args: []*emit.TypeRef{ref("T")}},
		}
		assert.Equal(t, execute(t, backend.AliasTemplate, a),
			"type Match[T Codec] = Keyed[T]\n",
			"the alias parameterizes and its target restates the argument")
	})

	t.Run("the file skeleton", func(t *testing.T) {
		t.Parallel()

		got := execute(t, backend.FileTemplate, struct {
			Name string
			Pkg  symbol.Identity
		}{Name: "store_stub.go", Pkg: symbol.Identity{Package: "svc/api"}})
		assert.Equal(t, got, "package api\n\nIMPORTS\n\nDECLS\n",
			"package clause, import block, a blank line, then the "+
				"declarations, which is the shape gofmt leaves")
	})
}
