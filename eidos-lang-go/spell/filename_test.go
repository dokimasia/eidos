// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The spelling is total over the units a plan admits, and every case
// is pinned: files are addressed by name, so a drift here orphans
// previously generated files.
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
				want: "store_stub.go",
			},
			{
				name: "returns the stem without the source's own extension",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/user.proto", Word: "stub"},
				want: "user_stub.go",
			},
			{
				name: "returns the tag after the word",
				give: plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub", Tag: "grpc"},
				want: "store_stub_grpc.go",
			},
			{
				name: "returns the word alone for a per-package unit",
				give: plugin.Unit{Per: plugin.PerPackage, Key: "svc/api", Word: "stub"},
				want: "stub.go",
			},
			{
				name: "returns a test file for a per-package unit of the test companion",
				give: plugin.Unit{Per: plugin.PerPackage, Key: "svc/api", Word: "suite", Tag: "test"},
				want: "suite_test.go",
			},
			{
				name: "returns a test file for a per-package unit of an external test package",
				give: plugin.Unit{Per: plugin.PerPackage, Key: testPath, Word: "stub", Pkg: origin(testPath)},
				want: "stub_test.go",
			},
			{
				name: "returns one test suffix for the test companion of an external test package",
				give: plugin.Unit{
					Per:  plugin.PerPackage,
					Key:  testPath,
					Word: "suite",
					Tag:  "test",
					Pkg:  origin(testPath),
				},
				want: "suite_test.go",
			},
			{
				name: "returns a test file for a per-source unit of an external test file",
				give: plugin.Unit{
					Per: plugin.PerSource, Key: "svc/store/store_test.go", Word: "stub", Pkg: origin(testPath),
				},
				want: "store_test_stub_test.go",
			},
			{
				name: "returns the word alone for a plan unit",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "registry"},
				want: "registry.go",
			},
			{
				name: "returns an acronym in the word as one name",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "HTTPClient"},
				want: "http_client.go",
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
