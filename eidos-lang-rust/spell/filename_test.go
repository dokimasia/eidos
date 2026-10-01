// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The naming is total over the units a plan admits, and every
// spelling is pinned: a Rust filename is a module name, so a drift
// here breaks every mod declaration that names it.
func TestFilename(t *testing.T) {
	t.Parallel()

	t.Run("Filename", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Unit
			want string
		}{
			{
				name: "returns the stem and the word of a per-source unit",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub"},
				want: "store_stub.rs",
			},
			{
				name: "returns a pascal stem in snake case",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/UserStore.java", Word: "stub"},
				want: "user_store_stub.rs",
			},
			{
				name: "returns the tag after the word",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub", Tag: "grpc"},
				want: "store_stub_grpc.rs",
			},
			{
				name: "returns the word alone for a per-package unit",
				give: plugin.Unit{Per: plugin.PerPackage, Key: "svc/api", Word: "stub"},
				want: "stub.rs",
			},
			{
				name: "returns the word alone for a plan unit",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "registry"},
				want: "registry.rs",
			},
			{
				name: "returns an acronym in the word as one name",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "HTTPClient"},
				want: "http_client.rs",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, spell.Filename(tt.give), tt.want, "the filename")
			})
		}
	})
}
