// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spoke_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/spoke"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The source spellings of the shapes in the cases, the spelling that the
// fixture spoke returns for text, and the language that the read-only
// cases name.
const (
	textSpelling   = "string"
	numberSpelling = "number"
	spokeSpelling  = "spelled"
	goName         = "Go"
)

// childrenAllocs is the list that Children returns for two shapes that
// the fixture spoke spells without allocating.
const childrenAllocs = 1

// errRefused is the fixture spoke's refusal of a number.
var errRefused = errors.New("spoke_test: the target has no number")

// spelled is the reference that the fixture spoke returns for text.
var spelled = &emit.TypeRef{Spelling: spokeSpelling}

// textOnly is the fixture spoke. It spells text as spelled, and it
// refuses every other shape with errRefused.
func textOnly(s rules.TypeShape) (*emit.TypeRef, error) {
	if s.Form != symbol.FormText {
		return nil, errRefused
	}
	return spelled, nil
}

// TestSpoke checks the noun phrase of each form, the refusal of a child,
// which starts with the source spelling of the child, and the refusal of
// a read-only shape.
func TestSpoke(t *testing.T) {
	t.Parallel()

	t.Run("Describe", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.TypeShape
			want string
		}{
			{name: "returns a named type for the named form", give: rules.TypeShape{}, want: "a named type"},
			{
				name: "returns an optional for the optional form",
				give: rules.TypeShape{Form: symbol.FormOptional}, want: "an optional",
			},
			{name: "returns a list for the list form", give: rules.TypeShape{Form: symbol.FormList}, want: "a list"},
			{
				name: "returns an array for the array form",
				give: rules.TypeShape{Form: symbol.FormArray},
				want: "an array",
			},
			{name: "returns a map for the map form", give: rules.TypeShape{Form: symbol.FormMap}, want: "a map"},
			{
				name: "returns a function type for the function form",
				give: rules.TypeShape{Form: symbol.FormFunc}, want: "a function type",
			},
			{
				name: "returns a tuple for the tuple form",
				give: rules.TypeShape{Form: symbol.FormTuple},
				want: "a tuple",
			},
			{
				name: "returns a union for the union form",
				give: rules.TypeShape{Form: symbol.FormUnion},
				want: "a union",
			},
			{
				name: "returns an intersection for the intersection form",
				give: rules.TypeShape{Form: symbol.FormIntersection}, want: "an intersection",
			},
			{
				name: "returns a synchronous stream for a stream that is not async",
				give: rules.TypeShape{Form: symbol.FormStream}, want: "a synchronous stream",
			},
			{
				name: "returns an asynchronous stream for an async stream",
				give: rules.TypeShape{Form: symbol.FormStream, Async: true}, want: "an asynchronous stream",
			},
			{
				name: "returns a borrow for the borrow form",
				give: rules.TypeShape{Form: symbol.FormBorrow},
				want: "a borrow",
			},
			{
				name: "returns a wildcard for the wildcard form",
				give: rules.TypeShape{Form: symbol.FormWildcard}, want: "a wildcard",
			},
			{
				name: "returns an inline type for the inline form",
				give: rules.TypeShape{Form: symbol.FormInline}, want: "an inline type",
			},
			{
				name: "returns a scalar for the scalar form",
				give: rules.Scalar(numberSpelling, rules.ScalarFloat, 64),
				want: "a scalar",
			},
			{name: "returns a boolean for Bool", give: rules.Leaf(symbol.FormBool, "bool"), want: "a boolean"},
			{name: "returns text for Text", give: rules.Leaf(symbol.FormText, textSpelling), want: "text"},
			{name: "returns bytes for Bytes", give: rules.Leaf(symbol.FormBytes, "bytes"), want: "bytes"},
			{
				name: "returns a reference for the reference form",
				give: rules.TypeShape{Form: symbol.FormReference}, want: "a reference",
			},
			{name: "returns a sum for the sum form", give: rules.TypeShape{Form: symbol.FormSum}, want: "a sum"},
			{
				name: "returns the description of an unclassified type for the opaque form",
				give: rules.Opaque(nil), want: "a type that the rules of its language do not classify",
			},
			{
				name: "returns the top type for the dynamic form",
				give: rules.Leaf(symbol.FormDynamic, "unknown"), want: "the top type",
			},
			{
				name: "returns the number of a form that the vocabulary does not declare",
				give: rules.TypeShape{Form: symbol.TypeForm(200)}, want: "the form 200",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, spoke.Describe(tt.give), tt.want, "the form's noun phrase")
			})
		}
	})

	t.Run("Children", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the spoke's references in the order of the shapes", func(t *testing.T) {
			t.Parallel()

			got, err := spoke.Children([]rules.TypeShape{
				rules.Leaf(symbol.FormText, textSpelling), rules.Leaf(symbol.FormText, "str"),
			}, textOnly)
			assert.NoError(t, err, "the spoke spells every shape")
			assert.Length(t, got, 2, "one reference for each shape")
			expect.Equal(t, got[0], spelled, "the first reference is the spoke's", assert.ByIdentity())
			expect.Equal(t, got[1], spelled, "the second reference is the spoke's", assert.ByIdentity())
		})

		t.Run("returns nil for no shapes", func(t *testing.T) {
			t.Parallel()

			got, err := spoke.Children(nil, textOnly)
			assert.NoError(t, err, "no shape has nothing to refuse")
			assert.Nil(t, got, "no shape has no reference")
		})

		t.Run("returns the spoke's refusal behind the spelling of the shape that it refuses", func(t *testing.T) {
			t.Parallel()

			_, err := spoke.Children([]rules.TypeShape{
				rules.Leaf(symbol.FormText, textSpelling), rules.Scalar(numberSpelling, rules.ScalarFloat, 64),
			}, textOnly)
			assert.ErrorIs(t, err, errRefused, "the error wraps the spoke's refusal")
			assert.Equal(t, err.Error(), numberSpelling+": "+errRefused.Error(),
				"the error starts with the spelling of the shape")
		})
	})

	t.Run("Child", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the spoke's reference for the child", func(t *testing.T) {
			t.Parallel()

			got, err := spoke.Child(rules.Leaf(symbol.FormText, textSpelling), textOnly)
			assert.NoError(t, err, "the spoke spells the child")
			assert.Equal(t, got, spelled, "the reference is the spoke's", assert.ByIdentity())
		})

		t.Run("returns the spoke's refusal behind the spelling of the child", func(t *testing.T) {
			t.Parallel()

			got, err := spoke.Child(rules.Scalar(numberSpelling, rules.ScalarFloat, 64), textOnly)
			assert.ErrorIs(t, err, errRefused, "the error wraps the spoke's refusal")
			assert.Equal(t, err.Error(), numberSpelling+": "+errRefused.Error(),
				"the error starts with the spelling of the child")
			assert.Nil(t, got, "a refusal returns no reference")
		})
	})

	t.Run("Arity", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a shape of the stated number of children", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, spoke.Arity(listOf(1), 1), "a list of one child passes")
		})

		tests := []struct {
			name  string
			give  rules.TypeShape
			arity int
			want  string
		}{
			{
				name: "returns an error that counts one child in words", give: listOf(2), arity: 1,
				want: "a list takes one child, and the shape has 2",
			},
			{
				name: "returns an error that counts two children in words",
				give: rules.TypeShape{Form: symbol.FormMap, Elems: listOf(1).Elems}, arity: 2,
				want: "a map takes two children, and the shape has 1",
			},
			{
				name: "returns an error that counts three children in digits", give: listOf(1), arity: 3,
				want: "a list takes 3 children, and the shape has 1",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := spoke.Arity(tt.give, tt.arity)
				assert.HasError(t, err, "a shape of another number of children fails")
				assert.Equal(t, err.Error(), tt.want, "the error names the form and both numbers")
			})
		}
	})

	t.Run("Split", func(t *testing.T) {
		t.Parallel()

		two := listOf(2).Elems
		passed := []struct {
			name string
			give int
		}{
			{name: "returns nil for a split before the first child", give: 0},
			{name: "returns nil for a split after the last child", give: 2},
		}
		for _, tt := range passed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.NoError(t, spoke.Split(rules.TypeShape{Form: symbol.FormFunc, Elems: two, Split: tt.give}),
					"a split inside the children passes")
			})
		}

		failed := []struct {
			name string
			give int
			want string
		}{
			{
				name: "returns an error for a split below zero", give: -1,
				want: "a function type splits its 2 children at -1",
			},
			{
				name: "returns an error for a split past the last child", give: 3,
				want: "a function type splits its 2 children at 3",
			},
		}
		for _, tt := range failed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := spoke.Split(rules.TypeShape{Form: symbol.FormFunc, Elems: two, Split: tt.give})
				assert.HasError(t, err, "a split outside the children fails")
				assert.Equal(t, err.Error(), tt.want, "the error names the children and the split")
			})
		}
	})

	t.Run("ReadOnly", func(t *testing.T) {
		t.Parallel()

		passed := []struct {
			name string
			give symbol.Mutability
		}{
			{name: "returns nil for a mutable shape", give: symbol.MutabilityMutable},
			{name: "returns nil for a shape without a mutability", give: symbol.MutabilityUnknown},
		}
		for _, tt := range passed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.NoError(t, spoke.ReadOnly(rules.TypeShape{Form: symbol.FormList, Mutability: tt.give}, goName),
					"a writable spelling keeps the facts of the shape")
			})
		}

		failed := []struct {
			name string
			give symbol.TypeForm
			want string
		}{
			{
				name: "returns an error for an immutable list", give: symbol.FormList,
				want: "a read-only list has no Go spelling",
			},
			{
				name: "returns an error for an immutable array", give: symbol.FormArray,
				want: "a read-only array has no Go spelling",
			},
		}
		for _, tt := range failed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := spoke.ReadOnly(rules.TypeShape{Form: tt.give, Mutability: symbol.MutabilityImmutable}, goName)
				assert.HasError(t, err, "a writable spelling drops the constraint of the shape")
				assert.Equal(t, err.Error(), tt.want, "the error names the form and the language")
			})
		}
	})
}

