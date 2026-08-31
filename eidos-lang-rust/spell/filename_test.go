// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/lang-rust/spell"
)

// The naming is total over the units a plan admits, and every
// spelling is pinned: a Rust filename is a module name, so a
// drift here breaks every mod declaration that names it.
func TestFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		unit plugin.Unit
		want string
	}{
		{
			name: "a source unit joins stem and word",
			unit: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub"},
			want: "store_stub.rs",
		},
		{
			name: "a pascal stem converts to snake",
			unit: plugin.Unit{Per: plugin.PerSource, Key: "svc/UserStore.java", Word: "stub"},
			want: "user_store_stub.rs",
		},
		{
			name: "a tag joins after the word",
			unit: plugin.Unit{
				Per: plugin.PerSource, Key: "svc/store.go", Word: "stub", Tag: "grpc",
			},
			want: "store_stub_grpc.rs",
		},
		{
			name: "a package unit stems from the package path",
			unit: plugin.Unit{Per: plugin.PerPackage, Key: "svc/api", Word: "stub"},
			want: "api_stub.rs",
		},
		{
			name: "a plan unit is the word alone",
			unit: plugin.Unit{Per: plugin.PerPlan, Word: "registry"},
			want: "registry.rs",
		},
		{
			name: "an acronym in the word converts as one name",
			unit: plugin.Unit{Per: plugin.PerPlan, Word: "HTTPClient"},
			want: "http_client.rs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, spell.Filename(tt.unit), tt.want, tt.name)
		})
	}
}
