// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
)

// The instance is what a handler reads, so its twin covers the
// reading surface: the one accessor and the zero value.
func TestInstance(t *testing.T) {
	t.Parallel()

	t.Run("Param", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the typed value of a key the instance has", func(t *testing.T) {
			t.Parallel()

			d := directive.Directive{Params: map[directive.ParamKey]directive.Value{
				"depth": {Kind: directive.TypeInt, Int: 3},
			}}
			got, held := d.Param("depth")
			assert.True(t, held, "the key is present")
			assert.Equal(t, got, directive.Value{Kind: directive.TypeInt, Int: 3}, "the value is typed")
		})

		t.Run("reports false for a key the instance omits", func(t *testing.T) {
			t.Parallel()

			d := directive.Directive{}
			_, held := d.Param("depth")
			assert.False(t, held, "the key is absent")
		})
	})

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		t.Run("types nothing at the zero value", func(t *testing.T) {
			t.Parallel()

			var v directive.Value
			assert.Equal(t, v.Kind, directive.ParamType(0), "the kind is the zero type")
		})
	})
}