// Describe allocates nothing for a declared form, and Children allocates
// the list of references. The ordinary run, which runs no benchmark,
// checks those ceilings here.
func TestSpokeAllocs(t *testing.T) {
	stream := rules.TypeShape{Form: symbol.FormStream, Async: true}
	var described string
	assert.MaxAllocs(t, func() { described = spoke.Describe(stream) }, 0, "Describe allocates nothing")
	assert.Equal(t, described, "an asynchronous stream", "Describe returns the phrase of the stream")

	shapes := []rules.TypeShape{rules.Leaf(symbol.FormText, textSpelling), rules.Leaf(symbol.FormText, textSpelling)}
	var (
		got []*emit.TypeRef
		err error
	)
	assert.MaxAllocs(t, func() { got, err = spoke.Children(shapes, textOnly) }, childrenAllocs,
		"Children allocates the list of references")
	assert.NoError(t, err, "Children spells the shapes")
	assert.Length(t, got, 2, "Children returns one reference for each shape")

	var child *emit.TypeRef
	assert.MaxAllocs(t, func() { child, err = spoke.Child(shapes[0], textOnly) }, 0,
		"Child allocates nothing beyond the spoke")
	assert.NoError(t, err, "Child spells the shape")
	assert.Equal(t, child, spelled, "Child returns the spoke's reference", assert.ByIdentity())

	list, fn := listOf(1), rules.TypeShape{Form: symbol.FormFunc, Elems: listOf(2).Elems, Split: 1}
	assert.MaxAllocs(t, func() { err = spoke.Arity(list, 1) }, 0, "Arity allocates nothing for a valid shape")
	assert.NoError(t, err, "Arity passes a list of one child")
	assert.MaxAllocs(t, func() { err = spoke.Split(fn) }, 0, "Split allocates nothing for a valid split")
	assert.NoError(t, err, "Split passes a split inside the children")
	assert.MaxAllocs(t, func() { err = spoke.ReadOnly(list, goName) }, 0,
		"ReadOnly allocates nothing for a shape that is not immutable")
	assert.NoError(t, err, "ReadOnly passes a list without a mutability")
}

