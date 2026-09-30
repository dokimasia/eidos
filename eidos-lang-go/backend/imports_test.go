// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/render"
)

// The import fixture: a standard library package imported for its
// side effects, and a module path imported under an alias.
const (
	sqlPkg    = "database/sql"
	sdkPkg    = "go.dokimi.dev/eidos/sdk"
	sdkAlias  = "eidos"
	assertPkg = "go.dokimi.dev/assert"
)

// line spells one import line of the block, tab-indented.
func line(name, path string) string {
	if name == "" {
		return "\t" + strconv.Quote(path) + "\n"
	}
	return "\t" + name + " " + strconv.Quote(path) + "\n"
}

// block spells an import block around its lines.
func block(lines ...string) string {
	out := "import (\n"
	for _, l := range lines {
		out += l
	}
	return out + ")\n"
}

// The import block is pinned byte for byte: it is the one part of
// the file the formatter reorders and does not reformat, so the
// renderer hands it over already in gofmt's shape.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("Imports", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the standard library group before the module group", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.Add(assertPkg)
			set.Add(contextPkg)
			assert.Equal(t, backend.Imports(&set),
				block(line("", contextPkg), "\n", line("", assertPkg)),
				"a blank line between the groups")
		})

		t.Run("writes the name of a blank import before its path", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.AddNamed(sqlPkg, golang.BlankAlias)
			assert.Equal(t, backend.Imports(&set), block(line(golang.BlankAlias, sqlPkg)),
				"the side-effect import")
		})

		t.Run("writes a name that differs from the path's last element before the path", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.AddNamed(sdkPkg, sdkAlias)
			set.Bind(yamlPkg, yamlName)
			assert.Equal(t, backend.Imports(&set),
				block(line(sdkAlias, sdkPkg), line(yamlName, yamlPkg)),
				"the alias the spelling qualified through")
		})

		t.Run("writes no name that equals the path's last element", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.Bind(storePkg, storeName)
			assert.Equal(t, backend.Imports(&set), block(line("", storePkg)),
				"the import binds that name without one")
		})

		t.Run("writes a line two entries share once", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.Add(storePkg)
			set.Bind(storePkg, storeName)
			assert.Equal(t, backend.Imports(&set), block(line("", storePkg)),
				"Go refuses one package imported twice under one name")
		})

		t.Run("writes each path once in sorted order", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.Add(storePkg)
			set.Add(legacyPkg)
			set.Add(storePkg)
			assert.Equal(t, backend.Imports(&set),
				block(line("", legacyPkg), line("", storePkg)), "sorted, quoted and tab-indented")
		})

		t.Run("returns nothing for an empty set", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			assert.Equal(t, backend.Imports(&set), "", "gofmt leaves no empty block either")
		})
	})
}
