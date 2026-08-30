// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package store_test

import (
	"slices"
	"testing"

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
// scope and records what it answered.
func TestReader(t *testing.T) {
	t.Parallel()

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		t.Run("answers the declaration and records it", func(t *testing.T) {
			t.Parallel()

			want := coretest.Struct(coretest.StorePath, "Store")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, want))

			got, held := r.Lookup(want.ID)
			if !held || got != symbol.Symbol(want) {
				t.Fatalf("Lookup(%v) = %v, %t; want the declaration", want.ID, got, held)
			}
			if !recorded(reads, want.ID) {
				t.Fatalf("Lookup(%v) recorded no edge: the read is invisible to invalidation",
					want.ID)
			}
		})

		t.Run("records an identity the graph does not hold", func(t *testing.T) {
			t.Parallel()

			absent := coretest.Struct(coretest.StorePath, "Absent").ID
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))

			if _, held := r.Lookup(absent); held {
				t.Fatalf("Lookup(%v) = true, want false", absent)
			}
			if !recorded(reads, absent) {
				t.Fatal("a miss recorded no edge: the reader would not run again " +
					"when the declaration it asked for appears")
			}
		})

		t.Run("neither answers nor records a declaration outside scope", func(t *testing.T) {
			t.Parallel()

			hidden := coretest.Struct(coretest.CachePath, "Cache")
			r, reads := coretest.Reading(t, onlyPackage(coretest.StorePath),
				coretest.Package(coretest.StorePath), coretest.Package(coretest.CachePath, hidden))

			if _, held := r.Lookup(hidden.ID); held {
				t.Fatalf("Lookup(%v) = true, want false: the package is out of scope",
					hidden.ID)
			}
			if recorded(reads, hidden.ID) {
				t.Fatal("an out-of-scope read recorded an edge: a change the reader " +
					"could never have seen would re-run it")
			}
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

			if got := slices.Collect(reads.Kinds()); !slices.Equal(
				got, []symbol.Kind{symbol.KindStruct},
			) {
				t.Fatalf("Kinds() = %v, want [Struct]", got)
			}
		})

		t.Run("records an enumeration that answered nothing", func(t *testing.T) {
			t.Parallel()

			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			for range r.ByKind(symbol.KindStruct) { // ranging is what records the edge
			}

			if len(slices.Collect(reads.Kinds())) != 1 {
				t.Fatal("an empty enumeration recorded no edge: the reader would not " +
					"run again when the first declaration of that kind arrives")
			}
		})

		t.Run("records only the declarations the caller reached", func(t *testing.T) {
			t.Parallel()

			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath,
				coretest.Struct(coretest.StorePath, "Store"), coretest.Struct(coretest.StorePath, "Cache")))

			for range r.ByKind(symbol.KindStruct) {
				break
			}
			if got := len(slices.Collect(reads.Identities())); got != 1 {
				t.Fatalf("a stopped enumeration recorded %d identities, want 1: "+
					"the read is priced at the size of the set", got)
			}
		})

		t.Run("skips a declaration outside scope", func(t *testing.T) {
			t.Parallel()

			hidden := coretest.Struct(coretest.CachePath, "Cache")
			r, reads := coretest.Reading(t, onlyPackage(coretest.StorePath),
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, hidden))

			got := coretest.Names(t, slices.Collect(r.ByKind(symbol.KindStruct)))
			if want := []string{"Store"}; !slices.Equal(got, want) {
				t.Fatalf("ByKind = %v, want %v", got, want)
			}
			if recorded(reads, hidden.ID) {
				t.Fatal("an out-of-scope declaration was recorded while enumerating")
			}
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
			if seen != 1 {
				t.Fatalf("ByKind yielded %d declarations after a break, want 1", seen)
			}
		})
	})

	t.Run("PackageOf", func(t *testing.T) {
		t.Parallel()

		t.Run("answers the package holding a declaration", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))

			got, held := r.PackageOf(decl.ID)
			if !held {
				t.Fatalf("PackageOf(%v) = false, want the package", decl.ID)
			}
			if got.ID != coretest.PackageID(coretest.StorePath) {
				t.Fatalf("PackageOf(%v) = %v, want %v", decl.ID, got.ID, coretest.PackageID(coretest.StorePath))
			}
		})

		t.Run("records a per-identity edge on the package", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))
			r.PackageOf(decl.ID)

			if !recorded(reads, coretest.PackageID(coretest.StorePath)) {
				t.Fatal("PackageOf recorded no edge on the package it answered")
			}
		})

		t.Run("answers false for a package nothing holds", func(t *testing.T) {
			t.Parallel()

			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			if _, held := r.PackageOf(coretest.Struct(coretest.CachePath, "Cache").ID); held {
				t.Fatal("PackageOf of an unheld package = true, want false")
			}
		})

		t.Run("answers the package the identity names, not the file that carried it", func(t *testing.T) {
			t.Parallel()

			// A declaration whose identity names an unloaded package is
			// malformed input from a frontend; the holder is derived
			// from the identity, so it answers nothing rather than the
			// package the file sat in.
			stray := coretest.Struct(coretest.CachePath, "Stray")
			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, stray))

			if _, held := r.PackageOf(stray.ID); held {
				t.Fatal("PackageOf answered a package the graph never loaded")
			}
		})

		t.Run("neither answers nor records outside scope", func(t *testing.T) {
			t.Parallel()

			hidden := coretest.Struct(coretest.CachePath, "Cache")
			r, reads := coretest.Reading(t, onlyPackage(coretest.StorePath),
				coretest.Package(coretest.StorePath), coretest.Package(coretest.CachePath, hidden))

			if _, held := r.PackageOf(hidden.ID); held {
				t.Fatal("PackageOf answered a package outside scope")
			}
			if recorded(reads, coretest.PackageID(coretest.CachePath)) {
				t.Fatal("PackageOf recorded an edge on a package outside scope")
			}
		})
	})

	t.Run("Scope", func(t *testing.T) {
		t.Parallel()

		t.Run("a nil scope admits every package", func(t *testing.T) {
			t.Parallel()

			r, _ := coretest.Reading(t, nil,
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Cache")))

			if got := len(slices.Collect(r.ByKind(symbol.KindStruct))); got != 2 {
				t.Fatalf("a nil scope admitted %d declarations, want 2", got)
			}
		})
	})
}

// onlyPackage answers a scope admitting one package path.
func onlyPackage(path string) store.Scope {
	return func(pkg symbol.Identity) bool { return pkg.Package == path }
}

// A tracked read costs an untracked one plus the bookkeeping. What
// these measure is that difference: every read a plugin makes pays
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
				b.Fatalf("ByKind answered %d declarations, want %d", seen, packages*files*decls)
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
