// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// publishing publishes one name it does not declare, the shape of a
// TypeScript barrel module.
type publishing struct{}

// Exports returns the declaring module's candidate for User.
func (publishing) Exports(_ plugin.ImportScope, name string) plugin.Candidates {
	return plugin.Candidates{{{Lang: "typescript", Package: "src/models/user", Name: name}}}
}

// The frontend SPI's data contracts: the depth vocabulary and the
// records the load driver passes across it.
func TestFrontend(t *testing.T) {
	t.Parallel()

	t.Run("Depth", func(t *testing.T) {
		t.Parallel()

		t.Run("the zero value loads everything", func(t *testing.T) {
			t.Parallel()

			var depth plugin.Depth
			assert.Equal(t, depth, plugin.DepthFull,
				"a unit built without a stated depth loads full")
			assert.True(t, plugin.DepthFull != plugin.DepthSignatures,
				"the two depths key differently")
		})
	})

	t.Run("SourceRef", func(t *testing.T) {
		t.Parallel()

		t.Run("names a file without opening it", func(t *testing.T) {
			t.Parallel()

			ref := plugin.SourceRef{Path: "svc/store/row.go", Shared: []string{"go.mod"}}
			assert.Equal(t, ref.Path, "svc/store/row.go", "the workspace-relative path")
			assert.Equal(t, ref.Shared, []string{"go.mod"},
				"the declared inputs whose bytes fold into dependent units")
		})
	})

	t.Run("Exporter", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the candidates a published name could mean", func(t *testing.T) {
			t.Parallel()

			var role plugin.Exporter = publishing{}
			got := role.Exports(plugin.ImportScope{}, "User")
			assert.Equal(t, got, plugin.Candidates{{{Lang: "typescript", Package: "src/models/user", Name: "User"}}},
				"in the language's shadowing tiers")
		})
	})

	t.Run("ImportScope", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps the language's own bindings in their own form", func(t *testing.T) {
			t.Parallel()

			scope := plugin.ImportScope{
				File:     symbol.Identity{Lang: "golang", Package: "svc/store", Kind: symbol.KindFile},
				Bindings: map[string]string{"emit": "core/emit"},
			}
			bound, is := scope.Bindings.(map[string]string)
			assert.True(t, is, "the kernel hands the record back untyped")
			assert.Equal(t, bound["emit"], "core/emit", "in the language's own form")
		})
	})
}
