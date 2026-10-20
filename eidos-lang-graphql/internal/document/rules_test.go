// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"testing"

	"github.com/dokimasia/gqlparser/v2/ast"
	"github.com/dokimasia/gqlparser/v2/parser"
	"github.com/dokimasia/gqlparser/v2/validator"
	validatorrules "github.com/dokimasia/gqlparser/v2/validator/rules"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/graphql/internal/document"
)

// rulesAllocs is a set of rules: the set and its map, and the 12 growths of
// the map as gqlparser adds its default rules.
const rulesAllocs = 2 + 12

// The documents of the validation cases against the schema of querySchema:
// two anonymous operations, a fragment that no operation spreads, a variable
// that no field reads, a directive that the schema does not define, a field
// that the schema does not declare, and two operations of one name.
const (
	twoAnonymous     = "{ a }\n{ a }\n"
	unusedFragment   = "query Q { a }\nfragment F on Query { a }\n"
	unusedVariable   = "query Q($v: Int) { a }\n"
	unknownDirective = "query Q @cached { a }\n"
	unknownField     = "query Q { b }\n"
	repeatedNames    = "query Q { a }\nquery Q { a }\n"
)

// TestRules pins the rules of the validation of the documents of a client
// and the message of the rule of a lone anonymous operation.
func TestRules(t *testing.T) {
	t.Parallel()

	t.Run("Rules", func(t *testing.T) {
		t.Parallel()

		accepted := []struct {
			name string
			give string
		}{
			{name: "returns rules that accept two anonymous operations of one document", give: twoAnonymous},
			{name: "returns rules that accept a fragment that no operation spreads", give: unusedFragment},
			{name: "returns rules that accept a variable that no field reads", give: unusedVariable},
			{name: "returns rules that accept a directive that the schema does not define", give: unknownDirective},
		}
		for _, tt := range accepted {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				errs := validator.ValidateWithRules(loadedQuery(t), parsedQuery(t, tt.give), document.Rules())
				assert.Empty(t, errs, "a client validates the document")
			})
		}

		t.Run("returns rules that refuse a field that the schema does not declare", func(t *testing.T) {
			t.Parallel()

			errs := validator.ValidateWithRules(loadedQuery(t), parsedQuery(t, unknownField), document.Rules())
			assert.Length(t, errs, 1, "the default rules refuse the field")
		})

		t.Run("returns rules that refuse an operation name that two operations declare", func(t *testing.T) {
			t.Parallel()

			unique := validatorrules.NewDefaultRules()
			want := validator.ValidateWithRules(loadedQuery(t), parsedQuery(t, repeatedNames), unique)
			assert.Length(t, want, 1, "the rule of gqlparser refuses the second operation")
			got := validator.ValidateWithRules(loadedQuery(t), parsedQuery(t, repeatedNames), document.Rules())
			assert.Length(t, got, 1, "the rules of a client refuse the second operation")
			expect.Equal(t, got[0].Message, want[0].Message, "the message of the rule of gqlparser")
			expect.Equal(t, got[0].Locations, want[0].Locations, "the location of the second operation")
		})

		t.Run("returns a set of the caller's own", func(t *testing.T) {
			t.Parallel()

			document.Rules().RemoveRule(validatorrules.FieldsOnCorrectTypeRule.Name)
			errs := validator.ValidateWithRules(loadedQuery(t), parsedQuery(t, unknownField), document.Rules())
			assert.Length(t, errs, 1, "a change of one set leaves the next set unchanged")
		})
	})

	t.Run("LoneAnonymous", func(t *testing.T) {
		t.Parallel()

		t.Run("is the message of the rule of gqlparser", func(t *testing.T) {
			t.Parallel()

			rules := validatorrules.NewDefaultRules()
			errs := validator.ValidateWithRules(loadedQuery(t), parsedQuery(t, twoAnonymous), rules)
			assert.NotEmpty(t, errs, "the default rules refuse the two anonymous operations")
			assert.Equal(t, errs[0].Message, document.LoneAnonymous, "the message of the rule")
		})
	})
}

// Rules builds the default rules of gqlparser and removes four of them. The
// ordinary run, which runs no benchmark, checks that ceiling here.
func TestRulesAllocs(t *testing.T) {
	var got *validatorrules.Rules
	assert.MaxAllocs(t, func() { got = document.Rules() }, rulesAllocs, "Rules allocates the set of rules")
	assert.NotNil(t, got, "Rules returns the set")
}

// BenchmarkRules measures the rules that the validation of every unit
// builds.
func BenchmarkRules(b *testing.B) {
	b.Run("Rules", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(rulesAllocs)
		defer c.End()
		var got *validatorrules.Rules
		for c.Loop() {
			got = document.Rules()
		}
		assert.NotNil(b, got, "Rules returns the set")
	})
}

// loadedQuery returns the schema of querySchema over the prelude.
func loadedQuery(tb assert.TB) *ast.Schema {
	tb.Helper()

	schema, err := document.Load([]*ast.SchemaDocument{parsedSchema(tb, querySchema)})
	assert.NoError(tb, err, "the schema loads")
	return schema
}

// parsedQuery returns the executable document text, which gqlparser parses.
func parsedQuery(tb assert.TB, text string) *ast.QueryDocument {
	tb.Helper()

	doc, err := parser.ParseQuery(&ast.Source{Name: fileName, Input: text})
	assert.NoError(tb, err, "the document parses")
	return doc
}
