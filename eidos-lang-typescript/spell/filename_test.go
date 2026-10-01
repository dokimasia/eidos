// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The naming is total over the units a plan admits, and every
// spelling is pinned: files are addressed by name, so a drift here
// orphans previously generated files.
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
				name: "returns the stem and the word of a per-source unit joined by dots",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub"},
				want: "store.stub.ts",
			},
			{
				name: "returns a pascal stem in kebab case",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/UserStore.java", Word: "stub"},
				want: "user-store.stub.ts",
			},
			{
				name: "returns the tag as a qualifier of its own",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub", Tag: "grpc"},
				want: "store.stub.grpc.ts",
			},
			{
				name: "returns the word alone for a per-package unit",
				give: plugin.Unit{Per: plugin.PerPackage, Key: "svc/api", Word: "stub"},
				want: "stub.ts",
			},
			{
				name: "returns a test file for a per-package unit of the test companion",
				give: plugin.Unit{Per: plugin.PerPackage, Key: "svc/api", Word: "suite", Tag: "test"},
				want: "suite.test.ts",
			},
			{
				name: "returns the word alone for a plan unit",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "registry"},
				want: "registry.ts",
			},
			{
				name: "returns an acronym in the word in kebab case",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "HTTPClient"},
				want: "http-client.ts",
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
