// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
)

// A role decodes its arity, and the arities are a closed list.
func TestRole(t *testing.T) {
	t.Parallel()

	t.Run("RoleArity", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes the arity of a role", func(t *testing.T) {
			t.Parallel()

			got := decodeAs[specfront.RoleArity](t, "{arity: optional}\n")
			assert.Equal(
				t,
				got,
				specfront.RoleArity{Arity: specfront.ArityOptional},
				"the role has the arity of the text",
			)
		})
	})

	t.Run("Arity", func(t *testing.T) {
		t.Parallel()

		t.Run("UnmarshalYAML", func(t *testing.T) {
			t.Parallel()

			arities := []specfront.Arity{
				specfront.ArityOne,
				specfront.ArityOptional,
				specfront.ArityMany,
				specfront.ArityAny,
			}
			for _, want := range arities {
				t.Run("decodes "+string(want), func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, decodeAs[specfront.Arity](t, string(want)), want, "the arity decodes")
				})
			}
		})
	})
}
