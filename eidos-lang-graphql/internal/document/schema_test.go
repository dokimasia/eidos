// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/graphql/internal/document"
)

// loadAllocs is the load of the schema of the allocation cases: the list of
// the definitions of the merged document, a copy of each of the 13
// definitions of the prelude, and the 37 allocations of the validator.
const loadAllocs = 1 + 13 + 37

// The sources of the schemas of the load cases: an extension of the root of
// queries, a reference to an undeclared type, and an extension of a scalar
// of the prelude.
const (
	queryExtension  = "extend type Query {\n  b: Int\n}\n"
	undeclaredType  = "type A {\n  a: Missing\n}\n"
	stringExtension = "extend scalar String @specifiedBy(url: \"https://example.com/text\")\n"
)

// TestSchema pins the load of a schema over the prelude of gqlparser.
func TestSchema(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a schema with the built-in scalars of the prelude", func(t *testing.T) {
			t.Parallel()

			schema, err := document.Load(nil)
			assert.NoError(t, err, "the prelude loads")
			assert.NotNil(t, schema.Types["String"], "the schema declares the scalar String")
		})

		t.Run("returns the types of the documents of the type system", func(t *testing.T) {
			t.Parallel()

			schema, err := document.Load([]*ast.SchemaDocument{parsedSchema(t, querySchema)})
			assert.NoError(t, err, "the schema loads")
			assert.Equal(t, schema.Query.Name, "Query", "the type Query is the root of queries")
		})

		t.Run("returns the fields of an extension in the type that it extends", func(t *testing.T) {
			t.Parallel()

			schema, err := document.Load([]*ast.SchemaDocument{
				parsedSchema(t, querySchema), parsedSchema(t, queryExtension),
			})
			assert.NoError(t, err, "the schema loads")
			expect.NotNil(t, schema.Query.Fields.ForName("a"), "the type declares a")
			expect.NotNil(t, schema.Query.Fields.ForName("b"), "the extension adds b")
		})

		t.Run("returns the error of the validator for a schema that does not load", func(t *testing.T) {
			t.Parallel()

			_, err := document.Load([]*ast.SchemaDocument{parsedSchema(t, undeclaredType)})
			assert.HasError(t, err, "the validator refuses the reference")
			assert.Contains(t, err.Error(), "Undefined type Missing", "the error states the type")
		})

		t.Run("returns a scalar of the prelude without the extension of an earlier load", func(t *testing.T) {
			t.Parallel()

			extended, err := document.Load([]*ast.SchemaDocument{parsedSchema(t, stringExtension)})
			assert.NoError(t, err, "the extension of String loads")
			assert.Length(t, extended.Types["String"].Directives, 1, "the extension applies its directive")
			plain, err := document.Load(nil)
			assert.NoError(t, err, "the prelude loads again")
			assert.Empty(t, plain.Types["String"].Directives, "the later load copies the prelude anew")
		})
	})
}

// Load copies the prelude and validates the merged document. A document
// loads once, so each call loads a document of its own, which the setup
// parses outside the count. The ordinary run, which runs no benchmark,
// checks that ceiling here.
func TestSchemaAllocs(t *testing.T) {
	var (
		schema *ast.Schema
		err    error
	)
	assert.MaxAllocsWithSetup(t, func() []*ast.SchemaDocument {
		return []*ast.SchemaDocument{parsedSchema(t, querySchema)}
	}, func(docs []*ast.SchemaDocument) {
		schema, err = document.Load(docs)
	}, loadAllocs, "Load allocates the copies of the prelude and the validation")
	assert.NoError(t, err, "Load loads the schema")
	assert.NotNil(t, schema.Query, "Load returns the root of queries")
}

// BenchmarkSchema measures the load of the schema of every unit. A document
// loads once, so each iteration loads a document of its own, which the
// contract parses outside the count.
func BenchmarkSchema(b *testing.B) {
	b.Run("Load", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(loadAllocs)
		defer c.End()
		var (
			got *ast.Schema
			err error
		)
		for c.Loop() {
			var docs []*ast.SchemaDocument
			c.Excluding(func() { docs = []*ast.SchemaDocument{parsedSchema(b, querySchema)} })
			got, err = document.Load(docs)
		}
		assert.NoError(b, err, "Load loads the schema")
		assert.NotNil(b, got.Query, "Load returns the root of queries")
	})
}
