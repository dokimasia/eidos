// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
)

// The keywords and the values of the root of the published schema.
const (
	keywordSchema     = "$schema"
	keywordTitle      = "title"
	keywordOneOf      = "oneOf"
	keywordRequired   = "required"
	keywordAdditional = "additionalProperties"
	draft             = "https://json-schema.org/draft/2020-12/schema"
	title             = "A spec of the shape catalog: a shape, a mixin or a contract"
	lineBreak         = '\n'
)

// The sections that one form alone has.
const (
	sectionPrecedence = "precedence"
	sectionRoles      = "roles"
)

// formChoices are the choices of the schema root in the order of the
// forms, shape, mixin and contract, with the names of their properties.
type formChoices struct {
	OneOf []struct {
		Properties map[string]json.RawMessage `json:"properties"`
	} `json:"oneOf"`
}

// The schema root is a oneOf of the three documents, whose closed objects
// require the sections that the decoder requires.
func TestSchema(t *testing.T) {
	t.Parallel()

	t.Run("Schema", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a schema of draft 2020-12 with one choice for each form", func(t *testing.T) {
			t.Parallel()

			var root map[string]any
			assert.NoError(t, json.Unmarshal(specfront.Schema(), &root), "the schema is JSON")
			expect.Equal(t, root[keywordSchema], any(draft), "the schema states its draft")
			expect.Equal(t, root[keywordTitle], any(title), "the schema states its title")
			choices, is := root[keywordOneOf].([]any)
			assert.True(t, is, "the schema has a list of choices")
			assert.Length(t, choices, 3, "the schema has a choice for each form")
		})

		t.Run("returns closed objects that require the sections of each form", func(t *testing.T) {
			t.Parallel()

			var root struct {
				OneOf []struct {
					Required   []string `json:"required"`
					Additional bool     `json:"additionalProperties"`
				} `json:"oneOf"`
			}
			assert.NoError(t, json.Unmarshal(specfront.Schema(), &root), "the schema is JSON")
			assert.Length(t, root.OneOf, 3, "the schema has a choice for each form")
			common := []string{"name", "form", "claim", "observation", "falsifiability", "counterexamples"}
			wants := [][]string{
				common,
				common,
				{"name", "form", "claim", "observation", "roles", "falsifiability", "counterexamples"},
			}
			for i, want := range wants {
				expect.Equal(t, root.OneOf[i].Required, want, "the choice requires the sections of its form")
				expect.False(t, root.OneOf[i].Additional, "the choice refuses an unknown section")
			}
		})

		t.Run("returns the precedence in the choice of a shape alone", func(t *testing.T) {
			t.Parallel()

			var root formChoices
			assert.NoError(t, json.Unmarshal(specfront.Schema(), &root), "the schema is JSON")
			assert.Length(t, root.OneOf, 3, "the schema has a choice for each form")
			for i, want := range []bool{true, false, false} {
				_, has := root.OneOf[i].Properties[sectionPrecedence]
				expect.Equal(t, has, want, "the choice has the precedence where its form is shape")
			}
		})

		t.Run("returns the roles in the choice of a contract alone", func(t *testing.T) {
			t.Parallel()

			var root formChoices
			assert.NoError(t, json.Unmarshal(specfront.Schema(), &root), "the schema is JSON")
			assert.Length(t, root.OneOf, 3, "the schema has a choice for each form")
			for i, want := range []bool{false, false, true} {
				_, has := root.OneOf[i].Properties[sectionRoles]
				expect.Equal(t, has, want, "the choice has the roles where its form is contract")
			}
		})

		t.Run("returns the same bytes on each call", func(t *testing.T) {
			t.Parallel()

			assert.Deterministic(
				t,
				func(struct{}) (string, error) { return string(specfront.Schema()), nil },
				struct{}{},
				"the encoder orders the keys of each object",
			)
		})

		t.Run("returns a text that ends with a line break", func(t *testing.T) {
			t.Parallel()

			got := specfront.Schema()
			assert.Equal(t, got[len(got)-1], byte(lineBreak), "the file ends with a line break")
		})
	})
}
