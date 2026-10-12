// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The candidates that the merge cases zip: a declaration of the module, a
// declaration of an outer scope, and a declaration that an import binds.
var (
	ownUser      = symbol.Identity{Lang: "typescript", Package: "src/models", Name: "User"}
	outerUser    = symbol.Identity{Lang: "typescript", Package: "src", Name: "User"}
	importedUser = symbol.Identity{Lang: "typescript", Package: "src/auth", Name: "User"}
)

// zipped is the two sources that the allocation check and the benchmark
// zip: two tiers of the module and one tier of its imports.
var zipped = []plugin.Candidates{{{ownUser}, {outerUser}}, {{importedUser}}}

// zipAllocs is the zip of zipped: the list of tiers and the two merged
// tiers.
const zipAllocs = 3

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
			assert.NotEqual(t, plugin.DepthFull, plugin.DepthSignatures, "the two depths key differently")
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

	t.Run("Zip", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give []plugin.Candidates
			want plugin.Candidates
		}{
			{
				name: "merges the tiers of the sources by position",
				give: []plugin.Candidates{{{ownUser}, {outerUser}}, {{importedUser}}},
				want: plugin.Candidates{{ownUser, importedUser}, {outerUser}},
			},
			{
				name: "keeps a merged tier without candidates nil",
				give: []plugin.Candidates{{nil, {outerUser}}, {{}}},
				want: plugin.Candidates{nil, {outerUser}},
			},
			{name: "returns nil for sources without tiers", give: []plugin.Candidates{nil, {}}},
			{name: "returns nil for no sources"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, plugin.Zip(tt.give...), tt.want, "the tiers are merged by position in source order")
			})
		}

		t.Run("returns tiers that a later change of a source does not alter", func(t *testing.T) {
			t.Parallel()

			own := plugin.Candidates{{ownUser}}
			got := plugin.Zip(own)
			own[0][0] = outerUser
			assert.Equal(t, got, plugin.Candidates{{ownUser}}, "the merged tier is a copy of the source's tier")
		})
	})
}

// TestFrontendAllocs checks that Zip allocates each merged tier once, in
// the ordinary run, which runs no benchmark. The check runs alone, because
// the count includes every goroutine's allocations.
func TestFrontendAllocs(t *testing.T) {
	var got plugin.Candidates
	assert.MaxAllocs(t, func() { got = plugin.Zip(zipped...) }, zipAllocs,
		"Zip allocates the list of tiers and each merged tier once")
	assert.Length(t, got, 2, "Zip returns the two merged tiers")
}

// BenchmarkFrontend measures the merge of two sources of candidates.
func BenchmarkFrontend(b *testing.B) {
	b.Run("Zip", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(zipAllocs)
		defer c.End()
		var got plugin.Candidates
		for c.Loop() {
			got = plugin.Zip(zipped...)
		}
		assert.Length(b, got, 2, "Zip returns the two merged tiers")
	})
}
