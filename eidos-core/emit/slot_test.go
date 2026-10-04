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

// concreteJSON is the encoding of the concrete fixture slot: the array
// of its two strings.
const concreteJSON = `["a","b"]`

// allocRuns is the number of calls [assert.MaxAllocs] makes: one to
// warm the function, and the 100 it counts.
const allocRuns = 101

// The ceilings of the slot's codec over the two fixture slots, each with
// the encoder's or the decoder's state pooled by an earlier call.
const (
	// marshalConcreteAllocs is one encoding of the concrete slot: the
	// contents boxed for the encoder, the encoder's addressable copy of
	// them, and its copy of the output.
	marshalConcreteAllocs = 3
	// marshalDeclarationAllocs is one encoding of the declaration slot:
	// the contents boxed for the encoder, the list's own encoding and
	// its splice, each declaration's encoding, and an addressable copy
	// and an output copy for each of the three encoder calls.
	marshalDeclarationAllocs = 11
	// unmarshalDeclarationAllocs is one decoding of the declaration slot:
	// each element's raw bytes and the growth of their list, the decoded
	// list and its slot, each decoded declaration, and the kind name of
	// the one whose name is not interned.
	unmarshalDeclarationAllocs = 11
	// decodeStateAllocs is the decoder's state, which it takes from a
	// pool per processor, as [encodeStateAllocs] is the encoder's: one
	// of 200 runs of one iteration decoded the declaration slot with 5
	// allocations more. The mean over many decodings rounds it away.
	decodeStateAllocs = 5
)

// A slot is the composition point every contributing plugin appends
// through, so its order, its emptiness and its codec are contract.
func TestSlot(t *testing.T) {
	t.Parallel()

	t.Run("Append", func(t *testing.T) {
		t.Parallel()

		t.Run("adds a value to the zero slot", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			slot.Append(1)
			assert.Equal(t, slot.Len(), 1, "the zero slot is ready to append to")
		})

		t.Run("keeps insertion order across calls", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			slot.Append(1, 2)
			slot.Append(3)

			assert.Equal(t, slot.Items(), []int{1, 2, 3},
				"insertion order holds across calls")
		})

		t.Run("leaves the slot unchanged for no values", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			slot.Append()
			assert.Equal(t, slot.Len(), 0, "appending nothing changes nothing")
		})
	})

	t.Run("Items", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for the zero slot", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			assert.Empty(t, slot.Items(), "the zero slot holds nothing")
		})
	})

	t.Run("Len", func(t *testing.T) {
		t.Parallel()

		t.Run("counts what was appended", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[string]
			for i := range 5 {
				slot.Append(string(rune('a' + i)))
			}
			assert.Equal(t, slot.Len(), 5, "Len counts what was appended")
		})
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for an untouched slot", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			assert.True(t, slot.IsZero(),
				"an untouched slot holds nothing, which is what lets an encoder omit it")
		})

		t.Run("reports false for a slot holding a value", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[int]
			slot.Append(1)
			assert.False(t, slot.IsZero(), "a slot holding a value is not omitted")
		})
	})

	t.Run("MarshalJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("encodes a concrete slot as the array of its contents", func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(concrete())
			assert.NoError(t, err, "the slot encodes")
			assert.Equal(t, string(encoded), concreteJSON,
				"as the array of its contents, from the element type's own tags")
		})
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("round-trips a concrete slot", func(t *testing.T) {
			t.Parallel()

			var decoded emit.Slot[string]
			assert.NoError(t, json.Unmarshal([]byte(concreteJSON), &decoded), "the slot decodes")
			assert.Equal(t, decoded.Items(), []string{"a", "b"},
				"returning the contents in insertion order")
		})

		t.Run("round-trips a declaration slot as each element's own kind", func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(declarations())
			assert.NoError(t, err, "the declaration slot encodes")

			var decoded emit.Slot[symbol.Symbol]
			assert.NoError(t, json.Unmarshal(encoded, &decoded), "and decodes")
			assert.Equal(t, decoded.Len(), 2, "returning what it held")
			alias, isAlias := decoded.Items()[0].(*emit.Alias)
			assert.True(t, isAlias,
				"each element decodes as the kind it encoded, which is what the "+
					"kind discriminator is for")
			assert.Equal(t, alias.Name, "RowID", "carrying the fields it wrote")
			constant, isConstant := decoded.Items()[1].(*emit.Constant)
			assert.True(t, isConstant, "and a second kind decodes as its own")
			assert.Equal(t, constant.Name, "Version", "carrying its fields too")
		})

		t.Run("returns an error for malformed JSON into a concrete slot", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[string]
			err := slot.UnmarshalJSON([]byte(`["a",`))
			assert.HasError(t, err, "malformed input is refused")
			assert.HasPrefix(t, err.Error(), "emit: ", "under the package prefix")
		})

		t.Run("keeps a concrete slot's contents on malformed JSON", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[string]
			slot.Append("held")
			_ = slot.UnmarshalJSON([]byte(`["a",`))
			assert.Equal(t, slot.Items(), []string{"held"}, "the slot keeps what it held rather than zeroing")
		})

		t.Run("returns an error for malformed JSON into a declaration slot", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[symbol.Symbol]
			err := slot.UnmarshalJSON([]byte(`[{"kind":`))
			assert.HasError(t, err, "malformed input is refused before the elements are allocated")
			assert.HasPrefix(t, err.Error(), "emit: ", "under the package prefix")
		})

		t.Run("keeps a declaration slot's contents on malformed JSON", func(t *testing.T) {
			t.Parallel()

			var slot emit.Slot[symbol.Symbol]
			slot.Append(&emit.Alias{Name: "RowID"})
			_ = slot.UnmarshalJSON([]byte(`[{"kind":`))
			assert.Equal(t, slot.Len(), 1, "the slot keeps what it held rather than zeroing")
		})
	})
}

