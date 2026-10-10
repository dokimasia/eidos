// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile_test

import (
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/frontend/classfile"
)

// The element value tags the assembler writes, which pin the tags the
// reader decodes (§4.7.16.1).
const (
	valueInt        = 'I'
	valueClass      = 'c'
	valueAnnotation = '@'
	valueArray      = '['
	valueUnknown    = 'x'
)

// The name of the element the assembled values are on, the binary name
// of the fixtures' run-time invisible annotation interface, and a
// descriptor of a class type without a name.
const (
	elementName  = "v"
	specName     = "com/acme/lib/Spec"
	namelessDesc = "L;"
)

// An annotation's element values are spelled as source spells them, so
// the spelling of each kind of value, and the bounds on their nesting,
// is pinned.
func TestAnnotation(t *testing.T) {
	t.Parallel()

	t.Run("annotations", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes a run-time invisible annotation", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spec(t).Type, specName, "Spec has the class retention")
		})
	})

	t.Run("elementValue", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "spells a byte value", give: "b", want: "1"},
			{name: "spells a char value", give: "c", want: "'q'"},
			{name: "spells a double value", give: "d", want: "2.5"},
			{name: "spells a float value", give: "f", want: "1.25f"},
			{name: "spells an int value", give: "i", want: "42"},
			{name: "spells a long value", give: "j", want: "7L"},
			{name: "spells a short value", give: "s", want: "3"},
			{name: "spells a boolean value", give: "z", want: "true"},
			{name: "spells a string value", give: "str", want: `"s\t"`},
			{
				name: "spells an enum value by its type's binary name", give: "e",
				want: "java.lang.annotation.RetentionPolicy.SOURCE",
			},
			{name: "spells a class value of an array type", give: "cls", want: "java.lang.String[].class"},
			{
				name: "spells a nested annotation with its pairs", give: "ann",
				want: `@com.acme.lib.Marker(value = "m", codes = {1, 2})`,
			},
			{name: "spells an empty array value", give: "arr", want: "{}"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				elements := spec(t).Elements
				i := slices.IndexFunc(elements, func(e classfile.Element) bool { return e.Name == tt.give })
				assert.NotEqual(t, i, -1, "Specced states the element")
				assert.Equal(t, elements[i].Value, tt.want, "the value as source spells it")
			})
		}

		t.Run("returns ErrMalformed for an element value of an unknown tag", func(t *testing.T) {
			t.Parallel()

			_, err := annotated(func(*classBuilder) []byte { return []byte{valueUnknown} })
			assert.ErrorIs(t, err, classfile.ErrMalformed, "no element value has that tag")
		})

		t.Run("decodes array values nested 64 deep", func(t *testing.T) {
			t.Parallel()

			_, err := annotated(nestedArrays(63))
			assert.NoError(t, err, "the cap admits 64 levels")
		})

		t.Run("returns ErrMalformed for array values nested deeper than 64", func(t *testing.T) {
			t.Parallel()

			_, err := annotated(nestedArrays(64))
			assert.ErrorIs(t, err, classfile.ErrMalformed, "the 65th level is past the cap")
		})
	})

	t.Run("classValue", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "spells void's class literal", give: "V", want: "void.class"},
			{name: "spells a base type's class literal", give: "I", want: "int.class"},
			{
				name: "spells a class's class literal by its binary name", give: "Ljava/util/Map$Entry;",
				want: "java.util.Map$Entry.class",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				a, err := annotated(func(b *classBuilder) []byte {
					return slices.Concat([]byte{valueClass}, u2(int(b.utf8(tt.give))))
				})
				assert.NoError(t, err, "the value decodes")
				assert.Equal(t, a.Elements[0].Value, tt.want, "the class literal")
			})
		}
	})

	t.Run("spellAnnotation", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a nested annotation without pairs without parentheses", func(t *testing.T) {
			t.Parallel()

			a, err := annotated(func(b *classBuilder) []byte {
				return slices.Concat([]byte{valueAnnotation}, u2(int(b.utf8(annotationA))), u2(0))
			})
			assert.NoError(t, err, "the value decodes")
			assert.Equal(t, a.Elements[0].Value, "@"+nameA, "a marker annotation")
		})
	})

	t.Run("annotation", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes an int value", func(t *testing.T) {
			t.Parallel()

			a, err := annotated(func(b *classBuilder) []byte {
				return slices.Concat([]byte{valueInt}, u2(int(b.constant(tagInteger, u4(9)...))))
			})
			assert.NoError(t, err, "the value decodes")
			assert.Equal(t, a, classfile.Annotation{
				Type: nameA, Elements: []classfile.Element{{Name: elementName, Value: "9"}},
			}, "the annotation's type and its one pair")
		})

		t.Run("returns ErrMalformed for an annotation type that does not parse", func(t *testing.T) {
			t.Parallel()

			b := newClassBuilder()
			b.attrs = append(b.attrs, b.attribute(attrVisible, b.marker(namelessDesc)...))
			_, err := classfile.Parse(b.bytes())
			assert.ErrorIs(t, err, classfile.ErrMalformed, "the type is a field descriptor")
		})
	})
}

// spec returns the Spec annotation on the fixture Specced.
func spec(tb assert.TB) classfile.Annotation {
	tb.Helper()

	c := fixture(tb, speccedFile)
	assert.Length(tb, c.Annotations, 1, "Specced has Spec alone")
	return c.Annotations[0]
}

// annotated assembles a class with one run-time visible annotation A of
// one element v, whose value the function writes, and returns the
// decoded annotation and the parse's error.
func annotated(value func(*classBuilder) []byte) (classfile.Annotation, error) {
	b := newClassBuilder()
	body := slices.Concat(u2(1), u2(int(b.utf8(annotationA))), u2(1), u2(int(b.utf8(elementName))), value(b))
	b.attrs = append(b.attrs, b.attribute(attrVisible, body...))
	c, err := classfile.Parse(b.bytes())
	if err != nil {
		return classfile.Annotation{}, err
	}
	return c.Annotations[0], nil
}

// nestedArrays returns a value of arrays nested a number of levels deep
// around one empty array.
func nestedArrays(levels int) func(*classBuilder) []byte {
	return func(*classBuilder) []byte {
		one := slices.Concat([]byte{valueArray}, u2(1))
		return []byte(strings.Repeat(string(one), levels) + string(slices.Concat([]byte{valueArray}, u2(0))))
	}
}
