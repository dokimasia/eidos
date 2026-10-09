// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"slices"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The plugins the skip cases name, and the fixture namespace.
const (
	skippedPlugin plugin.ID = "stubgen"
	otherPlugin   plugin.ID = "audit"
	flagNamespace           = "t"
)

// The store scale the routing surface has to serve: a thousand
// packages, ten thousand files, two hundred thousand declarations.
const (
	benchPackages = 1_000
	benchFiles    = 10
	benchDecls    = 20
)

// The allocations of the routing surface's construction, which
// TestIndexAllocs checks in the ordinary run and BenchmarkIndex in a
// benchmark run.
const (
	// newIndexAllocs is the ceiling of the index, with or without a scope.
	// A validated table does not add an allocation.
	newIndexAllocs = 1
	// readerAllocs is a reader handle.
	readerAllocs = 1
)

// The index is the dispatcher's routing surface. It enumerates the
// declarations under the run's scope without tracking the reads, reads
// the validated directive table and its skip rulings, and returns the
// tracked readers that handlers read through. Its filters are the
// visibility rule, so each filter is part of the contract.
func TestIndex(t *testing.T) {
	t.Parallel()

	t.Run("ValidatedMap", func(t *testing.T) {
		t.Parallel()

		t.Run("DirectivesOf", func(t *testing.T) {
			t.Parallel()

			g, inStore, inCache := twoPackages(t)
			want := []directive.Directive{{Name: "stub", Instance: 0}}
			table := plugin.ValidatedMap{inStore.ID: want}

			t.Run("returns the subject's instances", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, table.DirectivesOf(inStore.ID), want, "the map returns the subject's instances")
			})

			t.Run("returns nil for a subject that the map does not contain", func(t *testing.T) {
				t.Parallel()

				assert.Nil(t, table.DirectivesOf(inCache.ID), "the subject has no instance")
			})

			t.Run("returns the subject's instances through an index", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, index(t, g, table, nil).DirectivesOf(inStore.ID), want,
					"the index reads the table that it received")
			})
		})
	})

	t.Run("NewIndex", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for an unfrozen graph", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.NewIndex(store.New(), meta.NewFacts(meta.NewRegistry()), nil, nil)
			assert.HasError(t, err, "the index does not build")
		})

		t.Run("returns an error for a missing graph", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.NewIndex(nil, meta.NewFacts(meta.NewRegistry()), nil, nil)
			assert.HasError(t, err, "the index does not build")
		})

		t.Run("returns an error for a missing fact store", func(t *testing.T) {
			t.Parallel()

			g, _, _ := twoPackages(t)
			_, err := plugin.NewIndex(g, nil, nil, nil)
			assert.HasError(t, err, "the index does not build")
		})

		t.Run("asks the scope about no package", func(t *testing.T) {
			t.Parallel()

			g, _, _ := twoPackages(t)
			asked := 0
			index(t, g, nil, func(symbol.Identity) bool {
				asked++
				return true
			})
			assert.Equal(t, asked, 0, "NewIndex asks the scope nothing")
		})
	})

	t.Run("ByKind", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every declaration of one kind", func(t *testing.T) {
			t.Parallel()

			g, _, _ := twoPackages(t)
			got := names(t, index(t, g, nil, nil).ByKind(symbol.KindStruct))
			assert.Equal(t, got, []string{"Cache", "Store"}, "the declarations are in identity order")
		})

		t.Run("returns only the declarations the scope admits", func(t *testing.T) {
			t.Parallel()

			g, _, _ := twoPackages(t)
			got := names(t, index(t, g, nil, storeOnly).ByKind(symbol.KindStruct))
			assert.Equal(t, got, []string{"Store"}, "the declaration outside the scope is not returned")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			g, _, _ := twoPackages(t)
			var got int
			for range index(t, g, nil, nil).ByKind(symbol.KindStruct) {
				got++
				break
			}
			assert.Equal(t, got, 1, "the iteration yields once")
		})

		t.Run("stops when the range stops under a scope", func(t *testing.T) {
			t.Parallel()

			g, _, _ := twoPackages(t)
			var got int
			for range index(t, g, nil, everything).ByKind(symbol.KindStruct) {
				got++
				break
			}
			assert.Equal(t, got, 1, "the iteration yields once")
		})

		t.Run("asks the scope once for each package", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t,
				coretest.Package(coretest.StorePath,
					coretest.Struct(coretest.StorePath, "First"), coretest.Struct(coretest.StorePath, "Second")),
				coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Cache")),
			)
			asked, n := 0, 0
			for range index(t, g, nil, func(symbol.Identity) bool {
				asked++
				return true
			}).ByKind(symbol.KindStruct) {
				n++
			}
			assert.Equal(t, n, 3, "ByKind yields the three structs")
			assert.Equal(t, asked, 2, "ByKind asks the scope once for each of the two packages")
		})
	})

	t.Run("ByDirective", func(t *testing.T) {
		t.Parallel()

		t.Run("returns only the subjects the scope admits", func(t *testing.T) {
			t.Parallel()

			g, _, _ := twoPackages(t)
			got := names(t, index(t, g, nil, storeOnly).ByDirective(directive.Name("stub")))
			assert.Equal(t, got, []string{"Store"}, "the subject outside the scope is not returned")
		})
	})

	t.Run("ByFactKey", func(t *testing.T) {
		t.Parallel()

		t.Run("returns only the stamped subjects the scope admits", func(t *testing.T) {
			t.Parallel()

			g, inStore, inCache := twoPackages(t)
			facts, key := flagged(t, inStore.ID, inCache.ID)
			ix, err := plugin.NewIndex(g, facts, nil, storeOnly)
			assert.NoError(t, err, "the routing surface builds")
			assert.Equal(t, subjects(ix.ByFactKey(key.ID())), []string{"Store"},
				"the subject outside the scope is not returned")
		})

		t.Run("returns every stamped subject without a scope", func(t *testing.T) {
			t.Parallel()

			g, inStore, inCache := twoPackages(t)
			facts, key := flagged(t, inStore.ID, inCache.ID)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(t, err, "the routing surface builds")
			assert.Equal(t, subjects(ix.ByFactKey(key.ID())), []string{"Cache", "Store"},
				"the subjects are in identity order")
		})

		t.Run("stops when the range stops under a scope", func(t *testing.T) {
			t.Parallel()

			g, inStore, inCache := twoPackages(t)
			facts, key := flagged(t, inStore.ID, inCache.ID)
			ix, err := plugin.NewIndex(g, facts, nil, everything)
			assert.NoError(t, err, "the routing surface builds")
			var got int
			for range ix.ByFactKey(key.ID()) {
				got++
				break
			}
			assert.Equal(t, got, 1, "the iteration yields once")
		})
	})

	t.Run("PackageOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the package that contains a declaration", func(t *testing.T) {
			t.Parallel()

			g, inStore, _ := twoPackages(t)
			pkg, held := index(t, g, nil, nil).PackageOf(inStore.ID)
			assert.True(t, held, "the declaration has a package")
			assert.Equal(t, pkg.ID, coretest.PackageID(coretest.StorePath), "the package is the store's")
		})

		t.Run("reports false for a declaration outside the scope", func(t *testing.T) {
			t.Parallel()

			g, _, inCache := twoPackages(t)
			_, held := index(t, g, nil, storeOnly).PackageOf(inCache.ID)
			assert.False(t, held, "no package is returned")
		})
	})

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a declaration the graph contains", func(t *testing.T) {
			t.Parallel()

			g, inStore, _ := twoPackages(t)
			got, held := index(t, g, nil, nil).Lookup(inStore.ID)
			assert.True(t, held, "the identity resolves")
			assert.Equal(t, got, symbol.Symbol(inStore), "the declaration is the graph's", assert.ByIdentity())
		})

		t.Run("reports false for a declaration outside the scope", func(t *testing.T) {
			t.Parallel()

			g, _, inCache := twoPackages(t)
			_, held := index(t, g, nil, storeOnly).Lookup(inCache.ID)
			assert.False(t, held, "no declaration is returned")
		})
	})

	t.Run("DirectivesOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the validated instances in position order", func(t *testing.T) {
			t.Parallel()

			g, inStore, _ := twoPackages(t)
			want := []directive.Directive{
				{Name: "stub", Instance: 0},
				{Name: "stub", Instance: 1},
			}
			ix := index(t, g, plugin.ValidatedMap{inStore.ID: want}, nil)
			assert.Equal(t, ix.DirectivesOf(inStore.ID), want, "the instances are the validated table's")
		})

		t.Run("returns nil for an index without a validated table", func(t *testing.T) {
			t.Parallel()

			g, inStore, _ := twoPackages(t)
			assert.Nil(t, index(t, g, nil, nil).DirectivesOf(inStore.ID), "no instance was validated")
		})
	})

	t.Run("Skipped", func(t *testing.T) {
		t.Parallel()

		g, inStore, inCache := twoPackages(t)
		negatedAt := coretest.Struct(coretest.StorePath, "Negated").ID
		overriddenAt := coretest.Struct(coretest.StorePath, "Overridden").ID
		setAt := coretest.Struct(coretest.StorePath, "Set").ID
		clean := coretest.Struct(coretest.StorePath, "Clean").ID
		validated := plugin.ValidatedMap{
			inStore.ID: {{Name: directive.KernelSkip}},
			inCache.ID: {{
				Name: directive.KernelSkip,
				Params: map[directive.ParamKey]directive.Value{
					directive.SkipPlugin: {Kind: directive.TypeString, Str: string(skippedPlugin)},
				},
			}},
			negatedAt:    {{Name: directive.Name(string(skippedPlugin) + ":stub"), Negated: true}},
			overriddenAt: {{Name: directive.Name(string(skippedPlugin) + ":stub"), Overrides: true}},
			setAt:        {{Name: directive.Name(string(skippedPlugin) + ":stub")}},
		}
		ix := index(t, g, validated, nil)

		tests := []struct {
			name    string
			subject symbol.Identity
			plugin  plugin.ID
			want    bool
		}{
			{
				name:    "reports true for every plugin under a bare skip",
				subject: inStore.ID, plugin: otherPlugin, want: true,
			},
			{
				name:    "reports true for the plugin a narrowed skip names",
				subject: inCache.ID, plugin: skippedPlugin, want: true,
			},
			{
				name:    "reports false for another plugin under a narrowed skip",
				subject: inCache.ID, plugin: otherPlugin,
			},
			{
				name:    "reports true for the plugin that registered a negated directive's schema",
				subject: negatedAt, plugin: skippedPlugin, want: true,
			},
			{
				name:    "reports false for another plugin under a negated directive",
				subject: negatedAt, plugin: otherPlugin,
			},
			{
				name:    "reports true for the plugin that registered the override schema of an instance",
				subject: overriddenAt, plugin: skippedPlugin, want: true,
			},
			{
				name:    "reports false for another plugin under an instance of an override schema",
				subject: overriddenAt, plugin: otherPlugin,
			},
			{
				name:    "reports false for the plugin of a set directive",
				subject: setAt, plugin: skippedPlugin,
			},
			{
				name:    "reports false for a subject without a skip",
				subject: clean, plugin: skippedPlugin,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, ix.Skipped(tt.subject, tt.plugin), tt.want, "the exclusion is pinned")
			})
		}

		t.Run("reports false for a run without a skip", func(t *testing.T) {
			t.Parallel()

			bare, inStore, _ := twoPackages(t)
			assert.False(t, index(t, bare, nil, nil).Skipped(inStore.ID, skippedPlugin), "nothing is excluded")
		})
	})

	t.Run("Reader", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a reader that finds a declaration inside the scope", func(t *testing.T) {
			t.Parallel()

			g, inStore, _ := twoPackages(t)
			r, err := index(t, g, nil, storeOnly).Reader(store.NewReadSet())
			assert.NoError(t, err, "the reader mints over a frozen graph")
			_, held := r.Lookup(inStore.ID)
			assert.True(t, held, "the declaration is found")
		})

		t.Run("returns a reader that finds nothing outside the scope", func(t *testing.T) {
			t.Parallel()

			g, _, inCache := twoPackages(t)
			r, err := index(t, g, nil, storeOnly).Reader(store.NewReadSet())
			assert.NoError(t, err, "the reader mints over a frozen graph")
			_, held := r.Lookup(inCache.ID)
			assert.False(t, held, "the declaration is not found")
		})

		t.Run("returns a reader that records its reads", func(t *testing.T) {
			t.Parallel()

			g, inStore, _ := twoPackages(t)
			reads := store.NewReadSet()
			r, err := index(t, g, nil, storeOnly).Reader(reads)
			assert.NoError(t, err, "the reader mints over a frozen graph")
			r.Lookup(inStore.ID)
			assert.Equal(t, slices.Collect(reads.Identities()), []symbol.Identity{inStore.ID},
				"the read set records the declaration it read")
		})
	})
}

