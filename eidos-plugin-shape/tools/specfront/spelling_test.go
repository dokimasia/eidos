// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
)

// The keywords of JSON Schema that a vocabulary type writes.
const (
	keywordType  = "type"
	keywordEnum  = "enum"
	keywordConst = "const"
	typeString   = "string"
)

// A vocabulary type decodes one spelling of a closed list, refuses any
// other at the line of the value, and publishes the list as its schema.
// These cases state the rules through the arity and the form of a shape,
// and the files of the other types check their own lists.
func TestSpelling(t *testing.T) {
	t.Parallel()

	t.Run("Arity", func(t *testing.T) {
		t.Parallel()

		t.Run("UnmarshalYAML", func(t *testing.T) {
			t.Parallel()

			t.Run("decodes a spelling of the list", func(t *testing.T) {
				t.Parallel()

				var got struct {
					Arity specfront.Arity `yaml:"arity"`
				}
				assert.NoError(t, yaml.Unmarshal([]byte("arity: many\n"), &got), "the arity decodes")
				assert.Equal(t, got.Arity, specfront.ArityMany, "the arity is many")
			})

			t.Run("returns an error at the line of the value that lists the spellings", func(t *testing.T) {
				t.Parallel()

				var got struct {
					Arity specfront.Arity `yaml:"arity"`
				}
				err := yaml.Unmarshal([]byte("\narity: some\n"), &got)
				typed := assert.ErrorAs[*yaml.TypeError](t, err, "the decoder reports the fault as a type error")
				assert.Equal(
					t,
					typed.Errors,
					[]string{`line 2: "some" is not an arity: write "one" or "optional" or "many" or "any"`},
					"the entry has the line of the value and every spelling",
				)
			})

			t.Run("returns the error of the decoder for a value that is no text", func(t *testing.T) {
				t.Parallel()

				var got struct {
					Arity specfront.Arity `yaml:"arity"`
				}
				err := yaml.Unmarshal([]byte("arity: [one]\n"), &got)
				typed := assert.ErrorAs[*yaml.TypeError](t, err, "the decoder reports the fault as a type error")
				assert.Equal(t, typed.Errors, []string{"line 1: cannot unmarshal !!seq into string"},
					"the entry is the decoder's own")
			})
		})

		t.Run("JSONSchema", func(t *testing.T) {
			t.Parallel()

			t.Run("returns a string enum for two spellings or more", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, specfront.ArityOne.JSONSchema(), map[string]any{
					keywordType: typeString,
					keywordEnum: []any{
						string(specfront.ArityOne), string(specfront.ArityOptional),
						string(specfront.ArityMany), string(specfront.ArityAny),
					},
				}, "the schema lists the arities in order")
			})
		})
	})

	t.Run("ShapeForm", func(t *testing.T) {
		t.Parallel()

		t.Run("JSONSchema", func(t *testing.T) {
			t.Parallel()

			t.Run("returns a string constant for one spelling", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, specfront.FormShape.JSONSchema(), map[string]any{
					keywordType: typeString, keywordConst: string(specfront.FormShape),
				}, "the schema is the one form")
			})
		})
	})
}
