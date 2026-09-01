// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/conformance"
)

// The inventory is the shared bar every language answers, so its
// own discipline is pinned: unique ids spelled as package segments,
// and something to hold every feature to.
func TestInventory(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for _, f := range conformance.Inventory() {
		assert.False(t, seen[f.ID], "an id appears once: "+f.ID)
		seen[f.ID] = true
		assert.NotEmpty(t, f.Doc, f.ID+" states what it exercises")
		assert.True(t, f.ID == strings.ToLower(f.ID) && !strings.ContainsAny(f.ID, "-/ "),
			f.ID+" spells as a package segment in every language")
		assert.True(t, len(f.Declares) > 0 || f.Check != nil,
			f.ID+" holds its spellings to something")
	}
}
