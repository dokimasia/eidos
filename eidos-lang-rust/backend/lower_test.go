// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/lang-rust/backend"
)

// The lowering folds an announced failure into the Result return;
// everything else passes through untouched.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("a non-thrower passes through unchanged", func(t *testing.T) {
		t.Parallel()

		out, err := backend.Lower(&emit.Function{Name: "fetch"})
		assert.NoError(t, err, "a plain function is not reshaped")
		assert.Length(t, out, 0, "a nil list keeps the declaration as it stands")
	})

	t.Run("an announced failure wraps the result", func(t *testing.T) {
		t.Parallel()

		valued := &emit.TypeRef{Spelling: "row"}
		failure := &emit.TypeRef{Spelling: "fetchError"}
		f := &emit.Function{
			Name:    "fetch",
			Returns: []*emit.Return{{Type: valued}},
			Throws:  []*emit.TypeRef{failure},
		}
		_, err := backend.Lower(f)
		assert.NoError(t, err, "the throwing function lowers")
		assert.Length(t, f.Throws, 0, "the fact is consumed")
		assert.Length(t, f.Returns, 1, "into the one wrapped return")
		wrapped := f.Returns[0].Type
		assert.Equal(t, wrapped.Spelling, "Result", "as a Result")
		assert.Length(t, wrapped.Args, 2, "of value and failure")
		assert.True(t, wrapped.Args[0] == valued,
			"the value reference moves whole, so the settle keeps "+
				"following it")
		assert.True(t, wrapped.Args[1] == failure, "and so does the failure's")
	})

	t.Run("a bare thrower wraps the unit type", func(t *testing.T) {
		t.Parallel()

		host := &emit.Interface{Name: "store"}
		host.Methods.Append(&emit.Method{
			Name:   "close",
			Throws: []*emit.TypeRef{{Spelling: "closeError"}},
		})
		_, err := backend.Lower(host)
		assert.NoError(t, err, "a host's member methods lower with it")
		m := host.Methods.Items()[0]
		assert.Length(t, m.Throws, 0, "consumed")
		assert.Equal(t, m.Returns[0].Type.Args[0].Spelling, "()",
			"nothing returned wraps the unit type")
	})

	t.Run("refusals", func(t *testing.T) {
		t.Parallel()

		several := &emit.Function{
			Name: "fetch",
			Throws: []*emit.TypeRef{
				{Spelling: "notFound"}, {Spelling: "timeout"},
			},
		}
		_, err := backend.Lower(several)
		assert.HasError(t, err, "a result carries one failure type")

		wide := &emit.Function{
			Name: "fetch",
			Returns: []*emit.Return{
				{Type: &emit.TypeRef{Spelling: "row"}},
				{Type: &emit.TypeRef{Spelling: "u32"}},
			},
			Throws: []*emit.TypeRef{{Spelling: "fetchError"}},
		}
		_, err = backend.Lower(wide)
		assert.HasError(t, err, "a result wraps one value")
	})
}
