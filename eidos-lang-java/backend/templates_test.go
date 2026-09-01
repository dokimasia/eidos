// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
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
			"nested": func(indent string, s symbol.Symbol) string {
				return indent + "NESTED " + s.Kind().String()
			},
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

	t.Run("the inventory is class, interface and enum alone", func(t *testing.T) {
		t.Parallel()

		kinds := backend.KindTemplates()
		assert.Length(t, kinds, 3,
			"Java states everything inside a type, so nothing else "+
				"has a file-level spelling")
		_, held := kinds[symbol.KindStruct]
		assert.True(t, held, "the class")
		_, held = kinds[symbol.KindInterface]
		assert.True(t, held, "the interface")
		_, held = kinds[symbol.KindEnum]
		assert.True(t, held, "and the enum class")
	})

	t.Run("class carries fields and methods with bodies", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Doc: []string{"Row is one record."}, Name: "Row"}
		s.Fields.Append(&emit.Field{
			Doc: []string{"Key addresses the row."}, Name: "key",
			Type: &emit.TypeRef{Spelling: "String"},
		})
		s.Fields.Append(&emit.Field{
			Name: "count", Type: &emit.TypeRef{Spelling: "int"},
			Comment: "rows per call",
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
				"    public int count; // rows per call\n"+
				"    public Row load(String key) {\n"+
				"        body();\n"+
				"    }\n"+
				"}\n",
			"members public at member depth, docs indented whole, a "+
				"trailing comment behind its semicolon")
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

	t.Run("supertypes and throws", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{
			Name:       "Row",
			Extends:    []*emit.TypeRef{ref("Base")},
			Implements: []*emit.TypeRef{ref("Keyed")},
		}
		s.Methods.Append(&emit.Method{
			Name:    "load",
			Returns: []*emit.Return{{Type: ref("Row")}},
			Throws:  []*emit.TypeRef{ref("IOException")},
		})
		got, err := execute(backend.StructTemplate, s)
		assert.NoError(t, err, "the class renders")
		assert.Equal(t, got,
			"public class Row extends Base implements Keyed {\n"+
				"    public Row load() throws IOException {\n"+
				"        body();\n"+
				"    }\n"+
				"}\n",
			"the heritage behind the name, the throws clause behind the "+
				"parameter list")

		i := &emit.Interface{
			Name:    "Store",
			Extends: []*emit.TypeRef{ref("Keyed")},
		}
		i.Methods.Append(&emit.Method{
			Name:    "load",
			Returns: []*emit.Return{{Type: ref("Row")}},
			Throws:  []*emit.TypeRef{ref("IOException")},
		})
		got, err = execute(backend.InterfaceTemplate, i)
		assert.NoError(t, err, "the interface renders")
		assert.Equal(t, got,
			"public interface Store extends Keyed {\n"+
				"    Row load() throws IOException;\n"+
				"}\n",
			"a signature carries its throws clause too")
	})

	t.Run("modifiers", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{
			Name:        "Row",
			Abstract:    true,
			Annotations: symbol.Annotations{{Name: "Entity"}},
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

	t.Run("enum", func(t *testing.T) {
		t.Parallel()

		e := &emit.Enum{Doc: []string{"Phase names a step."}, Name: "Phase"}
		e.Variants.Append(
			&emit.EnumVariant{Doc: []string{"OPEN admits writes."}, Name: "OPEN"},
			&emit.EnumVariant{Name: "CLOSED"},
		)
		e.Fields.Append(&emit.Field{
			Name: "steps", Level: symbol.LevelType, Type: ref("int"), Value: "2",
		})
		got, err := execute(backend.EnumTemplate, e)
		assert.NoError(t, err, "the enum class renders")
		assert.Equal(t, got,
			"/**\n * Phase names a step.\n */\n"+
				"public enum Phase {\n"+
				"    /**\n     * OPEN admits writes.\n     */\n"+
				"    OPEN,\n"+
				"    CLOSED,\n"+
				"    ;\n"+
				"    public static int steps = 2;\n"+
				"}\n",
			"constants first, the members Java's enum class carries "+
				"behind the semicolon")

		valued := &emit.Enum{Name: "Phase"}
		valued.Variants.Append(&emit.EnumVariant{Name: "OPEN", Value: "1"})
		_, err = execute(backend.EnumTemplate, valued)
		assert.HasError(t, err,
			"a stated value refuses, because it takes the constructor form")
	})

	t.Run("nested types place at member depth", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Name: "Row"}
		s.Fields.Append(&emit.Field{
			Name: "key", Type: &emit.TypeRef{Spelling: "String"},
		})
		s.Types.Append(&emit.Struct{Name: "Inner"})
		got, err := execute(backend.StructTemplate, s)
		assert.NoError(t, err, "the nesting class renders")
		assert.Equal(t, got,
			"public class Row {\n"+
				"    public String key;\n"+
				"    NESTED Struct\n"+
				"}\n",
			"the nested builtin runs after the members, at member depth")

		i := &emit.Interface{Name: "Store"}
		i.Types.Append(&emit.Enum{Name: "Phase"})
		got, err = execute(backend.InterfaceTemplate, i)
		assert.NoError(t, err, "the nesting interface renders")
		assert.Equal(t, got,
			"public interface Store {\n    NESTED Enum\n}\n",
			"an interface nests the same way")
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
