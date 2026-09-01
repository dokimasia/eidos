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
// holds, so a drift here renames types, not just files.
func TestFilename(t *testing.T) {
	t.Parallel()

	t.Run("a lone type names its file", func(t *testing.T) {
		t.Parallel()

		u := plugin.Unit{
			Per: plugin.PerSource, Key: "svc/types.src", Word: "gen",
			Decls: []symbol.Symbol{&emit.Struct{Name: "Row"}},
		}
		assert.Equal(t, spell.Filename(u), "Row.java",
			"the type's own name, whatever the key and word spell")
	})

	tests := []struct {
		name string
		unit plugin.Unit
		want string
	}{
		{
			name: "a source unit joins stem and word as one type name",
			unit: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub"},
			want: "StoreStub.java",
		},
		{
			name: "a snake stem converts into the type name",
			unit: plugin.Unit{Per: plugin.PerSource, Key: "svc/user_profile.ts", Word: "stub"},
			want: "UserProfileStub.java",
		},
		{
			name: "a tag joins after the word",
			unit: plugin.Unit{
				Per: plugin.PerSource, Key: "svc/store.go", Word: "stub", Tag: "grpc",
			},
			want: "StoreStubGrpc.java",
		},
		{
			name: "a plan unit is the word alone",
			unit: plugin.Unit{Per: plugin.PerPlan, Word: "registry"},
			want: "Registry.java",
		},
		{
			name: "an acronym run in the word survives",
			unit: plugin.Unit{Per: plugin.PerPlan, Word: "HTTPClient"},
			want: "HTTPClient.java",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, spell.Filename(tt.unit), tt.want, tt.name)
		})
	}
}
