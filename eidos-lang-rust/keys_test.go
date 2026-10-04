// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/meta"
)

// rivalPlugin is a second registrant, and rivalKey the key it tries
// to register under the satellite's namespace.
const (
	rivalPlugin              = "rival"
	rivalKey    meta.KeyName = "rust.rival"
)

// allocRuns is how many calls an allocation check makes: one to warm
// up and the hundred it counts.
const allocRuns = 101

// keysAllocs is a registration into a fresh registry: the three kind
// lists of the keys, and ten allocations of the registry, its namespace
// claim and the growth of its spec list, type list and name map to five
// keys.
const keysAllocs = 3 + 10

// The frontend stamps these keys, and a composition registers them
// once. The registration is pinned: every key resolves, the namespace
// is the satellite's, and a second claim fails.
func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("Keys", func(t *testing.T) {
		t.Parallel()

		t.Run("registers every rust key", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, rust.Keys(r), "the vocabulary registers")
			for _, key := range []meta.KeyName{
				rust.TestKey, rust.CfgKey, rust.UnionKey, rust.VisibilityKey, rust.LifetimeParamsKey,
			} {
				_, known := r.Resolve(key)
				assert.True(t, known, string(key)+" resolves")
			}
		})

		t.Run("claims the namespace for the handle's registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, rust.Keys(r.For(string(rust.Name))),
				"the vocabulary registers through the plugin's handle")
			_, err := meta.Register[bool](r.For(rivalPlugin), meta.KeySpec{
				Name: rivalKey, Doc: "a key under another plugin's namespace",
			})
			assert.HasError(t, err, "another plugin registers no key under the namespace")
		})

		t.Run("returns an error for a second registration", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, rust.Keys(r), "the first registration succeeds")
			assert.HasError(t, rust.Keys(r), "the namespace is claimed once")
		})
	})
}

// A registration allocates its kind lists and the registry's growth.
// The ordinary run, which runs no benchmark, checks that ceiling here,
// each call into a registry of its own.
func TestKeysAllocs(t *testing.T) {
	registries := freshRegistries(allocRuns)
	next := 0
	var err error
	keys := func() {
		err = rust.Keys(registries[next])
		next++
	}
	assert.MaxAllocs(t, keys, keysAllocs, "Keys allocates the kind lists and the registry's growth")
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
			err = rust.Keys(r)
		}
		assert.NoError(b, err, "Keys registers the vocabulary")
	})
}

// freshRegistries returns n empty registries, one for each counted
// registration.
func freshRegistries(n int) []*meta.Registry {
	out := make([]*meta.Registry, 0, n)
	for range n {
		out = append(out, meta.NewRegistry())
	}
	return out
}
