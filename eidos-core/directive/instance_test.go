// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
)

// The instance is what a handler holds, so its twin covers the
// reading surface: the one accessor and the zero value's honesty.
func TestInstance(t *testing.T) {
	t.Parallel()

	t.Run("Param", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a carried key", func(t *testing.T) {
			t.Parallel()

			d := directive.Directive{Params: map[directive.ParamKey]directive.Value{
				"depth": {Kind: directive.TypeInt, Int: 3},
			}}
			got, held := d.Param("depth")
			assert.True(t, held, "a carried key returns")
			assert.Equal(t, got, directive.Value{Kind: directive.TypeInt, Int: 3},
				"with its typed value")
		})

		t.Run("returns false for a key the instance omits", func(t *testing.T) {
			t.Parallel()

			d := directive.Directive{}
			_, held := d.Param("depth")
			assert.False(t, held,
				"an omitted optional reads absent, never as a zero value")
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		var v directive.Value
		assert.NotEqual(t, v.Kind, directive.TypeString,
			"an unpopulated value types nothing, so no field reads as populated")
	})
}
