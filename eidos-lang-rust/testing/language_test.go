// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust"
	rustfrontend "go.dokimi.dev/eidos/lang/rust/frontend"
	rusttesting "go.dokimi.dev/eidos/lang/rust/testing"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/nodetest"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The tree that the Rust frontend loads for the parity cases: a crate whose
// function shapes has one parameter of each form that Rust spells.
const (
	manifestPath   = "Cargo.toml"
	manifestSource = "[package]\nname = \"fix\"\n"
	parityFile     = "src/lib.rs"
	paritySource   = `pub trait Left {}
pub trait Right {}
pub struct Boxed<T> {
    pub v: T,
}
pub fn shapes(a: &u8, b: [u8; 4], c: &[u8], d: (u8, bool), e: fn(u8) -> bool, g: Boxed<dyn Left + Right>, h: (u8,), m: &mut u8, n: &mut [u8], f: fn(u8), p: &Boxed<u8>) {}
`
)

// The names, the parameters of shapes and the types of the parity cases.
const (
	parityPackage = "fix"
	shapesName    = "shapes"
	boxedName     = "Boxed"
	leftName      = "Left"
	rightName     = "Right"
	paramName     = "T"
	fieldV        = "v"
	byteName      = "u8"
	boolName      = "bool"
	crateSegment  = "fix"
	modSegment    = "store"
	entryName     = "Entry"
	pathName      = "fix::store::Entry"

	borrowParam    = "a"
	arrayParam     = "b"
	sliceParam     = "c"
	tupleParam     = "d"
	funcParam      = "e"
	dynParam       = "g"
	singletonParam = "h"
	mutParam       = "m"
	mutSliceParam  = "n"
	procParam      = "f"
	genericParam   = "p"
)

// refShape is what the parity cases compare of a reference: everything but
// its position, its target and its package.
type refShape struct {
	Spelling   string
	Form       symbol.TypeForm
	Variance   symbol.Variance
	Mutability symbol.Mutability
	Split      int
	Length     int
	Elems      []refShape
	Args       []refShape
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the identity of Rust", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rusttesting.Language{}.Lang(), rust.Lang, "the language is Rust")
		})
	})

	t.Run("Overloads", func(t *testing.T) {
		t.Parallel()

		t.Run("reports what the Rust frontend reports", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rusttesting.Language{}.Overloads(), rustfrontend.New(nil).Overloads(),
				"the adapter overloads as the frontend does")
		})
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		want := frontendParams(t)
		got := builtParams(t)
		tests := []struct {
			name  string
			param string
		}{
			{name: "writes a borrow as the frontend records it", param: borrowParam},
			{name: "writes an array as the frontend records it", param: arrayParam},
			{name: "writes a borrowed slice as the frontend records it", param: sliceParam},
			{name: "writes a tuple as the frontend records it", param: tupleParam},
			{name: "writes a function pointer as the frontend records it", param: funcParam},
			{name: "writes a trait object as the frontend records it", param: dynParam},
			{name: "writes a tuple of one member as the frontend records it", param: singletonParam},
			{name: "writes a mutable borrow as the frontend records it", param: mutParam},
			{name: "writes a mutable borrow of a slice as the frontend records it", param: mutSliceParam},
			{name: "writes a function pointer without a result as the frontend records it", param: procParam},
			{name: "writes a borrow of a generic type as the frontend records it", param: genericParam},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, got[tt.param], want[tt.param], "the adapter spells the type as the frontend does")
			})
		}

		t.Run("returns an error for a tuple without members", func(t *testing.T) {
			t.Parallel()

			_, err := rusttesting.Language{}.Spell(nodetest.Tuple())
			assert.HasError(t, err, "the unit type of Rust is a named type")
		})

		t.Run("returns an error for an optional", func(t *testing.T) {
			t.Parallel()

			_, err := rusttesting.Language{}.Spell(nodetest.Optional(nodetest.Named(byteName)))
			assert.HasError(t, err, "Rust has no optional type syntax")
		})

		t.Run("returns an error for a function type of two results", func(t *testing.T) {
			t.Parallel()

			_, err := rusttesting.Language{}.Spell(nodetest.Func(
				[]*node.TypeRef{nodetest.Named(byteName)},
				[]*node.TypeRef{nodetest.Named(byteName), nodetest.Named(boolName)},
			))
			assert.HasError(t, err, "a function type of Rust has one return type at most")
		})

		t.Run("writes a child that the reference does not state as nothing", func(t *testing.T) {
			t.Parallel()

			got, err := rusttesting.Language{}.Spell(&node.TypeRef{
				Form: symbol.FormBorrow, Elems: []*node.TypeRef{nil},
			})
			assert.NoError(t, err, "a borrow spells")
			assert.Equal(t, got, "&", "the borrow of no child")
		})
	})

	t.Run("Qualify", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the path of the module before the name", func(t *testing.T) {
			t.Parallel()

			got := rusttesting.Language{}.Qualify(&node.Package{Path: []string{crateSegment, modSegment}}, entryName)
			assert.Equal(t, got, pathName, "the name follows the path of its module")
		})

		t.Run("returns the name alone for a module without a path", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rusttesting.Language{}.Qualify(&node.Package{}, entryName), entryName,
				"a declaration of a module without a path has its name alone")
		})
	})
}

