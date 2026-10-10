// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"os"
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

	t.Run("Verdict", func(t *testing.T) {
		t.Parallel()

		t.Run("String", func(t *testing.T) {
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
	})

	t.Run("AssertCoveredInventory", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give func(c *conformance.Corpus)
			want []string
		}{
			{
				name: "fails a coverage without a verdict for a feature",
				give: func(c *conformance.Corpus) { delete(c.Coverage, "interfaces") },
				want: []string{"the coverage states a verdict for interfaces: " +
					"silence on a capability is the gap this list exists to close"},
			},
			{
				name: "fails a verdict for a feature outside the inventory",
				give: func(c *conformance.Corpus) { c.Coverage["warp_drives"] = conformance.Loads },
				want: []string{"the inventory lists warp_drives, which the coverage names"},
			},
			{
				name: "fails a projection level under a corpus without rules",
				give: func(c *conformance.Corpus) { c.Coverage["struct_fields"] = conformance.Projects },
				want: []string{"struct_fields states projects: a corpus without rules states loads or refuses"},
			},
			{
				name: "fails a remainder for a feature outside the inventory",
				give: func(c *conformance.Corpus) {
					c.Remainder = map[string][]conformance.Remainder{"warp_drives": nil}
				},
				want: []string{"the inventory lists warp_drives, which the remainder names"},
			},
			{
				name: "reports stray remainders in id order",
				give: func(c *conformance.Corpus) {
					c.Remainder = map[string][]conformance.Remainder{}
					for _, id := range []string{"zz_c", "zz_a", "zz_e", "zz_b", "zz_d"} {
						c.Remainder[id] = nil
					}
				},
				want: []string{
					"the inventory lists zz_a, which the remainder names",
					"the inventory lists zz_b, which the remainder names",
					"the inventory lists zz_c, which the remainder names",
					"the inventory lists zz_d, which the remainder names",
					"the inventory lists zz_e, which the remainder names",
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				c := scriptedCorpus()
				tt.give(&c)
				got := assert.Rejects(t, "the coverage check rejects the corpus", func(tb assert.TB) {
					conformance.AssertCoveredInventory(tb, c)
				})
				assert.Equal(t, contracts(got), tt.want, "naming each feature that breaks a contract, in order")
			})
		}

		t.Run("fails a load verdict under a corpus with rules", func(t *testing.T) {
			t.Parallel()

			ruled := scriptedCorpus()
			ruled.Rules = rulestest.Scripted()
			got := assert.Rejects(t, "loads under a corpus with rules", func(tb assert.TB) {
				conformance.AssertCoveredInventory(tb, ruled)
			})
			assert.Contains(t, contracts(got),
				"struct_fields states loads: a corpus with rules states projects, projects partly, opaque or refuses",
				"a corpus that projects states the level of a feature that loads")
		})
	})

	t.Run("AssertRefusedFeature", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a refusal the tree contradicts", func(t *testing.T) {
			t.Parallel()

			contradicted := scriptedCorpus()
			contradicted.Coverage["struct_fields"] = conformance.Refuses
			got := assert.Rejects(t, "a refusal the tree contradicts", func(tb assert.TB) {
				conformance.AssertRefusedFeature(tb, contradicted, store.New(),
					featureByID(t, "struct_fields"))
			})
			assert.Equal(t, contracts(got), []string{
				"struct_fields is refused, so the tree spells no file under f/struct_fields: f/struct_fields/a.zz",
			}, "naming the file that contradicts the refusal")
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

			got := assert.Rejects(t, "a refusal the load contradicts", func(tb assert.TB) {
				conformance.AssertRefusedFeature(tb, hidden, g, featureByID(t, "interfaces"))
			})
			assert.Equal(t, contracts(got), []string{
				"interfaces is refused, so the load contains no package it occupies, wherever the spelling is",
			}, "for the load's contradiction")
			assert.Equal(t, got[0].Detail["needle"], any("f/interfaces"), "naming the package the load contains")
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
			"directive_sugar":     conformance.Refuses,
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
