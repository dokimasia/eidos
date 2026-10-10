// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/plugin"
)

// packageSegment is what an inventory id spells: lowercase words joined
// by underscores, a package segment in every language.
const packageSegment = `^[a-z][a-z0-9_]*$`

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

			assert.NoDuplicates(t, func() ([]string, error) {
				inventory := conformance.Inventory()
				ids := make([]string, 0, len(inventory))
				for _, f := range inventory {
					ids = append(ids, f.ID)
				}
				return ids, nil
			}, "the inventory lists each id once")
		})

		t.Run("returns ids that spell as package segments", func(t *testing.T) {
			t.Parallel()

			for _, f := range conformance.Inventory() {
				expect.Matches(t, f.ID, packageSegment, f.ID+" spells as a package segment in every language")
			}
		})

		t.Run("returns features that state what they exercise", func(t *testing.T) {
			t.Parallel()

			for _, f := range conformance.Inventory() {
				expect.NotEmpty(t, f.Doc, f.ID+" states what it exercises")
			}
		})

		t.Run("returns an expectation for every feature", func(t *testing.T) {
			t.Parallel()

			for _, f := range conformance.Inventory() {
				expect.True(t, len(f.Declares) > 0 || f.Check != nil, f.ID+" states what a spelling must load")
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
				g, _, err := load.Load(t.Context(), load.Config{
					FS:        tree,
					Frontends: []plugin.Frontend{frontendtest.NewScripted()},
					Sink:      diag.NewSink(),
					Brand:     frontendtest.Brand,
				})
				assert.NoError(t, err, "the flat holder loads")

				c := conformance.Corpus{Frontend: frontendtest.NewScripted(), Sources: tree}
				got := assert.Rejects(t, "a holder whose references are no composites", func(tb assert.TB) {
					conformance.AssertFeature(tb, c, g, featureByID(t, "composite_refs"))
				})
				assert.Equal(t, contracts(got), []string{
					"f0 resolves to the sibling's declaration below its reference's root",
					"f1 resolves to the sibling's declaration below its reference's root",
					"f2 resolves to the sibling's declaration below its reference's root",
				}, "naming each field that lacks the composite")
			})

		t.Run("returns a directive_sugar check that rejects a declaration without the marker's directive",
			func(t *testing.T) {
				t.Parallel()

				// The scripted language has no markers, so its Table has no
				// directive.
				tree := fstest.MapFS{
					"f/directive_sugar/a.zz": {Data: []byte("package f/directive_sugar\ntype Table string\n")},
				}
				g, _, err := load.Load(t.Context(), load.Config{
					FS:        tree,
					Frontends: []plugin.Frontend{frontendtest.NewScripted()},
					Sink:      diag.NewSink(),
					Brand:     frontendtest.Brand,
				})
				assert.NoError(t, err, "the bare table loads")

				c := conformance.Corpus{Frontend: frontendtest.NewScripted(), Sources: tree}
				got := assert.Rejects(t, "a table without the marker's directive", func(tb assert.TB) {
					conformance.AssertFeature(tb, c, g, featureByID(t, "directive_sugar"))
				})
				assert.Equal(t, contracts(got), []string{"the marker's directive attaches"},
					"the check fails at the missing directive")
			})
	})
}
