// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/go/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The allocations of a filename.
const (
	// filenameAllocs is a per-source unit's filename: the list of its
	// parts, their join, and the name with its extension.
	filenameAllocs = 3
	// testFilenameAllocs is a per-package unit's filename in an external
	// test package: the list of its parts, the name with the test
	// suffix, and the name with its extension.
	testFilenameAllocs = 3
)

// Files are addressed by name, so a change of spelling orphans the files
// an earlier run wrote. Every spelling of a unit a plan admits is
// pinned.
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
				name: "joins the stem to the word of a per-source unit",
				give: storeUnit(),
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
				give: externalTestUnit(),
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

// Filename allocates its parts, their join and the name. The ordinary
// run, which runs no benchmark, checks that ceiling here.
func TestFilenameAllocs(t *testing.T) {
	source, test := storeUnit(), externalTestUnit()
	var got string
	assert.MaxAllocs(t, func() { got = spell.Filename(source) }, filenameAllocs,
		"Filename allocates the parts, their join and the name")
	assert.Equal(t, got, "store_stub.go", "Filename spells the unit")
	assert.MaxAllocs(t, func() { got = spell.Filename(test) }, testFilenameAllocs,
		"Filename allocates the parts, the test suffix and the name")
	assert.Equal(t, got, "stub_test.go", "Filename spells the test file")
}

// BenchmarkFilename measures the filename a backend spells once per
// unit.
func BenchmarkFilename(b *testing.B) {
	units := []struct {
		name   string
		give   plugin.Unit
		allocs uint64
		want   string
	}{
		{name: "a unit of a package", give: storeUnit(), allocs: filenameAllocs, want: "store_stub.go"},
		{
			name:   "a unit of an external test package",
			give:   externalTestUnit(),
			allocs: testFilenameAllocs,
			want:   "stub_test.go",
		},
	}
	b.Run("Filename", func(b *testing.B) {
		for _, tt := range units {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var got string
				for c.Loop() {
					got = spell.Filename(tt.give)
				}
				assert.Equal(b, got, tt.want, "Filename spells the unit")
			})
		}
	})
}

// storeUnit returns the stub family's unit of svc/store.go.
func storeUnit() plugin.Unit {
	return plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub"}
}

// externalTestUnit returns the stub family's unit of the store
// directory's external test package.
func externalTestUnit() plugin.Unit {
	return plugin.Unit{Per: plugin.PerPackage, Key: testPath, Word: "stub", Pkg: origin(testPath)}
}
