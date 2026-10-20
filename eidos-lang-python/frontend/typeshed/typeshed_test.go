// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typeshed_test

import (
	"io/fs"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python/frontend/typeshed"
)

// TestTypeshed pins the commit and the layout of the embedded tree, which
// the store of the frontend reads by path.
func TestTypeshed(t *testing.T) {
	t.Parallel()

	t.Run("Commit", func(t *testing.T) {
		t.Parallel()

		t.Run("spells the pinned commit of typeshed", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, typeshed.Commit, "e75a39fec6f375f5002d54c5607b44bedc025a4b",
				"the commit whose stdlib the tree embeds")
		})
	})

	t.Run("VersionsFile", func(t *testing.T) {
		t.Parallel()

		t.Run("spells the table of versions below the stdlib directory", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, typeshed.VersionsFile, "stdlib/VERSIONS", "the path of the table of versions")
		})
	})

	t.Run("Files", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
		}{
			{name: "contains the stub of builtins", give: "stdlib/builtins.pyi"},
			{name: "contains the stub of a module of a package", give: "stdlib/collections/abc.pyi"},
			{name: "contains the stub of a package directory", give: "stdlib/collections/__init__.pyi"},
			{name: "contains the stub of a private package directory", give: "stdlib/_typeshed/__init__.pyi"},
			{
				name: "contains a stub four directories below the stdlib directory",
				give: "stdlib/xml/parsers/expat/errors.pyi",
			},
			{name: "contains the table of versions", give: typeshed.VersionsFile},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				data, err := fs.ReadFile(typeshed.Files, tt.give)
				assert.NoError(t, err, "the tree contains "+tt.give)
				assert.NotEmpty(t, data, "the file has its content")
			})
		}

		t.Run("contains 752 stubs", func(t *testing.T) {
			t.Parallel()

			stubs, err := fs.Glob(typeshed.Files, "stdlib/*.pyi")
			assert.NoError(t, err, "the pattern is valid")
			for _, pattern := range []string{"stdlib/*/*.pyi", "stdlib/*/*/*.pyi", "stdlib/*/*/*/*.pyi"} {
				deeper, err := fs.Glob(typeshed.Files, pattern)
				assert.NoError(t, err, "the pattern is valid")
				stubs = append(stubs, deeper...)
			}
			assert.Length(t, stubs, 752, "the stubs of the stdlib directory at the commit")
		})
	})
}
