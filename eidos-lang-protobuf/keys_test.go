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

// namespace pins the spelling every key opens with. rivalPlugin is a
// second registrant, and rivalKey the key it tries to register under
// the satellite's namespace.
const (
	namespace                = "protobuf"
	rivalPlugin              = "rival"
	rivalKey    meta.KeyName = "protobuf.rival"
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
// so their spellings, their value type, the kinds each admits and
// the namespace claim are pinned.
func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("Keys", func(t *testing.T) {
		t.Parallel()

		t.Run("registers every key as text", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, protobuf.Keys(r), "the keys register")
			for _, name := range every() {
				_, held := meta.Lookup[string](r, name)
				assert.True(t, held, string(name)+" is registered as text")
			}
		})

		t.Run("returns an error for a second registration", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, protobuf.Keys(r), "the first registration succeeds")
			assert.HasError(t, protobuf.Keys(r), "the namespace is claimed once")
		})

		t.Run("registers every key under the protobuf namespace", func(t *testing.T) {
			t.Parallel()

			for _, name := range every() {
				assert.Equal(t, name.Namespace(), namespace,
					string(name)+" is spelled under the satellite's namespace")
			}
		})

		t.Run("claims the namespace for the handle's registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, protobuf.Keys(r.For(string(protobuf.Name))),
				"the keys register through the plugin's handle")
			_, err := meta.Register[string](r.For(rivalPlugin), meta.KeySpec{
				Name: rivalKey, Doc: "a key under another plugin's namespace",
			})
			assert.HasError(t, err, "another plugin registers no key under the namespace")
		})

		t.Run("registers each key for the kinds a schema states it on", func(t *testing.T) {
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
				"a oneof mark is stamped on the sum it projected to")
			assert.Equal(t, spec(protobuf.StreamKey).Kinds, []symbol.Kind{symbol.KindMethod},
				"a stream mark is stamped on the rpc")
			assert.Equal(t, spec(protobuf.ExtensionsKey).Kinds, []symbol.Kind{symbol.KindStruct},
				"extension ranges are stamped on the message that reserves them")
			assert.True(t, len(spec(protobuf.OptionsKey).Kinds) > 1,
				"options are stamped on every level that states them")
			assert.Equal(t, spec(protobuf.FeaturesKey).Kinds, []symbol.Kind{
				symbol.KindFile, symbol.KindStruct, symbol.KindField, symbol.KindSum,
				symbol.KindEnum, symbol.KindEnumVariant, symbol.KindInterface, symbol.KindMethod,
			}, "features are stamped on every level an edition lets a schema state them")
		})
	})
}