// The index allocates itself alone, with or without a scope. A reader
// allocates its handle. The enumerations, lookups and skip rulings that a
// phase call reads through the index allocate nothing, with or without a
// scope. The check runs alone, because the count includes every
// goroutine's allocations.
func TestIndexAllocs(t *testing.T) {
	g, inStore, inCache := twoPackages(t)
	facts, key := flagged(t, inStore.ID, inCache.ID)
	validated := plugin.ValidatedMap{inStore.ID: {{Name: directive.KernelSkip}}}

	var (
		built *plugin.Index
		err   error
	)
	assert.MaxAllocs(t, func() { built, err = plugin.NewIndex(g, facts, nil, nil) }, newIndexAllocs,
		"NewIndex allocates the index of a run without a scope")
	assert.NoError(t, err, "the routing surface builds")
	assert.MaxAllocs(t, func() { built, err = plugin.NewIndex(g, facts, validated, nil) }, newIndexAllocs,
		"NewIndex allocates the index alone for a run with a skip")
	assert.NoError(t, err, "the routing surface builds with a validated table")
	assert.MaxAllocs(t, func() { built, err = plugin.NewIndex(g, facts, nil, storeOnly) }, newIndexAllocs,
		"NewIndex allocates the index alone for a run with a scope")
	assert.NoError(t, err, "the scoped routing surface builds")
	reads := store.NewReadSet()
	assert.MaxAllocs(t, func() { _, err = built.Reader(reads) }, readerAllocs, "Reader allocates the handle")
	assert.NoError(t, err, "the reader mints over a frozen graph")

	ix, err := plugin.NewIndex(g, facts, validated, everything)
	assert.NoError(t, err, "the routing surface builds")
	n := 0
	assert.MaxAllocs(t, func() {
		n = 0
		for range ix.ByDirective("stub") {
			n++
		}
	}, 0, "ByDirective allocates nothing")
	assert.Equal(t, n, 2, "ByDirective enumerates both subjects")
	assert.MaxAllocs(t, func() {
		n = 0
		for range ix.ByKind(symbol.KindStruct) {
			n++
		}
	}, 0, "ByKind allocates nothing")
	assert.Equal(t, n, 2, "ByKind enumerates both structs")
	assert.MaxAllocs(t, func() {
		n = 0
		for range ix.ByFactKey(key.ID()) {
			n++
		}
	}, 0, "ByFactKey allocates nothing for a key whose presence did not change")
	assert.Equal(t, n, 2, "ByFactKey enumerates both subjects")
	var held bool
	assert.MaxAllocs(t, func() { _, held = ix.Lookup(inStore.ID) }, 0, "Lookup allocates nothing")
	assert.True(t, held, "Lookup finds the store's struct")
	assert.MaxAllocs(t, func() { _, held = ix.PackageOf(inStore.ID) }, 0, "PackageOf allocates nothing")
	assert.True(t, held, "PackageOf finds the store's struct")
	var directives []directive.Directive
	assert.MaxAllocs(t, func() {
		held, directives = ix.Skipped(inStore.ID, skippedPlugin), ix.DirectivesOf(inStore.ID)
	}, 0, "Skipped and DirectivesOf allocate nothing")
	assert.True(t, held, "Skipped reports the store's struct")
	assert.Length(t, directives, 1, "DirectivesOf returns its skip")
	assert.MaxAllocs(t, func() { directives = validated.DirectivesOf(inStore.ID) }, 0,
		"ValidatedMap.DirectivesOf allocates nothing")
	assert.Length(t, directives, 1, "ValidatedMap.DirectivesOf returns the skip")

	bare, err := plugin.NewIndex(g, facts, nil, nil)
	assert.NoError(t, err, "the routing surface builds without a scope")
	var found, contained bool
	assert.MaxAllocs(t, func() {
		n = 0
		for range bare.ByKind(symbol.KindStruct) {
			n++
		}
		_, found = bare.Lookup(inStore.ID)
		_, contained = bare.PackageOf(inStore.ID)
	}, 0, "ByKind, Lookup and PackageOf allocate nothing without a scope")
	expect.Equal(t, n, 2, "ByKind enumerates both structs without a scope")
	expect.True(t, found, "Lookup finds the struct without a scope")
	expect.True(t, contained, "PackageOf finds its package without a scope")
}

