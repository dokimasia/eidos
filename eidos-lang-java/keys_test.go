// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package java_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/sdk/meta"
)

// rivalPlugin is a second registrant, and rivalKey the key it tries
// to register under the satellite's namespace.
const (
	rivalPlugin              = "rival"
	rivalKey    meta.KeyName = "java.rival"
)

// keysAllocs is a registration into a fresh registry: the three kind
// lists of the keys, and eight allocations of the registry, its
// namespace claim and the growth of its spec list, type list and name
// map to four keys.
const keysAllocs = 3 + 8

// The frontend stamps these keys, and a composition registers them
// once. The registration is pinned: every key resolves, the namespace
// is the satellite's, a second registration of one registrant repeats
// the first, and a claim of another registrant fails.
func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("Keys", func(t *testing.T) {
		t.Parallel()

		t.Run("registers every java key", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, java.Keys(r), "the vocabulary registers")
			for _, key := range []meta.KeyName{
				java.TestFileKey, java.RecordKey, java.AnnotationTypeKey, java.ModuleKey,
			} {
				_, known := r.Resolve(key)
				assert.True(t, known, string(key)+" resolves")
			}
		})

		t.Run("claims the namespace for the handle's registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, java.Keys(r.For(string(java.Name))),
				"the vocabulary registers through the plugin's handle")
			_, err := meta.Register[bool](r.For(rivalPlugin), meta.KeySpec{
				Name: rivalKey, Doc: "a key under another plugin's namespace",
			})
			assert.HasError(t, err, "another plugin registers no key under the namespace")
		})

		t.Run("returns no error for a second registration of one registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry().For(string(java.Name))
			assert.NoError(t, java.Keys(r), "the first registration succeeds")
			assert.NoError(t, java.Keys(r), "the registrant repeats its registration")
		})

		t.Run("returns an error for a registration of another registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, java.Keys(r.For(string(java.Name))), "the first registration succeeds")
			assert.HasError(t, java.Keys(r.For(rivalPlugin)), "the namespace has one registrant")
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
	assert.MaxAllocsWithSetup(t, meta.NewRegistry, func(r *meta.Registry) { err = cmp.Or(err, java.Keys(r)) },
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
			err = java.Keys(r)
		}
		assert.NoError(b, err, "Keys registers the vocabulary")
	})
}
