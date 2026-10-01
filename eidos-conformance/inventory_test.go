// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/plugin"
)

// Every language's corpus entry runs against the inventory, so the
// inventory's own rules are pinned: unique ids that spell as package
// segments, an expectation for every feature, and expectations that reject
// the spellings they rule out.
func TestInventory(t *testing.T) {
	t.Parallel()

	t.Run("Inventory", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each id once", func(t *testing.T) {
			t.Parallel()

			seen := map[string]bool{}
			for _, f := range conformance.Inventory() {
				assert.False(t, seen[f.ID], "an id appears once: "+f.ID)
				seen[f.ID] = true
			}
		})

		t.Run("returns ids that spell as package segments", func(t *testing.T) {
			t.Parallel()

			for _, f := range conformance.Inventory() {
				assert.True(t, f.ID == strings.ToLower(f.ID) && !strings.ContainsAny(f.ID, "-/ "),
					f.ID+" spells as a package segment in every language")
			}
		})

		t.Run("returns features that state what they exercise", func(t *testing.T) {
			t.Parallel()

			for _, f := range conformance.Inventory() {
				assert.NotEmpty(t, f.Doc, f.ID+" states what it exercises")
			}
		})

		t.Run("returns an expectation for every feature", func(t *testing.T) {
			t.Parallel()

			for _, f := range conformance.Inventory() {
				assert.True(t, len(f.Declares) > 0 || f.Check != nil, f.ID+" states what a spelling must load")
			}
		})

		t.Run("returns a composite_refs check that rejects a reference naming the sibling at its root",
			func(t *testing.T) {
				t.Parallel()

				// The scripted language spells no composite, so each field's
				// reference names the sibling's declaration at its root.
				tree := fstest.MapFS{
					"f/composite_refs/a.zz": {Data: []byte("package f/composite_refs\n" +
						"import dep f/composite_refs/dep\ntype Holder" + strings.Repeat(" dep.Target", 3) + "\n")},
					"f/composite_refs/dep/d.zz": {Data: []byte("package f/composite_refs/dep\ntype Target string\n")},
				}
				g, _, err := load.Load(context.Background(), load.Config{
					FS:        tree,
					Frontends: []plugin.Frontend{frontendtest.NewScripted()},
					Sink:      diag.NewSink(),
					Brand:     frontendtest.Brand,
				})
				assert.NoError(t, err, "the flat holder loads")

				c := conformance.Corpus{Frontend: frontendtest.NewScripted(), Sources: tree}
				msg := assert.Rejects(t, "a holder whose references are no composites", func(tb assert.TB) {
					conformance.AssertFeature(tb, c, g, featureByID(t, "composite_refs"))
				})
				assert.Contains(t, msg, "below its reference's root", "naming the composite the field lacks")
			})
	})
}
