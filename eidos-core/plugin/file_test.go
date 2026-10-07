// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// spelling is a fixture target that files every unit whole and names
// each file after its file key and word.
type spelling struct {
	named
}

// SplitUnit returns the unit whole.
func (spelling) SplitUnit(u plugin.Unit) []plugin.Unit { return []plugin.Unit{u} }

// FileName joins the unit's file key and word.
func (spelling) FileName(u plugin.Unit) string { return u.FileKey() + "." + u.Word }

// A routed file is the render's unit of work, and the filename half
// is a surface a target opts into.
func TestFile(t *testing.T) {
	t.Parallel()

	t.Run("File", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the zero package for a file no target derived a package for", func(t *testing.T) {
			t.Parallel()

			var f plugin.File
			expect.Equal(t, f.Pkg, symbol.Identity{}, "a zero file declares no package")
			expect.Empty(t, f.Units, "and assembles no unit")
		})
	})

	t.Run("FileSpeller", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name a target's speller spells", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = spelling{name: "speller"}
			s, spells := p.(plugin.FileSpeller)
			assert.True(t, spells, "the filename surface asserts")
			u := plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub"}
			assert.Length(t, s.SplitUnit(u), 1, "the fixture files the unit whole")
			assert.Equal(t, s.FileName(u), "svc/store.go.stub", "the name joins the file key and the word")
		})

		t.Run("reports false asserting a plugin that spells nothing", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = named{name: "bare"}
			_, spells := p.(plugin.FileSpeller)
			assert.False(t, spells, "the filename surface is opt-in")
		})
	})
}
