// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/meta"
)

// rivalPlugin is a second registrant, and rivalKey the key it tries
// to register under the satellite's namespace.
const (
	rivalPlugin              = "rival"
	rivalKey    meta.KeyName = "golang.rival"
)

// One namespace claim serves both stamping roles, so the
// registration round is pinned: every key resolves, the handles
// come back typed, and a second claim fails.
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

		t.Run("returns an error for a second registration", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, golang.Keys(r), "the first registration succeeds")
			assert.HasError(t, golang.Keys(r), "the namespace is claimed once")
		})
	})
}