// frontendParams loads the parity source through the Rust frontend and
// returns the shape of the type of each parameter of shapes, by name.
func frontendParams(tb testing.TB) map[string]refShape {
	tb.Helper()

	fx := rulestest.Loaded(tb, rustfrontend.New(nil), fstest.MapFS{
		manifestPath: {Data: []byte(manifestSource)},
		parityFile:   {Data: []byte(paritySource)},
	})
	var shapes *node.Function
	for p := range fx.Graph.Packages() {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				if fn, is := d.(*node.Function); is && fn.Name == shapesName {
					shapes = fn
				}
			}
		}
	}
	assert.NotNil(tb, shapes, "the load of the parity source declares shapes")
	return paramShapes(shapes)
}

// builtParams builds the declarations of the parity source through
// nodetest with the adapter, and returns the shape of the type of each
// parameter of shapes, by name.
func builtParams(tb testing.TB) map[string]refShape {
	tb.Helper()

	b := nodetest.New(rusttesting.Language{})
	left, right := &node.Interface{Name: leftName}, &node.Interface{Name: rightName}
	boxed := &node.Struct{
		Name:       boxedName,
		TypeParams: []*node.TypeParam{{Name: paramName}},
		Fields:     []*node.Field{{Name: fieldV, Type: nodetest.Named(paramName)}},
	}
	shapes := &node.Function{Name: shapesName, Params: []*node.Param{
		{Name: borrowParam, Type: borrowOf(symbol.MutabilityImmutable, nodetest.Named(byteName))},
		{Name: arrayParam, Type: nodetest.Array(4, nodetest.Named(byteName))},
		{Name: sliceParam, Type: borrowOf(symbol.MutabilityImmutable, nodetest.List(nodetest.Named(byteName)))},
		{Name: tupleParam, Type: nodetest.Tuple(nodetest.Named(byteName), nodetest.Named(boolName))},
		{Name: funcParam, Type: nodetest.Func(
			[]*node.TypeRef{nodetest.Named(byteName)}, []*node.TypeRef{nodetest.Named(boolName)},
		)},
		{Name: dynParam, Type: b.Ref(boxed, nodetest.Intersection(b.Ref(left), b.Ref(right)))},
		{Name: singletonParam, Type: nodetest.Tuple(nodetest.Named(byteName))},
		{Name: mutParam, Type: borrowOf(symbol.MutabilityMutable, nodetest.Named(byteName))},
		{Name: mutSliceParam, Type: borrowOf(symbol.MutabilityMutable, nodetest.List(nodetest.Named(byteName)))},
		{Name: procParam, Type: nodetest.Func([]*node.TypeRef{nodetest.Named(byteName)}, nil)},
		{Name: genericParam, Type: borrowOf(symbol.MutabilityImmutable, b.Ref(boxed, nodetest.Named(byteName)))},
	}}
	b.File(parityPackage, parityFile, left, right, boxed, shapes)
	b.Build(tb)
	return paramShapes(shapes)
}

// borrowOf returns a borrow of ref with the mutability m, as the Rust
// frontend lowers &T and &mut T.
func borrowOf(m symbol.Mutability, ref *node.TypeRef) *node.TypeRef {
	borrow := nodetest.Borrow(ref)
	borrow.Mutability = m
	return borrow
}

// paramShapes returns the shape of the type of each parameter of fn, by
// name.
func paramShapes(fn *node.Function) map[string]refShape {
	out := make(map[string]refShape, len(fn.Params))
	for _, p := range fn.Params {
		out[p.Name] = shapeOf(p.Type)
	}
	return out
}

// shapeOf returns the shape of a reference and of its children and its
// arguments.
func shapeOf(ref *node.TypeRef) refShape {
	shape := refShape{
		Spelling:   ref.Spelling,
		Form:       ref.Form,
		Variance:   ref.Variance,
		Mutability: ref.Mutability,
		Split:      ref.Split,
		Length:     ref.Length,
	}
	for _, e := range ref.Elems {
		shape.Elems = append(shape.Elems, shapeOf(e))
	}
	for _, a := range ref.Args {
		shape.Args = append(shape.Args, shapeOf(a))
	}
	return shape
}
