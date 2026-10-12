// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript"
	tsfrontend "go.dokimi.dev/eidos/lang/typescript/frontend"
	tstesting "go.dokimi.dev/eidos/lang/typescript/testing"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/nodetest"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The tree that the TypeScript frontend loads for the parity cases: one
// file, whose interface Row has one property of each form that TypeScript
// spells.
const (
	parityFile   = "row.ts"
	paritySource = `export interface Box<T> { v: T; }
export interface Foo { a: string; }
export interface Bar { b: string; }
export interface Row {
  l: string[];
  o: string | undefined;
  t: [string, number];
  u: string | number;
  i: Foo & Bar;
  f: (p0: string, p1: number) => boolean;
  m: { [key: string]: number };
  b: { a: string; m(): void };
  g: Box<string>[];
  p: (string | number)[];
  rl: readonly string[];
  rt: readonly [string, number];
  rm: { readonly [key: string]: number };
  n: string | null;
}
`
)

// The names, the properties of Row and the builtins of the parity cases.
const (
	parityPackage = "fix"
	rowName       = "Row"
	boxName       = "Box"
	fooName       = "Foo"
	barName       = "Bar"
	paramName     = "T"
	stringName    = "string"
	numberName    = "number"
	booleanName   = "boolean"
	voidName      = "void"
	undefinedName = "undefined"
	nullName      = "null"
	propA         = "a"
	propB         = "b"
	propV         = "v"
	methodM       = "m"
	cacheName     = "cache"
	entryName     = "Entry"

	listProp      = "l"
	optionalProp  = "o"
	tupleProp     = "t"
	unionProp     = "u"
	intersectProp = "i"
	funcProp      = "f"
	mapProp       = "m"
	bodyProp      = "b"
	argsProp      = "g"
	groupProp     = "p"
	roListProp    = "rl"
	roTupleProp   = "rt"
	roMapProp     = "rm"
	nullProp      = "n"
)

// refShape is what the parity cases compare of a reference: everything but
// its position, its target and its package. Absent contains the spellings
// of the absent members of an optional.
type refShape struct {
	Spelling   string
	Form       symbol.TypeForm
	Variance   symbol.Variance
	Mutability symbol.Mutability
	Split      int
	Length     int
	Elems      []refShape
	Args       []refShape
	Absent     []string
	Members    []string
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the identity of TypeScript", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tstesting.Language{}.Lang(), typescript.Lang, "the language is TypeScript")
		})
	})

	t.Run("Overloads", func(t *testing.T) {
		t.Parallel()

		t.Run("reports what the TypeScript frontend reports", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tstesting.Language{}.Overloads(), tsfrontend.New().Overloads(),
				"the adapter overloads as the frontend does")
		})
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		want := frontendProps(t)
		got := builtProps(t)
		tests := []struct {
			name string
			prop string
		}{
			{name: "writes a list as the frontend records it", prop: listProp},
			{name: "writes an optional as the frontend records it", prop: optionalProp},
			{name: "writes a tuple as the frontend records it", prop: tupleProp},
			{name: "writes a union as the frontend records it", prop: unionProp},
			{name: "writes an intersection as the frontend records it", prop: intersectProp},
			{name: "writes a function type as the frontend records it", prop: funcProp},
			{name: "writes a map as the frontend records it", prop: mapProp},
			{name: "writes an inline body as the frontend records it", prop: bodyProp},
			{name: "writes a child with type arguments as the frontend records it", prop: argsProp},
			{name: "writes a union in a list in parentheses as the frontend records it", prop: groupProp},
			{name: "writes an immutable list as the frontend records it", prop: roListProp},
			{name: "writes an immutable tuple as the frontend records it", prop: roTupleProp},
			{name: "writes an immutable map as the frontend records it", prop: roMapProp},
			{name: "writes an optional of null as the frontend records it", prop: nullProp},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, got[tt.prop], want[tt.prop], "the adapter spells the type as the frontend does")
			})
		}

		t.Run("returns an error for a function type without a result", func(t *testing.T) {
			t.Parallel()

			_, err := tstesting.Language{}.Spell(nodetest.Func([]*node.TypeRef{nodetest.Named(stringName)}, nil))
			assert.HasError(t, err, "a TypeScript function type has one return type")
		})

		t.Run("returns an error for an array of a fixed length", func(t *testing.T) {
			t.Parallel()

			_, err := tstesting.Language{}.Spell(nodetest.Array(3, nodetest.Named(stringName)))
			assert.HasError(t, err, "TypeScript has no array type of a fixed length")
		})
	})

	t.Run("Qualify", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name alone", func(t *testing.T) {
			t.Parallel()

			got := tstesting.Language{}.Qualify(&node.Package{Name: cacheName}, entryName)
			assert.Equal(t, got, entryName, "an import binds the name without its module")
		})
	})
}

