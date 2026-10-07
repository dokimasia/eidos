// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fixture values: the text of an integer literal, the type a
// conversion and a composite spell, and the function a call names.
const (
	fortyTwo  = "42"
	pointType = "Point"
)

// unix is the function the call fixtures name.
var unix = symbol.Identity{Lang: "golang", Package: "time", Name: "Unix", Kind: symbol.KindFunction}

// The value tree is what a sample crosses a package boundary as,
// so its shape, its spellings and its codec are pinned here.
func TestValue(t *testing.T) {
	t.Parallel()

	t.Run("ValueKind.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give emit.ValueKind
			want string
		}{
			{name: "returns literal for ValueLiteral", give: emit.ValueLiteral, want: "literal"},
			{name: "returns conversion for ValueConversion", give: emit.ValueConversion, want: "conversion"},
			{name: "returns composite for ValueComposite", give: emit.ValueComposite, want: "composite"},
			{name: "returns call for ValueCall", give: emit.ValueCall, want: "call"},
			{name: "returns address for ValueAddress", give: emit.ValueAddress, want: "address"},
			{name: "returns the number of a kind nothing declares", give: emit.ValueKind(9), want: "9"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the kind spells as its name")
			})
		}
	})

	t.Run("LiteralKind.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give emit.LiteralKind
			want string
		}{
			{name: "returns int for LiteralInt", give: emit.LiteralInt, want: "int"},
			{name: "returns float for LiteralFloat", give: emit.LiteralFloat, want: "float"},
			{name: "returns string for LiteralString", give: emit.LiteralString, want: "string"},
			{name: "returns bool for LiteralBool", give: emit.LiteralBool, want: "bool"},
			{name: "returns nil for LiteralNil", give: emit.LiteralNil, want: "nil"},
			{name: "returns raw for LiteralRaw", give: emit.LiteralRaw, want: "raw"},
			{name: "returns the number of a kind nothing declares", give: emit.LiteralKind(9), want: "9"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the literal kind spells as its name")
			})
		}
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for the zero value", func(t *testing.T) {
			t.Parallel()

			assert.True(t, emit.Value{}.IsZero(), "the zero value names nothing")
		})

		t.Run("reports false for a literal", func(t *testing.T) {
			t.Parallel()

			assert.False(t, forty().IsZero(), "a literal names something")
		})
	})

	t.Run("Literal", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a literal of the kind over the text", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, forty(), emit.Value{Kind: emit.ValueLiteral, Literal: emit.LiteralInt, Text: fortyTwo},
				"the literal fields alone are set")
		})
	})

	t.Run("Number", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a literal with its width beside the text", func(t *testing.T) {
			t.Parallel()

			n := emit.Number(emit.LiteralFloat, "1.5", 32)
			assert.Equal(t, n, emit.Value{Kind: emit.ValueLiteral, Literal: emit.LiteralFloat, Text: "1.5", Bits: 32},
				"the width is the one field a plain literal leaves zero")
		})

		t.Run("returns the plain literal for a width of 0", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, emit.Number(emit.LiteralInt, fortyTwo, 0), forty(), "a width of 0 states none")
		})
	})

	t.Run("Raw", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a raw literal in its language", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, emit.Raw("golang", "time.Second"),
				emit.Value{Kind: emit.ValueLiteral, Literal: emit.LiteralRaw, Text: "time.Second", Lang: "golang"},
				"only a backend of the text's own language spells it")
		})
	})

	t.Run("NamedField", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a field under its name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, emit.NamedField("X", forty()), emit.ValueField{Name: "X", Value: forty()},
				"a named field has no key")
		})
	})

	t.Run("KeyedEntry", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an entry under its key", func(t *testing.T) {
			t.Parallel()

			key := emit.Literal(emit.LiteralString, "k")
			got := emit.KeyedEntry(key, forty())
			assert.Equal(t, *got.Key, key, "the entry points at its key")
			assert.Equal(t, got.Value, forty(), "beside its value")
		})
	})

	t.Run("Element", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a positional element", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, emit.Element(forty()), emit.ValueField{Value: forty()}, "an element has no name or key")
		})
	})

	t.Run("Conversion", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the value converted to the type", func(t *testing.T) {
			t.Parallel()

			ref := &emit.TypeRef{Spelling: pointType}
			got := emit.Conversion(ref, forty())
			assert.Equal(t, got.Kind, emit.ValueConversion, "a conversion")
			assert.Equal(t, got.Type, ref, "to the type given", assert.ByIdentity())
			assert.Equal(t, *got.Inner, forty(), "of the value given")
		})
	})

	t.Run("Composite", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a composite of the type over its fields", func(t *testing.T) {
			t.Parallel()

			ref := &emit.TypeRef{Spelling: pointType}
			x := emit.NamedField("X", forty())
			got := emit.Composite(ref, x)
			assert.Equal(t, got.Kind, emit.ValueComposite, "a composite")
			assert.Equal(t, got.Type, ref, "of the type given", assert.ByIdentity())
			assert.Equal(t, got.Fields, []emit.ValueField{x}, "over the fields given")
		})
	})

	t.Run("Call", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a call of the callee on the arguments", func(t *testing.T) {
			t.Parallel()

			zero := emit.Literal(emit.LiteralInt, "0")
			got := emit.Call(unix, forty(), zero)
			assert.Equal(t, got.Kind, emit.ValueCall, "a call")
			assert.Equal(t, got.Callee, unix, "of the function given")
			assert.Equal(t, got.Args, []emit.Value{forty(), zero}, "on the arguments given")
		})
	})

	t.Run("Address", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the address of the whole value", func(t *testing.T) {
			t.Parallel()

			comp := emit.Composite(&emit.TypeRef{Spelling: pointType}, emit.NamedField("X", forty()))
			got := emit.Address(comp)
			assert.Equal(t, got.Kind, emit.ValueAddress, "an address")
			assert.Equal(t, *got.Inner, comp, "of the whole value")
		})
	})

	t.Run("JSON", func(t *testing.T) {
		t.Parallel()

		t.Run("round-trips a nested value", func(t *testing.T) {
			t.Parallel()

			ref := &emit.TypeRef{Spelling: pointType}
			in := emit.Address(emit.Composite(ref,
				emit.ValueField{Name: "X", Value: emit.Conversion(ref, forty())},
				emit.ValueField{Name: "Y", Value: emit.Number(emit.LiteralFloat, "2.5", 32)},
			))
			assert.RoundTrip(t, func(v emit.Value) ([]byte, error) { return json.Marshal(v) },
				func(b []byte) (emit.Value, error) {
					var v emit.Value
					err := json.Unmarshal(b, &v)
					return v, err
				}, in, "the tree decodes to the tree it encodes")
		})
	})
}