// A first append allocates the slot's storage, the reads allocate
// nothing, and the codec allocates within its ceilings, in the ordinary
// run, which runs no benchmark. Each first append takes a declaration
// built before the count, because an append changes the slot it writes.
// The check runs alone, because AllocsPerRun counts every goroutine's
// allocations and refuses to run beside parallel tests.
func TestSlotAllocs(t *testing.T) {
	empties := emptyStructs(allocRuns)
	field := &emit.Field{Name: "ID"}
	at := 0
	assert.MaxAllocs(t, func() {
		empties[at].Fields.Append(field)
		at++
	}, 1, "Append allocates the storage of an empty slot")

	slot := concrete()
	assert.MaxAllocs(t, func() {
		if len(slot.Items()) != 2 {
			t.Fatal("Items missed a value")
		}
	}, 0, "Items allocates nothing")
	assert.MaxAllocs(t, func() {
		if slot.Len() != 2 {
			t.Fatal("Len miscounted the slot")
		}
	}, 0, "Len allocates nothing")
	assert.MaxAllocs(t, func() {
		if slot.IsZero() {
			t.Fatal("IsZero reported a filled slot as empty")
		}
	}, 0, "IsZero allocates nothing")

	assert.MaxAllocs(t, func() {
		if _, err := slot.MarshalJSON(); err != nil {
			t.Fatalf("MarshalJSON: unexpected error: %v", err)
		}
	}, marshalConcreteAllocs, "MarshalJSON allocates what the encoder does for a concrete slot")
	declared := declarations()
	assert.MaxAllocs(t, func() {
		if _, err := declared.MarshalJSON(); err != nil {
			t.Fatalf("MarshalJSON: unexpected error: %v", err)
		}
	}, marshalDeclarationAllocs, "MarshalJSON allocates what the encoder does for a declaration slot")

	data := []byte(concreteJSON)
	var decoded emit.Slot[string]
	assert.MaxAllocs(t, func() {
		if err := decoded.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON: unexpected error: %v", err)
		}
	}, 0, "UnmarshalJSON allocates nothing into a concrete slot with room")
	encoded, err := declared.MarshalJSON()
	assert.NoError(t, err, "the declaration slot encodes")
	var decodedDeclarations emit.Slot[symbol.Symbol]
	assert.MaxAllocs(t, func() {
		if err := decodedDeclarations.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("UnmarshalJSON: unexpected error: %v", err)
		}
	}, unmarshalDeclarationAllocs, "UnmarshalJSON allocates the declarations it decodes")
}

