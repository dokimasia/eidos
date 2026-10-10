// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
)

// minimum is the least value of the int param of the fixture.
const minimum int64 = 2

// A param decodes every section that a spec writes for it, and its type
// and its resolution kind are closed lists.
func TestParam(t *testing.T) {
	t.Parallel()

	t.Run("Param", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes every section of a param", func(t *testing.T) {
			t.Parallel()

			got := decodeAs[specfront.Param](t, "{key: limit, type: int, resolve: host-param, required: true, "+
				"counterexample: true, roles: [begin], minimum: 2, excludes: [mode], also_on: [read], doc: a param}\n")
			least := minimum
			assert.Equal(t, got, specfront.Param{
				Key: limitKey, Type: specfront.TypeInt, Resolve: specfront.ResolveHostParam, Required: true,
				Counterexample: true, Roles: []specfront.Name{beginRole}, Minimum: &least,
				Excludes: []specfront.Name{modeKey}, AlsoOn: []specfront.Name{readKey}, Doc: paramDoc,
			}, "the param has every section of the text")
		})
	})

	t.Run("ParamType", func(t *testing.T) {
		t.Parallel()

		t.Run("UnmarshalYAML", func(t *testing.T) {
			t.Parallel()

			for _, want := range []specfront.ParamType{specfront.TypeString, specfront.TypeInt, specfront.TypeReference} {
				t.Run("decodes "+string(want), func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, decodeAs[specfront.ParamType](t, string(want)), want, "the param type decodes")
				})
			}

			t.Run("returns an error for a type that the catalog has no param of", func(t *testing.T) {
				t.Parallel()

				var got specfront.ParamType
				typed := assert.ErrorAs[*yaml.TypeError](t, yaml.Unmarshal([]byte("bool"), &got),
					"the decoder reports the fault as a type error")
				assert.Equal(
					t,
					typed.Errors,
					[]string{`line 1: "bool" is not a param type: write "string" or "int" or "reference"`},
					"the entry lists the param types",
				)
			})
		})

		t.Run("JSONSchema", func(t *testing.T) {
			t.Parallel()

			t.Run("returns an enum of the param types", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, specfront.TypeString.JSONSchema(), map[string]any{
					keywordType: typeString,
					keywordEnum: []any{
						string(specfront.TypeString),
						string(specfront.TypeInt),
						string(specfront.TypeReference),
					},
				}, "the schema lists the param types in order")
			})
		})
	})

	t.Run("Resolution", func(t *testing.T) {
		t.Parallel()

		resolutions := []specfront.Resolution{
			specfront.ResolveCallableInScope, specfront.ResolvePackageVar, specfront.ResolveValueField,
			specfront.ResolveHostParam, specfront.ResolveMemberOnHandle, specfront.ResolveTypeInScope,
		}

		t.Run("UnmarshalYAML", func(t *testing.T) {
			t.Parallel()

			for _, want := range resolutions {
				t.Run("decodes "+string(want), func(t *testing.T) {
					t.Parallel()

					assert.Equal(
						t,
						decodeAs[specfront.Resolution](t, string(want)),
						want,
						"the resolution kind decodes",
					)
				})
			}

			t.Run("returns an error for another kind", func(t *testing.T) {
				t.Parallel()

				var got specfront.Resolution
				typed := assert.ErrorAs[*yaml.TypeError](t, yaml.Unmarshal([]byte("anything"), &got),
					"the decoder reports the fault as a type error")
				assert.Equal(t, typed.Errors, []string{`line 1: "anything" is not a resolution kind: write ` +
					`"callable-in-scope" or "package-var" or "value-field" or "host-param" or "member-on-handle" or "type-in-scope"`},
					"the entry lists the resolution kinds")
			})
		})

		t.Run("JSONSchema", func(t *testing.T) {
			t.Parallel()

			t.Run("returns an enum of the resolution kinds", func(t *testing.T) {
				t.Parallel()

				enum := make([]any, 0, len(resolutions))
				for _, r := range resolutions {
					enum = append(enum, string(r))
				}
				assert.Equal(t, specfront.ResolveCallableInScope.JSONSchema(), map[string]any{
					keywordType: typeString, keywordEnum: enum,
				}, "the schema lists the resolution kinds in order")
			})
		})
	})
}