// The spellings, the question and the plain constructors allocate
// nothing, and a constructor that points at a value or takes a list
// allocates that one, in the ordinary run, which runs no benchmark.
func TestValueAllocs(t *testing.T) {
	valueKind, literalKind := emit.ValueCall, emit.LiteralFloat
	value, key := forty(), emit.Literal(emit.LiteralString, "k")
	ref := &emit.TypeRef{Spelling: pointType}
	var (
		spelled string
		got     emit.Value
		field   emit.ValueField
	)
	assert.MaxAllocs(t, func() { spelled = valueKind.String() }, 0, "ValueKind.String allocates nothing")
	assert.Equal(t, spelled, "call", "ValueKind.String spells ValueCall")
	assert.MaxAllocs(t, func() { spelled = literalKind.String() }, 0, "LiteralKind.String allocates nothing")
	assert.Equal(t, spelled, "float", "LiteralKind.String spells LiteralFloat")
	var zero bool
	assert.MaxAllocs(t, func() { zero = value.IsZero() }, 0, "IsZero allocates nothing")
	assert.False(t, zero, "a literal names something")
	assert.MaxAllocs(t, func() { got = emit.Literal(emit.LiteralInt, fortyTwo) }, 0, "Literal allocates nothing")
	assert.MaxAllocs(t, func() { got = emit.Number(emit.LiteralFloat, "1.5", 32) }, 0, "Number allocates nothing")
	assert.MaxAllocs(t, func() { got = emit.Raw("golang", "time.Second") }, 0, "Raw allocates nothing")
	assert.MaxAllocs(t, func() { field = emit.NamedField("X", value) }, 0, "NamedField allocates nothing")
	assert.MaxAllocs(t, func() { field = emit.KeyedEntry(key, value) }, 1, "KeyedEntry allocates its key")
	assert.MaxAllocs(t, func() { field = emit.Element(value) }, 0, "Element allocates nothing")
	assert.Equal(t, field.Value, value, "Element returns the value given")
	assert.MaxAllocs(t, func() { got = emit.Conversion(ref, value) }, 1, "Conversion allocates the value converted")
	assert.MaxAllocs(t, func() { got = emit.Composite(ref, field) }, 1, "Composite allocates its list of fields")
	assert.MaxAllocs(t, func() { got = emit.Call(unix, value) }, 1, "Call allocates its list of arguments")
	assert.MaxAllocs(t, func() { got = emit.Address(value) }, 1, "Address allocates the value it points at")
	assert.Equal(t, *got.Inner, value, "Address points at the value given")
}

