// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-java/backend"
)

// execute runs one template over one declaration the way the
// render pass does, with the body builtin stubbed to a marker.
func execute(src string, data any) (string, error) {
	tmpl, err := template.New("kind").
		Funcs(backend.Funcs()).
		Funcs(template.FuncMap{
			"body":    func(any) string { return "        body();\n" },
			"use":     func(string) string { return "" },
			"imports": func() string { return "IMPORTS\n" },
			"decls":   func() string { return "DECLS\n" },
			"slots":   func() string { return "" },
			"slot":    func(string) string { return "" },
		}).
		Parse(src)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

// The inventory is two templates by design, and both are pinned
// byte for byte; so is the refusal that keeps the return model
// honest.
func TestTemplates(t *testing.T) {
	t.Parallel()

	t.Run("the inventory is class and interface alone", func(t *testing.T) {
		t.Parallel()

		kinds := backend.KindTemplates()
		assert.Length(t, kinds, 2,
			"Java states everything inside a type, so nothing else "+
				"has a file-level spelling")
		_, held := kinds[symbol.KindStruct]
		assert.True(t, held, "the class")
		_, held = kinds[symbol.KindInterface]
		assert.True(t, held, "and the interface")
	})

	t.Run("class carries fields and methods with bodies", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Doc: []string{"Row is one record."}, Name: "Row"}
		s.Fields.Append(&emit.Field{
			Doc: []string{"Key addresses the row."}, Name: "key",
			Type: &emit.TypeRef{Spelling: "String"},
		})
		s.Methods.Append(&emit.Method{
			Name:    "load",
			Params:  []*emit.Param{{Name: "key", Type: &emit.TypeRef{Spelling: "String"}}},
			Returns: []*emit.Return{{Type: &emit.TypeRef{Spelling: "Row"}}},
		})
		got, err := execute(backend.StructTemplate, s)
		assert.NoError(t, err, "the class renders")
		assert.Equal(t, got,
			"/**\n * Row is one record.\n */\n"+
				"public class Row {\n"+
				"    /**\n     * Key addresses the row.\n     */\n"+
				"    public String key;\n"+
				"    public Row load(String key) {\n"+
				"        body();\n"+
				"    }\n"+
				"}\n",
			"members public at member depth, docs indented whole")
	})

	t.Run("interface carries signatures alone", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Name: "Store"}
		i.Methods.Append(&emit.Method{Name: "close"})
		got, err := execute(backend.InterfaceTemplate, i)
		assert.NoError(t, err, "the interface renders")
		assert.Equal(t, got,
			"public interface Store {\n    void close();\n}\n",
			"implicitly public, no body, void for no result")
	})

	t.Run("generics", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Name: "Box", TypeParams: []*emit.TypeParam{{Name: "T"}}}
		s.Fields.Append(&emit.Field{Name: "item", Type: ref("T")})
		s.Methods.Append(&emit.Method{
			Name:       "map",
			TypeParams: []*emit.TypeParam{{Name: "U", Bounds: []*emit.TypeRef{ref("Codec")}}},
			Params:     []*emit.Param{{Name: "item", Type: ref("U")}},
			Returns:    []*emit.Return{{Type: ref("U")}},
		})
		got, err := execute(backend.StructTemplate, s)
		assert.NoError(t, err, "the generic class renders")
		assert.Equal(t, got,
			"public class Box<T> {\n"+
				"    public T item;\n"+
				"    public <U extends Codec> U map(U item) {\n"+
				"        body();\n"+
				"    }\n"+
				"}\n",
			"the class's parameter list behind its name, the method's "+
				"before its return type")

		i := &emit.Interface{
			Name: "Keyed",
			TypeParams: []*emit.TypeParam{
				{Name: "K", Bounds: []*emit.TypeRef{ref("Codec")}},
			},
		}
		i.Methods.Append(&emit.Method{
			Name:    "pick",
			Params:  []*emit.Param{{Name: "key", Type: ref("K")}},
			Returns: []*emit.Return{{Type: ref("K")}},
		})
		got, err = execute(backend.InterfaceTemplate, i)
		assert.NoError(t, err, "the generic interface renders")
		assert.Equal(t, got,
			"public interface Keyed<K extends Codec> {\n"+
				"    K pick(K key);\n"+
				"}\n",
			"the bound behind the parameter, members referencing it")

		v := &emit.Struct{Name: "Sink", TypeParams: []*emit.TypeParam{
			{Name: "T", Variance: symbol.VarianceIn},
		}}
		_, err = execute(backend.StructTemplate, v)
		assert.HasError(t, err,
			"declaration-site variance refuses, because Java's wildcard is use-site")
	})

	t.Run("modifiers", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{
			Name:        "Row",
			Abstract:    true,
			Annotations: emit.Annotations{{Name: "Entity"}},
		}
		s.Fields.Append(&emit.Field{
			Name:       "MAX",
			Level:      symbol.LevelType,
			Mutability: symbol.MutabilityImmutable,
			Type:       ref("int"),
			Value:      "8",
		})
		s.Methods.Append(
			&emit.Method{
				Name: "load", Override: true,
				Returns: []*emit.Return{{Type: ref("Row")}},
			},
			&emit.Method{
				Name: "pick", Abstract: true,
				Returns: []*emit.Return{{Type: ref("Row")}},
			},
		)
		got, err := execute(backend.StructTemplate, s)
		assert.NoError(t, err, "the modified class renders")
		assert.Equal(t, got,
			"@Entity\n"+
				"public abstract class Row {\n"+
				"    public static final int MAX = 8;\n"+
				"    @Override\n"+
				"    public Row load() {\n"+
				"        body();\n"+
				"    }\n"+
				"    public abstract Row pick();\n"+
				"}\n",
			"annotations above, keywords in stated order, the abstract "+
				"method a signature alone")

		i := &emit.Interface{Name: "Store"}
		i.Methods.Append(
			&emit.Method{Name: "close"},
			&emit.Method{
				Name: "load", HasDefault: true,
				Returns: []*emit.Return{{Type: ref("Row")}},
				Body:    emit.Body{Verbatim: "        return null;\n"},
			},
		)
		got, err = execute(backend.InterfaceTemplate, i)
		assert.NoError(t, err, "the interface with a default renders")
		assert.Equal(t, got,
			"public interface Store {\n"+
				"    void close();\n"+
				"    default Row load() {\n"+
				"        body();\n"+
				"    }\n"+
				"}\n",
			"the default keyword opens the one signature that places a body")
	})

	t.Run("a second return value refuses at render", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Name: "Store"}
		i.Methods.Append(&emit.Method{Name: "load", Returns: []*emit.Return{
			{Type: &emit.TypeRef{Spelling: "Row"}},
			{Type: &emit.TypeRef{Spelling: "Exception"}},
		}})
		_, err := execute(backend.InterfaceTemplate, i)
		assert.HasError(t, err,
			"a Java callable returns one value, and the second arrives thrown")
	})

	t.Run("the file skeleton opens with the package clause", func(t *testing.T) {
		t.Parallel()

		got, err := execute(backend.FileTemplate, struct {
			Name string
			Pkg  symbol.Identity
		}{Pkg: symbol.Identity{Package: "svc/api"}})
		assert.NoError(t, err, "the skeleton renders")
		assert.Equal(t, got, "package svc.api;\n\nIMPORTS\nDECLS\n",
			"dots for slashes, then imports, then declarations")
	})
}
