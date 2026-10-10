// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
)

// A binding decodes its list, its index and its doc, and its list is a
// closed list.
func TestBinding(t *testing.T) {
	t.Parallel()

	t.Run("Binding", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes every section of a binding", func(t *testing.T) {
			t.Parallel()

			got := decodeAs[specfront.Binding](t, "{from: result, index: 1, doc: a binding}\n")
			assert.Equal(t, got, specfront.Binding{From: specfront.SourceResult, Index: 1, Doc: bindingDoc},
				"the binding has every section of the text")
		})
	})

	t.Run("Source", func(t *testing.T) {
		t.Parallel()

		t.Run("UnmarshalYAML", func(t *testing.T) {
			t.Parallel()

			for _, want := range []specfront.Source{specfront.SourceInput, specfront.SourceResult} {
				t.Run("decodes "+string(want), func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, decodeAs[specfront.Source](t, string(want)), want, "the source decodes")
				})
			}

			t.Run("returns an error for another list", func(t *testing.T) {
				t.Parallel()

				var got specfront.Source
				typed := assert.ErrorAs[*yaml.TypeError](t, yaml.Unmarshal([]byte("output"), &got),
					"the decoder reports the fault as a type error")
				assert.Equal(
					t,
					typed.Errors,
					[]string{`line 1: "output" is not the source of a binding: write "input" or "result"`},
					"the entry lists the sources",
				)
			})
		})

		t.Run("JSONSchema", func(t *testing.T) {
			t.Parallel()

			t.Run("returns an enum of the sources", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, specfront.SourceInput.JSONSchema(), map[string]any{
					keywordType: typeString,
					keywordEnum: []any{string(specfront.SourceInput), string(specfront.SourceResult)},
				}, "the schema lists the sources in order")
			})
		})
	})
}