// BenchmarkSlot measures a first append, the slot's reads, and its
// codec over a concrete slot and a declaration slot. Each codec case
// calls the method once before the contract counts, which pools the
// encoder's or the decoder's state, and its ceiling allows that state
// once more for a run of one iteration.
func BenchmarkSlot(b *testing.B) {
	b.Run("Append/a first value into an empty slot", func(b *testing.B) {
		field := &emit.Field{Name: "ID"}
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var s *emit.Struct
		for c.Loop() {
			c.Excluding(func() { s = &emit.Struct{} })
			s.Fields.Append(field)
		}
		assert.Equal(b, s.Fields.Len(), 1, "the slot holds the field")
	})

	b.Run("Items", func(b *testing.B) {
		slot := concrete()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []string
		for c.Loop() {
			got = slot.Items()
		}
		assert.Equal(b, got, []string{"a", "b"}, "Items returns the contents")
	})

	b.Run("Len", func(b *testing.B) {
		slot := concrete()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = slot.Len()
		}
		assert.Equal(b, got, 2, "Len counts the contents")
	})

	b.Run("IsZero", func(b *testing.B) {
		slot := concrete()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		got := true
		for c.Loop() {
			got = slot.IsZero()
		}
		assert.False(b, got, "a filled slot is not empty")
	})

	b.Run("MarshalJSON/a concrete slot", func(b *testing.B) {
		slot := concrete()
		_, err := slot.MarshalJSON()
		assert.NoError(b, err, "the slot encodes before the measurement")
		c := bench.Start(b).MaxAllocs(marshalConcreteAllocs + encodeStateAllocs)
		defer c.End()
		var got []byte
		for c.Loop() {
			got, err = slot.MarshalJSON()
		}
		assert.NoError(b, err, "the slot encodes")
		assert.Equal(b, string(got), concreteJSON, "as the array of its contents")
	})

	b.Run("MarshalJSON/a declaration slot", func(b *testing.B) {
		slot := declarations()
		_, err := slot.MarshalJSON()
		assert.NoError(b, err, "the slot encodes before the measurement")
		c := bench.Start(b).MaxAllocs(marshalDeclarationAllocs + encodeStateAllocs)
		defer c.End()
		var got []byte
		for c.Loop() {
			got, err = slot.MarshalJSON()
		}
		assert.NoError(b, err, "the slot encodes")
		assert.Contains(b, string(got), "RowID", "with its declarations")
	})

	b.Run("UnmarshalJSON/a concrete slot", func(b *testing.B) {
		data := []byte(concreteJSON)
		var slot emit.Slot[string]
		err := slot.UnmarshalJSON(data)
		assert.NoError(b, err, "the slot decodes before the measurement")
		c := bench.Start(b).MaxAllocs(decodeStateAllocs)
		defer c.End()
		for c.Loop() {
			err = slot.UnmarshalJSON(data)
		}
		assert.NoError(b, err, "the array decodes")
		assert.Equal(b, slot.Items(), []string{"a", "b"}, "into the contents")
	})

	b.Run("UnmarshalJSON/a declaration slot", func(b *testing.B) {
		data, err := declarations().MarshalJSON()
		assert.NoError(b, err, "the declaration slot encodes")
		var slot emit.Slot[symbol.Symbol]
		err = slot.UnmarshalJSON(data)
		assert.NoError(b, err, "the slot decodes before the measurement")
		c := bench.Start(b).MaxAllocs(unmarshalDeclarationAllocs + decodeStateAllocs)
		defer c.End()
		for c.Loop() {
			err = slot.UnmarshalJSON(data)
		}
		assert.NoError(b, err, "the array decodes")
		assert.Equal(b, slot.Len(), 2, "into the declarations")
	})
}

// concrete returns a slot of the two strings [concreteJSON] encodes.
func concrete() emit.Slot[string] {
	var slot emit.Slot[string]
	slot.Append("a", "b")
	return slot
}

// declarations returns a slot of an alias and a constant.
func declarations() emit.Slot[symbol.Symbol] {
	var slot emit.Slot[symbol.Symbol]
	slot.Append(&emit.Alias{Name: "RowID"}, &emit.Constant{Name: "Version"})
	return slot
}

// emptyStructs returns n structs whose field slots are empty.
func emptyStructs(n int) []*emit.Struct {
	out := make([]*emit.Struct, n)
	for i := range out {
		out[i] = &emit.Struct{}
	}
	return out
}
