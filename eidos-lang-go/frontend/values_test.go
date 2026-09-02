// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// stampsOf collects one parse's stamps by key, keyed by the
// subject-free value order they were recorded in.
func stampsOf(gb *plugin.GraphBuilder, key string) []any {
	var out []any
	for _, s := range gb.StampRecords() {
		if string(s.Stamp.Key) == key {
			out = append(out, s.Stamp.Value)
		}
	}
	return out
}

// Constant evaluation is the package's own scope through the
// checker's machinery, so the exact values and the refusals to
// guess are pinned together.
func TestStampConstValues(t *testing.T) {
	t.Parallel()

	t.Run("evaluates iota arithmetic exactly", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\nconst (\n\ta = 1 << (iota * 2)\n\tb\n\tc\n)\n")
		values := stampsOf(gb, string(golang.ConstValueKey))
		assert.Equal(t, values, []any{"1", "4", "16"},
			"each row's exact value, the implicit carriers included")
	})

	t.Run("stamps promoted variants under their enum", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype Color int\n\nconst (\n\tRed Color = iota\n\tGreen\n)\n")
		values := stampsOf(gb, string(golang.ConstValueKey))
		assert.Equal(t, values, []any{"0", "1"}, "the ordinals survive the promotion")
	})

	t.Run("leaves a cross-package constant unstamped", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\nimport \"example.test/far\"\n\nconst near = 2\n\nconst carried = far.Base + 1\n")
		values := stampsOf(gb, string(golang.ConstValueKey))
		assert.Equal(t, values, []any{"2"},
			"what the package cannot evaluate stays absent, never wrong")
	})
}
