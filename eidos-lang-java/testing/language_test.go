// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java"
	javafrontend "go.dokimi.dev/eidos/lang/java/frontend"
	javatesting "go.dokimi.dev/eidos/lang/java/testing"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/nodetest"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The tree that the Java frontend loads for the parity cases: a module with
// one class, whose fields have the forms that Java spells.
const (
	pomPath      = "pom.xml"
	pomSource    = "<project><groupId>com.acme</groupId><artifactId>fix</artifactId></project>"
	parityFile   = "src/main/java/fix/Row.java"
	paritySource = `package fix;

import java.util.List;

public class Row {
    public int[] a;
    public String[][] b;
    public List<String>[] c;
    public List<?> d;
    public List<? extends Number> e;
    public List<? super Integer> f;
}
`
)

// The names, the fields of Row and the types of the parity cases.
const (
	parityPackage = "fix"
	rowName       = "Row"
	intName       = "int"
	stringName    = "String"
	listName      = "List"
	numberName    = "Number"
	integerName   = "Integer"
	acmeSegment   = "acme"
	storeSegment  = "store"
	entryName     = "Entry"
	canonicalName = "acme.store.Entry"

	arrayField    = "a"
	nestedField   = "b"
	argsField     = "c"
	wildcardField = "d"
	extendsField  = "e"
	superField    = "f"
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
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the identity of Java", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, javatesting.Language{}.Lang(), java.Lang, "the language is Java")
		})
	})

	t.Run("Overloads", func(t *testing.T) {
		t.Parallel()

		t.Run("reports what the Java frontend reports", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, javatesting.Language{}.Overloads(), javafrontend.New(nil).Overloads(),
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
			{name: "writes an array type as the frontend records it", field: arrayField},
			{name: "writes an array type of an array type as the frontend records it", field: nestedField},
			{name: "writes an element with type arguments as the frontend records it", field: argsField},
			{name: "writes an unbounded wildcard as the frontend records it", field: wildcardField},
			{name: "writes an upper bound as the frontend records it", field: extendsField},
			{name: "writes a lower bound as the frontend records it", field: superField},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, got[tt.field], want[tt.field], "the adapter spells the type as the frontend does")
			})
		}

		t.Run("returns an error for a bounded wildcard without a variance", func(t *testing.T) {
			t.Parallel()

			wildcard := nodetest.Wildcard(symbol.VarianceInvariant, nodetest.Named(numberName))
			_, err := javatesting.Language{}.Spell(wildcard)
			assert.HasError(t, err, "a bound of Java follows extends or super")
		})

		t.Run("returns an error for a map", func(t *testing.T) {
			t.Parallel()

			_, err := javatesting.Language{}.Spell(nodetest.Map(nodetest.Named(stringName), nodetest.Named(intName)))
			assert.HasError(t, err, "Java has no map type")
		})
	})

	t.Run("Qualify", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the canonical name of the declaration", func(t *testing.T) {
			t.Parallel()

			got := javatesting.Language{}.Qualify(&node.Package{Path: []string{acmeSegment, storeSegment}}, entryName)
			assert.Equal(t, got, canonicalName, "the name follows the parts of its package")
		})

		t.Run("returns the name alone in the unnamed package", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, javatesting.Language{}.Qualify(&node.Package{}, entryName), entryName,
				"a declaration of the unnamed package has its name alone")
		})
	})
}

// frontendFields loads the parity source through the Java frontend and
// returns the shape of the type of each field of Row, by name.
func frontendFields(tb testing.TB) map[string]refShape {
	tb.Helper()

	fx := rulestest.Loaded(tb, javafrontend.New(nil), fstest.MapFS{
		pomPath:    {Data: []byte(pomSource)},
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
// field of Row, by name.
func builtFields(tb testing.TB) map[string]refShape {
	tb.Helper()

	b := nodetest.New(javatesting.Language{})
	row := &node.Struct{Name: rowName, Fields: []*node.Field{
		{Name: arrayField, Type: nodetest.List(nodetest.Named(intName))},
		{Name: nestedField, Type: nodetest.List(nodetest.List(nodetest.Named(stringName)))},
		{Name: argsField, Type: nodetest.List(nodetest.Named(listName, nodetest.Named(stringName)))},
		{Name: wildcardField, Type: nodetest.Named(listName, nodetest.Wildcard(symbol.VarianceInvariant, nil))},
		{Name: extendsField, Type: nodetest.Named(listName,
			nodetest.Wildcard(symbol.VarianceOut, nodetest.Named(numberName)))},
		{Name: superField, Type: nodetest.Named(listName,
			nodetest.Wildcard(symbol.VarianceIn, nodetest.Named(integerName)))},
	}}
	b.File(parityPackage, parityFile, row)
	b.Build(tb)
	return fieldShapes(row)
}

// fieldShapes returns the shape of the type of each field of s, by name.
func fieldShapes(s *node.Struct) map[string]refShape {
	out := make(map[string]refShape, len(s.Fields))
	for _, f := range s.Fields {
		out[f.Name] = shapeOf(f.Type)
	}
	return out
}

// shapeOf returns the shape of a reference and of its children and its
// arguments.
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
	return shape
}
