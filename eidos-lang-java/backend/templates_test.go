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
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Each stubbed builtin writes a marker, so a template's own bytes are
// distinguishable from the pass's.
const (
	bodyStub    = "        body();\n"
	importsStub = "IMPORTS\n"
	declsStub   = "DECLS\n"
	nestedStub  = "NESTED "
)

// templateMapAllocs is a map of kinds onto templates or reasons: the
// map and its one group.
const templateMapAllocs = 2

// The inventory is three templates by design, and each is pinned
// byte for byte, as is the refusal that keeps the return model
// honest and the kinds the backend refuses at file level.
func TestTemplates(t *testing.T) {
	t.Parallel()

	kind := func(k symbol.Kind) string { return backend.KindTemplates()[k] }

	t.Run("KindTemplates", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the templates of the three file-level type kinds alone", func(t *testing.T) {
			t.Parallel()

			kinds := backend.KindTemplates()
			assert.Length(t, kinds, 3, "Java states everything inside a type")
			for _, k := range []symbol.Kind{symbol.KindStruct, symbol.KindInterface, symbol.KindEnum} {
				assert.Contains(t, kinds, k, "the inventory spells "+k.String())
			}
		})

		t.Run("writes a class's members public at member depth", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Doc: []string{"Row is one record."}, Name: rowName}
			s.Fields.Append(
				&emit.Field{Doc: []string{"Key addresses the row."}, Name: "key", Type: ref("String")},
				&emit.Field{Name: "count", Type: ref("int"), Comment: "rows per call"},
			)
			s.Methods.Append(&emit.Method{
				Name: "load", Params: []*emit.Param{{Name: "key", Type: ref("String")}},
				Returns: []*emit.Return{{Type: ref(rowName)}},
			})
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"/**\n * Row is one record.\n */\n"+
					"public class Row {\n"+
					"    /**\n     * Key addresses the row.\n     */\n"+
					"    public String key;\n"+
					"    public int count; // rows per call\n"+
					"    public Row load(String key) {\n"+bodyStub+"    }\n"+
					"}\n",
				"docs indented whole, a trailing comment behind its semicolon")
		})

		t.Run("binds the import of a field's type", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Holder"}
			s.Fields.Append(&emit.Field{Name: "row", Type: imported(storePkg, rowName)})
			out, set, err := run(t, kind(symbol.KindStruct), s)
			assert.NoError(t, err, "the template executes")
			assert.Contains(t, out, "    public Row row;\n", "the simple name")
			assert.Equal(t, set.Paths(), []string{storePkg}, "and its package")
		})

		t.Run("writes an interface method's comments as block comments", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store", Comment: "read side"}
			i.Methods.Append(&emit.Method{
				Name: "get", Comment: "by key",
				Params:  []*emit.Param{{Name: "key", Type: ref("String"), Comment: "the row key"}},
				Returns: []*emit.Return{{Type: ref("String"), Comment: "the row"}},
			})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"public interface Store {\n"+
					"    String /* the row */ get(String key /* the row key */); // by key\n"+
					"} // read side\n",
				"the method's and the interface's comments close their lines")
		})

		t.Run("writes an interface's signatures without bodies", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store"}
			i.Methods.Append(&emit.Method{Name: "close"})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"public interface Store {\n    void close();\n}\n", "void for no result")
		})

		t.Run("writes an interface's fields as its constants", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store"}
			i.Fields.Append(&emit.Field{
				Doc: []string{"MAX bounds a batch."}, Name: "MAX", Type: ref("int"), Value: "8", Comment: "rows",
			})
			i.Methods.Append(&emit.Method{Name: "close"})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"public interface Store {\n"+
					"    /**\n     * MAX bounds a batch.\n     */\n"+
					"    int MAX = 8; // rows\n"+
					"    void close();\n"+
					"}\n",
				"the constants before the signatures, with no keyword")
		})

		t.Run("returns an error for an interface field without an initializer", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store"}
			i.Fields.Append(&emit.Field{Name: "max", Type: ref("int")})
			assert.HasError(t, refused(t, kind(symbol.KindInterface), i), "a constant takes its value")
		})

		t.Run("writes a generic class's parameters behind its name", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Box", TypeParams: []*emit.TypeParam{{Name: "T"}}}
			s.Fields.Append(&emit.Field{Name: "item", Type: ref("T")})
			s.Methods.Append(&emit.Method{
				Name:       "map",
				TypeParams: []*emit.TypeParam{{Name: "U", Bounds: []*emit.TypeRef{ref("Codec")}}},
				Params:     []*emit.Param{{Name: "item", Type: ref("U")}},
				Returns:    []*emit.Return{{Type: ref("U")}},
			})
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"public class Box<T> {\n"+
					"    public T item;\n"+
					"    public <U extends Codec> U map(U item) {\n"+bodyStub+"    }\n"+
					"}\n",
				"the method's list before its return type")
		})

		t.Run("writes a generic interface's bound behind its parameter", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{
				Name:       "Keyed",
				TypeParams: []*emit.TypeParam{{Name: "K", Bounds: []*emit.TypeRef{ref("Codec")}}},
			}
			i.Methods.Append(&emit.Method{
				Name: "pick", Params: []*emit.Param{{Name: "key", Type: ref("K")}},
				Returns: []*emit.Return{{Type: ref("K")}},
			})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"public interface Keyed<K extends Codec> {\n    K pick(K key);\n}\n", "members referencing it")
		})

		t.Run("returns an error for declaration-site variance", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Sink", TypeParams: []*emit.TypeParam{{Name: "T", Variance: symbol.VarianceIn}}}
			assert.HasError(t, refused(t, kind(symbol.KindStruct), s), "Java's wildcard is use-site")
		})

		t.Run("writes a class's heritage behind its name", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{
				Name:       rowName,
				Extends:    []*emit.TypeRef{ref(baseName)},
				Implements: []*emit.TypeRef{ref("Keyed")},
			}
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"public class Row extends Base implements Keyed {\n}\n", "the clauses")
		})

		t.Run("writes a method's throws clause behind its parameters", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: rowName}
			s.Methods.Append(&emit.Method{
				Name: "load", Returns: []*emit.Return{{Type: ref(rowName)}},
				Throws: []*emit.TypeRef{ref("IOException")},
			})
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"public class Row {\n    public Row load() throws IOException {\n"+bodyStub+"    }\n}\n",
				"the declared failure")
		})

		t.Run("writes an interface signature's throws clause", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store", Extends: []*emit.TypeRef{ref("Keyed")}}
			i.Methods.Append(&emit.Method{
				Name: "load", Returns: []*emit.Return{{Type: ref(rowName)}},
				Throws: []*emit.TypeRef{ref("IOException")},
			})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"public interface Store extends Keyed {\n    Row load() throws IOException;\n}\n",
				"a signature states it too")
		})

		t.Run("writes a class's modifiers in Java's stated order", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: rowName, Abstract: true, Annotations: symbol.Annotations{{Name: "Entity"}}}
			s.Fields.Append(&emit.Field{
				Name: "MAX", Level: symbol.LevelType, Mutability: symbol.MutabilityImmutable,
				Type: ref("int"), Value: "8",
			})
			s.Methods.Append(
				&emit.Method{Name: "load", Override: true, Returns: []*emit.Return{{Type: ref(rowName)}}},
				&emit.Method{Name: "pick", Abstract: true, Returns: []*emit.Return{{Type: ref(rowName)}}},
			)
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"@Entity\n"+
					"public abstract class Row {\n"+
					"    public static final int MAX = 8;\n"+
					"    @Override\n"+
					"    public Row load() {\n"+bodyStub+"    }\n"+
					"    public abstract Row pick();\n"+
					"}\n",
				"annotations above, the abstract method a signature alone")
		})

		t.Run("writes default before an interface method with a default body", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store"}
			i.Methods.Append(
				&emit.Method{Name: "close"},
				&emit.Method{
					Name: "load", HasDefault: true, Returns: []*emit.Return{{Type: ref(rowName)}},
					Body: emit.Body{Verbatim: "        return null;\n"},
				},
			)
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"public interface Store {\n"+
					"    void close();\n"+
					"    default Row load() {\n"+bodyStub+"    }\n"+
					"}\n",
				"the one signature that places a body")
		})

		t.Run("returns an error for a second return value", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store"}
			i.Methods.Append(&emit.Method{
				Name: "load", Returns: []*emit.Return{{Type: ref(rowName)}, {Type: ref("Exception")}},
			})
			assert.HasError(t, refused(t, kind(symbol.KindInterface), i), "the second arrives thrown")
		})

		t.Run("writes the enum constants before the members", func(t *testing.T) {
			t.Parallel()

			e := &emit.Enum{Doc: []string{"Phase names a step."}, Name: "Phase"}
			e.Variants.Append(
				&emit.EnumVariant{Doc: []string{"OPEN admits writes."}, Name: "OPEN"},
				&emit.EnumVariant{Name: "CLOSED", Comment: "terminal"},
			)
			e.Fields.Append(&emit.Field{Name: "steps", Level: symbol.LevelType, Type: ref("int"), Value: "2"})
			assert.Equal(t, execute(t, kind(symbol.KindEnum), e),
				"/**\n * Phase names a step.\n */\n"+
					"public enum Phase {\n"+
					"    /**\n     * OPEN admits writes.\n     */\n"+
					"    OPEN,\n"+
					"    CLOSED, // terminal\n"+
					"    ;\n"+
					"    public static int steps = 2;\n"+
					"}\n",
				"the members behind the semicolon")
		})

		t.Run("returns an error for an enum constant with a value", func(t *testing.T) {
			t.Parallel()

			e := &emit.Enum{Name: "Phase"}
			e.Variants.Append(&emit.EnumVariant{Name: "OPEN", Value: "1"})
			assert.HasError(t, refused(t, kind(symbol.KindEnum), e), "it takes the constructor form")
		})

		t.Run("writes a class's nested types after its members", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: rowName}
			s.Fields.Append(&emit.Field{Name: "key", Type: ref("String")})
			s.Types.Append(&emit.Struct{Name: "Inner"})
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"public class Row {\n    public String key;\n    "+nestedStub+symbol.KindStruct.String()+"\n}\n",
				"at member depth")
		})

		t.Run("writes an interface's nested types at member depth", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store"}
			i.Types.Append(&emit.Enum{Name: "Phase"})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"public interface Store {\n    "+nestedStub+symbol.KindEnum.String()+"\n}\n", "the same way")
		})

		t.Run("returns an error for a private member type of an interface", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store"}
			i.Types.Append(&emit.Struct{Name: "Inner", Visibility: symbol.VisibilityPrivate})
			assert.HasError(t, refused(t, kind(symbol.KindInterface), i), "every one is public")
		})
	})

	t.Run("FileTemplate", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the package clause then the imports then the declarations", func(t *testing.T) {
			t.Parallel()

			got := execute(t, backend.FileTemplate, struct {
				Name string
				Pkg  symbol.Identity
			}{Pkg: symbol.Identity{Package: "svc/api"}})
			assert.Equal(t, got, "package svc.api;\n\n"+importsStub+declsStub, "dots for slashes")
		})
	})

	t.Run("RefusedKinds", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every file-level kind a type contains in Java", func(t *testing.T) {
			t.Parallel()

			refused := backend.RefusedKinds()
			assert.Length(t, refused, 5, "the callables, the alias and the bindings")
			for _, k := range []symbol.Kind{
				symbol.KindFunction, symbol.KindMethod, symbol.KindAlias,
				symbol.KindConstant, symbol.KindVariable,
			} {
				assert.NotEqual(t, refused[k], "", "the refusal of "+k.String()+" states its reason")
			}
		})

		t.Run("returns no kind the templates spell", func(t *testing.T) {
			t.Parallel()

			kinds := backend.KindTemplates()
			for k := range backend.RefusedKinds() {
				assert.NotContains(t, kinds, k, "a kind is spelt or refused: "+k.String())
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
			check: func(tb assert.TB) { assert.Length(tb, kinds, 3, "KindTemplates returns three templates") },
		},
		{
			name: "RefusedKinds", allocs: templateMapAllocs,
			call:  func() { kinds = backend.RefusedKinds() },
			check: func(tb assert.TB) { assert.Length(tb, kinds, 5, "RefusedKinds returns five kinds") },
		},
	}
}

