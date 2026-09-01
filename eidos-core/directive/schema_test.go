// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
)

// A schema is a value with no behaviour, so what its twin covers is
// the vocabulary's contracts: the zero values, the distinctness of
// the closed sets, and the one method the value carries.
func TestSchema(t *testing.T) {
	t.Parallel()

	t.Run("Canonical", func(t *testing.T) {
		t.Parallel()

		t.Run("a plugin's schema spells prefixed", func(t *testing.T) {
			t.Parallel()

			s := directive.Schema{Plugin: "mockgen", Name: "stub"}
			assert.Equal(t, s.Canonical(), directive.Name("mockgen:stub"),
				"the canonical spelling carries the owner")
		})

		t.Run("a kernel schema spells bare", func(t *testing.T) {
			t.Parallel()

			s := directive.Schema{Name: directive.KernelMeta}
			assert.Equal(t, s.Canonical(), directive.KernelMeta,
				"the kernel owns its bare names")
		})
	})

	t.Run("ParamType", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero value types nothing", func(t *testing.T) {
			t.Parallel()

			var got directive.ParamType
			assert.NotEqual(t, got, directive.TypeString,
				"an undeclared type cannot read as a declared one")
		})

		t.Run("the vocabulary stays distinct", func(t *testing.T) {
			t.Parallel()

			seen := map[directive.ParamType]struct{}{}
			for _, pt := range []directive.ParamType{
				directive.TypeString, directive.TypeInt, directive.TypeBool,
				directive.TypeList, directive.TypeReference,
			} {
				seen[pt] = struct{}{}
			}
			assert.Length(t, seen, 5, "five types, five values")
		})
	})

	t.Run("ResolutionKind", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero value marks a non-reference param", func(t *testing.T) {
			t.Parallel()

			var got directive.ResolutionKind
			assert.Equal(t, got, directive.ResolveNone,
				"a param that declares no resolution resolves nothing")
		})

		t.Run("the kinds stay distinct", func(t *testing.T) {
			t.Parallel()

			seen := map[directive.ResolutionKind]struct{}{}
			for _, rk := range []directive.ResolutionKind{
				directive.ResolveNone, directive.ResolveCallableInScope,
				directive.ResolvePackageVar, directive.ResolveValueField,
				directive.ResolveHostParam, directive.ResolveMemberOnHandle,
				directive.ResolveMetadataKey,
			} {
				seen[rk] = struct{}{}
			}
			assert.Length(t, seen, 7, "seven kinds, seven values")
		})
	})

	t.Run("reserved keys", func(t *testing.T) {
		t.Parallel()

		assert.NotEqual(t, directive.ReservedOut, directive.ReservedTag,
			"the two routing overrides stay two keys")
	})
}
