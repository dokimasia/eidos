// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The members of a type lower to fields and methods, so each member
// kind's shape is pinned.
func TestMember(t *testing.T) {
	t.Parallel()

	t.Run("members", func(t *testing.T) {
		t.Parallel()

		t.Run("declares nothing for an initializer block", func(t *testing.T) {
			t.Parallel()

			st := memberClass(t, "    { }\n    static { }\n")
			assert.Empty(t, st.Fields, "no field")
			assert.Empty(t, st.Methods, "and no method")
		})

		t.Run("reports UnaddressedCarrier for a carrier on an initializer block", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public class A {\n    "+carrierLine+"    static { }\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "a block is no declaration")
		})

		t.Run("names an initializer block as the refused carrier's subject", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public class A {\n    "+carrierLine+"    static { }\n}\n")
			assert.NotEmpty(t, found, "the carrier reports")
			assert.Contains(t, found[0].Msg, onInitializer, "where the author put it")
		})
	})

	t.Run("member", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers the fields of a compact source file into its class", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldNames(implicitClassOf(t).Fields), []string{"n"}, "n is a member")
		})

		t.Run("lowers the classes of a compact source file into its class", func(t *testing.T) {
			t.Parallel()

			helper := implicitClassOf(t).Types[0].(*node.Struct)
			assert.Equal(t, helper.Name, "Helper", "Helper is a member")
		})

		t.Run("reports UnmodeledItem for a type an enum declares", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public enum Color {\n    RED;\n    static class Nested {}\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnmodeledItem}, "the nested type has no place")
			assert.Equal(t, found[0].Severity, diag.SeverityInfo, "as information")
		})

		t.Run("declares no type an enum declares", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"public enum Color {\n    RED;\n    static class Nested {}\n}\n")
			assert.Length(t, fileIn(t, gb, pkgPath).Decls, 1, "the enum alone loads")
		})
	})

	t.Run("fields", func(t *testing.T) {
		t.Parallel()

		t.Run("declares one field per declarator", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldNames(memberClass(t, "    int a, b;\n").Fields), []string{"a", "b"}, "both")
		})

		t.Run("lowers a declarator's initializer verbatim as its field's value", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberClass(t, "    int a = 1 << 4;\n").Fields[0].Value, "1 << 4", "as written")
		})

		t.Run("lowers a declaration's access as each field's visibility", func(t *testing.T) {
			t.Parallel()

			st := memberClass(t, "    private int a, b;\n")
			assert.Equal(t, st.Fields[1].Visibility, symbol.VisibilityPrivate, "the second declarator's")
		})

		t.Run("lowers a field without an access modifier as package-visible", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberClass(t, "    int n;\n").Fields[0].Visibility, symbol.VisibilityPackage,
				"package access")
		})

		t.Run("lowers a static field at the type level", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberClass(t, "    static int n;\n").Fields[0].Level, symbol.LevelType, "a static")
		})

		t.Run("lowers a field without static at the instance level", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberClass(t, "    int n;\n").Fields[0].Level, symbol.LevelInstance,
				"each instance's own")
		})

		t.Run("lowers a final field as immutable", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberClass(t, "    final int n = 1;\n").Fields[0].Mutability, symbol.MutabilityImmutable,
				"assigned once")
		})

		t.Run("lowers a field without final as mutable", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberClass(t, "    int n;\n").Fields[0].Mutability, symbol.MutabilityMutable,
				"assignable")
		})

		t.Run("lowers an interface's field as public", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberIface(t, "    int K = 1;\n").Fields[0].Visibility, symbol.VisibilityPublic,
				"an interface's member")
		})

		t.Run("lowers an interface's field at the type level", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberIface(t, "    int K = 1;\n").Fields[0].Level, symbol.LevelType,
				"an interface's field is static")
		})

		t.Run("lowers an interface's field as immutable", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberIface(t, "    int K = 1;\n").Fields[0].Mutability, symbol.MutabilityImmutable,
				"an interface's field is final")
		})

		t.Run("annotates each field of a declaration with its annotations", func(t *testing.T) {
			t.Parallel()

			st := memberClass(t, "    @Deprecated int a, b;\n")
			assert.Equal(t, st.Fields[1].Annotations, symbol.Annotations{{Name: "Deprecated"}}, "the second's")
		})

		t.Run("documents each field of a declaration with its Javadoc", func(t *testing.T) {
			t.Parallel()

			st := memberClass(t, "    /** The pair. */\n    int a, b;\n")
			assert.Equal(t, st.Fields[1].Doc, []string{"The pair."}, "the second's")
		})

		t.Run("attaches a declaration's carriers to each field it declares", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"public class A {\n    "+carrierLine+"    public int a, b;\n}\n")
			assert.Length(t, gb.Attachments(), 2, "one directive per field")
		})

		t.Run("leaves out a private field at signature depth", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, shallowDecls(t, "public class A {\n    private int a;\n    "+
				"protected int b;\n}\n"), "A")
			assert.Equal(t, fieldNames(st.Fields), []string{"b"}, "the protected field alone")
		})
	})

	t.Run("method", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a method's type parameters", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, memberClass(t, "    <U> U conv() { return null; }\n").Methods[0].TypeParams, 1, "U")
		})

		t.Run("lowers a method's parameters in order", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    void f(int a, String b) {}\n").Methods[0]
			assert.Equal(t, paramNames(m.Params), []string{"a", "b"}, "both")
		})

		t.Run("lowers a method's result type as its one result", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    String name() { return null; }\n").Methods[0]
			assert.Equal(t, m.Returns[0].Type.Spelling, "String", "the result")
		})

		t.Run("lowers a void method without a result", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, memberClass(t, "    void f() {}\n").Methods[0].Returns, "void is no result")
		})

		t.Run("lowers a method's dimensions into its result", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    int f()[] { return null; }\n").Methods[0]
			assert.Equal(t, m.Returns[0].Type.Form, symbol.FormList, "int f()[] returns an array")
		})

		t.Run("lowers the exceptions a method throws", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    void f() throws java.io.IOException, E {}\n").Methods[0]
			assert.Equal(t, spellingsOf(m.Throws), []string{"java.io.IOException", "E"}, "both in order")
		})

		t.Run("lowers a static method at the type level", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberClass(t, "    static void f() {}\n").Methods[0].Level, symbol.LevelType, "static")
		})

		t.Run("lowers an abstract method as abstract", func(t *testing.T) {
			t.Parallel()

			st := classOf(t, "public abstract class A {\n    abstract void f();\n}\n", "A")
			assert.True(t, st.Methods[0].Abstract, "a subclass supplies the body")
		})

		t.Run("lowers a final method as final", func(t *testing.T) {
			t.Parallel()

			assert.True(t, memberClass(t, "    final void g() {}\n").Methods[0].Final, "no subclass overrides it")
		})

		t.Run("lowers a native method as not abstract", func(t *testing.T) {
			t.Parallel()

			assert.False(t, memberClass(t, "    native void f();\n").Methods[0].Abstract, "the platform's body")
		})

		t.Run("attaches a method's carriers to the method", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"public class A {\n    "+carrierLine+"    void f() {}\n}\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onMethod := gb.Attachments()[0].Subject.(*node.Method)
			assert.True(t, onMethod, "on the method")
		})

		t.Run("lowers a method without an access modifier as package-visible", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberClass(t, "    void f() {}\n").Methods[0].Visibility, symbol.VisibilityPackage,
				"package access")
		})

		t.Run("annotates a method with its annotations", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    @Override\n    public String toString() { return null; }\n").Methods[0]
			assert.Equal(t, m.Annotations, symbol.Annotations{{Name: "Override"}}, "the annotation")
		})

		t.Run("lowers an interface's method without a body as abstract", func(t *testing.T) {
			t.Parallel()

			assert.True(t, memberIface(t, "    void m();\n").Methods[0].Abstract, "the implementation supplies it")
		})

		t.Run("lowers an interface's method without an access modifier as public", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberIface(t, "    void m();\n").Methods[0].Visibility, symbol.VisibilityPublic,
				"an interface's member")
		})

		t.Run("lowers an interface's default method as having a default", func(t *testing.T) {
			t.Parallel()

			assert.True(t, memberIface(t, "    default int d() { return 0; }\n").Methods[0].HasDefault,
				"an implementation inherits the body")
		})

		t.Run("lowers an interface's default method as not abstract", func(t *testing.T) {
			t.Parallel()

			assert.False(t, memberIface(t, "    default int d() { return 0; }\n").Methods[0].Abstract,
				"the method has a body")
		})

		t.Run("lowers an interface's static method without a default", func(t *testing.T) {
			t.Parallel()

			m := memberIface(t, "    static int s() { return 0; }\n").Methods[0]
			assert.False(t, m.HasDefault, "an implementation inherits no static method")
		})

		t.Run("lowers an interface's private method as private", func(t *testing.T) {
			t.Parallel()

			m := memberIface(t, "    private int p() { return 0; }\n").Methods[0]
			assert.Equal(t, m.Visibility, symbol.VisibilityPrivate, "a helper of the default methods")
		})

		t.Run("leaves out a package-visible method at signature depth", func(t *testing.T) {
			t.Parallel()

			decls := shallowDecls(t, "public class A {\n    void f() {}\n    public void g() {}\n}\n")
			assert.Equal(t, methodNames(named[*node.Struct](t, decls, "A").Methods), []string{"g"},
				"g alone is published")
		})
	})

	t.Run("constructor", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a constructor as a method that constructs", func(t *testing.T) {
			t.Parallel()

			assert.True(t, memberClass(t, "    A() {}\n").Methods[0].Constructs, "constructs an A")
		})

		t.Run("names a constructor after its class", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberClass(t, "    A() {}\n").Methods[0].Name, "A", "the class's name")
		})

		t.Run("lowers a constructor's parameters", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, paramNames(memberClass(t, "    A(int a) {}\n").Methods[0].Params), []string{"a"}, "a")
		})

		t.Run("lowers a constructor's type parameters", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, memberClass(t, "    <T> A(T t) {}\n").Methods[0].TypeParams, 1, "T")
		})

		t.Run("annotates a constructor with its annotations", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    @Deprecated\n    A() {}\n").Methods[0]
			assert.Equal(t, m.Annotations, symbol.Annotations{{Name: "Deprecated"}}, "the annotation")
		})

		t.Run("attaches a constructor's carriers to its method", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"public class A {\n    "+carrierLine+"    A() {}\n}\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onMethod := gb.Attachments()[0].Subject.(*node.Method)
			assert.True(t, onMethod, "on the constructor")
		})

		t.Run("lowers the exceptions a constructor throws", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    A() throws Exception {}\n").Methods[0]
			assert.Equal(t, spellingsOf(m.Throws), []string{"Exception"}, "the exception")
		})

		t.Run("lowers a constructor's access as its visibility", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    protected A() {}\n").Methods[0]
			assert.Equal(t, m.Visibility, symbol.VisibilityProtected, "protected")
		})

		t.Run("lowers a constructor without an access modifier as package-visible", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, memberClass(t, "    A() {}\n").Methods[0].Visibility, symbol.VisibilityPackage,
				"package access")
		})

		t.Run("lowers an enum's constructor without an access modifier as private", func(t *testing.T) {
			t.Parallel()

			e := enumOf(t, "public enum Color {\n    RED;\n    Color() {}\n}\n")
			assert.Equal(t, e.Methods[0].Visibility, symbol.VisibilityPrivate, "an enum's constructor is private")
		})

		t.Run("lowers a compact constructor with the record's components as its parameters", func(t *testing.T) {
			t.Parallel()

			st := classOf(t, "public record Pt(int x, int y) {\n    public Pt {}\n}\n", "Pt")
			assert.Equal(t, paramNames(st.Methods[0].Params), []string{"x", "y"}, "the components")
		})

		t.Run("leaves out a private constructor at signature depth", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, shallowDecls(t, "public class A {\n    private A() {}\n}\n"), "A")
			assert.Empty(t, st.Methods, "the constructor is private")
		})
	})

	t.Run("element", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an annotation type's element as an abstract method", func(t *testing.T) {
			t.Parallel()

			assert.True(t, elementOf(t, "    int[] n();\n").Abstract, "an annotation states the value")
		})

		t.Run("lowers an annotation type's element as public", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, elementOf(t, "    int[] n();\n").Visibility, symbol.VisibilityPublic, "public")
		})

		t.Run("lowers an annotation type's element's type as its result", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, elementOf(t, "    int[] n();\n").Returns[0].Type.Form, symbol.FormList, "int[]")
		})

		t.Run("lowers an element's dimensions into its result", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, elementOf(t, "    int n()[];\n").Returns[0].Type.Form, symbol.FormList, "int n()[]")
		})

		t.Run("annotates an element with its annotations", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, elementOf(t, "    @Deprecated int n();\n").Annotations,
				symbol.Annotations{{Name: "Deprecated"}}, "the annotation")
		})

		t.Run("lowers an annotation type's element with a default as its method", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, elementOf(t, "    String value() default \"x\";\n").Name, "value", "the element")
		})
	})

	t.Run("params", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a variadic parameter as positionally variadic", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    void f(int... xs) {}\n").Methods[0]
			assert.Equal(t, m.Params[0].Variadic, symbol.VariadicPositional, "varargs")
		})

		t.Run("lowers a variadic parameter's type as the element it collects", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    void f(String... xs) {}\n").Methods[0]
			assert.Equal(t, m.Params[0].Type.Spelling, "String", "each argument's type")
		})

		t.Run("lowers a variadic parameter's type past its modifiers", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    void f(final String... xs) {}\n").Methods[0]
			assert.Equal(t, m.Params[0].Type.Spelling, "String", "final is no type")
		})

		t.Run("lowers a variadic parameter's type past a comment", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    void f(final /* names */ String... xs) {}\n").Methods[0]
			assert.Equal(t, m.Params[0].Type.Spelling, "String", "the comment is no type")
		})

		t.Run("names a variadic parameter", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, paramNames(memberClass(t, "    void f(int... xs) {}\n").Methods[0].Params),
				[]string{"xs"}, "the declarator's name")
		})

		t.Run("lowers a parameter's dimensions into its type", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    void f(int a[]) {}\n").Methods[0]
			assert.Equal(t, m.Params[0].Type.Form, symbol.FormList, "int a[] is an array")
		})

		t.Run("lowers an explicit receiver apart from the parameters", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    void f(A this, int n) {}\n").Methods[0]
			assert.Equal(t, paramNames(m.Params), []string{"n"}, "the receiver is no parameter")
		})

		t.Run("lowers an explicit receiver's type", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    void f(A this) {}\n").Methods[0]
			assert.Equal(t, m.Receiver.Type.Spelling, "A", "the receiver's class")
		})

		t.Run("lowers an annotated explicit receiver apart from the parameters", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    void f(@Ann A this, int n) {}\n").Methods[0]
			assert.Equal(t, paramNames(m.Params), []string{"n"}, "the grammar reads it as a parameter named this")
		})

		t.Run("annotates an annotated explicit receiver with its annotations", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    void f(@Ann A this) {}\n").Methods[0]
			assert.Equal(t, m.Receiver.Annotations, symbol.Annotations{{Name: "Ann"}}, "the annotation")
		})

		t.Run("annotates a parameter with its annotations", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    void f(@NonNull String s) {}\n").Methods[0]
			assert.Equal(t, m.Params[0].Annotations, symbol.Annotations{{Name: "NonNull"}}, "the annotation")
		})

		t.Run("annotates a variadic parameter with its annotations", func(t *testing.T) {
			t.Parallel()

			m := memberClass(t, "    void f(@NonNull String... s) {}\n").Methods[0]
			assert.Equal(t, m.Params[0].Annotations, symbol.Annotations{{Name: "NonNull"}}, "the annotation")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a parameter", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public class A {\n    void f(\n        "+carrierLine+
				"        int n) {}\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "a parameter takes no carrier")
		})

		t.Run("names a parameter as the refused carrier's subject", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public class A {\n    void f(\n        "+carrierLine+
				"        int n) {}\n}\n")
			assert.NotEmpty(t, found, "the carrier reports")
			assert.Contains(t, found[0].Msg, onParameter, "where the author put it")
		})
	})

	t.Run("typeParams", func(t *testing.T) {
		t.Parallel()

		t.Run("names a type parameter", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, classOf(t, "public class A<T> {}\n", "A").TypeParams[0].Name, "T", "T")
		})

		t.Run("lowers a type parameter's bounds", func(t *testing.T) {
			t.Parallel()

			tp := classOf(t, "public class A<T extends Comparable<T> & Cloneable> {}\n", "A").TypeParams[0]
			assert.Equal(t, spellingsOf(tp.Bounds), []string{"Comparable", "Cloneable"}, "each bound")
		})

		t.Run("lowers no type parameter for a comment in the list", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, classOf(t, "public class A</* key */ K, V> {}\n", "A").TypeParams, 2, "K and V")
		})
	})

	t.Run("typesOf", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers no type for a comment in a clause", func(t *testing.T) {
			t.Parallel()

			st := classOf(t, "public class A implements I /* and */, J {}\n", "A")
			assert.Equal(t, spellingsOf(st.Implements), []string{"I", "J"}, "the comment is no interface")
		})
	})

	t.Run("unmodeled", func(t *testing.T) {
		t.Parallel()

		t.Run("reports UnaddressedCarrier for a carrier on a type an enum declares", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public enum Color {\n    RED;\n    "+carrierLine+
				"    static class Nested {}\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnmodeledItem, frontend.UnaddressedCarrier},
				"the type and its carrier")
		})

		t.Run("names a type an enum declares as the refused carrier's subject", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public enum Color {\n    RED;\n    "+carrierLine+
				"    static class Nested {}\n}\n")
			assert.Length(t, found, 2, "the type and its carrier")
			assert.Contains(t, found[1].Msg, onEnumType, "where the author put it")
		})

		t.Run("passes the sweep over a type an enum declares", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public enum Color {\n    RED;\n    static class Nested {\n        "+
				carrierLine+"        int n;\n    }\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnmodeledItem}, "the carrier inside leaves with it")
		})
	})
}

