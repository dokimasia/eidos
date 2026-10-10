// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/meta"
)

// rivalPlugin is a second registrant, and rivalKey the key it tries
// to register under the satellite's namespace.
const (
	rivalPlugin              = "rival"
	rivalKey    meta.KeyName = "golang.rival"
)

// registerAllocs is a registration into a fresh registry: the eleven
// kind lists of the keys, and seventeen allocations of the registry,
// its namespace claim and the growth of its spec list, type list and
// name map to sixteen keys.
const registerAllocs = 11 + 17

// One namespace claim serves both stamping roles. The registration is
// pinned: every key resolves, the handles return typed, a second
// registration of one registrant repeats the first, and a claim of
// another registrant fails.
func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("registers every golang key", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			_, err := golang.Register(r)
			assert.NoError(t, err, "the vocabulary registers")
			for _, key := range []meta.KeyName{
				golang.TestFileKey, golang.ConstraintKey, golang.GeneratedKey,
				golang.CgoKey, golang.TypeSetKey, golang.ConstraintInterfaceKey,
				golang.EmptyInterfaceKey, golang.ReceiverPointerKey,
				golang.UnderlyingKey, golang.IterSeqKey, golang.IterSeq2Key,
				golang.ConstValueKey, golang.SatisfiesErrorKey,
				golang.SatisfiesStringerKey, golang.EmbedsInterfaceKey,
				golang.ComparableKey,
			} {
				_, held := r.Resolve(key)
				assert.True(t, held, string(key)+" resolves")
			}
		})

		t.Run("returns the annotator's handles", func(t *testing.T) {
			t.Parallel()

			handles, err := golang.Register(meta.NewRegistry())
			assert.NoError(t, err, "the vocabulary registers")
			assert.False(t, handles.SatisfiesError.IsZero(), "the error handle names its key")
			assert.False(t, handles.SatisfiesStringer.IsZero(), "the stringer handle names its key")
			assert.False(t, handles.EmbedsInterface.IsZero(), "the embedding handle names its key")
			assert.False(t, handles.Comparable.IsZero(), "the comparable handle names its key")
		})

		t.Run("returns an error for a key whose spelling a group took", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			rival := r.For(rivalPlugin)
			assert.NoError(t, rival.ClaimNamespace(rivalPlugin), "the rival claims its own namespace")
			_, err := meta.Register[bool](rival, meta.KeySpec{
				Name: rivalPlugin + ".grouped", Group: meta.GroupName(golang.TestFileKey), Doc: "a key in a group",
			})
			assert.NoError(t, err, "the rival's key registers into a group that spells a golang key")
			_, err = golang.Register(r)
			assert.HasError(t, err, "a key and a group share no spelling")
			assert.Contains(t, err.Error(), string(golang.TestFileKey), "the error names the key")
		})

		t.Run("claims the namespace for the handle's registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			_, err := golang.Register(r.For(string(golang.Name)))
			assert.NoError(t, err, "the vocabulary registers through the plugin's handle")
			_, err = meta.Register[bool](r.For(rivalPlugin), meta.KeySpec{
				Name: rivalKey, Doc: "a key under another plugin's namespace",
			})
			assert.HasError(t, err, "another plugin registers no key under the namespace")
		})
	})

	t.Run("Keys", func(t *testing.T) {
		t.Parallel()

		t.Run("returns no error for a second registration of one registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry().For(string(golang.Name))
			assert.NoError(t, golang.Keys(r), "the first registration succeeds")
			assert.NoError(t, golang.Keys(r), "the registrant repeats its registration")
		})

		t.Run("returns an error for a registration of another registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, golang.Keys(r.For(string(golang.Name))), "the first registration succeeds")
			assert.HasError(t, golang.Keys(r.For(rivalPlugin)), "the namespace has one registrant")
		})
	})
}

// A registration allocates its kind lists and the registry's growth.
// The ordinary run, which runs no benchmark, checks that ceiling here,
// each call into a registry of its own, built outside the count. Each
// count keeps the first error of its calls, which cmp.Or returns without
// allocating.
func TestKeysAllocs(t *testing.T) {
	var (
		handles golang.Handles
		err     error
	)
	assert.MaxAllocsWithSetup(t, meta.NewRegistry, func(r *meta.Registry) { err = cmp.Or(err, golang.Keys(r)) },
		registerAllocs, "Keys allocates the kind lists and the registry's growth")
	assert.NoError(t, err, "Keys registers the vocabulary")
	assert.MaxAllocsWithSetup(t, meta.NewRegistry, func(r *meta.Registry) {
		var rerr error
		handles, rerr = golang.Register(r)
		err = cmp.Or(err, rerr)
	}, registerAllocs, "Register allocates the kind lists and the registry's growth")
	assert.NoError(t, err, "Register registers the vocabulary")
	assert.False(t, handles.Comparable.IsZero(), "Register returns the handles")
}

// BenchmarkKeys measures the registration a composition makes once,
// each into a registry of its own.
func BenchmarkKeys(b *testing.B) {
	b.Run("Keys", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(registerAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			var r *meta.Registry
			c.Excluding(func() { r = meta.NewRegistry() })
			err = golang.Keys(r)
		}
		assert.NoError(b, err, "Keys registers the vocabulary")
	})

	b.Run("Register", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(registerAllocs)
		defer c.End()
		var (
			handles golang.Handles
			err     error
		)
		for c.Loop() {
			var r *meta.Registry
			c.Excluding(func() { r = meta.NewRegistry() })
			handles, err = golang.Register(r)
		}
		assert.NoError(b, err, "Register registers the vocabulary")
		assert.False(b, handles.Comparable.IsZero(), "Register returns the handles")
	})
}
