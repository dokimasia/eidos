// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
)

// A code is API: consumers script against it and the published
// index anchors to it, so its number and meaning are contract.
func TestCodes(t *testing.T) {
	t.Parallel()

	t.Run("RefusedStamp", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, eidos.RefusedStamp.String(), "EID-0020",
			"the code spells its prefix and padded number")
		meaning, held := diag.Kernel().Meaning(eidos.RefusedStamp)
		assert.True(t, held, "the code registered at initialization")
		assert.Equal(t, meaning, "the fact store refused a stamp",
			"the meaning anchors the published index")
	})
}
