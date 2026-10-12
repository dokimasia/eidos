// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package java_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/sdk/meta"
)

// rivalPlugin is a second registrant, and rivalKey is the key that it
// tries to register under the namespace of the satellite.
const (
	rivalPlugin              = "rival"
	rivalKey    meta.KeyName = "java.rival"
)

// keysAllocs is the cost of a registration into a fresh registry: the
// three kind lists of the keys, and ten allocations of the registry for
// its namespace claim and the growth of its spec list, type list and name
// map to five keys.
const keysAllocs = 3 + 10

// The frontend stamps these keys, and a composition registers them once.
// The cases pin the registration: every key resolves, the namespace
// belongs to the satellite, a second registration of one registrant
// repeats the first, and a claim of another registrant fails.
func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("Keys", func(t *testing.T) {
		t.Parallel()

		t.Run("registers every java key", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, java.Keys(r), "the vocabulary registers")
			for _, key := range []meta.KeyName{
				java.TestFileKey, java.GeneratedKey, java.RecordKey, java.AnnotationTypeKey, java.ModuleKey,
			} {
				_, known := r.Resolve(key)
				assert.True(t, known, string(key)+" resolves")
			}
		})

		t.Run("claims the namespace for the registrant of the handle", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, java.Keys(r.For(string(java.Name))),
				"the vocabulary registers through the handle of the plugin")
			_, err := meta.Register[bool](r.For(rivalPlugin), meta.KeySpec{
				Name: rivalKey, Doc: "a key under the namespace of another plugin",
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

// A registration allocates its kind lists and the growth of the registry.
// The ordinary run, which runs no benchmark, checks that ceiling here. Each
// call registers into a registry of its own, which the setup builds
// outside the count. The count keeps the first error of its calls, which
// cmp.Or returns without allocating.
func TestKeysAllocs(t *testing.T) {
	var err error
	assert.MaxAllocsWithSetup(t, meta.NewRegistry, func(r *meta.Registry) { err = cmp.Or(err, java.Keys(r)) },
		keysAllocs, "Keys allocates the kind lists and the growth of the registry")
	assert.NoError(t, err, "Keys registers the vocabulary")
}

// BenchmarkKeys measures the registration that a composition makes once,
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
