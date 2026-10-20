// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typeshed_test

import (
	"io/fs"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python/frontend/typeshed"
)

// The minor versions of Python 3 of the cases.
const (
	python310 = 10
	python311 = 11
	python312 = 12
	python314 = 14
)

// parseAllocs is the parse of the table of the cases: its text, the table
// and its growth to four modules, and the slash path of its one dotted
// module.
const parseAllocs = 1 + 2 + 1

// table is the text of a table of versions with a comment, a blank line,
// an open range, a closed range, a submodule and a trailing comment.
const table = "# The ranges of the modules.\n\nasyncio: 3.4-\nasyncio.taskgroups: 3.11-  # new\n" +
	"distutils: 3.0-3.11\ncollections: 3.0-\n"

// TestVersions pins the reading of the table of versions and the ranges
// that it decides.
func TestVersions(t *testing.T) {
	t.Parallel()

	t.Run("ParseVersions", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the range of each module by its slash path", func(t *testing.T) {
			t.Parallel()

			got, err := typeshed.ParseVersions([]byte(table))
			assert.NoError(t, err, "the table parses")
			assert.Equal(t, got, typeshed.Versions{
				"asyncio":            {First: 4, Last: typeshed.OpenEnd},
				"asyncio/taskgroups": {First: 11, Last: typeshed.OpenEnd},
				"distutils":          {First: 0, Last: 11},
				"collections":        {First: 0, Last: typeshed.OpenEnd},
			}, "the range of each listed module")
		})

		t.Run("parses the table of the embedded tree", func(t *testing.T) {
			t.Parallel()

			data, err := fs.ReadFile(typeshed.Files, typeshed.VersionsFile)
			assert.NoError(t, err, "the tree contains the table")
			got, err := typeshed.ParseVersions(data)
			assert.NoError(t, err, "the table of typeshed parses")
			assert.True(t, got.Includes("tomllib", python311), "tomllib exists from 3.11")
		})

		tests := []struct {
			name string
			give string
		}{
			{name: "returns an error for a line without a colon", give: "asyncio 3.4-\n"},
			{name: "returns an error for a range without a dash", give: "asyncio: 3.4\n"},
			{name: "returns an error for a first version of Python 2", give: "asyncio: 2.7-\n"},
			{name: "returns an error for a last version that is not a number", give: "asyncio: 3.4-3.x\n"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := typeshed.ParseVersions([]byte("collections: 3.0-\n" + tt.give))
				assert.HasError(t, err, "the line does not read")
				assert.Contains(t, err.Error(), "VERSIONS:2", "the error names the line")
			})
		}
	})

	t.Run("Includes", func(t *testing.T) {
		t.Parallel()

		v, err := typeshed.ParseVersions([]byte(table))
		assert.NoError(t, err, "the table parses")
		tests := []struct {
			name       string
			giveModule string
			giveMinor  int
			want       bool
		}{
			{
				name:       "reports true for a module in its open range",
				giveModule: "asyncio",
				giveMinor:  python312,
				want:       true,
			},
			{
				name:       "reports true for a module in its closed range",
				giveModule: "distutils",
				giveMinor:  python311,
				want:       true,
			},
			{name: "reports false for a module after its closed range", giveModule: "distutils", giveMinor: python312},
			{
				name:       "reports false for a module before its range",
				giveModule: "asyncio/taskgroups",
				giveMinor:  python310,
			},
			{
				name: "reports true for a listed submodule in its range", giveModule: "asyncio/taskgroups",
				giveMinor: python314, want: true,
			},
			{
				name:       "reports true for an unlisted submodule in the range of its parent",
				giveModule: "collections/abc",
				giveMinor:  python314,
				want:       true,
			},
			{name: "reports false for a module that the table does not list", giveModule: "shop", giveMinor: python314},
			{name: "reports false for the empty module", giveModule: "", giveMinor: python314},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, v.Includes(tt.giveModule, tt.giveMinor), tt.want,
					"Includes reports whether the module exists in the version")
			})
		}
	})
}

// TestVersionsAllocs checks the ceilings of the parse and of the check of
// a module in the ordinary run, which does not run benchmarks.
func TestVersionsAllocs(t *testing.T) {
	t.Run("ParseVersions", func(t *testing.T) {
		src := []byte(table)
		var got typeshed.Versions
		var err error
		assert.MaxAllocs(t, func() { got, err = typeshed.ParseVersions(src) }, parseAllocs,
			"ParseVersions allocates the text, the table and the slash path of a dotted module")
		assert.NoError(t, err, "the table parses")
		assert.Length(t, got, 4, "the table lists four modules")
	})

	t.Run("Versions", func(t *testing.T) {
		t.Run("Includes", func(t *testing.T) {
			v, err := typeshed.ParseVersions([]byte(table))
			assert.NoError(t, err, "the table parses")
			included := false
			assert.MaxAllocs(t, func() { included = v.Includes("collections/abc", python314) }, 0,
				"Includes does not allocate")
			assert.True(t, included, "collections.abc exists in 3.14")
		})
	})
}

// BenchmarkVersions measures the parse that a dependency round makes once,
// and the check that it makes for each need.
func BenchmarkVersions(b *testing.B) {
	b.Run("ParseVersions", func(b *testing.B) {
		src := []byte(table)
		c := bench.Start(b).MaxAllocs(parseAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			_, err = typeshed.ParseVersions(src)
		}
		assert.NoError(b, err, "the table parses")
	})

	b.Run("Versions", func(b *testing.B) {
		b.Run("Includes", func(b *testing.B) {
			v, err := typeshed.ParseVersions([]byte(table))
			assert.NoError(b, err, "the table parses")
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			included := false
			for c.Loop() {
				included = v.Includes("collections/abc", python314)
			}
			assert.True(b, included, "collections.abc exists in 3.14")
		})
	})
}
