// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"testing"

	"go.dokimi.dev/assert"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/meta"
)

// rivalPlugin is a second registrant, and rivalKey the key it tries
// to register under the satellite's namespace.
const (
	rivalPlugin              = "rival"
	rivalKey    meta.KeyName = "typescript.rival"
)

// The frontend stamps these keys, and a composition registers them
// once, so the registration round is pinned: every key resolves, the
// namespace is the satellite's, and a second claim fails.
func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("Keys", func(t *testing.T) {
		t.Parallel()

		t.Run("registers every typescript key", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, typescript.Keys(r), "the vocabulary registers")
			for _, key := range []meta.KeyName{
				typescript.TestFileKey, typescript.NamespaceKey, typescript.CallSignatureKey,
			} {
				_, known := r.Resolve(key)
				assert.True(t, known, string(key)+" resolves")
			}
		})

		t.Run("claims the namespace for the handle's registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, typescript.Keys(r.For(string(typescript.Name))),
				"the vocabulary registers through the plugin's handle")
			_, err := meta.Register[bool](r.For(rivalPlugin), meta.KeySpec{
				Name: rivalKey, Doc: "a key under another plugin's namespace",
			})
			assert.HasError(t, err, "another plugin registers no key under the namespace")
		})

		t.Run("returns an error for a second registration", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, typescript.Keys(r), "the first registration succeeds")
			assert.HasError(t, typescript.Keys(r), "the namespace is claimed once")
		})
	})
}
