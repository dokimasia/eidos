// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"slices"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The reader is the only path a plugin's read takes: it filters by
// scope and records what it returned.
func TestReader(t *testing.T) {
	t.Parallel()

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declaration itself", func(t *testing.T) {
			t.Parallel()

			want := coretest.Struct(coretest.StorePath, "Store")
			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, want))

			got, held := r.Lookup(want.ID)
			assert.True(t, held, "Lookup returns the declaration")
			assert.Equal(t, got, symbol.Symbol(want), "the very declaration, not a copy", assert.ByIdentity())
		})

		t.Run("records the declaration edge", func(t *testing.T) {
			t.Parallel()

			want := coretest.Struct(coretest.StorePath, "Store")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, want))

			r.Lookup(want.ID)
			assert.Contains(t, slices.Collect(reads.Identities()), want.ID, "the edge invalidation follows")
		})

		t.Run("records a package edge for a package's own identity", func(t *testing.T) {
			t.Parallel()

			pkg := coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store"))
			r, reads := coretest.Reading(t, nil, pkg)

			got, held := r.Lookup(pkg.ID)
			assert.True(t, held, "Lookup returns the package")
			expect.Equal(t, got, symbol.Symbol(pkg), "whole, as the graph contains it", assert.ByIdentity())
			expect.Contains(t, slices.Collect(reads.Packages()), pkg.ID, "and records a package edge")
			expect.NotContains(t, slices.Collect(reads.Identities()), pkg.ID, "in place of a declaration edge")
		})

		t.Run("records an identity the graph does not contain", func(t *testing.T) {
			t.Parallel()

			absent := coretest.Struct(coretest.StorePath, "Absent").ID
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))

			_, held := r.Lookup(absent)
			assert.False(t, held, "an identity the graph does not contain returns nothing")
			assert.Contains(t, slices.Collect(reads.Identities()), absent,
				"and still records, so the reader runs again when it appears")
		})

		t.Run("records no edge for a declaration outside scope", func(t *testing.T) {
			t.Parallel()

			hidden := coretest.Struct(coretest.CachePath, "Cache")
			r, reads := coretest.Reading(t, onlyPackage(coretest.StorePath),
				coretest.Package(coretest.StorePath), coretest.Package(coretest.CachePath, hidden))

			_, held := r.Lookup(hidden.ID)
			assert.False(t, held, "a declaration outside scope is not returned")
			assert.NotContains(t, slices.Collect(reads.Identities()), hidden.ID,
				"and not recorded: a change the reader could never see must not run it again")
		})
	})

	t.Run("ByKind", func(t *testing.T) {
		t.Parallel()

		t.Run("records a membership edge", func(t *testing.T) {
			t.Parallel()

			r, reads := coretest.Reading(t, nil,
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")))
			for range r.ByKind(symbol.KindStruct) { // ranging is what records the edge
			}

			assert.Equal(t, slices.Collect(reads.Kinds()), []symbol.Kind{symbol.KindStruct},
				"an enumeration records a membership edge")
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

		t.Run("records only the declarations the caller ranged over", func(t *testing.T) {
			t.Parallel()

			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath,
				coretest.Struct(coretest.StorePath, "Store"), coretest.Struct(coretest.StorePath, "Cache")))

			for range r.ByKind(symbol.KindStruct) {
				break
			}
			assert.Length(t, slices.Collect(reads.Identities()), 1,
				"a stopped enumeration records what the caller ranged over, not the set")
		})

		t.Run("skips a declaration outside scope", func(t *testing.T) {
			t.Parallel()

			hidden := coretest.Struct(coretest.CachePath, "Cache")
			r, reads := coretest.Reading(t, onlyPackage(coretest.StorePath),
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, hidden))

			assert.Equal(t, coretest.Names(t, slices.Collect(r.ByKind(symbol.KindStruct))),
				[]string{"Store"}, "the enumeration skips a declaration outside scope")
			assert.NotContains(t, slices.Collect(reads.Identities()), hidden.ID, "and does not record it either")
		})

		t.Run("returns the declarations of every package under a nil scope", func(t *testing.T) {
			t.Parallel()

			r, _ := coretest.Reading(t, nil,
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Cache")))

			assert.Length(t, slices.Collect(r.ByKind(symbol.KindStruct)), 2,
				"a nil scope admits every package")
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

		t.Run("asks the scope once per package", func(t *testing.T) {
			t.Parallel()

			scope, asked := counting(coretest.StorePath)
			r, _ := coretest.Reading(t, scope,
				coretest.Package(coretest.StorePath,
					coretest.Struct(coretest.StorePath, "Store"), coretest.Struct(coretest.StorePath, "Index")),
				coretest.Package(coretest.CachePath,
					coretest.Struct(coretest.CachePath, "Cache"), coretest.Struct(coretest.CachePath, "Entry")))

			assert.Equal(t, coretest.Names(t, slices.Collect(r.ByKind(symbol.KindStruct))),
				[]string{"Index", "Store"}, "the scope admits one package, in identity order")
			assert.Equal(t, *asked, 2, "and is asked once for each of the two packages")
		})

		t.Run("keeps the edges a log loaded into the set", func(t *testing.T) {
			t.Parallel()

			cache := coretest.Struct(coretest.CachePath, "Cache")
			g := coretest.Frozen(t, coretest.Package(coretest.CachePath, cache))
			reads := loadedFrom(everyGrain(t))
			r, err := g.Reader(reads, nil)
			assert.NoError(t, err, "a sealed graph hands out a reader")
			for range r.ByKind(symbol.KindStruct) { // ranging is what records the edges
			}
			ids := []symbol.Identity{cache.ID, coretest.Struct(coretest.StorePath, "Store").ID}
			slices.SortFunc(ids, symbol.Identity.Compare)
			assert.Equal(t, slices.Collect(reads.Identities()), ids,
				"the loaded declaration and the enumerated one")
			assert.Equal(t, reads.Len(), 6, "beside the loaded package, kind, directive and fact edges")
		})
	})

	t.Run("ByDirective", func(t *testing.T) {
		t.Parallel()

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			one := coretest.Struct(coretest.StorePath, "Store")
			two := coretest.Struct(coretest.StorePath, "Cache")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, one, two)),
				"the package loads")
			assert.NoError(t, g.AttachDirectives(one.ID, []directive.Raw{stubAt(1)}), "one attaches")
			assert.NoError(t, g.AttachDirectives(two.ID, []directive.Raw{stubAt(2)}), "and two")
			g.Freeze()
			reads := store.NewReadSet()
			r, err := g.Reader(reads, nil)
			assert.NoError(t, err, "the sealed graph hands out a reader")

			seen := 0
			for range r.ByDirective("stub") {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the enumeration stops when the range stops")
			assert.Length(t, slices.Collect(reads.Identities()), 1,
				"and records what the caller ranged over, not the set")
		})

		t.Run("asks the scope once per package", func(t *testing.T) {
			t.Parallel()

			store1 := coretest.Struct(coretest.StorePath, "Store")
			store2 := coretest.Struct(coretest.StorePath, "Index")
			cache1 := coretest.Struct(coretest.CachePath, "Cache")
			cache2 := coretest.Struct(coretest.CachePath, "Entry")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, store1, store2)),
				"the store package loads")
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.CachePath, cache1, cache2)),
				"and the cache package")
			for i, decl := range []*node.Struct{store1, store2, cache1, cache2} {
				assert.NoError(t, g.AttachDirectives(decl.ID, []directive.Raw{stubAt(i + 1)}),
					"each declaration has the directive")
			}
			g.Freeze()
			scope, asked := counting(coretest.StorePath)
			r, err := g.Reader(store.NewReadSet(), scope)
			assert.NoError(t, err, "the sealed graph hands out a reader")

			assert.Length(t, slices.Collect(r.ByDirective("stub")), 2,
				"the scope admits one package")
			assert.Equal(t, *asked, 2, "and is asked once for each of the two packages")
		})
	})

	t.Run("PackageOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the package that contains a declaration", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))

			got, held := r.PackageOf(decl.ID)
			assert.True(t, held, "PackageOf returns the package that contains the declaration")
			assert.Equal(t, got.ID, coretest.PackageID(coretest.StorePath),
				"the one the declaration's identity names")
		})

		t.Run("records a package edge", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))
			r.PackageOf(decl.ID)

			assert.Equal(t, slices.Collect(reads.Packages()),
				[]symbol.Identity{coretest.PackageID(coretest.StorePath)},
				"PackageOf records a package edge on the package it returns")
			assert.Empty(t, slices.Collect(reads.Identities()),
				"and no declaration edge")
		})

		t.Run("reports false for a package the graph does not contain", func(t *testing.T) {
			t.Parallel()

			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			_, held := r.PackageOf(coretest.Struct(coretest.CachePath, "Cache").ID)
			assert.False(t, held, "a package the graph does not contain returns nothing")
		})

		t.Run("reports false for an identity naming a package the graph does not contain", func(t *testing.T) {
			t.Parallel()

			// A declaration whose identity names an unloaded package is
			// malformed input from a frontend. The package derives from
			// the identity, so PackageOf returns nothing, and not the
			// package of the file the declaration is in.
			stray := coretest.Struct(coretest.CachePath, "Stray")
			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, stray))

			_, held := r.PackageOf(stray.ID)
			assert.False(t, held,
				"the package derives from the identity, not from the file the declaration is in")
		})

		t.Run("records no edge for a package outside scope", func(t *testing.T) {
			t.Parallel()

			hidden := coretest.Struct(coretest.CachePath, "Cache")
			r, reads := coretest.Reading(t, onlyPackage(coretest.StorePath),
				coretest.Package(coretest.StorePath), coretest.Package(coretest.CachePath, hidden))

			_, held := r.PackageOf(hidden.ID)
			assert.False(t, held, "a package outside scope is not returned")
			assert.Equal(t, reads.Len(), 0, "and not recorded")
		})
	})
}

