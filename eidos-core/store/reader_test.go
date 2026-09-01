// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// recorded reports whether the read set holds a per-identity edge on
// id.
func recorded(reads *store.ReadSet, id symbol.Identity) bool {
	return slices.Contains(slices.Collect(reads.Identities()), id)
}

// The reader is the only path a plugin's read takes: it filters by
// scope and records what it returned.
func TestReader(t *testing.T) {
	t.Parallel()

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declaration and records it", func(t *testing.T) {
			t.Parallel()

			want := coretest.Struct(coretest.StorePath, "Store")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, want))

			got, held := r.Lookup(want.ID)
			assert.True(t, held, "Lookup returns a held declaration")
			assert.True(t, got == symbol.Symbol(want), "the very declaration, not a copy")
			assert.True(t, recorded(reads, want.ID),
				"and records the edge invalidation follows")
		})

		t.Run("records an identity the graph does not hold", func(t *testing.T) {
			t.Parallel()

			absent := coretest.Struct(coretest.StorePath, "Absent").ID
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))

			_, held := r.Lookup(absent)
			assert.False(t, held, "an unheld identity returns nothing")
			assert.True(t, recorded(reads, absent),
				"and still records: the reader runs again when it appears")
		})

		t.Run("neither returns nor records a declaration outside scope", func(t *testing.T) {
			t.Parallel()

			hidden := coretest.Struct(coretest.CachePath, "Cache")
			r, reads := coretest.Reading(t, onlyPackage(coretest.StorePath),
				coretest.Package(coretest.StorePath), coretest.Package(coretest.CachePath, hidden))

			_, held := r.Lookup(hidden.ID)
			assert.False(t, held, "a declaration outside scope is not returned")
			assert.False(t, recorded(reads, hidden.ID),
				"and not recorded: a change the reader could never see must not re-run it")
		})
	})

	t.Run("ByKind", func(t *testing.T) {
		t.Parallel()

		t.Run("records a set-membership edge", func(t *testing.T) {
			t.Parallel()

			r, reads := coretest.Reading(t, nil,
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")))
			for range r.ByKind(symbol.KindStruct) { // ranging is what records the edge
			}

			assert.Equal(t, slices.Collect(reads.Kinds()), []symbol.Kind{symbol.KindStruct},
				"an enumeration records a set-membership edge")
		})

		t.Run("records an enumeration that returned nothing", func(t *testing.T) {
			t.Parallel()

			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			for range r.ByKind(symbol.KindStruct) { // ranging is what records the edge
			}

			assert.Length(t, slices.Collect(reads.Kinds()), 1,
				"an empty enumeration still records: the reader runs again "+
					"when the first declaration of that kind arrives")
		})

		t.Run("records only the declarations the caller reached", func(t *testing.T) {
			t.Parallel()

			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath,
				coretest.Struct(coretest.StorePath, "Store"), coretest.Struct(coretest.StorePath, "Cache")))

			for range r.ByKind(symbol.KindStruct) {
				break
			}
			assert.Length(t, slices.Collect(reads.Identities()), 1,
				"a stopped enumeration records what the caller reached, not the set")
		})

		t.Run("skips a declaration outside scope", func(t *testing.T) {
			t.Parallel()

			hidden := coretest.Struct(coretest.CachePath, "Cache")
			r, reads := coretest.Reading(t, onlyPackage(coretest.StorePath),
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, hidden))

			assert.Equal(t, coretest.Names(t, slices.Collect(r.ByKind(symbol.KindStruct))),
				[]string{"Store"}, "the enumeration skips a declaration outside scope")
			assert.False(t, recorded(reads, hidden.ID),
				"and does not record it either")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath,
				coretest.Struct(coretest.StorePath, "Store"), coretest.Struct(coretest.StorePath, "Cache")))

			seen := 0
			for range r.ByKind(symbol.KindStruct) {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the enumeration stops when the range stops")
		})
	})

	t.Run("PackageOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the package holding a declaration", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))

			got, held := r.PackageOf(decl.ID)
			assert.True(t, held, "PackageOf returns the holding package")
			assert.Equal(t, got.ID, coretest.PackageID(coretest.StorePath),
				"the one the declaration's identity names")
		})

		t.Run("records a per-identity edge on the package", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))
			r.PackageOf(decl.ID)

			assert.True(t, recorded(reads, coretest.PackageID(coretest.StorePath)),
				"PackageOf records a per-identity edge on the package")
		})

		t.Run("returns false for a package nothing holds", func(t *testing.T) {
			t.Parallel()

			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			_, held := r.PackageOf(coretest.Struct(coretest.CachePath, "Cache").ID)
			assert.False(t, held, "a package nothing holds returns nothing")
		})

		t.Run("returns the package the identity names, not the file that carried it", func(t *testing.T) {
			t.Parallel()

			// A declaration whose identity names an unloaded package is
			// malformed input from a frontend; the holder is derived
			// from the identity, so it returns nothing rather than the
			// package the file sat in.
			stray := coretest.Struct(coretest.CachePath, "Stray")
			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, stray))

			_, held := r.PackageOf(stray.ID)
			assert.False(t, held,
				"the holder derives from the identity, not from the file that carried it")
		})

		t.Run("neither returns nor records outside scope", func(t *testing.T) {
			t.Parallel()

			hidden := coretest.Struct(coretest.CachePath, "Cache")
			r, reads := coretest.Reading(t, onlyPackage(coretest.StorePath),
				coretest.Package(coretest.StorePath), coretest.Package(coretest.CachePath, hidden))

			_, held := r.PackageOf(hidden.ID)
			assert.False(t, held, "a package outside scope is not returned")
			assert.False(t, recorded(reads, coretest.PackageID(coretest.CachePath)),
				"and not recorded")
		})
	})

	t.Run("Scope", func(t *testing.T) {
		t.Parallel()

		t.Run("a nil scope admits every package", func(t *testing.T) {
			t.Parallel()

			r, _ := coretest.Reading(t, nil,
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Cache")))

			assert.Length(t, slices.Collect(r.ByKind(symbol.KindStruct)), 2,
				"a nil scope admits every package")
		})
	})
}

