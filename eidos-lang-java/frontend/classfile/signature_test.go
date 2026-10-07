// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/java/frontend/classfile"
)

// nestedNameAllocs is a nested class's binary name, written into one
// buffer.
const nestedNameAllocs = 1

// The type names the signature cases decode, the opening and closing of
// a parameterized List, and the descriptors of two interfaces and an
// exception the assembled signatures state.
const (
	mapName         = "java/util/Map"
	numberName      = "java/lang/Number"
	comparableName  = "java/lang/Comparable"
	exceptionName   = "java/lang/Exception"
	listOpen        = "Ljava/util/List<"
	argsClose       = ">;"
	runnableDesc    = "Ljava/lang/Runnable;"
	closeableDesc   = "Ljava/lang/AutoCloseable;"
	ioExceptionDesc = "Ljava/io/IOException;"
)

// Every generic declaration is read through the signature grammar. Each
// production, and each way a signature breaks it, is pinned.
func TestSignature(t *testing.T) {
	t.Parallel()

	t.Run("Keyword", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give byte
			want string
		}{
			{name: "returns byte for B", give: 'B', want: "byte"},
			{name: "returns char for C", give: 'C', want: "char"},
			{name: "returns double for D", give: 'D', want: "double"},
			{name: "returns float for F", give: 'F', want: "float"},
			{name: "returns int for I", give: 'I', want: "int"},
			{name: "returns long for J", give: 'J', want: "long"},
			{name: "returns short for S", give: 'S', want: "short"},
			{name: "returns boolean for Z", give: 'Z', want: "boolean"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				typ := classfile.Type{Kind: classfile.KindBase, Base: tt.give}
				assert.Equal(t, typ.Keyword(), tt.want, "the keyword")
			})
		}

		t.Run("returns no keyword for a class type", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, classfile.Type{Kind: classfile.KindClass}.Keyword(), "", "a class is no base type")
		})
	})

	t.Run("BinaryName", func(t *testing.T) {
		t.Parallel()

		t.Run("joins a class type's names with $", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, mapEntry().BinaryName(), mapName+"$Entry", "Map.Entry's binary name")
		})

		t.Run("returns a top-level class's name", func(t *testing.T) {
			t.Parallel()

			typ := classfile.Type{Kind: classfile.KindClass, Class: []classfile.ClassName{{Name: mapName}}}
			assert.Equal(t, typ.BinaryName(), mapName, "Map's binary name")
		})

		t.Run("returns no binary name for a type variable", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, classfile.Type{Kind: classfile.KindVar, Var: "T"}.BinaryName(), "", "T names no class")
		})
	})

	t.Run("reference", func(t *testing.T) {
		t.Parallel()

		t.Run("parses a wildcard with an upper bound", func(t *testing.T) {
			t.Parallel()

			arg := fieldSignature(t, "numbers").Class[0].Args[0]
			assert.Equal(t, arg.Bound, classfile.BoundExtends, "? extends")
			assert.Equal(t, arg.Type.BinaryName(), numberName, "Number")
		})

		t.Run("parses an unbounded wildcard without a type", func(t *testing.T) {
			t.Parallel()

			arg := fieldSignature(t, "any").Class[0].Args[0]
			assert.Equal(t, arg, classfile.TypeArg{Bound: classfile.BoundAny}, "?")
		})

		t.Run("parses a wildcard with a lower bound", func(t *testing.T) {
			t.Parallel()

			conv := methodNamed(t, fixture(t, boxFile), "conv", "")
			assert.Equal(t, conv.Signature.Params[0].Class[0].Args[0].Bound, classfile.BoundSuper, "? super U")
		})

		t.Run("parses nested type arguments", func(t *testing.T) {
			t.Parallel()

			index := fieldSignature(t, "index").Class[0]
			assert.Equal(t, index.Name, mapName, "Map")
			assert.Equal(t, index.Args[1].Type.Class[0].Args[0].Type.Var, "T", "of String and List<T>")
		})

		t.Run("parses a type variable", func(t *testing.T) {
			t.Parallel()

			get := methodNamed(t, fixture(t, boxFile), "get", "")
			assert.Equal(t, *get.Signature.Result, classfile.Type{Kind: classfile.KindVar, Var: "T"}, "T")
		})

		t.Run("parses an array type", func(t *testing.T) {
			t.Parallel()

			grid := methodNamed(t, fixture(t, boxFile), "grid", "")
			assert.Equal(t, grid.Result.Elem.Elem.Keyword(), "int", "int[][]")
		})

		t.Run("parses a class type with a suffix", func(t *testing.T) {
			t.Parallel()

			inner := methodNamed(t, fixture(t, boxFile), "inner", "")
			assert.Equal(t, inner.Signature.Result.BinaryName(), boxName+"$Inner", "Box<T>.Inner")
			assert.Equal(t, inner.Signature.Result.Class[0].Args[0].Type.Var, "T", "with Box's argument")
		})

		t.Run("parses type arguments nested 64 deep", func(t *testing.T) {
			t.Parallel()

			_, err := signedField(strings.Repeat(listOpen, 63) + objectDesc + strings.Repeat(argsClose, 63))
			assert.NoError(t, err, "the cap admits 64 levels")
		})

		t.Run("returns ErrMalformed for type arguments nested deeper than 64", func(t *testing.T) {
			t.Parallel()

			_, err := signedField(strings.Repeat(listOpen, 64) + objectDesc + strings.Repeat(argsClose, 64))
			assert.ErrorIs(t, err, classfile.ErrMalformed, "the 65th level is past the cap")
		})

		t.Run("parses 65 reference types side by side", func(t *testing.T) {
			t.Parallel()

			c, err := signedMethod("(" + strings.Repeat(objectDesc, 65) + ")V")
			assert.NoError(t, err, "siblings nest no deeper than one level")
			assert.Length(t, c.Methods[0].Signature.Params, 65, "every parameter")
		})
	})

	t.Run("typeParams", func(t *testing.T) {
		t.Parallel()

		t.Run("parses a type parameter without a class bound", func(t *testing.T) {
			t.Parallel()

			param := fixture(t, boxFile).Signature.TypeParams[0]
			assert.Equal(t, param.Name, "T", "T")
			assert.Length(t, param.Bounds, 1, "the interface bound alone")
			assert.Equal(t, param.Bounds[0].BinaryName(), comparableName, "Comparable<T>")
		})

		t.Run("parses a type parameter with a class bound", func(t *testing.T) {
			t.Parallel()

			fail := methodNamed(t, fixture(t, boxFile), "fail", "")
			assert.Equal(t, fail.Signature.TypeParams[0].Bounds[0].BinaryName(), exceptionName, "E extends Exception")
		})

		t.Run("parses a type parameter bounded by a type variable", func(t *testing.T) {
			t.Parallel()

			conv := methodNamed(t, fixture(t, boxFile), "conv", "")
			assert.Equal(t, conv.Signature.TypeParams[0].Bounds[0].Var, "T", "U extends T")
		})

		t.Run("parses a type parameter without a bound", func(t *testing.T) {
			t.Parallel()

			c, err := signedClass("<T:>" + objectDesc)
			assert.NoError(t, err, "an empty class bound and no interface bound")
			assert.Empty(t, c.Signature.TypeParams[0].Bounds, "T is bounded by nothing it states")
		})

		t.Run("parses a type parameter with an array class bound", func(t *testing.T) {
			t.Parallel()

			c, err := signedClass("<T:[I>" + objectDesc)
			assert.NoError(t, err, "the grammar admits any reference type as a class bound")
			assert.Equal(t, c.Signature.TypeParams[0].Bounds[0].Kind, classfile.KindArray, "int[]")
		})

		t.Run("parses a type parameter with two interface bounds", func(t *testing.T) {
			t.Parallel()

			c, err := signedClass("<T::" + runnableDesc + ":" + closeableDesc + ">" + objectDesc)
			assert.NoError(t, err, "T extends Runnable & AutoCloseable")
			assert.Length(t, c.Signature.TypeParams[0].Bounds, 2, "both interfaces")
		})
	})

	t.Run("parseClassSignature", func(t *testing.T) {
		t.Parallel()

		t.Run("parses a class signature's superclass", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fixture(t, boxFile).Signature.Super.BinaryName(), baseName, "Base<T>")
		})

		t.Run("parses a class signature's interfaces", func(t *testing.T) {
			t.Parallel()

			sig := fixture(t, boxFile).Signature
			assert.Equal(t, []string{sig.Interfaces[0].BinaryName(), sig.Interfaces[1].BinaryName()},
				[]string{holderName, cloneable}, "Holder<T> and Cloneable")
		})

		bad := []struct {
			name string
			give string
		}{
			{name: "returns ErrMalformed for an empty type parameter list", give: "<>" + objectDesc},
			{name: "returns ErrMalformed for a type parameter without a bound mark", give: "<T>" + objectDesc},
			{name: "returns ErrMalformed for an interface bound of a base type", give: "<T::I>" + objectDesc},
			{name: "returns ErrMalformed for a class bound that does not parse", give: "<T:L;>" + objectDesc},
			{name: "returns ErrMalformed for a superclass that is no class type", give: "TT;"},
			{name: "returns ErrMalformed for a superclass that does not parse", give: "Ljava/lang/Object"},
			{name: "returns ErrMalformed for an interface that is no class type", give: objectDesc + intDesc},
			{name: "returns ErrMalformed for an interface that is a type variable", give: objectDesc + "TT;"},
			{name: "returns ErrMalformed for an interface that does not parse", give: objectDesc + "L;"},
		}
		for _, tt := range bad {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := signedClass(tt.give)
				assert.ErrorIs(t, err, classfile.ErrMalformed, "the signature breaks the grammar")
			})
		}
	})

	t.Run("parseMethodSignature", func(t *testing.T) {
		t.Parallel()

		t.Run("parses a throws clause of a type variable", func(t *testing.T) {
			t.Parallel()

			fail := methodNamed(t, fixture(t, boxFile), "fail", "")
			assert.Equal(t, fail.Signature.Throws, []classfile.Type{{Kind: classfile.KindVar, Var: "E"}}, "throws E")
		})

		t.Run("parses a throws clause of two types", func(t *testing.T) {
			t.Parallel()

			c, err := signedMethod("()V^" + ioExceptionDesc + "^TE;")
			assert.NoError(t, err, "throws IOException, E")
			assert.Length(t, c.Methods[0].Signature.Throws, 2, "both")
		})

		bad := []struct {
			name string
			give string
		}{
			{name: "returns ErrMalformed for a method signature without parameters", give: "V"},
			{name: "returns ErrMalformed for type parameters that do not parse", give: "<T>()V"},
			{name: "returns ErrMalformed for a parameter that does not parse", give: "(Q)V"},
			{name: "returns ErrMalformed for a method signature without a result", give: "()"},
			{name: "returns ErrMalformed for a result that does not parse", give: "()Q"},
			{name: "returns ErrMalformed for a throws clause of a base type", give: "()V^I"},
			{name: "returns ErrMalformed for text after the method signature", give: "()VV"},
		}
		for _, tt := range bad {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := signedMethod(tt.give)
				assert.ErrorIs(t, err, classfile.ErrMalformed, "the signature breaks the grammar")
			})
		}
	})

	t.Run("parseFieldType", func(t *testing.T) {
		t.Parallel()

		bad := []struct {
			name string
			give string
		}{
			{name: "returns ErrMalformed for an empty signature", give: ""},
			{name: "returns ErrMalformed for an unknown type letter", give: "Q"},
			{name: "returns ErrMalformed for a class type without a name", give: "L;"},
			{name: "returns ErrMalformed for a class type without its end", give: "Ljava/util/List"},
			{name: "returns ErrMalformed for empty type arguments", give: "Ljava/util/List<>;"},
			{name: "returns ErrMalformed for type arguments without their end", give: listOpen + objectDesc},
			{name: "returns ErrMalformed for a wildcard without its type", give: "Ljava/util/List<+>;"},
			{name: "returns ErrMalformed for a suffix without a name", give: "Ljava/util/Map.;"},
			{name: "returns ErrMalformed for a class name with a byte no identifier has", give: "Ljava/util/List>;"},
			{name: "returns ErrMalformed for a suffix with a slash", give: "Ljava/util/Map.a/B;"},
			{name: "returns ErrMalformed for a name that runs on after its type arguments", give: listOpen + "TT;>x;"},
			{name: "returns ErrMalformed for a type variable without its end", give: "TT"},
			{name: "returns ErrMalformed for an array without its component", give: "["},
			{name: "returns ErrMalformed for text after the type", give: objectDesc + "X"},
		}
		for _, tt := range bad {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := signedField(tt.give)
				assert.ErrorIs(t, err, classfile.ErrMalformed, "the signature breaks the grammar")
			})
		}
	})
}