// A tracked read or enumeration into a set that recorded it before
// allocates nothing, under a scope as without one. The count's warm-up
// call records the edges. The check runs alone, because the count
// includes every goroutine's allocations.
func TestReaderAllocs(t *testing.T) {
	decl := coretest.Struct(coretest.StorePath, "Store")
	r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))
	var held bool
	assert.MaxAllocs(t, func() { _, held = r.Lookup(decl.ID) }, 0,
		"Lookup allocates nothing for an edge the set recorded")
	assert.True(t, held, "Lookup finds the declaration")
	assert.MaxAllocs(t, func() { _, held = r.PackageOf(decl.ID) }, 0,
		"PackageOf allocates nothing for an edge the set recorded")
	assert.True(t, held, "PackageOf finds the package")
	n := 0
	assert.MaxAllocs(t, func() {
		n = 0
		for range r.ByKind(symbol.KindStruct) {
			n++
		}
	}, 0, "ByKind allocates nothing for edges the set recorded")
	assert.Equal(t, n, 1, "ByKind enumerates the one struct")

	scoped, _ := coretest.Reading(t, onlyPackage(coretest.StorePath),
		coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
		coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Cache")))
	assert.MaxAllocs(t, func() {
		n = 0
		for range scoped.ByKind(symbol.KindStruct) {
			n++
		}
	}, 0, "ByKind allocates nothing under a scope for edges the set recorded")
	assert.Equal(t, n, 1, "ByKind enumerates the one admitted struct")

	carriers := directed(t, 2)
	assert.MaxAllocs(t, func() {
		n = 0
		for range carriers.ByDirective("stub") {
			n++
		}
	}, 0, "ByDirective allocates nothing for edges the set recorded")
	assert.Equal(t, n, 2, "ByDirective enumerates both carriers")
}

