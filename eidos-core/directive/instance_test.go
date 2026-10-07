// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/directive"
)

// depthKey is the one key of the instance the cases read.
const depthKey directive.ParamKey = "depth"

// depthValue is the typed value the instance has under [depthKey].
var depthValue = directive.Value{Kind: directive.TypeInt, Int: 3}

// The instance is what a handler reads, so its twin covers the
// reading surface: the one accessor and the zero value.
func TestInstance(t *testing.T) {
	t.Parallel()

	t.Run("Param", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the typed value of a key the instance has", func(t *testing.T) {
			t.Parallel()

			d := deep()
			got, held := d.Param(depthKey)
			assert.True(t, held, "the key is present")
			assert.Equal(t, got, depthValue, "the value is typed")
		})

		t.Run("reports false for a key the instance omits", func(t *testing.T) {
			t.Parallel()

			d := directive.Directive{}
			_, held := d.Param(depthKey)
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

// A param read allocates nothing in the ordinary run, which runs no
// benchmark.
func TestInstanceZeroAlloc(t *testing.T) {
	d := deep()
	var (
		got  directive.Value
		held bool
	)
	assert.MaxAllocs(t, func() { got, held = d.Param(depthKey) }, 0, "Param allocates nothing")
	assert.True(t, held, "Param finds the depth key")
	assert.Equal(t, got, depthValue, "Param returns the typed value")
}

// BenchmarkInstance measures a handler's read of one param.
func BenchmarkInstance(b *testing.B) {
	b.Run("Param", func(b *testing.B) {
		d := deep()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got directive.Value
		for c.Loop() {
			got, _ = d.Param(depthKey)
		}
		assert.Equal(b, got, depthValue, "Param returns the typed value")
	})
}

// deep returns an instance with [depthValue] under [depthKey].
func deep() *directive.Directive {
	return &directive.Directive{Params: map[directive.ParamKey]directive.Value{depthKey: depthValue}}
}
