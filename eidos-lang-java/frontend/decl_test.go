// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// compactSource is a compact source file: a field, the method that makes
// the file compact, and a class, all members of the class the file
// implicitly declares.
const compactSource = "int n;\n\nvoid main() {}\n\nclass Helper {}\n"

// implicitName is the name of the class compactSource declares at
// srcFile: the file's name without its extension.
const implicitName = "A"

// Each Java type declaration lowers to one model kind, so the shape of
// every kind's declaration is pinned.
func TestDecl(t *testing.T) {
	t.Parallel()

	t.Run("implicitClass", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the class of a compact source file under the file's name", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, compactSource)
			named[*node.Struct](t, fileIn(t, gb, "").Decls, implicitName)
		})

		t.Run("declares the class of a compact source file final", func(t *testing.T) {
			t.Parallel()

			assert.True(t, implicitClassOf(t).Final, "no class extends it")
		})

		t.Run("declares the class of a compact source file with package access", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, implicitClassOf(t).Visibility, symbol.VisibilityPackage, "it is in the unnamed package")
		})

		t.Run("lowers the methods of a compact source file into its class", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, methodNames(implicitClassOf(t).Methods), []string{"main"}, "main is a member")
		})
	})

	t.Run("typeDecl", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a type without an access modifier as package-visible", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, classOf(t, "class A {}\n", "A").Visibility, symbol.VisibilityPackage,
				"no modifier is package access")
		})

		tests := []struct {
			name string
			give string
			want symbol.Visibility
		}{
			{name: "lowers a public type as public", give: "public", want: symbol.VisibilityPublic},
			{name: "lowers a protected type as protected", give: "protected", want: symbol.VisibilityProtected},
			{name: "lowers a private type as private", give: "private", want: symbol.VisibilityPrivate},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				outer := classOf(t, "public class O {\n    "+tt.give+" static class A {}\n}\n", "O")
				assert.Equal(t, outer.Types[0].(*node.Struct).Visibility, tt.want, "the modifier's access")
			})
		}

		t.Run("lowers a type in an interface without an access modifier as public", func(t *testing.T) {
			t.Parallel()

			it := named[*node.Interface](t, declsOf(t, "public interface I {\n    class A {}\n}\n"), "I")
			assert.Equal(t, it.Types[0].(*node.Struct).Visibility, symbol.VisibilityPublic, "an interface's member")
		})

		t.Run("leaves out a package-visible type at signature depth", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, shallowDecls(t, "class A {}\npublic class B {}\n"), 1, "B alone is published")
		})

		t.Run("attaches a type's carriers to the type", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+carrierLine+publicClass)
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onStruct := gb.Attachments()[0].Subject.(*node.Struct)
			assert.True(t, onStruct, "on the class")
		})
	})

	t.Run("class", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an abstract class as abstract", func(t *testing.T) {
			t.Parallel()

			assert.True(t, classOf(t, "public abstract class A {}\n", "A").Abstract, "no instance of A is made")
		})

		t.Run("lowers a final class as final", func(t *testing.T) {
			t.Parallel()

			assert.True(t, classOf(t, "public final class A {}\n", "A").Final, "no class extends A")
		})

		t.Run("lowers a sealed class as sealed", func(t *testing.T) {
			t.Parallel()

			assert.True(t, classOf(t, "public sealed class A permits B {}\n", "A").Sealed, "B alone extends A")
		})

		t.Run("lowers a class's type parameters", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, classOf(t, "public class A<K, V> {}\n", "A").TypeParams, 2, "K and V")
		})

		t.Run("lowers a class's superclass as what it extends", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spellingsOf(classOf(t, "public class A<T> extends B<T> {}\n", "A").Extends),
				[]string{"B"}, "the superclass's bare name")
		})

		t.Run("lowers a class's interfaces as what it implements", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spellingsOf(classOf(t, "public class A implements I, J {}\n", "A").Implements),
				[]string{"I", "J"}, "both interfaces in order")
		})

		t.Run("lowers a class's permitted subclasses as what it permits", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spellingsOf(classOf(t, "public sealed class A permits C, D {}\n", "A").Permits),
				[]string{"C", "D"}, "both subclasses in order")
		})

		t.Run("annotates a class with its annotations", func(t *testing.T) {
			t.Parallel()

			st := classOf(t, "@Deprecated\n@SuppressWarnings({\"a\", \"b\"})\npublic class A {}\n", "A")
			assert.Equal(t, st.Annotations, symbol.Annotations{
				{Name: "Deprecated"}, {Name: "SuppressWarnings", Args: []string{"{\"a\", \"b\"}"}},
			}, "each annotation's name and arguments verbatim")
		})

		t.Run("annotates a class with each named argument as written", func(t *testing.T) {
			t.Parallel()

			st := classOf(t, "@a.b.Table(name = \"rows\", schema = \"s\")\npublic class A {}\n", "A")
			assert.Equal(t, st.Annotations, symbol.Annotations{
				{Name: "a.b.Table", Args: []string{"name = \"rows\"", "schema = \"s\""}},
			}, "a qualified name and two pairs")
		})

		t.Run("spells an annotation's name without the whitespace between its names", func(t *testing.T) {
			t.Parallel()

			st := classOf(t, "@a . b.Table\npublic class A {}\n", "A")
			assert.Equal(t, st.Annotations, symbol.Annotations{{Name: "a.b.Table"}}, "dots between the names")
		})

		t.Run("lowers a static nested class at the type level", func(t *testing.T) {
			t.Parallel()

			outer := classOf(t, "public class O {\n    public static class A {}\n}\n", "O")
			assert.Equal(t, outer.Types[0].(*node.Struct).Level, symbol.LevelType, "a static nested class")
		})

		t.Run("lowers an inner class at the instance level", func(t *testing.T) {
			t.Parallel()

			outer := classOf(t, "public class O {\n    public class A {}\n}\n", "O")
			assert.Equal(t, outer.Types[0].(*node.Struct).Level, symbol.LevelInstance, "an inner class")
		})

		t.Run("lowers a class in an interface at the type level", func(t *testing.T) {
			t.Parallel()

			it := named[*node.Interface](t, declsOf(t, "public interface I {\n    class A {}\n}\n"), "I")
			assert.Equal(t, it.Types[0].(*node.Struct).Level, symbol.LevelType, "an interface's class is static")
		})

		t.Run("lowers a class of a compact source file as an inner class of its class", func(t *testing.T) {
			t.Parallel()

			helper := implicitClassOf(t).Types[0].(*node.Struct)
			assert.Equal(t, helper.Level, symbol.LevelInstance, "a member class without static is inner")
		})
	})

	t.Run("iface", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a sealed interface as sealed", func(t *testing.T) {
			t.Parallel()

			assert.True(t, ifaceOf(t, "public sealed interface I permits A {}\n").Sealed, "A alone implements I")
		})

		t.Run("lowers an interface's type parameters", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, ifaceOf(t, "public interface I<T> {}\n").TypeParams, 1, "T")
		})

		t.Run("lowers the interfaces an interface extends", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spellingsOf(ifaceOf(t, "public interface I<T> extends J, K<T> {}\n").Extends),
				[]string{"J", "K"}, "both interfaces in order")
		})

		t.Run("lowers the subtypes an interface permits", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spellingsOf(ifaceOf(t, "public sealed interface I permits A, B {}\n").Permits),
				[]string{"A", "B"}, "both subtypes in order")
		})

		t.Run("annotates an interface with its annotations", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, ifaceOf(t, "@FunctionalInterface\npublic interface I {}\n").Annotations,
				symbol.Annotations{{Name: "FunctionalInterface"}}, "the annotation")
		})

		t.Run("stamps an annotation type java.annotationType", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"public @interface Ann {}\n")
			named[*node.Interface](t, fileIn(t, gb, pkgPath).Decls, "Ann")
			assert.Equal(t, stampsOf(gb, java.AnnotationTypeKey), []any{true}, "the annotation type")
		})

		t.Run("stamps no plain interface java.annotationType", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"public interface I {}\n")
			assert.Empty(t, stampsOf(gb, java.AnnotationTypeKey), "an interface is no annotation type")
		})
	})

	t.Run("enum", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an enum's constants as its variants", func(t *testing.T) {
			t.Parallel()

			e := enumOf(t, "public enum Color {\n    RED, GREEN, BLUE\n}\n")
			assert.Equal(t, variantNames(e.Variants), []string{"RED", "GREEN", "BLUE"}, "in source order")
		})

		t.Run("lowers an enum's fields", func(t *testing.T) {
			t.Parallel()

			e := enumOf(t, "public enum Color {\n    RED;\n    private final int code = 0;\n}\n")
			assert.Equal(t, fieldNames(e.Fields), []string{"code"}, "the body's field")
		})

		t.Run("lowers an enum's methods", func(t *testing.T) {
			t.Parallel()

			e := enumOf(t, "public enum Color {\n    RED;\n    public int code() { return 0; }\n}\n")
			assert.Equal(t, methodNames(e.Methods), []string{"code"}, "the body's method")
		})

		t.Run("annotates an enum with its annotations", func(t *testing.T) {
			t.Parallel()

			e := enumOf(t, "@Deprecated\npublic enum Color {\n    RED\n}\n")
			assert.Equal(t, e.Annotations, symbol.Annotations{{Name: "Deprecated"}}, "the annotation")
		})
	})

	t.Run("variant", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a constant's arguments verbatim as its value", func(t *testing.T) {
			t.Parallel()

			e := enumOf(t, "public enum Color {\n    RED(\"r\", 1);\n    Color(String s, int n) {}\n}\n")
			assert.Equal(t, e.Variants[0].Value, "\"r\", 1", "between the parentheses")
		})

		t.Run("lowers a constant without arguments without a value", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, enumOf(t, "public enum Color {\n    RED\n}\n").Variants[0].Value, "", "no arguments")
		})

		t.Run("lowers a constant with an empty argument list without a value", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, enumOf(t, "public enum Color {\n    RED()\n}\n").Variants[0].Value, "", "no arguments")
		})

		t.Run("lowers a constant with a body without the body", func(t *testing.T) {
			t.Parallel()

			e := enumOf(t, "public enum Op {\n    PLUS {\n        int apply() { return 1; }\n    };\n}\n")
			assert.Empty(t, e.Methods, "the body is an anonymous class")
		})

		t.Run("documents a constant with its Javadoc", func(t *testing.T) {
			t.Parallel()

			e := enumOf(t, "public enum Color {\n    /** The red. */\n    RED\n}\n")
			assert.Equal(t, e.Variants[0].Doc, []string{"The red."}, "the comment above the constant")
		})

		t.Run("annotates a constant with its annotations", func(t *testing.T) {
			t.Parallel()

			e := enumOf(t, "public enum Color {\n    @Deprecated RED\n}\n")
			assert.Equal(t, e.Variants[0].Annotations, symbol.Annotations{{Name: "Deprecated"}}, "the annotation")
		})

		t.Run("attaches a constant's carriers to its variant", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"public enum Color {\n    "+carrierLine+"    RED\n}\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onVariant := gb.Attachments()[0].Subject.(*node.EnumVariant)
			assert.True(t, onVariant, "on the variant")
		})
	})

	t.Run("record", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a record as final", func(t *testing.T) {
			t.Parallel()

			assert.True(t, classOf(t, "public record Pt(int x) {}\n", "Pt").Final, "no class extends a record")
		})

		t.Run("lowers each component as a field", func(t *testing.T) {
			t.Parallel()

			st := classOf(t, "public record Pt(int x, int y) {}\n", "Pt")
			assert.Equal(t, fieldNames(st.Fields), []string{"x", "y"}, "in order")
		})

		t.Run("lowers a component as immutable", func(t *testing.T) {
			t.Parallel()

			st := classOf(t, "public record Pt(int x) {}\n", "Pt")
			assert.Equal(t, st.Fields[0].Mutability, symbol.MutabilityImmutable, "a component is final")
		})

		t.Run("lowers a component as public", func(t *testing.T) {
			t.Parallel()

			st := classOf(t, "public record Pt(int x) {}\n", "Pt")
			assert.Equal(t, st.Fields[0].Visibility, symbol.VisibilityPublic, "its accessor is public")
		})

		t.Run("annotates a component with its annotations", func(t *testing.T) {
			t.Parallel()

			st := classOf(t, "public record Pt(@NonNull String y) {}\n", "Pt")
			assert.Equal(t, st.Fields[0].Annotations, symbol.Annotations{{Name: "NonNull"}}, "the annotation")
		})

		t.Run("lowers a variadic component as a list", func(t *testing.T) {
			t.Parallel()

			st := classOf(t, "public record R(int... xs) {}\n", "R")
			assert.Equal(t, st.Fields[0].Type.Form, symbol.FormList, "the array the component is")
		})

		t.Run("lowers a record's type parameters", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, classOf(t, "public record Box<T>(T v) {}\n", "Box").TypeParams, 1, "T")
		})

		t.Run("lowers a record's interfaces as what it implements", func(t *testing.T) {
			t.Parallel()

			st := classOf(t, "public record Pt(int x) implements I {}\n", "Pt")
			assert.Equal(t, spellingsOf(st.Implements), []string{"I"}, "the interface")
		})

		t.Run("lowers a record's members", func(t *testing.T) {
			t.Parallel()

			st := classOf(t, "public record Pt(int x) {\n    public int twice() { return 2 * x; }\n}\n", "Pt")
			assert.Equal(t, methodNames(st.Methods), []string{"twice"}, "the body's method")
		})

		t.Run("stamps a record java.record", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"public record Pt(int x) {}\n")
			assert.Equal(t, stampsOf(gb, java.RecordKey), []any{true}, "the record")
		})

		t.Run("lowers a nested record at the type level", func(t *testing.T) {
			t.Parallel()

			outer := classOf(t, "public class O {\n    public record R() {}\n}\n", "O")
			assert.Equal(t, outer.Types[0].(*node.Struct).Level, symbol.LevelType, "a record is static")
		})

		t.Run("lowers a top-level record at the instance level", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, classOf(t, "public record R() {}\n", "R").Level, symbol.LevelInstance,
				"a top-level type binds nothing")
		})
	})
}