// A keyword and a top-level class's name allocate nothing, and a nested
// class's name its buffer. The ordinary run, which runs no benchmark,
// checks those ceilings here.
func TestSignatureAllocs(t *testing.T) {
	base := classfile.Type{Kind: classfile.KindBase, Base: 'I'}
	top := classfile.Type{Kind: classfile.KindClass, Class: []classfile.ClassName{{Name: mapName}}}
	nested := mapEntry()
	var got string
	assert.MaxAllocs(t, func() { got = base.Keyword() }, 0, "Keyword allocates nothing")
	assert.Equal(t, got, "int", "Keyword returns int")
	assert.MaxAllocs(t, func() { got = top.BinaryName() }, 0, "BinaryName allocates nothing for a top-level class")
	assert.Equal(t, got, mapName, "BinaryName returns the class's name")
	assert.MaxAllocs(t, func() { got = nested.BinaryName() }, nestedNameAllocs,
		"BinaryName allocates the joined name of a nested class")
	assert.Equal(t, got, mapName+"$Entry", "BinaryName joins the names")
}

// BenchmarkSignature measures the keyword and binary-name lookups the
// frontend makes for every type a signature names.
func BenchmarkSignature(b *testing.B) {
	b.Run("Keyword", func(b *testing.B) {
		typ := classfile.Type{Kind: classfile.KindBase, Base: 'I'}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = typ.Keyword()
		}
		assert.Equal(b, got, "int", "Keyword returns int")
	})

	b.Run("BinaryName", func(b *testing.B) {
		b.Run("a top-level class", func(b *testing.B) {
			typ := classfile.Type{Kind: classfile.KindClass, Class: []classfile.ClassName{{Name: mapName}}}
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got string
			for c.Loop() {
				got = typ.BinaryName()
			}
			assert.Equal(b, got, mapName, "BinaryName returns the class's name")
		})

		b.Run("a nested class", func(b *testing.B) {
			typ := mapEntry()
			c := bench.Start(b).MaxAllocs(nestedNameAllocs)
			defer c.End()
			var got string
			for c.Loop() {
				got = typ.BinaryName()
			}
			assert.Equal(b, got, mapName+"$Entry", "BinaryName joins the names")
		})
	})
}