// BenchmarkValue measures the kinds' spellings, the zero question, and
// each constructor a sample derivation calls.
func BenchmarkValue(b *testing.B) {
	value, key := forty(), emit.Literal(emit.LiteralString, "k")
	ref := &emit.TypeRef{Spelling: pointType}

	b.Run("ValueKind.String", func(b *testing.B) {
		kind := emit.ValueCall
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = kind.String()
		}
		assert.Equal(b, got, "call", "String spells ValueCall")
	})

	b.Run("LiteralKind.String", func(b *testing.B) {
		kind := emit.LiteralFloat
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = kind.String()
		}
		assert.Equal(b, got, "float", "String spells LiteralFloat")
	})

	b.Run("IsZero", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		got := true
		for c.Loop() {
			got = value.IsZero()
		}
		assert.False(b, got, "a literal names something")
	})

	b.Run("Literal", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got emit.Value
		for c.Loop() {
			got = emit.Literal(emit.LiteralInt, fortyTwo)
		}
		assert.Equal(b, got, value, "Literal returns the literal")
	})

	b.Run("Number", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got emit.Value
		for c.Loop() {
			got = emit.Number(emit.LiteralFloat, "1.5", 32)
		}
		assert.Equal(b, got.Bits, 32, "Number states the width")
	})

	b.Run("Raw", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got emit.Value
		for c.Loop() {
			got = emit.Raw("golang", "time.Second")
		}
		assert.Equal(b, got.Lang, symbol.Lang("golang"), "Raw states the language")
	})

	b.Run("NamedField", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got emit.ValueField
		for c.Loop() {
			got = emit.NamedField("X", value)
		}
		assert.Equal(b, got.Name, "X", "NamedField names the field")
	})

	b.Run("KeyedEntry", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got emit.ValueField
		for c.Loop() {
			got = emit.KeyedEntry(key, value)
		}
		assert.Equal(b, *got.Key, key, "KeyedEntry points at its key")
	})

	b.Run("Element", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got emit.ValueField
		for c.Loop() {
			got = emit.Element(value)
		}
		assert.Equal(b, got.Value, value, "Element holds the value")
	})

	b.Run("Conversion", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got emit.Value
		for c.Loop() {
			got = emit.Conversion(ref, value)
		}
		assert.Equal(b, *got.Inner, value, "Conversion points at the value converted")
	})

	b.Run("Composite", func(b *testing.B) {
		x := emit.NamedField("X", value)
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got emit.Value
		for c.Loop() {
			got = emit.Composite(ref, x)
		}
		assert.Length(b, got.Fields, 1, "Composite holds its field")
	})

	b.Run("Call", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got emit.Value
		for c.Loop() {
			got = emit.Call(unix, value)
		}
		assert.Length(b, got.Args, 1, "Call holds its argument")
	})

	b.Run("Address", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got emit.Value
		for c.Loop() {
			got = emit.Address(value)
		}
		assert.Equal(b, *got.Inner, value, "Address points at the value")
	})
}

// forty returns the integer literal 42.
func forty() emit.Value { return emit.Literal(emit.LiteralInt, fortyTwo) }