// onlyPackage returns a scope admitting one package path.
func onlyPackage(path string) store.Scope {
	return func(pkg symbol.Identity) bool { return pkg.Package == path }
}

// A tracked read costs an untracked one plus the bookkeeping. What
// these measure is that difference: every read a plugin makes carries
// it, and nothing consumes the edges yet.
func BenchmarkReader(b *testing.B) {
	const packages, files, decls = benchPackages, benchFiles, benchDecls

	b.Run("Lookup", func(b *testing.B) {
		b.ReportAllocs()
		r, _ := coretest.Reading(b, nil, coretest.Workspace(packages, files, decls)...)
		id := coretest.Struct(coretest.StorePath+"/0", "Decl0_0").Identity()

		for b.Loop() {
			if _, held := r.Lookup(id); !held {
				b.Fatalf("Lookup(%v) = false, want the declaration", id)
			}
		}
	})

	b.Run("ByKind", func(b *testing.B) {
		b.ReportAllocs()
		r, _ := coretest.Reading(b, nil, coretest.Workspace(packages, files, decls)...)

		for b.Loop() {
			seen := 0
			for range r.ByKind(symbol.KindStruct) {
				seen++
			}
			if seen != packages*files*decls {
				b.Fatalf("ByKind returned %d declarations, want %d", seen, packages*files*decls)
			}
		}
	})

	b.Run("ByKind under a scope", func(b *testing.B) {
		b.ReportAllocs()
		admitted := coretest.StorePath + "/0"
		r, _ := coretest.Reading(b,
			func(pkg symbol.Identity) bool { return pkg.Package == admitted },
			coretest.Workspace(packages, files, decls)...)

		for b.Loop() {
			for range r.ByKind(symbol.KindStruct) { // ranging is what records the edge
			}
		}
	})
}
