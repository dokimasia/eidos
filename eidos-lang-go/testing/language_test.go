// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/nodetest"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The tree that the Go frontend loads for the parity cases: a module with
// one file, whose struct Row has one field of each form that Go spells.
const (
	parityModule = "example.test/fix"
	parityFile   = "row.go"
	modPath      = "go.mod"
	modSource    = "module " + parityModule + "\n"
	paritySource = `package fix

type Box[T any] struct{ V T }

type Row struct {
	P *int
	S []string
	A [3]int
	M map[string]int
	C chan int
	F func(int, string) (bool, error)
	I struct{ A int }
	J interface{ M() }
	L []Box[int]
}
`
)

// The names, the fields of Row and the builtins of the parity cases, and
// the qualifier of a reference to another package.
const (
	rowName    = "Row"
	boxName    = "Box"
	paramName  = "T"
	anyName    = "any"
	intName    = "int"
	stringName = "string"
	boolName   = "bool"
	errorName  = "error"
	fieldA     = "A"
	methodM    = "M"
	fieldV     = "V"
	cacheName  = "cache"
	entryName  = "Entry"
	qualifier  = "."

	pointerField   = "P"
	sliceField     = "S"
	arrayField     = "A"
	mapField       = "M"
	channelField   = "C"
	funcField      = "F"
	structField    = "I"
	interfaceField = "J"
	argsField      = "L"
)

// refShape is what the parity cases compare of a reference: everything but
// its position, its target and its package.
type refShape struct {
	Spelling string
	Form     symbol.TypeForm
	Variance symbol.Variance
	Split    int
	Length   int
	Elems    []refShape
	Args     []refShape
	Members  []string
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the identity of Go", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, gotesting.Language{}.Lang(), golang.Lang, "the language is Go")
		})
	})

	t.Run("Overloads", func(t *testing.T) {
		t.Parallel()

		t.Run("reports what the Go frontend reports", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, gotesting.Language{}.Overloads(), gofrontend.New(nil).Overloads(),
				"the adapter overloads as the frontend does")
		})
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		want := frontendFields(t)
		got := builtFields(t)
		tests := []struct {
			name  string
			field string
		}{
			{name: "writes a pointer as the frontend records it", field: pointerField},
			{name: "writes a slice as the frontend records it", field: sliceField},
			{name: "writes an array as the frontend records it", field: arrayField},
			{name: "writes a map as the frontend records it", field: mapField},
			{name: "writes a channel as the frontend records it", field: channelField},
			{name: "writes a function type as the frontend records it", field: funcField},
			{name: "writes an inline struct as the frontend records it", field: structField},
			{name: "writes an inline interface as the frontend records it", field: interfaceField},
			{name: "writes a child with type arguments as the frontend records it", field: argsField},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, got[tt.field], want[tt.field], "the adapter spells the type as the frontend does")
			})
		}

		t.Run("returns an error for a tuple", func(t *testing.T) {
			t.Parallel()

			_, err := gotesting.Language{}.Spell(nodetest.Tuple(nodetest.Named(intName)))
			assert.HasError(t, err, "Go has no tuple type")
		})

		t.Run("returns an error for an inline body with fields and methods", func(t *testing.T) {
			t.Parallel()

			_, err := gotesting.Language{}.Spell(nodetest.Inline(
				[]*node.Field{{Name: fieldA, Type: nodetest.Named(intName)}}, []*node.Method{{Name: methodM}},
			))
			assert.HasError(t, err, "an inline body of Go is a struct or an interface")
		})
	})

	t.Run("Qualify", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the name of the package before the name", func(t *testing.T) {
			t.Parallel()

			got := gotesting.Language{}.Qualify(&node.Package{Name: cacheName}, entryName)
			assert.Equal(t, got, cacheName+qualifier+entryName, "a qualified name follows the name of its package")
		})
	})
}

// frontendFields loads the parity source through the Go frontend and
// returns the shape of the type of each field of Row, by field name.
func frontendFields(tb testing.TB) map[string]refShape {
	tb.Helper()

	fx := rulestest.Loaded(tb, gofrontend.New(nil), fstest.MapFS{
		modPath:    {Data: []byte(modSource)},
		parityFile: {Data: []byte(paritySource)},
	})
	var row *node.Struct
	for p := range fx.Graph.Packages() {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				if s, is := d.(*node.Struct); is && s.Name == rowName {
					row = s
				}
			}
		}
	}
	assert.NotNil(tb, row, "the load of the parity source declares Row")
	return fieldShapes(row)
}

// builtFields builds the declarations of the parity source through
// nodetest with the adapter, and returns the shape of the type of each
// field of Row, by field name.
func builtFields(tb testing.TB) map[string]refShape {
	tb.Helper()

	b := nodetest.New(gotesting.Language{})
	box := &node.Struct{
		Name:       boxName,
		TypeParams: []*node.TypeParam{{Name: paramName, Bounds: []*node.TypeRef{nodetest.Named(anyName)}}},
		Fields:     []*node.Field{{Name: fieldV, Type: nodetest.Named(paramName)}},
	}
	row := &node.Struct{Name: rowName, Fields: []*node.Field{
		{Name: pointerField, Type: nodetest.Optional(nodetest.Named(intName))},
		{Name: sliceField, Type: nodetest.List(nodetest.Named(stringName))},
		{Name: arrayField, Type: nodetest.Array(3, nodetest.Named(intName))},
		{Name: mapField, Type: nodetest.Map(nodetest.Named(stringName), nodetest.Named(intName))},
		{Name: channelField, Type: nodetest.Stream(nodetest.Named(intName))},
		{Name: funcField, Type: nodetest.Func(
			[]*node.TypeRef{nodetest.Named(intName), nodetest.Named(stringName)},
			[]*node.TypeRef{nodetest.Named(boolName), nodetest.Named(errorName)},
		)},
		{Name: structField, Type: nodetest.Inline([]*node.Field{{Name: fieldA, Type: nodetest.Named(intName)}}, nil)},
		{Name: interfaceField, Type: nodetest.Inline(nil, []*node.Method{{Name: methodM}})},
		{Name: argsField, Type: nodetest.List(b.Ref(box, nodetest.Named(intName)))},
	}}
	b.File(parityModule, parityFile, box, row)
	b.Build(tb)
	return fieldShapes(row)
}

// fieldShapes returns the shape of the type of each field of s, by field
// name.
func fieldShapes(s *node.Struct) map[string]refShape {
	out := make(map[string]refShape, len(s.Fields))
	for _, f := range s.Fields {
		out[f.Name] = shapeOf(f.Type)
	}
	return out
}

// shapeOf returns the shape of a reference and of its children, its
// arguments and the members of an inline body.
func shapeOf(ref *node.TypeRef) refShape {
	shape := refShape{
		Spelling: ref.Spelling,
		Form:     ref.Form,
		Variance: ref.Variance,
		Split:    ref.Split,
		Length:   ref.Length,
	}
	for _, e := range ref.Elems {
		shape.Elems = append(shape.Elems, shapeOf(e))
	}
	for _, a := range ref.Args {
		shape.Args = append(shape.Args, shapeOf(a))
	}
	for _, f := range ref.Fields {
		shape.Members = append(shape.Members, f.Name)
	}
	for _, m := range ref.Methods {
		shape.Members = append(shape.Members, m.Name)
	}
	return shape
}
