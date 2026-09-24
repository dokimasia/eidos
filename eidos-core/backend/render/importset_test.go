// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
)

// The set is the one meeting point between spelling and the import
// block: deduplicated, sorted, per file.
func TestImportSet(t *testing.T) {
	t.Parallel()

	t.Run("dedupes and sorts", func(t *testing.T) {
		t.Parallel()

		var s render.ImportSet
		s.Add("zeta")
		s.Add("alpha")
		s.Add("zeta")
		assert.Equal(t, s.Paths(), []string{"alpha", "zeta"},
			"one mention per path, in path order")
		assert.Equal(t, s.Len(), 2, "the count agrees")
	})

	t.Run("binds names beside paths", func(t *testing.T) {
		t.Parallel()

		var s render.ImportSet
		s.AddNamed("svc/store", "Store")
		s.AddNamed("svc/store", "Row")
		s.AddNamed("svc/store", "Row")
		s.Add("svc/store")
		s.Add("side/effect")
		assert.Equal(t, s.Entries(), []render.Entry{
			{Path: "side/effect"},
			{Path: "svc/store"},
			{Path: "svc/store", Name: "Row"},
			{Path: "svc/store", Name: "Store"},
		}, "entries dedupe and sort by path then name, the bare form first")
		assert.Equal(t, s.Paths(), []string{"side/effect", "svc/store"},
			"paths stay distinct whatever names bind under them")
		assert.Equal(t, s.Len(), 2, "the count follows the paths")
	})

	t.Run("keeps type-only bindings apart from value bindings", func(t *testing.T) {
		t.Parallel()

		var s render.ImportSet
		s.AddType("svc/store", "Row")
		s.AddNamed("svc/store", "Row")
		s.AddType("svc/store", "Row")
		assert.Equal(t, s.Entries(), []render.Entry{
			{Path: "svc/store", Name: "Row"},
			{Path: "svc/store", Name: "Row", TypeOnly: true},
		}, "one name binds twice, the value form first, so a renderer's "+
			"join reads the pair in one pass")
	})

	t.Run("records nothing under the file's own package", func(t *testing.T) {
		t.Parallel()

		var s render.ImportSet
		s.SetHome("svc/store")
		s.Add("svc/store")
		s.AddNamed("svc/store", "Row")
		s.AddType("svc/store", "Store")
		s.Add("svc/audit")
		assert.Equal(t, s.Home(), "svc/store", "the set names the file's package")
		assert.Equal(t, s.Paths(), []string{"svc/audit"}, "and only the other package imports")
	})

	t.Run("resets for the next file", func(t *testing.T) {
		t.Parallel()

		var s render.ImportSet
		s.Add("alpha")
		s.AddNamed("alpha", "A")
		s.Reset()
		assert.Equal(t, s.Len(), 0, "a reset set holds nothing")
		s.Add("beta")
		assert.Equal(t, s.Paths(), []string{"beta"}, "and records again")
	})
}
