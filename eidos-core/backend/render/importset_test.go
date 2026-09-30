// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
)

// The set's own fixture: the paths and names beyond the shared
// binding fixture that a case records.
const (
	// auditPkg and sidePkg are packages whose names clash with nothing.
	auditPkg = "svc/audit"
	sidePkg  = "side/effect"
	// storeName is a second declaration of storePkg.
	storeName = "Store"
	// storeLocal2 and storeLocal3 are the names storeLocal takes with
	// the first two suffixes.
	storeLocal2 = storeLocal + "2"
	storeLocal3 = storeLocal + "3"
	// rowName2 is the name rowName takes with the first suffix.
	rowName2 = rowName + "2"
)

// The set is the one meeting point between spelling and the import
// block: per file, deduplicated, sorted, and the one place local
// names are assigned.
func TestImportSet(t *testing.T) {
	t.Parallel()

	t.Run("Home", func(t *testing.T) {
		t.Parallel()

		t.Run("returns empty for a set no home was named for", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			assert.Equal(t, s.Home(), "", "the zero set names no package")
		})

		t.Run("returns the path SetHome named", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.SetHome(storePkg)
			assert.Equal(t, s.Home(), storePkg, "the file's own package")
		})
	})

	t.Run("Reserve", func(t *testing.T) {
		t.Parallel()

		t.Run("suffixes a package name a declaration of the file takes", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Reserve(storeLocal)
			assert.Equal(t, s.Bind(storePkg, storeLocal), storeLocal2,
				"the import binds the next free name")
		})

		t.Run("suffixes a declaration name a declaration of the file takes", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Reserve(rowName)
			assert.Equal(t, s.BindItem(storePkg, rowName, false), rowName2,
				"the imported declaration binds the next free name")
		})

		t.Run("makes Claim report false for a reserved name", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Reserve(rowName)
			assert.False(t, s.Claim(storePkg, rowName),
				"the file's own declaration keeps the simple name")
		})

		t.Run("reserves nothing for an empty name", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Reserve("")
			assert.True(t, s.Claim(storePkg, ""), "the empty name is still free")
		})

		t.Run("leaves a name an import claimed bound to the import", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			assert.True(t, s.Claim(storePkg, rowName), "the name is free")
			s.Reserve(rowName)
			assert.True(t, s.Claim(storePkg, rowName),
				"a later reservation leaves the import's binding in place")
		})
	})

	t.Run("Bind", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name for the first package that binds it", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			assert.Equal(t, s.Bind(storePkg, storeLocal), storeLocal, "the name is free")
		})

		t.Run("records the entry under the bound name", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Bind(storePkg, storeLocal)
			s.Bind(legacyPkg, storeLocal)
			assert.Equal(t, s.Entries(), []render.Entry{
				{Path: legacyPkg, Name: storeLocal2},
				{Path: storePkg, Name: storeLocal},
			}, "each package's entry names what the file calls it")
		})

		t.Run("returns the earlier name for a package bound before", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Bind(storePkg, storeLocal)
			assert.Equal(t, s.Bind(storePkg, auditPkg), storeLocal,
				"a package binds one name per file")
		})

		t.Run("suffixes the name for a second package that binds it", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Bind(storePkg, storeLocal)
			assert.Equal(t, s.Bind(legacyPkg, storeLocal), storeLocal2,
				"the first package to bind the name keeps it")
		})

		t.Run("suffixes with the lowest free number", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Reserve(storeLocal2)
			s.Bind(storePkg, storeLocal)
			assert.Equal(t, s.Bind(legacyPkg, storeLocal), storeLocal3,
				"a suffixed name the file reserves is skipped")
		})

		t.Run("returns empty for the file's own package", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.SetHome(storePkg)
			assert.Equal(t, s.Bind(storePkg, storeLocal), "",
				"a file never qualifies its own package's names")
			assert.Equal(t, s.Len(), 0, "and never imports its own package")
		})

		t.Run("returns empty for an empty path", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			assert.Equal(t, s.Bind("", storeLocal), "", "no package, no qualifier")
			assert.Equal(t, s.Len(), 0, "and no import")
		})
	})

	t.Run("BindItem", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declaration's name for the first import that binds it", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			assert.Equal(t, s.BindItem(storePkg, rowName, false), rowName, "the name is free")
		})

		t.Run("records a value entry for a value binding", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.BindItem(storePkg, rowName, false)
			assert.Equal(t, s.Entries(), []render.Entry{{Path: storePkg, Name: rowName}},
				"the declaration imports under its own name")
		})

		t.Run("records a type-only entry for a type binding", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.BindItem(storePkg, rowName, true)
			assert.Equal(t, s.Entries(), []render.Entry{
				{Path: storePkg, Name: rowName, TypeOnly: true},
			}, "the entry is marked for the type checker alone")
		})

		t.Run("renames a declaration whose name another import binds", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.BindItem(storePkg, rowName, false)
			assert.Equal(t, s.BindItem(legacyPkg, rowName, false), rowName2,
				"the second declaration takes the next free name")
			assert.Equal(t, s.Entries(), []render.Entry{
				{Path: legacyPkg, Name: rowName2, Item: rowName},
				{Path: storePkg, Name: rowName},
			}, "the renamed entry names the declaration it binds")
		})

		t.Run("returns the earlier name for a declaration bound before", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.BindItem(storePkg, rowName, false)
			s.BindItem(legacyPkg, rowName, false)
			assert.Equal(t, s.BindItem(legacyPkg, rowName, false), rowName2,
				"a declaration binds one name per file")
		})

		t.Run("records a value entry beside a type-only entry of one declaration", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.BindItem(storePkg, rowName, true)
			assert.Equal(t, s.BindItem(storePkg, rowName, false), rowName,
				"the value use binds the same name")
			assert.Equal(t, s.Entries(), []render.Entry{
				{Path: storePkg, Name: rowName},
				{Path: storePkg, Name: rowName, TypeOnly: true},
			}, "both entries are recorded for the renderer to join")
		})

		t.Run("returns the name unchanged for the file's own package", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.SetHome(storePkg)
			assert.Equal(t, s.BindItem(storePkg, rowName, false), rowName,
				"a declaration of the file's own package needs no import")
			assert.Equal(t, s.Len(), 0, "so none is recorded")
		})

		t.Run("returns the name unchanged for an empty path", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			assert.Equal(t, s.BindItem("", rowName, false), rowName, "no package, no import")
			assert.Equal(t, s.Len(), 0, "so none is recorded")
		})
	})

	t.Run("Claim", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a free name", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			assert.True(t, s.Claim(storePkg, rowName), "no binding takes the name")
			assert.Equal(t, s.Entries(), []render.Entry{{Path: storePkg, Name: rowName}},
				"the claim records its import")
		})

		t.Run("reports true for a name the same declaration claimed before", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Claim(storePkg, rowName)
			assert.True(t, s.Claim(storePkg, rowName), "the name binds this declaration")
			assert.Length(t, s.Entries(), 1, "one import for both claims")
		})

		t.Run("reports false for a name another import claimed", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Claim(storePkg, rowName)
			assert.False(t, s.Claim(legacyPkg, rowName), "the first claimant keeps the name")
			assert.Equal(t, s.Paths(), []string{storePkg},
				"the refused claim records no import")
		})

		t.Run("records no entry for the file's own package", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.SetHome(storePkg)
			assert.True(t, s.Claim(storePkg, rowName), "the name is free")
			assert.Equal(t, s.Len(), 0, "a file never imports its own package")
		})

		t.Run("reports false for a name the file's own package claimed", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.SetHome(storePkg)
			s.Claim(storePkg, rowName)
			assert.False(t, s.Claim(legacyPkg, rowName),
				"an import of the name would shadow the package's own declaration")
		})

		t.Run("reports false for the file's own name another import claimed", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.SetHome(storePkg)
			s.Claim(legacyPkg, rowName)
			assert.False(t, s.Claim(storePkg, rowName),
				"the import shadows the package's own declaration")
		})

		t.Run("reports true for an empty path", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			assert.True(t, s.Claim("", rowName), "no package, no import")
			assert.Equal(t, s.Len(), 0, "so none is recorded")
		})
	})

	t.Run("Add", func(t *testing.T) {
		t.Parallel()

		t.Run("records a path once however often it is added", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Add(storePkg)
			s.Add(storePkg)
			assert.Equal(t, s.Entries(), []render.Entry{{Path: storePkg}}, "one bare entry")
		})

		t.Run("records nothing under the file's own package", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.SetHome(storePkg)
			s.Add(storePkg)
			assert.Equal(t, s.Len(), 0, "a file never imports its own package")
		})
	})

	t.Run("AddNamed", func(t *testing.T) {
		t.Parallel()

		t.Run("records one entry for each name a path binds", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.AddNamed(storePkg, storeName)
			s.AddNamed(storePkg, rowName)
			s.AddNamed(storePkg, rowName)
			assert.Equal(t, s.Entries(), []render.Entry{
				{Path: storePkg, Name: rowName},
				{Path: storePkg, Name: storeName},
			}, "one entry per pair")
		})

		t.Run("records a named entry apart from the bare entry of one path", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Add(storePkg)
			s.AddNamed(storePkg, rowName)
			assert.Length(t, s.Entries(), 2, "the bare and the named form are two imports")
		})

		t.Run("records nothing under the file's own package", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.SetHome(storePkg)
			s.AddNamed(storePkg, rowName)
			assert.Equal(t, s.Len(), 0, "a file never imports its own package")
		})
	})

	t.Run("AddType", func(t *testing.T) {
		t.Parallel()

		t.Run("records a type-only entry apart from the value entry of one name", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.AddType(storePkg, rowName)
			s.AddNamed(storePkg, rowName)
			s.AddType(storePkg, rowName)
			assert.Equal(t, s.Entries(), []render.Entry{
				{Path: storePkg, Name: rowName},
				{Path: storePkg, Name: rowName, TypeOnly: true},
			}, "one name binds twice, the value form first")
		})

		t.Run("records nothing under the file's own package", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.SetHome(storePkg)
			s.AddType(storePkg, rowName)
			assert.Equal(t, s.Len(), 0, "a file never imports its own package")
		})
	})

	t.Run("Paths", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every path once in sorted order", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Add(storePkg)
			s.AddNamed(sidePkg, rowName)
			s.Add(sidePkg)
			s.Bind(auditPkg, storeLocal)
			assert.Equal(t, s.Paths(), []string{sidePkg, auditPkg, storePkg},
				"one path per package, in path order")
		})
	})

	t.Run("Entries", func(t *testing.T) {
		t.Parallel()

		t.Run("sorts by path then name then item then type-only last", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Reserve(rowName)
			s.AddType(storePkg, rowName2)
			s.BindItem(storePkg, rowName, false)
			s.AddNamed(storePkg, rowName2)
			s.AddNamed(storePkg, storeName)
			s.Add(storePkg)
			s.Add(auditPkg)
			assert.Equal(t, s.Entries(), []render.Entry{
				{Path: auditPkg},
				{Path: storePkg},
				{Path: storePkg, Name: rowName2},
				{Path: storePkg, Name: rowName2, TypeOnly: true},
				{Path: storePkg, Name: rowName2, Item: rowName},
				{Path: storePkg, Name: storeName},
			}, "two renders of one set spell one block")
		})
	})

	t.Run("Len", func(t *testing.T) {
		t.Parallel()

		t.Run("counts each path once", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Add(storePkg)
			s.AddNamed(storePkg, rowName)
			s.Add(auditPkg)
			assert.Equal(t, s.Len(), 2, "two packages, three entries")
		})
	})

	t.Run("Reset", func(t *testing.T) {
		t.Parallel()

		t.Run("drops every entry", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Add(storePkg)
			s.AddNamed(auditPkg, rowName)
			s.Reset()
			assert.Equal(t, s.Len(), 0, "the next file starts empty")
		})

		t.Run("drops every binding", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Bind(storePkg, storeLocal)
			s.Bind(legacyPkg, storeLocal)
			s.Reset()
			assert.Equal(t, s.Bind(legacyPkg, storeLocal), storeLocal,
				"the next file assigns its names from scratch")
		})

		t.Run("drops every reservation", func(t *testing.T) {
			t.Parallel()

			var s render.ImportSet
			s.Reserve(storeLocal)
			s.Reset()
			assert.Equal(t, s.Bind(storePkg, storeLocal), storeLocal,
				"the next file declares its own names")
		})
	})
}
