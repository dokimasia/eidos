// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"testing"

	"go.dokimi.dev/assert"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// every is the satellite's whole key set, which a composition
// registers as one and a corpus fixture declares as one.
func every() []meta.KeyName {
	return []meta.KeyName{
		protobuf.FieldKey, protobuf.ReservedKey, protobuf.OptionsKey,
		protobuf.SyntaxKey, protobuf.PackageKey, protobuf.OneofKey,
		protobuf.StreamKey, protobuf.MapEntryKey, protobuf.FeaturesKey,
		protobuf.LabelKey, protobuf.JSONNameKey, protobuf.ExtensionsKey,
		protobuf.ImportKey,
	}
}

// The keys are the boundary a consumer reads the residue through,
// so their spellings, their value type and the kinds each admits
// are pinned.
func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("registers every key as text, once", func(t *testing.T) {
		t.Parallel()

		r := meta.NewRegistry()
		assert.NoError(t, protobuf.Keys(r), "the keys register")
		assert.HasError(t, protobuf.Keys(r), "and refuse a second registration")

		for _, name := range every() {
			_, held := meta.Lookup[string](r, name)
			assert.True(t, held, string(name)+" is registered as text")
		}
	})

	t.Run("spells every key under the satellite's namespace", func(t *testing.T) {
		t.Parallel()

		for _, name := range every() {
			assert.Equal(t, name.Namespace(), "protobuf",
				string(name)+" is claimed by this satellite and no other")
		}
	})

	t.Run("admits each key on the kinds that state it", func(t *testing.T) {
		t.Parallel()

		r := meta.NewRegistry()
		assert.NoError(t, protobuf.Keys(r), "the keys register")
		spec := func(name meta.KeyName) meta.KeySpec {
			id, held := r.Resolve(name)
			assert.True(t, held, string(name)+" resolves")
			out, held := r.Spec(id)
			assert.True(t, held, string(name)+" has a spec")
			return out
		}
		assert.Equal(t, spec(protobuf.FieldKey).Kinds, []symbol.Kind{symbol.KindField},
			"a wire number is stamped on a field and nowhere else")
		assert.Equal(t, spec(protobuf.OneofKey).Kinds, []symbol.Kind{symbol.KindSum},
			"a oneof mark on the sum it projected to")
		assert.Equal(t, spec(protobuf.StreamKey).Kinds, []symbol.Kind{symbol.KindMethod},
			"a stream mark on the rpc")
		assert.Equal(t, spec(protobuf.ExtensionsKey).Kinds, []symbol.Kind{symbol.KindStruct},
			"extension ranges on the message that reserves them")
		assert.True(t, len(spec(protobuf.OptionsKey).Kinds) > 1,
			"options on every level that states them")
		assert.Equal(t, spec(protobuf.FeaturesKey).Kinds, []symbol.Kind{
			symbol.KindFile, symbol.KindStruct, symbol.KindField, symbol.KindSum,
			symbol.KindEnum, symbol.KindEnumVariant, symbol.KindInterface, symbol.KindMethod,
		}, "and features on every level an edition lets a schema state them")
	})
}