// BenchmarkIndex measures the routing surface at the store scale it
// serves: the enumerations a phase call makes, with and without a scope,
// and the lookups and skip checks it makes per subject.
func BenchmarkIndex(b *testing.B) {
	g := coretest.Frozen(b, coretest.Workspace(benchPackages, benchFiles, benchDecls)...)
	facts := meta.NewFacts(meta.NewRegistry())
	one := coretest.StorePath + "/0"
	inOne := func(pkg symbol.Identity) bool { return pkg.Package == one }
	hit := coretest.Struct(one, "Decl0_0").ID
	miss := coretest.Struct(one, "Decl0_1").ID

	b.Run("ByKind", func(b *testing.B) {
		b.Run("unscoped", func(b *testing.B) {
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(b, err, "the routing surface builds")
			// The warm-up pass runs at the call site it measures, where the
			// runtime builds the site's 48-byte cache for converting a
			// declaration to a symbol. A pass at any other call site builds a
			// cache of its own.
			c := bench.Start(b).Warmup(1).MaxAllocs(0)
			defer c.End()
			n := 0
			for c.Loop() {
				n = 0
				for range ix.ByKind(symbol.KindStruct) {
					n++
				}
			}
			assert.Equal(b, n, benchPackages*benchFiles*benchDecls, "ByKind yields every struct")
		})

		b.Run("scoped to one package", func(b *testing.B) {
			ix, err := plugin.NewIndex(g, facts, nil, inOne)
			assert.NoError(b, err, "the routing surface builds")
			// The warm-up pass builds the call site's conversion cache, as in
			// the unscoped case.
			c := bench.Start(b).Warmup(1).MaxAllocs(0)
			defer c.End()
			n := 0
			for c.Loop() {
				n = 0
				for range ix.ByKind(symbol.KindStruct) {
					n++
				}
			}
			assert.Equal(b, n, benchFiles*benchDecls, "ByKind yields the package's structs")
		})
	})

	b.Run("ByFactKey", func(b *testing.B) {
		b.Run("scoped", func(b *testing.B) {
			reg := meta.NewRegistry()
			assert.NoError(b, reg.ClaimNamespace(flagNamespace), "the namespace is claimed")
			key, err := meta.Register[bool](reg, meta.KeySpec{Name: "t.flag", Doc: "marks a bench subject"})
			assert.NoError(b, err, "the key registers")
			stamped := meta.NewFacts(reg)
			for pkg := range benchPackages {
				id := coretest.Struct(coretest.StorePath+"/"+strconv.Itoa(pkg), "Decl0_0").ID
				assert.NoError(b, meta.Stamp(stamped, key, true, meta.Claim{Subject: id}), "the fact stamps")
			}
			ix, err := plugin.NewIndex(g, stamped, nil, inOne)
			assert.NoError(b, err, "the routing surface builds")
			// The warm-up enumeration sorts the key's subjects.
			c := bench.Start(b).Warmup(1).MaxAllocs(0)
			defer c.End()
			n := 0
			for c.Loop() {
				n = 0
				for range ix.ByFactKey(key.ID()) {
					n++
				}
			}
			assert.Equal(b, n, 1, "ByFactKey yields the one stamped subject in scope")
		})
	})

	b.Run("Skipped", func(b *testing.B) {
		validated := plugin.ValidatedMap{}
		for pkg := range benchPackages {
			id := coretest.Struct(coretest.StorePath+"/"+strconv.Itoa(pkg), "Decl0_0").ID
			validated[id] = []directive.Directive{{Name: directive.KernelSkip}}
		}
		ix, err := plugin.NewIndex(g, facts, validated, nil)
		assert.NoError(b, err, "the routing surface builds")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		skipped, clean := false, true
		for c.Loop() {
			skipped, clean = ix.Skipped(hit, skippedPlugin), !ix.Skipped(miss, skippedPlugin)
		}
		assert.True(b, skipped, "the skipped subject is skipped")
		assert.True(b, clean, "the clean subject is not")
	})

	b.Run("Lookup", func(b *testing.B) {
		ix, err := plugin.NewIndex(g, facts, nil, nil)
		assert.NoError(b, err, "the routing surface builds")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		held := false
		for c.Loop() {
			_, held = ix.Lookup(hit)
		}
		assert.True(b, held, "the declaration is found")
	})

	b.Run("PackageOf", func(b *testing.B) {
		ix, err := plugin.NewIndex(g, facts, nil, nil)
		assert.NoError(b, err, "the routing surface builds")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		held := false
		for c.Loop() {
			_, held = ix.PackageOf(hit)
		}
		assert.True(b, held, "the declaration's package is found")
	})

	scopes := []struct {
		name   string
		scope  store.Scope
		allocs uint64
	}{
		{name: "a run without a scope", allocs: newIndexAllocs},
		{name: "a run scoped to one package", scope: inOne, allocs: newIndexAllocs},
	}
	b.Run("NewIndex", func(b *testing.B) {
		for _, tt := range scopes {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					ix  *plugin.Index
					err error
				)
				for c.Loop() {
					ix, err = plugin.NewIndex(g, facts, nil, tt.scope)
				}
				assert.NoError(b, err, "the routing surface builds")
				_, held := ix.Lookup(hit)
				assert.True(b, held, "the index finds a declaration its scope admits")
			})
		}
	})

	b.Run("ByDirective", func(b *testing.B) {
		small, inStore, _ := twoPackages(b)
		ix := index(b, small, nil, nil)
		// The warm-up pass builds the call site's conversion cache, as in
		// the ByKind cases.
		c := bench.Start(b).Warmup(1).MaxAllocs(0)
		defer c.End()
		n := 0
		for c.Loop() {
			n = 0
			for range ix.ByDirective("stub") {
				n++
			}
		}
		assert.Equal(b, n, 2, "ByDirective yields both stubbed structs")
		assert.Empty(b, ix.DirectivesOf(inStore.ID), "no directive was validated")
	})

	b.Run("DirectivesOf", func(b *testing.B) {
		validated := plugin.ValidatedMap{hit: {{Name: directive.KernelSkip}}}
		ix, err := plugin.NewIndex(g, facts, validated, nil)
		assert.NoError(b, err, "the routing surface builds")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []directive.Directive
		for c.Loop() {
			got = ix.DirectivesOf(hit)
		}
		assert.Length(b, got, 1, "DirectivesOf returns the subject's instance")
	})

	b.Run("ValidatedMap", func(b *testing.B) {
		b.Run("DirectivesOf", func(b *testing.B) {
			validated := plugin.ValidatedMap{hit: {{Name: directive.KernelSkip}}}
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got []directive.Directive
			for c.Loop() {
				got = validated.DirectivesOf(hit)
			}
			assert.Length(b, got, 1, "DirectivesOf returns the subject's instance")
		})
	})

	b.Run("Reader", func(b *testing.B) {
		ix, err := plugin.NewIndex(g, facts, nil, nil)
		assert.NoError(b, err, "the routing surface builds")
		reads := store.NewReadSet()
		c := bench.Start(b).MaxAllocs(readerAllocs)
		defer c.End()
		var r *store.Reader
		for c.Loop() {
			r, err = ix.Reader(reads)
		}
		assert.NoError(b, err, "the reader is minted")
		_, held := r.Lookup(hit)
		assert.True(b, held, "the reader finds a declaration")
	})
}

