// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
)

// A code is API: consumers script against it and the published
// index anchors to it, so its number and meaning are contract.
func TestCodes(t *testing.T) {
	t.Parallel()

	t.Run("RefusedStamp", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, meta.RefusedStamp.String(), "EID-0020",
			"the code spells its prefix and padded number")
		meaning, held := diag.Kernel().Meaning(meta.RefusedStamp)
		assert.True(t, held, "the code registered at initialization")
		assert.Equal(t, meaning, "a stamp was refused",
			"the meaning anchors the published index")
	})
}
