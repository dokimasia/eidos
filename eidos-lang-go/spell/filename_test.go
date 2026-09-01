// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang-go/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The spelling is total over the units a plan admits, and every
// case is pinned: files are addressed by name, so a drift here
// orphans previously generated files.
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
			want: "store_stub.go",
		},
		{
			name: "the stem drops the source extension whatever it is",
			unit: plugin.Unit{Per: plugin.PerSource, Key: "svc/user.proto", Word: "stub"},
			want: "user_stub.go",
		},
		{
			name: "a tag joins after the word",
			unit: plugin.Unit{
				Per: plugin.PerSource, Key: "svc/store.go", Word: "stub", Tag: "grpc",
			},
			want: "store_stub_grpc.go",
		},
		{
			name: "a package unit stems from the package path",
			unit: plugin.Unit{Per: plugin.PerPackage, Key: "svc/api", Word: "stub"},
			want: "api_stub.go",
		},
		{
			name: "a plan unit is the word alone",
			unit: plugin.Unit{Per: plugin.PerPlan, Word: "registry"},
			want: "registry.go",
		},
		{
			name: "an acronym in the word converts as one name",
			unit: plugin.Unit{Per: plugin.PerPlan, Word: "HTTPClient"},
			want: "http_client.go",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, spell.Filename(tt.unit), tt.want, tt.name)
		})
	}
}