// BenchmarkSpoke measures the description of a form, which every refusal
// writes, and the spelling of a list of children, which every
// structural form of a translation makes.
func BenchmarkSpoke(b *testing.B) {
	b.Run("Describe", func(b *testing.B) {
		union := rules.TypeShape{Form: symbol.FormUnion}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = spoke.Describe(union)
		}
		assert.Equal(b, got, "a union", "Describe returns the phrase of the union")
	})

	b.Run("Children", func(b *testing.B) {
		shapes := []rules.TypeShape{
			rules.Leaf(symbol.FormText, textSpelling),
			rules.Leaf(symbol.FormText, textSpelling),
		}
		c := bench.Start(b).MaxAllocs(childrenAllocs)
		defer c.End()
		var (
			got []*emit.TypeRef
			err error
		)
		for c.Loop() {
			got, err = spoke.Children(shapes, textOnly)
		}
		assert.NoError(b, err, "Children spells the shapes")
		assert.Length(b, got, 2, "Children returns one reference for each shape")
	})

	b.Run("Child", func(b *testing.B) {
		shape := rules.Leaf(symbol.FormText, textSpelling)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			got *emit.TypeRef
			err error
		)
		for c.Loop() {
			got, err = spoke.Child(shape, textOnly)
		}
		assert.NoError(b, err, "Child spells the shape")
		assert.Equal(b, got, spelled, "Child returns the spoke's reference", assert.ByIdentity())
	})

	b.Run("Arity", func(b *testing.B) {
		list := listOf(1)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var err error
		for c.Loop() {
			err = spoke.Arity(list, 1)
		}
		assert.NoError(b, err, "Arity passes a list of one child")
	})

	b.Run("Split", func(b *testing.B) {
		fn := rules.TypeShape{Form: symbol.FormFunc, Elems: listOf(2).Elems, Split: 1}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var err error
		for c.Loop() {
			err = spoke.Split(fn)
		}
		assert.NoError(b, err, "Split passes a split inside the children")
	})

	b.Run("ReadOnly", func(b *testing.B) {
		list := rules.TypeShape{Form: symbol.FormList, Mutability: symbol.MutabilityMutable}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var err error
		for c.Loop() {
			err = spoke.ReadOnly(list, goName)
		}
		assert.NoError(b, err, "ReadOnly passes a mutable list")
	})
}

// listOf returns a list of n children of text.
func listOf(n int) rules.TypeShape {
	s := rules.TypeShape{Form: symbol.FormList}
	for range n {
		s.Elems = append(s.Elems, rules.Leaf(symbol.FormText, textSpelling))
	}
	return s
}