// storeOnly is the scope admitting the store package alone.
func storeOnly(pkg symbol.Identity) bool {
	return pkg.Package == coretest.StorePath
}

// twoPackages returns a frozen graph with one struct per fixture
// package and one raw stub directive on each struct.
func twoPackages(tb assert.TB) (*store.Graph, *node.Struct, *node.Struct) {
	tb.Helper()

	inStore := coretest.Struct(coretest.StorePath, "Store")
	inCache := coretest.Struct(coretest.CachePath, "Cache")
	g := store.New()
	assert.NoError(tb, g.AddPackage(coretest.Package(coretest.StorePath, inStore)),
		"the store package is admitted")
	assert.NoError(tb, g.AddPackage(coretest.Package(coretest.CachePath, inCache)),
		"the cache package is admitted")
	assert.NoError(tb, g.AttachDirectives(inStore.ID, []directive.Raw{{Name: "stub"}}),
		"the raw instance attaches before the seal")
	assert.NoError(tb, g.AttachDirectives(inCache.ID, []directive.Raw{{Name: "stub"}}),
		"the second raw instance attaches before the seal")
	g.Freeze()
	return g, inStore, inCache
}

// index returns a routing surface over the fixture graph, no facts
// stamped and no directives validated unless the case adds them.
func index(tb assert.TB, g *store.Graph, validated plugin.Validated, sc store.Scope) *plugin.Index {
	tb.Helper()

	ix, err := plugin.NewIndex(g, meta.NewFacts(meta.NewRegistry()), validated, sc)
	assert.NoError(tb, err, "the routing surface builds over a frozen graph")
	return ix
}