// BenchmarkReader measures the tracked reads at the canonical scale: a
// tracked read costs an untracked one plus the bookkeeping, which every
// read a plugin makes costs. Each case reads once before the measurement,
// so the set has its edges and the measured reads record into storage it
// grew.
func BenchmarkReader(b *testing.B) {
	const packages, files, decls = benchPackages, benchFiles, benchDecls
	pkgs := coretest.Workspace(packages, files, decls)
	id := coretest.Struct(coretest.StorePath+"/0", "Decl0_0").Identity()

	b.Run("Lookup", func(b *testing.B) {
		r, _ := coretest.Reading(b, nil, pkgs...)
		r.Lookup(id)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		held := false
		for c.Loop() {
			_, held = r.Lookup(id)
		}
		assert.True(b, held, "Lookup returns the declaration")
	})

	b.Run("PackageOf", func(b *testing.B) {
		r, _ := coretest.Reading(b, nil, pkgs...)
		r.PackageOf(id)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		held := false
		for c.Loop() {
			_, held = r.PackageOf(id)
		}
		assert.True(b, held, "PackageOf returns the package")
	})

	b.Run("ByKind", func(b *testing.B) {
		b.Run("without a scope", func(b *testing.B) {
			r, _ := coretest.Reading(b, nil, pkgs...)
			// One warm-up iteration runs at the call site the contract
			// measures. It grows the read set, and the runtime builds the
			// site's 48-byte cache for converting a declaration to a symbol.
			// A pass at any other call site builds a cache of its own.
			c := bench.Start(b).Warmup(1).MaxAllocs(0)
			defer c.End()
			seen := 0
			for c.Loop() {
				seen = 0
				for range r.ByKind(symbol.KindStruct) {
					seen++
				}
			}
			assert.Equal(b, seen, packages*files*decls, "ByKind returns every struct")
		})

		b.Run("under a scope", func(b *testing.B) {
			admitted := coretest.StorePath + "/0"
			r, _ := coretest.Reading(b, onlyPackage(admitted), pkgs...)
			// The warm-up iteration grows the read set and builds the call
			// site's conversion cache before the contract counts.
			c := bench.Start(b).Warmup(1).MaxAllocs(0)
			defer c.End()
			seen := 0
			for c.Loop() {
				seen = 0
				for range r.ByKind(symbol.KindStruct) {
					seen++
				}
			}
			assert.Equal(b, seen, files*decls, "ByKind returns the admitted package's structs")
		})
	})

	b.Run("ByDirective", func(b *testing.B) {
		const carriers = 20_000
		r := directed(b, carriers)
		// The warm-up iteration grows the read set and builds the call
		// site's conversion cache before the contract counts.
		c := bench.Start(b).Warmup(1).MaxAllocs(0)
		defer c.End()
		seen := 0
		for c.Loop() {
			seen = 0
			for range r.ByDirective("stub") {
				seen++
			}
		}
		assert.Equal(b, seen, carriers, "ByDirective returns every carrier")
	})
}

