// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package edit_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/edit"
)

// The spellings of the Near cases: a brand, a spelling that two of its
// letters swapped, and the name of another tool.
const (
	brand   = "dokimi"
	swapped = "dokmii"
	tool    = "go"
)

func TestEdit(t *testing.T) {
	t.Parallel()

	t.Run("Distance", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			giveA      string
			giveB      string
			wantEdited int
		}{
			{name: "returns 0 for two equal spellings", giveA: brand, giveB: brand, wantEdited: 0},
			{name: "returns the length of the other spelling for an empty one", giveA: "", giveB: brand, wantEdited: 6},
			{name: "counts an insertion as one edit", giveA: "dokmi", giveB: brand, wantEdited: 1},
			{name: "counts a deletion as one edit", giveA: "dokimii", giveB: brand, wantEdited: 1},
			{name: "counts a substitution as one edit", giveA: "dokimy", giveB: brand, wantEdited: 1},
			{name: "counts a swap of two adjacent bytes as one edit", giveA: swapped, giveB: brand, wantEdited: 1},
			{name: "edits no byte twice", giveA: "ca", giveB: "abc", wantEdited: 3},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, edit.Distance(tt.giveA, tt.giveB), tt.wantEdited, "the distance of the two spellings")
			})
		}
	})

	t.Run("Near", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			giveA      string
			giveB      string
			wantEdited int
			wantNear   bool
		}{
			{
				name:       "reports a spelling one edit away as near",
				giveA:      swapped,
				giveB:      brand,
				wantEdited: 1,
				wantNear:   true,
			},
			{
				name:       "reports a spelling two edits away as near",
				giveA:      "dokim",
				giveB:      "dokimiy",
				wantEdited: 2,
				wantNear:   true,
			},
			{name: "reports a spelling three edits away as far", giveA: "dok", giveB: brand, wantNear: false},
			{
				name:     "reports a spelling whose edits pass a third of the longer as far",
				giveA:    "acmx",
				giveB:    "axmy",
				wantNear: false,
			},
			{name: "reports the name of another tool as far", giveA: tool, giveB: brand, wantNear: false},
			{
				name:       "reports a spelling within a third of the longer as near",
				giveA:      "acmx",
				giveB:      "acme",
				wantEdited: 1,
				wantNear:   true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				edited, near := edit.Near(tt.giveA, tt.giveB)
				assert.Equal(t, near, tt.wantNear, "whether the spellings are near")
				if tt.wantNear {
					assert.Equal(t, edited, tt.wantEdited, "the distance of a near spelling")
				}
			})
		}
	})
}

// Near computes no distance for two spellings whose lengths differ by more
// than the bound, so the check of each comment line of a parse allocates
// nothing for the mark of another tool.
func TestEditAllocs(t *testing.T) {
	assert.MaxAllocs(t, func() { edit.Near(tool, brand) }, 0, "Near of two lengths too far apart allocates nothing")
}

func BenchmarkEdit(b *testing.B) {
	b.Run("Near", func(b *testing.B) {
		b.Run("far", func(b *testing.B) {
			var near bool
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			for c.Loop() {
				_, near = edit.Near(tool, brand)
			}
			assert.False(b, near, "the name of another tool is far from the brand")
		})

		b.Run("near", func(b *testing.B) {
			var near bool
			c := bench.Start(b).MaxAllocs(3)
			defer c.End()
			for c.Loop() {
				_, near = edit.Near(swapped, brand)
			}
			assert.True(b, near, "a swap is near the brand")
		})
	})
}
