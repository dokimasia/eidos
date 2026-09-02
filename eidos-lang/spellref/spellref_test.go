// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spellref_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/sdk/emit"
)

// One walk serves every bracket pair, so the recursion and the
// stand-in are pinned once.
func TestSpell(t *testing.T) {
	t.Parallel()

	ref := &emit.TypeRef{Spelling: "Map", Args: []*emit.TypeRef{
		{Spelling: "K"}, {Spelling: "List", Args: []*emit.TypeRef{{Spelling: "V"}}},
	}}
	assert.Equal(t, spellref.Spell(ref, "<", ">", "?"), "Map<K, List<V>>",
		"arguments nest inside the given brackets")
	assert.Equal(t, spellref.Spell(ref, "[", "]", "?"), "Map[K, List[V]]",
		"whichever brackets the language states")
	assert.Equal(t, spellref.Spell(nil, "<", ">", "()"), "()",
		"an absent reference writes the language's stand-in")
	assert.Equal(t, spellref.Spell(&emit.TypeRef{}, "<", ">", "unknown"), "unknown",
		"and so does an empty spelling")
}
