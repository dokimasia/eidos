// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

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
func index(
	tb assert.TB, g *store.Graph,
	validated map[symbol.Identity][]directive.Directive, sc store.Scope,
) *plugin.Index {
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

// The index is the dispatcher's routing surface: untracked and
// scope-filtered, with the validated directive table and the skip
// table, and the source of the tracked readers handlers read
// through. Its filters are the visibility rule, so every one is
// contract.
func TestIndex(t *testing.T) {
	t.Parallel()

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
	})

	t.Run("ByKind", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every declaration of one kind", func(t *testing.T) {
			t.Parallel()

			g, _, _ := twoPackages(t)
			got := names(t, index(t, g, nil, nil).ByKind(symbol.KindStruct))
			assert.Equal(t, got, []string{"Cache", "Store"}, "the declarations are in the graph's order")
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
			assert.True(t, got == symbol.Symbol(inStore), "the declaration is the graph's")
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
			ix := index(t, g, map[symbol.Identity][]directive.Directive{inStore.ID: want}, nil)
			assert.Equal(t, ix.DirectivesOf(inStore.ID), want, "the instances are the validated table's")
		})
	})

	t.Run("Skipped", func(t *testing.T) {
		t.Parallel()

		g, inStore, inCache := twoPackages(t)
		negatedAt := coretest.Struct(coretest.StorePath, "Negated").ID
		setAt := coretest.Struct(coretest.StorePath, "Set").ID
		clean := coretest.Struct(coretest.StorePath, "Clean").ID
		validated := map[symbol.Identity][]directive.Directive{
			inStore.ID: {{Name: directive.KernelSkip}},
			inCache.ID: {{
				Name: directive.KernelSkip,
				Params: map[directive.ParamKey]directive.Value{
					directive.SkipPlugin: {Kind: directive.TypeString, Str: string(skippedPlugin)},
				},
			}},
			negatedAt: {{Name: directive.Name(string(skippedPlugin) + ":stub"), Negated: true}},
			setAt:     {{Name: directive.Name(string(skippedPlugin) + ":stub")}},
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
			assert.True(t, reads.Len() > 0, "the read set records an edge")
		})
	})
}

// The store scale the routing surface has to serve: a thousand
// packages, ten thousand files, two hundred thousand declarations.
const (
	benchPackages = 1_000
	benchFiles    = 10
	benchDecls    = 20
)

func BenchmarkIndex(b *testing.B) {
	g := coretest.Frozen(b, coretest.Workspace(benchPackages, benchFiles, benchDecls)...)
	facts := meta.NewFacts(meta.NewRegistry())
	one := coretest.StorePath + "/0"

	b.Run("ByKind/unscoped", func(b *testing.B) {
		b.ReportAllocs()
		ix, err := plugin.NewIndex(g, facts, nil, nil)
		if err != nil {
			b.Fatalf("NewIndex: unexpected error: %v", err)
		}
		for b.Loop() {
			var n int
			for range ix.ByKind(symbol.KindStruct) {
				n++
			}
			if n != benchPackages*benchFiles*benchDecls {
				b.Fatalf("ByKind yielded %d declarations", n)
			}
		}
	})

	b.Run("ByKind/scoped to one package", func(b *testing.B) {
		b.ReportAllocs()
		ix, err := plugin.NewIndex(g, facts, nil,
			func(pkg symbol.Identity) bool { return pkg.Package == one })
		if err != nil {
			b.Fatalf("NewIndex: unexpected error: %v", err)
		}
		for b.Loop() {
			var n int
			for range ix.ByKind(symbol.KindStruct) {
				n++
			}
			if n != benchFiles*benchDecls {
				b.Fatalf("ByKind yielded %d declarations", n)
			}
		}
	})

	b.Run("ByFactKey/scoped", func(b *testing.B) {
		b.ReportAllocs()
		reg := meta.NewRegistry()
		if err := reg.ClaimNamespace(flagNamespace); err != nil {
			b.Fatalf("ClaimNamespace: unexpected error: %v", err)
		}
		key, err := meta.Register[bool](reg, meta.KeySpec{
			Name: "t.flag", Doc: "marks a bench subject",
		})
		if err != nil {
			b.Fatalf("Register: unexpected error: %v", err)
		}
		stamped := meta.NewFacts(reg)
		for pkg := range benchPackages {
			id := coretest.Struct(
				coretest.StorePath+"/"+strconv.Itoa(pkg), "Decl0_0",
			).ID
			if serr := meta.Stamp(stamped, key, true, meta.Claim{Subject: id}); serr != nil {
				b.Fatalf("Stamp: unexpected error: %v", serr)
			}
		}
		ix, err := plugin.NewIndex(g, stamped, nil,
			func(pkg symbol.Identity) bool { return pkg.Package == one })
		if err != nil {
			b.Fatalf("NewIndex: unexpected error: %v", err)
		}
		for b.Loop() {
			var n int
			for range ix.ByFactKey(key.ID()) {
				n++
			}
			if n != 1 {
				b.Fatalf("ByFactKey yielded %d subjects", n)
			}
		}
	})

	b.Run("Skipped", func(b *testing.B) {
		b.ReportAllocs()
		validated := map[symbol.Identity][]directive.Directive{}
		for pkg := range benchPackages {
			id := coretest.Struct(
				coretest.StorePath+"/"+strconv.Itoa(pkg), "Decl0_0",
			).ID
			validated[id] = []directive.Directive{{Name: directive.KernelSkip}}
		}
		ix, err := plugin.NewIndex(g, facts, validated, nil)
		if err != nil {
			b.Fatalf("NewIndex: unexpected error: %v", err)
		}
		hit := coretest.Struct(one, "Decl0_0").ID
		miss := coretest.Struct(one, "Decl0_1").ID
		for b.Loop() {
			if !ix.Skipped(hit, skippedPlugin) {
				b.Fatal("the skipped subject must return true")
			}
			if ix.Skipped(miss, skippedPlugin) {
				b.Fatal("the clean subject must return false")
			}
		}
	})
}