// mapEntry returns the class type Map.Entry.
func mapEntry() classfile.Type {
	return classfile.Type{Kind: classfile.KindClass, Class: []classfile.ClassName{{Name: mapName}, {Name: "Entry"}}}
}

// fieldSignature returns the signature of a Box field of a name.
func fieldSignature(tb assert.TB, name string) classfile.Type {
	tb.Helper()

	f := fieldNamed(tb, fixture(tb, boxFile), name)
	assert.NotNil(tb, f.Signature, "the field has a signature")
	return *f.Signature
}

// signedField assembles a class with one Object field of a signature,
// and returns the parse's class and error.
func signedField(sig string) (*classfile.Class, error) {
	b := newClassBuilder()
	b.fields = append(b.fields, b.member(classfile.AccPublic, fieldLabel, objectDesc,
		b.attribute(attrSignature, u2(int(b.utf8(sig)))...)))
	return classfile.Parse(b.bytes())
}

// signedMethod assembles a class with one method of a signature, and
// returns the parse's class and error.
func signedMethod(sig string) (*classfile.Class, error) {
	b := newClassBuilder()
	b.methods = append(b.methods, b.member(classfile.AccPublic, methodLabel, voidMethod,
		b.attribute(attrSignature, u2(int(b.utf8(sig)))...)))
	return classfile.Parse(b.bytes())
}

// signedClass assembles a class of a class signature, and returns the
// parse's class and error.
func signedClass(sig string) (*classfile.Class, error) {
	b := newClassBuilder()
	b.attrs = append(b.attrs, b.attribute(attrSignature, u2(int(b.utf8(sig)))...))
	return classfile.Parse(b.bytes())
}
