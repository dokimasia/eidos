// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package meta_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/meta"
)

func TestClaim(t *testing.T) {
	t.Parallel()

	t.Run("Authority", func(t *testing.T) {
		t.Parallel()

		t.Run("orders plugin below directive below manual", func(t *testing.T) {
			t.Parallel()

			assert.True(t, meta.AuthorityPlugin < meta.AuthorityDirective,
				"a human override at the source beats any inference")
			assert.True(t, meta.AuthorityDirective < meta.AuthorityManual,
				"and consumer tooling outranks even the directives it rewrites")
		})

		t.Run("the zero value is the plugin authority", func(t *testing.T) {
			t.Parallel()

			var got meta.Authority
			assert.Equal(t, got, meta.AuthorityPlugin,
				"a claim that returned no authority claims the least")
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		var c meta.Claim
		assert.True(t, c.Subject.IsZero(), "a zero claim is about nothing")
		assert.Equal(t, c.Authority, meta.AuthorityPlugin,
			"and speaks with the least authority")
		assert.True(t, c.Pos.IsZero(), "from no carrier")
		assert.Nil(t, c.Derived, "derived from nothing")
	})
}
