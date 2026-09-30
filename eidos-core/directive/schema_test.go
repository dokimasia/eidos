// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
)

// A schema is a value with no behaviour, so its twin covers the
// vocabulary's contracts: the zero values, the distinctness of the
// closed sets, and the two methods.
func TestSchema(t *testing.T) {
	t.Parallel()

	t.Run("Canonical", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the prefixed spelling of a plugin's schema", func(t *testing.T) {
			t.Parallel()

			s := directive.Schema{Plugin: "mockgen", Name: "stub"}
			assert.Equal(t, s.Canonical(), directive.Name("mockgen:stub"), "the spelling has the prefix")
		})

		t.Run("returns the bare spelling of a kernel schema", func(t *testing.T) {
			t.Parallel()

			s := directive.Schema{Name: directive.KernelMeta}
			assert.Equal(t, s.Canonical(), directive.KernelMeta, "the spelling is bare")
		})
	})

	t.Run("ParamType", func(t *testing.T) {
		t.Parallel()

		t.Run("types nothing at the zero value", func(t *testing.T) {
			t.Parallel()

			var got directive.ParamType
			for _, declared := range []directive.ParamType{
				directive.TypeString, directive.TypeInt, directive.TypeBool,
				directive.TypeList, directive.TypeReference,
			} {
				assert.NotEqual(t, got, declared, "the zero value is no declared type")
			}
		})

		t.Run("declares five distinct values", func(t *testing.T) {
			t.Parallel()

			seen := map[directive.ParamType]struct{}{}
			for _, pt := range []directive.ParamType{
				directive.TypeString, directive.TypeInt, directive.TypeBool,
				directive.TypeList, directive.TypeReference,
			} {
				seen[pt] = struct{}{}
			}
			assert.Length(t, seen, 5, "each type has its own value")
		})
	})

	t.Run("ResolutionKind", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves nothing at the zero value", func(t *testing.T) {
			t.Parallel()

			var got directive.ResolutionKind
			assert.Equal(t, got, directive.ResolveNone, "the zero value is ResolveNone")
		})

		t.Run("declares eight distinct values", func(t *testing.T) {
			t.Parallel()

			seen := map[directive.ResolutionKind]struct{}{}
			for _, rk := range []directive.ResolutionKind{
				directive.ResolveNone, directive.ResolveCallableInScope,
				directive.ResolvePackageVar, directive.ResolveValueField,
				directive.ResolveHostParam, directive.ResolveMemberOnHandle,
				directive.ResolveMetadataKey, directive.ResolveTypeInScope,
			} {
				seen[rk] = struct{}{}
			}
			assert.Length(t, seen, 8, "each kind has its own value")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give directive.ResolutionKind
			want string
		}{
			{name: "returns the spelling of ResolveNone", give: directive.ResolveNone, want: "no resolution"},
			{
				name: "returns the spelling of ResolveCallableInScope",
				give: directive.ResolveCallableInScope, want: "a callable in scope",
			},
			{
				name: "returns the spelling of ResolvePackageVar",
				give: directive.ResolvePackageVar, want: "a package variable",
			},
			{
				name: "returns the spelling of ResolveValueField",
				give: directive.ResolveValueField, want: "a field on the subject's type",
			},
			{
				name: "returns the spelling of ResolveHostParam",
				give: directive.ResolveHostParam, want: "a parameter of the host callable",
			},
			{
				name: "returns the spelling of ResolveMemberOnHandle",
				give: directive.ResolveMemberOnHandle, want: "a member on a handle",
			},
			{
				name: "returns the spelling of ResolveMetadataKey",
				give: directive.ResolveMetadataKey, want: "a metadata key or group",
			},
			{
				name: "returns the spelling of ResolveTypeInScope",
				give: directive.ResolveTypeInScope, want: "a type in scope",
			},
			{
				name: "returns the number of an undeclared kind",
				give: directive.ResolutionKind(99), want: "resolution kind 99",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})

	t.Run("ParamKey", func(t *testing.T) {
		t.Parallel()

		t.Run("reserves two distinct routing keys", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, directive.ReservedOut, directive.ReservedTag, "the keys differ")
		})
	})
}