// run runs one template over one declaration the way the render pass
// does, the builtins stubbed, and returns the text or the refusal
// beside the file's import set.
func run(t *testing.T, src string, data any) (string, *render.ImportSet, error) {
	t.Helper()

	set := &render.ImportSet{}
	tmpl, err := template.New("kind").
		Funcs(backend.Funcs(set)).
		Funcs(template.FuncMap{
			render.BuiltinBody:       func(any) string { return bodyStub },
			render.BuiltinMemberBody: func(any) string { return bodyStub },
			render.BuiltinUse:        func(string) string { return "" },
			render.BuiltinImports:    func() string { return importsStub },
			render.BuiltinDecls:      func() string { return declsStub },
			render.BuiltinSlots:      func() string { return "" },
			render.BuiltinSlot:       func(string) string { return "" },
			render.BuiltinNested: func(indent string, s symbol.Symbol) string {
				return indent + nestedStub + s.Kind().String()
			},
		}).
		Parse(src)
	assert.NoError(t, err, "the template parses")
	var b strings.Builder
	err = tmpl.Execute(&b, data)
	return b.String(), set, err
}

// execute runs one template over one declaration and asserts it
// executes.
func execute(t *testing.T, src string, data any) string {
	t.Helper()

	got, _, err := run(t, src, data)
	assert.NoError(t, err, "the template executes")
	return got
}

// refused runs one template over one declaration and returns the
// refusal.
func refused(t *testing.T, src string, data any) error {
	t.Helper()

	_, _, err := run(t, src, data)
	return err
}