// frontendProps loads the parity source through the TypeScript frontend
// and returns the shape of the type of each property of Row, by name.
func frontendProps(tb testing.TB) map[string]refShape {
	tb.Helper()

	fx := rulestest.Loaded(tb, tsfrontend.New(), fstest.MapFS{parityFile: {Data: []byte(paritySource)}})
	var row *node.Interface
	for p := range fx.Graph.Packages() {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				if i, is := d.(*node.Interface); is && i.Name == rowName {
					row = i
				}
			}
		}
	}
	assert.NotNil(tb, row, "the load of the parity source declares Row")
	return propShapes(row)
}

// builtProps builds the declarations of the parity source through
// nodetest with the adapter, and returns the shape of the type of each
// property of Row, by name.
func builtProps(tb testing.TB) map[string]refShape {
	tb.Helper()

	b := nodetest.New(tstesting.Language{})
	box := &node.Interface{
		Name:       boxName,
		TypeParams: []*node.TypeParam{{Name: paramName}},
		Fields:     []*node.Field{{Name: propV, Type: nodetest.Named(paramName)}},
	}
	foo := &node.Interface{Name: fooName, Fields: []*node.Field{{Name: propA, Type: nodetest.Named(stringName)}}}
	bar := &node.Interface{Name: barName, Fields: []*node.Field{{Name: propB, Type: nodetest.Named(stringName)}}}
	row := &node.Interface{Name: rowName, Fields: []*node.Field{
		{Name: listProp, Type: mutable(symbol.MutabilityMutable, nodetest.List(nodetest.Named(stringName)))},
		{Name: optionalProp, Type: absent(nodetest.Optional(nodetest.Named(stringName)), undefinedName)},
		{Name: tupleProp, Type: mutable(symbol.MutabilityMutable,
			nodetest.Tuple(nodetest.Named(stringName), nodetest.Named(numberName)))},
		{Name: unionProp, Type: nodetest.Union(nodetest.Named(stringName), nodetest.Named(numberName))},
		{Name: intersectProp, Type: nodetest.Intersection(b.Ref(foo), b.Ref(bar))},
		{Name: funcProp, Type: nodetest.Func(
			[]*node.TypeRef{nodetest.Named(stringName), nodetest.Named(numberName)},
			[]*node.TypeRef{nodetest.Named(booleanName)},
		)},
		{Name: mapProp, Type: mutable(symbol.MutabilityMutable,
			nodetest.Map(nodetest.Named(stringName), nodetest.Named(numberName)))},
		{Name: bodyProp, Type: nodetest.Inline(
			[]*node.Field{{Name: propA, Type: nodetest.Named(stringName)}},
			[]*node.Method{{Name: methodM, Returns: []*node.Return{{Type: nodetest.Named(voidName)}}}},
		)},
		{Name: argsProp, Type: mutable(symbol.MutabilityMutable,
			nodetest.List(b.Ref(box, nodetest.Named(stringName))))},
		{Name: groupProp, Type: mutable(symbol.MutabilityMutable,
			nodetest.List(nodetest.Union(nodetest.Named(stringName), nodetest.Named(numberName))))},
		{Name: roListProp, Type: mutable(symbol.MutabilityImmutable, nodetest.List(nodetest.Named(stringName)))},
		{Name: roTupleProp, Type: mutable(symbol.MutabilityImmutable,
			nodetest.Tuple(nodetest.Named(stringName), nodetest.Named(numberName)))},
		{Name: roMapProp, Type: mutable(symbol.MutabilityImmutable,
			nodetest.Map(nodetest.Named(stringName), nodetest.Named(numberName)))},
		{Name: nullProp, Type: absent(nodetest.Optional(nodetest.Named(stringName)), nullName)},
	}}
	b.File(parityPackage, parityFile, box, foo, bar, row)
	b.Build(tb)
	return propShapes(row)
}

// mutable sets the mutability of the structural reference ref to m, as
// the TypeScript frontend sets it, and returns ref.
func mutable(m symbol.Mutability, ref *node.TypeRef) *node.TypeRef {
	ref.Mutability = m
	return ref
}

// absent sets the absent members of the optional ref to the named types
// of names, as the TypeScript frontend sets them, and returns ref.
func absent(ref *node.TypeRef, names ...string) *node.TypeRef {
	for _, name := range names {
		ref.Absent = append(ref.Absent, nodetest.Named(name))
	}
	return ref
}

// propShapes returns the shape of the type of each property of i, by name.
func propShapes(i *node.Interface) map[string]refShape {
	out := make(map[string]refShape, len(i.Fields))
	for _, f := range i.Fields {
		out[f.Name] = shapeOf(f.Type)
	}
	return out
}

// shapeOf returns the shape of a reference and of its children, its
// arguments and the members of an inline body.
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
	for _, a := range ref.Absent {
		shape.Absent = append(shape.Absent, a.Spelling)
	}
	for _, f := range ref.Fields {
		shape.Members = append(shape.Members, f.Name)
	}
	for _, m := range ref.Methods {
		shape.Members = append(shape.Members, m.Name)
	}
	return shape
}
