// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The naming is total over the units a plan admits, and every
// spelling is pinned: Java names a file after the public type it
// declares, so a drift here renames types, not just files.
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
				name: "returns a lone struct's name whatever the key and word spell",
				give: plugin.Unit{
					Per: plugin.PerSource, Key: "svc/types.src", Word: "gen",
					Decls: []symbol.Symbol{&emit.Struct{Name: "Row"}},
				},
				want: "Row.java",
			},
			{
				name: "returns a lone enum's name",
				give: plugin.Unit{
					Per: plugin.PerSource, Key: "svc/types.src", Word: "gen",
					Decls: []symbol.Symbol{&emit.Enum{Name: "Phase"}},
				},
				want: "Phase.java",
			},
			{
				name: "returns the stem and the word of a per-source unit as one type name",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub"},
				want: "StoreStub.java",
			},
			{
				name: "returns a snake stem in the type name's case",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/user_profile.ts", Word: "stub"},
				want: "UserProfileStub.java",
			},
			{
				name: "returns the tag after the word",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub", Tag: "grpc"},
				want: "StoreStubGrpc.java",
			},
			{
				name: "returns the word alone for a per-package unit",
				give: plugin.Unit{Per: plugin.PerPackage, Key: "svc/api", Word: "registry"},
				want: "Registry.java",
			},
			{
				name: "returns the word alone for a plan unit",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "registry"},
				want: "Registry.java",
			},
			{
				name: "returns an acronym run in the word as written",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "HTTPClient"},
				want: "HTTPClient.java",
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
