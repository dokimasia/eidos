// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
)

// The value tree is what a sample crosses a package boundary as,
// so its shape, its spellings and its codec are pinned here.
func TestValue(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("names every kind and numbers the rest", func(t *testing.T) {
			t.Parallel()

			kinds := map[emit.ValueKind]string{
				emit.ValueLiteral: "literal", emit.ValueConversion: "conversion",
				emit.ValueComposite: "composite", emit.ValueCall: "call",
				emit.ValueAddress: "address", emit.ValueKind(9): "9",
			}
			for k, want := range kinds {
				assert.Equal(t, k.String(), want, "the kind spells as its name")
			}
			literals := map[emit.LiteralKind]string{
				emit.LiteralInt: "int", emit.LiteralFloat: "float", emit.LiteralString: "string",
				emit.LiteralBool: "bool", emit.LiteralNil: "nil", emit.LiteralRaw: "raw",
				emit.LiteralKind(9): "9",
			}
			for k, want := range literals {
				assert.Equal(t, k.String(), want, "the literal kind spells as its name")
			}
		})
	})

	t.Run("constructors", func(t *testing.T) {
		t.Parallel()

		t.Run("build each form with the fields its kind selects", func(t *testing.T) {
			t.Parallel()

			ref := &emit.TypeRef{Spelling: "Point"}
			forty := emit.Literal(emit.LiteralInt, "42")
			assert.Equal(t, forty.Kind, emit.ValueLiteral, "a literal")
			assert.False(t, forty.IsZero(), "names something")
			assert.True(t, emit.Value{}.IsZero(), "and the zero value names nothing")

			conv := emit.Conversion(ref, forty)
			assert.Equal(t, conv.Type, ref, "a conversion carries its type")
			assert.Equal(t, *conv.Inner, forty, "and the value converted")

			comp := emit.Composite(ref, emit.ValueField{Name: "X", Value: forty})
			assert.Length(t, comp.Fields, 1, "a composite carries its fields")

			callee := symbol.Identity{Lang: "golang", Package: "time", Name: "Unix", Kind: symbol.KindFunction}
			call := emit.Call(callee, forty, emit.Literal(emit.LiteralInt, "0"))
			assert.Equal(t, call.Callee, callee, "a call names what it calls")
			assert.Length(t, call.Args, 2, "and carries its arguments")

			addr := emit.Address(comp)
			assert.Equal(t, addr.Kind, emit.ValueAddress, "an address wraps its inner")
			assert.Equal(t, addr.Inner.Kind, emit.ValueComposite, "whole")
		})
	})

	t.Run("Number", func(t *testing.T) {
		t.Parallel()

		t.Run("states the width beside the literal", func(t *testing.T) {
			t.Parallel()

			n := emit.Number(emit.LiteralFloat, "1.5", 32)
			assert.Equal(t, n.Kind, emit.ValueLiteral, "a number is a literal")
			assert.Equal(t, n.Literal, emit.LiteralFloat, "of the kind given")
			assert.Equal(t, n.Text, "1.5", "with the text given")
			assert.Equal(t, n.Bits, 32, "and the width given")
			assert.Equal(t, emit.Number(emit.LiteralInt, "42", 0), emit.Literal(emit.LiteralInt, "42"),
				"a width of 0 is the plain literal")
		})
	})

	t.Run("JSON", func(t *testing.T) {
		t.Parallel()

		t.Run("round-trips a nested value", func(t *testing.T) {
			t.Parallel()

			ref := &emit.TypeRef{Spelling: "Point"}
			in := emit.Address(emit.Composite(ref,
				emit.ValueField{Name: "X", Value: emit.Conversion(ref, emit.Literal(emit.LiteralInt, "42"))},
				emit.ValueField{Name: "Y", Value: emit.Number(emit.LiteralFloat, "2.5", 32)},
			))
			b, err := json.Marshal(in)
			assert.NoError(t, err, "the tree encodes")
			var out emit.Value
			assert.NoError(t, json.Unmarshal(b, &out), "and decodes")
			assert.Equal(t, out, in, "to the same tree")
		})
	})
}
