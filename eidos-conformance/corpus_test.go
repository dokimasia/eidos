// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"os"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontendtest"
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

	t.Run("rejects a spelling under a refusal", func(t *testing.T) {
		t.Parallel()

		contradicted := scriptedCorpus()
		contradicted.Coverage["struct_fields"] = conformance.Refuses
		msg := assert.Rejects(t, "a refusal the tree contradicts", func(tb assert.TB) {
			conformance.AssertRefusedFeature(tb, contradicted, featureByID(t, "struct_fields"))
		})
		assert.Contains(t, msg, "struct_fields", "naming the contradiction")
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