// directed returns a reader over a frozen graph of one package of n
// structs, each with one stub directive.
func directed(tb assert.TB, n int) *store.Reader {
	tb.Helper()

	decls := make([]symbol.Symbol, 0, n)
	for i := range n {
		decls = append(decls, coretest.Struct(coretest.StorePath, "Decl"+strconv.Itoa(i)))
	}
	g := store.New()
	assert.NoError(tb, g.AddPackage(coretest.Package(coretest.StorePath, decls...)), "the package loads")
	for i, decl := range decls {
		named, _ := decl.(node.Declaration)
		assert.NoError(tb, g.AttachDirectives(named.Identity(), []directive.Raw{stubAt(i + 1)}),
			"the directive attaches")
	}
	g.Freeze()
	r, err := g.Reader(store.NewReadSet(), nil)
	assert.NoError(tb, err, "the frozen graph hands out a reader")
	return r
}

// onlyPackage returns a scope admitting one package path.
func onlyPackage(path string) store.Scope {
	return func(pkg symbol.Identity) bool { return pkg.Package == path }
}

// counting returns a scope admitting one package path, and the
// number of times a reader asked it.
func counting(path string) (store.Scope, *int) {
	asked := 0
	return func(pkg symbol.Identity) bool {
		asked++
		return pkg.Package == path
	}, &asked
}
