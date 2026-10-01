// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust_test

import (
	"testing"

	"go.dokimi.dev/assert"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/meta"
)

// rivalPlugin is a second registrant, and rivalKey the key it tries
// to register under the satellite's namespace.
const (
	rivalPlugin              = "rival"
	rivalKey    meta.KeyName = "rust.rival"
)

// The frontend stamps these keys, and a composition registers them
// once, so the registration round is pinned: every key resolves, the
// namespace is the satellite's, and a second claim fails.
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
