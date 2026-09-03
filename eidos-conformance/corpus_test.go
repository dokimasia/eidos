// Copyright ThesmOS B.V. 2026
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

// scriptedCorpus is the scripted language's entry: the corpus tree
// under testdata, and a coverage refusing what the language cannot
// spell.
func scriptedCorpus() conformance.Corpus {
	return conformance.Corpus{
		Frontend: frontendtest.NewScripted(),
		Sources:  os.DirFS("testdata/zz"),
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

// The armature must hold its own reference language: every covered
// feature's expectations, every refusal's absence, the whole
// frontend bar.
func TestRun(t *testing.T) {
	t.Parallel()

	conformance.Run(t, scriptedCorpus())
}

// The totality and refusal checks exist to catch silent languages,
// so the silences are simulated and each rejection asserted.
func TestCoverage(t *testing.T) {
	t.Parallel()

	t.Run("rejects a feature without a verdict", func(t *testing.T) {
		t.Parallel()

		gapped := scriptedCorpus()
		delete(gapped.Coverage, "interfaces")
		msg := assert.Rejects(t, "a coverage silent on one feature", func(tb assert.TB) {
			conformance.AssertCoveredInventory(tb, gapped)
		})
		assert.Contains(t, msg, "interfaces", "naming the silence")
	})

	t.Run("rejects a verdict outside the inventory", func(t *testing.T) {
		t.Parallel()

		stray := scriptedCorpus()
		stray.Coverage["warp_drives"] = conformance.Loads
		msg := assert.Rejects(t, "a verdict naming no feature", func(tb assert.TB) {
			conformance.AssertCoveredInventory(tb, stray)
		})
		assert.Contains(t, msg, "warp_drives", "naming the stray")
	})

	t.Run("holds the verdict set to what the corpus may state", func(t *testing.T) {
		t.Parallel()

		ruled := scriptedCorpus()
		ruled.Rules = rulestest.Scripted()
		msg := assert.Rejects(t, "loads under a corpus with rules", func(tb assert.TB) {
			conformance.AssertCoveredInventory(tb, ruled)
		})
		assert.Contains(t, msg, "states the level", "a corpus that projects states the level")

		leveled := scriptedCorpus()
		leveled.Coverage["struct_fields"] = conformance.Projects
		msg = assert.Rejects(t, "a level under a corpus without rules", func(tb assert.TB) {
			conformance.AssertCoveredInventory(tb, leveled)
		})
		assert.Contains(t, msg, "without rules", "which nothing can evaluate")

		strayRemainder := scriptedCorpus()
		strayRemainder.Remainder = map[string][]conformance.Remainder{"warp_drives": nil}
		msg = assert.Rejects(t, "a remainder naming no feature", func(tb assert.TB) {
			conformance.AssertCoveredInventory(tb, strayRemainder)
		})
		assert.Contains(t, msg, "warp_drives", "naming the stray")
	})

	t.Run("rejects a spelling under a refusal", func(t *testing.T) {
		t.Parallel()

		contradicted := scriptedCorpus()
		contradicted.Coverage["struct_fields"] = conformance.Refuses
		msg := assert.Rejects(t, "a refusal the tree contradicts", func(tb assert.TB) {
			conformance.AssertRefusedFeature(tb, contradicted, store.New(),
				featureByID(t, "struct_fields"))
		})
		assert.Contains(t, msg, "struct_fields", "naming the contradiction")
	})

	t.Run("rejects a refusal the load contradicts", func(t *testing.T) {
		t.Parallel()

		// The tree is empty under the feature, so only the graph can
		// catch the spelling: the corpus convention's package stands
		// in the load whatever file declared it.
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
		assert.Contains(t, msg, "f/interfaces", "naming the package that stood")
	})
}

// featureByID returns one inventory feature.
func featureByID(tb assert.TB, id string) conformance.Feature {
	tb.Helper()

	for _, f := range conformance.Inventory() {
		if f.ID == id {
			return f
		}
	}
	tb.Fatalf("the inventory holds no %s", id)
	return conformance.Feature{}
}