// classOf parses a body below the package clause of com.acme and
// returns the struct of a name its file declares.
func classOf(tb testing.TB, body, name string) *node.Struct {
	tb.Helper()

	return named[*node.Struct](tb, declsOf(tb, body), name)
}

// ifaceOf parses a body below the package clause of com.acme that
// declares the interface I, and returns it.
func ifaceOf(tb testing.TB, body string) *node.Interface {
	tb.Helper()

	return named[*node.Interface](tb, declsOf(tb, body), "I")
}

// enumOf parses a body below the package clause of com.acme that
// declares the enum its first declaration is, and returns it.
func enumOf(tb testing.TB, body string) *node.Enum {
	tb.Helper()

	decls := declsOf(tb, body)
	assert.NotEmpty(tb, decls, "the body declares the enum")
	e, is := decls[0].(*node.Enum)
	assert.True(tb, is, "the first declaration is an enum")
	return e
}

// implicitClassOf parses compactSource and returns the class it
// implicitly declares.
func implicitClassOf(tb testing.TB) *node.Struct {
	tb.Helper()

	gb, _ := parsedSource(tb, compactSource)
	return named[*node.Struct](tb, fileIn(tb, gb, "").Decls, implicitName)
}

// spellingsOf returns the spellings of references, in order.
func spellingsOf(refs []*node.TypeRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Spelling)
	}
	return out
}

// variantNames returns the names of enum variants, in order.
func variantNames(variants []*node.EnumVariant) []string {
	out := make([]string, 0, len(variants))
	for _, v := range variants {
		out = append(out, v.Name)
	}
	return out
}
