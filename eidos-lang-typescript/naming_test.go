// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/lang-typescript"
)

// The naming is total over the units a plan admits, and every
// spelling is pinned: files are addressed by name, so a drift
// here orphans previously generated files.
func TestNaming(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		unit plugin.Unit
		want string
	}{
		{
			name: "a source unit joins stem and word with dots",
			unit: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub"},
			want: "store.stub.ts",
		},
		{
			name: "a pascal stem converts to kebab",
			unit: plugin.Unit{Per: plugin.PerSource, Key: "svc/UserStore.java", Word: "stub"},
			want: "user-store.stub.ts",
		},
		{
			name: "a tag joins as its own qualifier",
			unit: plugin.Unit{
				Per: plugin.PerSource, Key: "svc/store.go", Word: "stub", Tag: "grpc",
			},
			want: "store.stub.grpc.ts",
		},
		{
			name: "a plan unit is the word alone",
			unit: plugin.Unit{Per: plugin.PerPlan, Word: "registry"},
			want: "registry.ts",
		},
		{
			name: "an acronym in the word converts to kebab",
			unit: plugin.Unit{Per: plugin.PerPlan, Word: "HTTPClient"},
			want: "http-client.ts",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, typescript.Naming(tt.unit), tt.want, tt.name)
		})
	}
}