// memberClass parses a public class A of a body and returns it.
func memberClass(tb testing.TB, body string) *node.Struct {
	tb.Helper()

	return classOf(tb, "public class A {\n"+body+"}\n", "A")
}

// memberIface parses a public interface I of a body and returns it.
func memberIface(tb testing.TB, body string) *node.Interface {
	tb.Helper()

	return ifaceOf(tb, "public interface I {\n"+body+"}\n")
}

// elementOf parses a public annotation type Ann of a body and returns
// its first method.
func elementOf(tb testing.TB, body string) *node.Method {
	tb.Helper()

	it := named[*node.Interface](tb, declsOf(tb, "public @interface Ann {\n"+body+"}\n"), "Ann")
	assert.NotEmpty(tb, it.Methods, "the annotation type declares an element")
	return it.Methods[0]
}

// fieldNames returns the names of fields, in order.
func fieldNames(fields []*node.Field) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, f.Name)
	}
	return out
}

// methodNames returns the names of methods, in order.
func methodNames(methods []*node.Method) []string {
	out := make([]string, 0, len(methods))
	for _, m := range methods {
		out = append(out, m.Name)
	}
	return out
}

// paramNames returns the names of parameters, in order.
func paramNames(params []*node.Param) []string {
	out := make([]string, 0, len(params))
	for _, p := range params {
		out = append(out, p.Name)
	}
	return out
}
