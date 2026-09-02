// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/naming"
)

// The predicate stands before every wire-name refusal, so the
// shape's edges are pinned here.
func TestIdentifier(t *testing.T) {
	t.Parallel()

	t.Run("IsIdentifier/reports the shape without touching it", func(t *testing.T) {
		t.Parallel()

		assert.True(t, naming.IsIdentifier("content_type"), "the plain shape holds")
		assert.True(t, naming.IsIdentifier("_x9"), "an underscore opening holds")
		assert.False(t, naming.IsIdentifier("content-type"), "a hyphen is outside the shape")
		assert.False(t, naming.IsIdentifier("9lives"), "a leading digit is outside it")
		assert.False(t, naming.IsIdentifier(""), "and emptiness names nothing")
	})
}
