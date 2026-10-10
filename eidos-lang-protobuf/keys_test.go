// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"cmp"
	"math"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

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

// keysAllocs is a registration into a fresh registry: the eleven kind
// lists of the keys, and fifteen allocations of the registry, its
// namespace claim and the growth of its spec list, type list and name
// map to thirteen keys.
const keysAllocs = 11 + 15

// A consumer reads the residue through these keys. Their spellings,
// their value type, the kinds each admits and the namespace claim are
// pinned.
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

		t.Run("returns no error for a second registration of one registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry().For(string(protobuf.Name))
			assert.NoError(t, protobuf.Keys(r), "the first registration succeeds")
			assert.NoError(t, protobuf.Keys(r), "the registrant repeats its registration")
		})

		t.Run("returns an error for a registration of another registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, protobuf.Keys(r.For(string(protobuf.Name))), "the first registration succeeds")
			assert.HasError(t, protobuf.Keys(r.For(rivalPlugin)), "the namespace has one registrant")
		})

		t.Run("returns an error for a key whose spelling a group took", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			rival := r.For(rivalPlugin)
			assert.NoError(t, rival.ClaimNamespace(rivalPlugin), "the rival claims its own namespace")
			_, err := meta.Register[bool](rival, meta.KeySpec{
				Name: rivalPlugin + ".grouped", Group: meta.GroupName(protobuf.FieldKey), Doc: "a key in a group",
			})
			assert.NoError(t, err, "the rival's key registers into a group that spells a protobuf key")
			err = protobuf.Keys(r)
			assert.HasError(t, err, "a key and a group share no spelling")
			assert.Contains(t, err.Error(), string(protobuf.FieldKey), "the error names the key")
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
			assert.InRange(t, len(spec(protobuf.OptionsKey).Kinds), 2, math.Inf(1),
				"options are stamped on every level that states them")
			assert.Equal(t, spec(protobuf.FeaturesKey).Kinds, []symbol.Kind{
				symbol.KindFile, symbol.KindStruct, symbol.KindField, symbol.KindSum,
				symbol.KindEnum, symbol.KindEnumVariant, symbol.KindInterface, symbol.KindMethod,
			}, "features are stamped on every level an edition lets a schema state them")
		})
	})
}

// A registration allocates its kind lists and the registry's growth.
// The ordinary run, which runs no benchmark, checks that ceiling here,
// each call into a registry of its own, built outside the count. The
// count keeps the first error of its calls, which cmp.Or returns without
// allocating.
func TestKeysAllocs(t *testing.T) {
	var err error
	assert.MaxAllocsWithSetup(t, meta.NewRegistry, func(r *meta.Registry) { err = cmp.Or(err, protobuf.Keys(r)) },
		keysAllocs, "Keys allocates the kind lists and the registry's growth")
	assert.NoError(t, err, "Keys registers the vocabulary")
}

// BenchmarkKeys measures the registration a composition makes once,
// each into a registry of its own.
func BenchmarkKeys(b *testing.B) {
	b.Run("Keys", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(keysAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			var r *meta.Registry
			c.Excluding(func() { r = meta.NewRegistry() })
			err = protobuf.Keys(r)
		}
		assert.NoError(b, err, "Keys registers the vocabulary")
	})
}

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