// everything is the scope that admits every package, which is not
// the same as no scope: a case about the scoped code path states
// one and does not pass nil.
func everything(symbol.Identity) bool { return true }

// names collects the yielded declarations' names.
func names(tb assert.TB, seq func(func(symbol.Symbol) bool)) []string {
	tb.Helper()

	var out []string
	for s := range seq {
		decl, ok := s.(node.Declaration)
		assert.True(tb, ok, "the index yields declarations")
		out = append(out, decl.Identity().Name)
	}
	return out
}

// flagged stamps one boolean fact on each subject and returns the
// store together with the key it registered.
func flagged(tb assert.TB, subjects ...symbol.Identity) (*meta.Facts, meta.Key[bool]) {
	tb.Helper()

	reg := meta.NewRegistry()
	assert.NoError(tb, reg.ClaimNamespace(flagNamespace), "the namespace is claimed")
	key, err := meta.Register[bool](reg, meta.KeySpec{
		Name: "t.flag", Doc: "marks a fixture subject",
	})
	assert.NoError(tb, err, "the key registers")
	facts := meta.NewFacts(reg)
	for _, id := range subjects {
		assert.NoError(tb, meta.Stamp(facts, key, true, meta.Claim{Subject: id}),
			"the fixture fact stamps")
	}
	return facts, key
}

// subjects collects the identities a fact enumeration yielded.
func subjects(seq func(func(symbol.Identity) bool)) []string {
	var out []string
	for id := range seq {
		out = append(out, id.Name)
	}
	return out
}
