// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
)

// The form of a spec has one spelling for each directory, so a spec in
// the wrong directory refuses its form.
func TestForm(t *testing.T) {
	t.Parallel()

	t.Run("ShapeForm", func(t *testing.T) {
		t.Parallel()

		t.Run("UnmarshalYAML", func(t *testing.T) {
			t.Parallel()

			t.Run("decodes the form shape", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, decodeAs[specfront.ShapeForm](t, string(specfront.FormShape)), specfront.FormShape,
					"the form decodes")
			})

			t.Run("returns an error for another form", func(t *testing.T) {
				t.Parallel()

				var got specfront.ShapeForm
				typed := assert.ErrorAs[*yaml.TypeError](t, yaml.Unmarshal([]byte(specfront.FormMixin), &got),
					"the decoder reports the fault as a type error")
				assert.Equal(
					t,
					typed.Errors,
					[]string{`line 1: "mixin" is not the form of a spec in spec/shapes: write "shape"`},
					"the entry names the one form of the directory",
				)
			})
		})
	})

	t.Run("MixinForm", func(t *testing.T) {
		t.Parallel()

		t.Run("UnmarshalYAML", func(t *testing.T) {
			t.Parallel()

			t.Run("decodes the form mixin", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, decodeAs[specfront.MixinForm](t, string(specfront.FormMixin)), specfront.FormMixin,
					"the form decodes")
			})

			t.Run("returns an error for another form", func(t *testing.T) {
				t.Parallel()

				var got specfront.MixinForm
				typed := assert.ErrorAs[*yaml.TypeError](t, yaml.Unmarshal([]byte(specfront.FormShape), &got),
					"the decoder reports the fault as a type error")
				assert.Equal(
					t,
					typed.Errors,
					[]string{`line 1: "shape" is not the form of a spec in spec/mixins: write "mixin"`},
					"the entry names the one form of the directory",
				)
			})
		})

		t.Run("JSONSchema", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the constant mixin", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, specfront.FormMixin.JSONSchema(), map[string]any{
					keywordType: typeString, keywordConst: string(specfront.FormMixin),
				}, "the schema is the one form")
			})
		})
	})

	t.Run("ContractForm", func(t *testing.T) {
		t.Parallel()

		t.Run("UnmarshalYAML", func(t *testing.T) {
			t.Parallel()

			t.Run("decodes the form contract", func(t *testing.T) {
				t.Parallel()

				assert.Equal(
					t,
					decodeAs[specfront.ContractForm](t, string(specfront.FormContract)),
					specfront.FormContract,
					"the form decodes",
				)
			})

			t.Run("returns an error for another form", func(t *testing.T) {
				t.Parallel()

				var got specfront.ContractForm
				typed := assert.ErrorAs[*yaml.TypeError](t, yaml.Unmarshal([]byte(specfront.FormShape), &got),
					"the decoder reports the fault as a type error")
				assert.Equal(
					t,
					typed.Errors,
					[]string{`line 1: "shape" is not the form of a spec in spec/contracts: write "contract"`},
					"the entry names the one form of the directory",
				)
			})
		})

		t.Run("JSONSchema", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the constant contract", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, specfront.FormContract.JSONSchema(), map[string]any{
					keywordType: typeString, keywordConst: string(specfront.FormContract),
				}, "the schema is the one form")
			})
		})
	})
}
