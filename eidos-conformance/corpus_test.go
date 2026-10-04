// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"os"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// scriptedTree is the scripted language's corpus tree.
const scriptedTree = "testdata/zz"

// The scripted language passes its run against the shared inventory,
// and the totality and refusal checks fail the silences they exist to
// catch. Each real language's run is in its package under lang.
func TestCorpus(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the scripted language's corpus", func(t *testing.T) {
			t.Parallel()

			conformance.Run(t, scriptedCorpus())
		})
	})

	t.Run("Verdict.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give conformance.Verdict
			want string
		}{
			{name: "returns loads for Loads", give: conformance.Loads, want: "loads"},
			{name: "returns refuses for Refuses", give: conformance.Refuses, want: "refuses"},
			{name: "returns projects for Projects", give: conformance.Projects, want: "projects"},
			{
				name: "returns projects partly for ProjectsPartly",
				give: conformance.ProjectsPartly,
				want: "projects partly",
			},
			{name: "returns opaque for Opaque", give: conformance.Opaque, want: "opaque"},
			{name: "returns the number of a value outside the set", give: conformance.Verdict(9), want: "9"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the verdict's spelling")
			})
		}
	})

	t.Run("AssertCoveredInventory", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a coverage without a verdict for a feature", func(t *testing.T) {
			t.Parallel()

			gapped := scriptedCorpus()
			delete(gapped.Coverage, "interfaces")
			msg := assert.Rejects(t, "a coverage silent on one feature", func(tb assert.TB) {
				conformance.AssertCoveredInventory(tb, gapped)
			})
			assert.Contains(t, msg, "interfaces", "naming the silence")
		})

		t.Run("fails a verdict for a feature outside the inventory", func(t *testing.T) {
			t.Parallel()

			stray := scriptedCorpus()
			stray.Coverage["warp_drives"] = conformance.Loads
			msg := assert.Rejects(t, "a verdict naming no feature", func(tb assert.TB) {
				conformance.AssertCoveredInventory(tb, stray)
			})
			assert.Contains(t, msg, "warp_drives", "naming the stray")
		})

		t.Run("fails a load verdict under a corpus with rules", func(t *testing.T) {
			t.Parallel()

			ruled := scriptedCorpus()
			ruled.Rules = rulestest.Scripted()
			msg := assert.Rejects(t, "loads under a corpus with rules", func(tb assert.TB) {
				conformance.AssertCoveredInventory(tb, ruled)
			})
			assert.Contains(t, msg, "states the level", "a corpus that projects states the level")
		})

		t.Run("fails a projection level under a corpus without rules", func(t *testing.T) {
			t.Parallel()

			leveled := scriptedCorpus()
			leveled.Coverage["struct_fields"] = conformance.Projects
			msg := assert.Rejects(t, "a level under a corpus without rules", func(tb assert.TB) {
				conformance.AssertCoveredInventory(tb, leveled)
			})
			assert.Contains(t, msg, "without rules", "which nothing can evaluate")
		})

		t.Run("fails a remainder for a feature outside the inventory", func(t *testing.T) {
			t.Parallel()

			strayRemainder := scriptedCorpus()
			strayRemainder.Remainder = map[string][]conformance.Remainder{"warp_drives": nil}
			msg := assert.Rejects(t, "a remainder naming no feature", func(tb assert.TB) {
				conformance.AssertCoveredInventory(tb, strayRemainder)
			})
			assert.Contains(t, msg, "warp_drives", "naming the stray")
		})

		t.Run("reports stray remainders in id order", func(t *testing.T) {
			t.Parallel()

			strays := []string{"zz_a", "zz_b", "zz_c", "zz_d", "zz_e"}
			scattered := scriptedCorpus()
			scattered.Remainder = map[string][]conformance.Remainder{}
			for _, id := range strays {
				scattered.Remainder[id] = nil
			}
			rec := assert.NewRecorder()
			conformance.AssertCoveredInventory(rec, scattered)
			reported := make([]string, 0, len(strays))
			for _, msg := range rec.Messages() {
				for _, id := range strays {
					if strings.Contains(msg, id) {
						reported = append(reported, id)
					}
				}
			}
			assert.Equal(t, reported, strays,
				"one finding per stray, in id order whatever order the map ranges in")
		})
	})

	t.Run("AssertRefusedFeature", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a refusal the tree contradicts", func(t *testing.T) {
			t.Parallel()

			contradicted := scriptedCorpus()
			contradicted.Coverage["struct_fields"] = conformance.Refuses
			msg := assert.Rejects(t, "a refusal the tree contradicts", func(tb assert.TB) {
				conformance.AssertRefusedFeature(tb, contradicted, store.New(),
					featureByID(t, "struct_fields"))
			})
			assert.Contains(t, msg, "struct_fields", "naming the contradiction")
		})

		t.Run("fails a refusal the load contradicts", func(t *testing.T) {
			t.Parallel()

			// The tree is empty under the feature, so only the graph can
			// catch the spelling: the load contains the corpus convention's
			// package whatever file declared it.
			hidden := scriptedCorpus()
			hidden.Coverage["interfaces"] = conformance.Refuses
			g := store.New()
			assert.NoError(t, g.AddPackage(&node.Package{
				ID: symbol.Identity{
					Lang: frontendtest.ScriptedLang, Package: "f/interfaces",
					Kind: symbol.KindPackage,
				},
				Path: []string{"f", "interfaces"},
			}), "the hidden package is admitted")
			g.Freeze()

			msg := assert.Rejects(t, "a refusal the load contradicts", func(tb assert.TB) {
				conformance.AssertRefusedFeature(tb, hidden, g, featureByID(t, "interfaces"))
			})
			assert.Contains(t, msg, "f/interfaces", "naming the package the load contains")
		})
	})
}

// scriptedCorpus is the scripted language's entry: the corpus tree
// under testdata, and a coverage refusing what the language cannot
// spell.
func scriptedCorpus() conformance.Corpus {
	return conformance.Corpus{
		Frontend: frontendtest.NewScripted(),
		Sources:  os.DirFS(scriptedTree),
		Coverage: conformance.Coverage{
			"struct_fields":       conformance.Loads,
			"struct_methods":      conformance.Loads,
			"method_overloads":    conformance.Loads,
			"constants":           conformance.Loads,
			"cross_package_ref":   conformance.Loads,
			"composite_refs":      conformance.Refuses,
			"builtin_ref":         conformance.Loads,
			"directive_carrier":   conformance.Loads,
			"test_classification": conformance.Loads,
			"interfaces":          conformance.Refuses,
			"enum_values":         conformance.Refuses,
		},
		Schemas: frontendtest.ScriptedSchemas(),
		Keys:    frontendtest.ScriptedKeys,
	}
}

// featureByID returns one inventory feature.
func featureByID(tb assert.TB, id string) conformance.Feature {
	tb.Helper()

	for _, f := range conformance.Inventory() {
		if f.ID == id {
			return f
		}
	}
	tb.Fatalf("the inventory lists no %s", id)
	return conformance.Feature{}
}
