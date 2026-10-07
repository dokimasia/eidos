// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

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

// The size of [filledSet]: four entries over three paths.
const (
	filledEntries = 4
	filledPaths   = 3
)

// importWrite is one write a file's spellings make into its set: the
// method, which names its benchmark, the case it measures where the
// method has more than one write, and the ceiling of the write after a
// Reset. The writes of one method are adjacent in a list of writes.
type importWrite struct {
	name     string
	caseName string
	allocs   uint64
	write    func(*render.ImportSet)
}

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

// A set the pass reuses across files records and binds without
// allocating once its storage has grown, a suffixed name allocates its
// spelling, and the lists allocate what they return, in the ordinary
// run, which runs no benchmark. Each write runs after a Reset, as the
// pass runs it for each file. The check runs alone, because the count
// includes every goroutine's allocations.
func TestImportSetAllocs(t *testing.T) {
	s := grownSet()
	for _, tt := range importWrites() {
		msg := tt.name + " allocates within its ceiling after a Reset"
		if tt.caseName != "" {
			msg = tt.name + " for " + tt.caseName + " allocates within its ceiling after a Reset"
		}
		assert.MaxAllocs(t, func() {
			s.Reset()
			tt.write(s)
		}, tt.allocs, msg)
		assert.InRange(t, s.Len(), 0, 2, tt.name+" records at most two paths")
	}
	full := filledSet()
	var home string
	assert.MaxAllocs(t, func() { home = full.Home() }, 0, "Home allocates nothing")
	assert.Equal(t, home, storePkg, "Home returns the file's own package")
	var paths []string
	assert.MaxAllocs(t, func() { paths = full.Paths() }, 1, "Paths allocates the list it returns")
	assert.Length(t, paths, filledPaths, "Paths returns every distinct path")
	var entries []render.Entry
	assert.MaxAllocs(t, func() { entries = full.Entries() }, 1, "Entries allocates the list it returns")
	assert.Length(t, entries, filledEntries, "Entries returns every entry")
	var n int
	assert.MaxAllocs(t, func() { n = full.Len() }, 0, "Len allocates nothing for up to eight entries")
	assert.Equal(t, n, filledPaths, "Len counts the distinct paths")
}

// BenchmarkImportSet measures each write into a set the pass reuses,
// after the Reset the pass runs between files, and the reads an import
// renderer makes once per file.
func BenchmarkImportSet(b *testing.B) {
	writes := importWrites()
	for len(writes) > 0 {
		n := 1
		for n < len(writes) && writes[n].name == writes[0].name {
			n++
		}
		method := writes[:n]
		writes = writes[n:]
		b.Run(method[0].name, func(b *testing.B) {
			if len(method) == 1 && method[0].caseName == "" {
				benchWrite(b, method[0])
				return
			}
			for _, tt := range method {
				b.Run(tt.caseName, func(b *testing.B) { benchWrite(b, tt) })
			}
		})
	}

	full := filledSet()

	b.Run("Home", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = full.Home()
		}
		assert.Equal(b, got, storePkg, "Home returns the file's own package")
	})

	b.Run("Paths", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got []string
		for c.Loop() {
			got = full.Paths()
		}
		assert.Length(b, got, filledPaths, "Paths returns every distinct path")
	})

	b.Run("Entries", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got []render.Entry
		for c.Loop() {
			got = full.Entries()
		}
		assert.Length(b, got, filledEntries, "Entries returns every entry")
	})

	b.Run("Len", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = full.Len()
		}
		assert.Equal(b, got, filledPaths, "Len counts the distinct paths")
	})
}

// benchWrite measures one write after a Reset into a set whose storage
// has grown, under the bench contract at the write's ceiling.
func benchWrite(b *testing.B, tt importWrite) {
	b.Helper()

	s := grownSet()
	c := bench.Start(b).MaxAllocs(tt.allocs)
	defer c.End()
	for c.Loop() {
		s.Reset()
		tt.write(s)
	}
	assert.InRange(b, s.Len(), 0, 2, tt.name+" records at most two paths")
}

// importWrites returns every write a file's spellings make into its set,
// each with its ceiling in a set whose storage has grown.
func importWrites() []importWrite {
	return []importWrite{
		{name: "SetHome", write: func(s *render.ImportSet) { s.SetHome(storePkg) }},
		{name: "Reserve", write: func(s *render.ImportSet) { s.Reserve(rowName) }},
		{
			name:     "Bind",
			caseName: "a name no other package binds",
			write:    func(s *render.ImportSet) { s.Bind(storePkg, storeLocal) },
		},
		{
			name: "Bind", caseName: "a name another package binds", allocs: 1,
			write: func(s *render.ImportSet) {
				s.Bind(storePkg, storeLocal)
				s.Bind(legacyPkg, storeLocal)
			},
		},
		{name: "BindItem", write: func(s *render.ImportSet) { s.BindItem(storePkg, rowName, false) }},
		{name: "Claim", write: func(s *render.ImportSet) { s.Claim(storePkg, rowName) }},
		{name: "Add", write: func(s *render.ImportSet) { s.Add(storePkg) }},
		{name: "AddNamed", write: func(s *render.ImportSet) { s.AddNamed(storePkg, rowName) }},
		{name: "AddType", write: func(s *render.ImportSet) { s.AddType(storePkg, rowName) }},
		{
			name: "Reset", caseName: "a set of two entries",
			write: func(s *render.ImportSet) {
				s.Add(sidePkg)
				s.Bind(auditPkg, storeLocal)
			},
		},
	}
}

// grownSet returns a set that recorded every kind of write once, so its
// maps and journal have the storage a Reset keeps.
func grownSet() *render.ImportSet {
	s := &render.ImportSet{}
	s.Reserve(storeName)
	s.Bind(storePkg, storeLocal)
	s.Bind(legacyPkg, storeLocal)
	s.BindItem(auditPkg, rowName, false)
	s.Claim(sidePkg, storeName+"s")
	s.Add(sidePkg)
	s.AddNamed(auditPkg, rowName2)
	s.AddType(auditPkg, storeName)
	return s
}

// filledSet returns a set for the file of storePkg holding
// [filledEntries] entries over [filledPaths] paths.
func filledSet() *render.ImportSet {
	s := &render.ImportSet{}
	s.SetHome(storePkg)
	s.Add(sidePkg)
	s.AddNamed(auditPkg, rowName)
	s.AddType(auditPkg, storeName)
	s.Bind(legacyPkg, storeLocal)
	return s
}
