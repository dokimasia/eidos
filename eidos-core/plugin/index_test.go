// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

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

// storeOnly is the scope admitting the store package alone.
func storeOnly(pkg symbol.Identity) bool {
	return pkg.Package == coretest.StorePath
}

// twoPackages returns a frozen graph holding one struct per fixture
// package, with the store struct carrying one raw stub directive.
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

// names collects the yielded declarations' names.
func names(tb assert.TB, seq func(func(symbol.Symbol) bool)) []string {
	tb.Helper()

	var out []string
	for s := range seq {
		decl, ok := s.(node.Declaration)
		assert.True(tb, ok, "the index yields held declarations")
		out = append(out, decl.Identity().Name)
	}
	return out
}

// The index is the dispatcher's routing surface: untracked and
// scope-filtered, holding the validated directive table and the skip
// table, and minting the tracked readers handlers hold. Its filters
// are the visibility rule, so every one is contract.
func TestIndex(t *testing.T) {
	t.Parallel()

	t.Run("NewIndex", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses an unfrozen graph", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.NewIndex(
				store.New(), meta.NewFacts(meta.NewRegistry()), nil, nil,
			)
			assert.HasError(t, err,
				"routing over a moving graph would return partial results")
		})

		t.Run("refuses a missing graph", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.NewIndex(nil, meta.NewFacts(meta.NewRegistry()), nil, nil)
			assert.HasError(t, err, "there is nothing to route over")
		})

		t.Run("refuses a missing fact store", func(t *testing.T) {
			t.Parallel()

			g, _, _ := twoPackages(t)
			_, err := plugin.NewIndex(g, nil, nil, nil)
			assert.HasError(t, err, "fact gates would have nothing to evaluate against")
		})
	})

	t.Run("ByKind", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every declaration of one kind", func(t *testing.T) {
			t.Parallel()

			g, _, _ := twoPackages(t)
			got := names(t, index(t, g, nil, nil).ByKind(symbol.KindStruct))
			assert.Equal(t, got, []string{"Cache", "Store"},
				"a nil scope admits everything, in the graph's own order")
		})

		t.Run("filters by scope", func(t *testing.T) {
			t.Parallel()

			g, _, _ := twoPackages(t)
			got := names(t, index(t, g, nil, storeOnly).ByKind(symbol.KindStruct))
			assert.Equal(t, got, []string{"Store"},
				"a declaration outside scope is not returned")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			g, _, _ := twoPackages(t)
			var got int
			for range index(t, g, nil, nil).ByKind(symbol.KindStruct) {
				got++
				break
			}
			assert.Equal(t, got, 1, "the iteration stops when the range stops")
		})
	})

	t.Run("ByDirective", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the carriers under scope", func(t *testing.T) {
			t.Parallel()

			g, _, _ := twoPackages(t)
			ix := index(t, g, nil, storeOnly)
			got := names(t, ix.ByDirective(directive.Name("stub")))
			assert.Equal(t, got, []string{"Store"},
				"a carrier outside scope is not returned")
		})
	})

	t.Run("ByFactKey", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the stamped subjects under scope", func(t *testing.T) {
			t.Parallel()

			g, inStore, inCache := twoPackages(t)

			reg := meta.NewRegistry()
			assert.NoError(t, reg.ClaimNamespace("t", "the test"),
				"the namespace is claimed")
			key, err := meta.Register[bool](reg, meta.KeySpec{
				Name: "t.flag", Doc: "marks a fixture subject",
			})
			assert.NoError(t, err, "the key registers")
			facts := meta.NewFacts(reg)
			for _, id := range []symbol.Identity{inStore.ID, inCache.ID} {
				assert.NoError(t,
					meta.Stamp(facts, key, true, meta.Claim{Subject: id}),
					"the fixture fact stamps")
			}

			ix, err := plugin.NewIndex(g, facts, nil, storeOnly)
			assert.NoError(t, err, "the routing surface builds")

			var got []string
			for id := range ix.ByFactKey(key.ID()) {
				got = append(got, id.Name)
			}
			assert.Equal(t, got, []string{"Store"},
				"a stamped subject outside scope is not returned")
		})
	})

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a held declaration", func(t *testing.T) {
			t.Parallel()

			g, inStore, _ := twoPackages(t)
			got, held := index(t, g, nil, nil).Lookup(inStore.ID)
			assert.True(t, held, "a held identity resolves")
			assert.True(t, got == symbol.Symbol(inStore), "to the very declaration")
		})

		t.Run("returns false outside scope", func(t *testing.T) {
			t.Parallel()

			g, _, inCache := twoPackages(t)
			_, held := index(t, g, nil, storeOnly).Lookup(inCache.ID)
			assert.False(t, held,
				"a declaration outside scope is neither returned nor reachable")
		})
	})

	t.Run("DirectivesOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the validated instances as handed", func(t *testing.T) {
			t.Parallel()

			g, inStore, _ := twoPackages(t)
			want := []directive.Directive{
				{Name: "stub", Instance: 0},
				{Name: "stub", Instance: 1},
			}
			ix := index(t, g,
				map[symbol.Identity][]directive.Directive{inStore.ID: want}, nil)
			assert.Equal(t, ix.DirectivesOf(inStore.ID), want,
				"the gate reads the table validation returned, position order intact")
		})
	})

	t.Run("Skipped", func(t *testing.T) {
		t.Parallel()

		g, inStore, inCache := twoPackages(t)
		validated := map[symbol.Identity][]directive.Directive{
			inStore.ID: {{Name: directive.KernelSkip}},
			inCache.ID: {{
				Name: directive.KernelSkip,
				Params: map[directive.ParamKey]directive.Value{
					directive.SkipPlugin: {
						Kind: directive.TypeString, Str: "stubgen",
					},
				},
			}},
		}
		ix := index(t, g, validated, nil)

		t.Run("excludes every plugin under a bare skip", func(t *testing.T) {
			t.Parallel()

			assert.True(t, ix.Skipped(inStore.ID, "stubgen"),
				"a bare skip excludes every plugin")
			assert.True(t, ix.Skipped(inStore.ID, "audit"),
				"whatever the plugin's name")
		})

		t.Run("excludes one plugin under a narrowed skip", func(t *testing.T) {
			t.Parallel()

			assert.True(t, ix.Skipped(inCache.ID, "stubgen"),
				"the named plugin is excluded")
			assert.False(t, ix.Skipped(inCache.ID, "audit"),
				"every other plugin still matches")
		})

		t.Run("excludes nothing without skip", func(t *testing.T) {
			t.Parallel()

			other := coretest.Struct(coretest.StorePath, "Other")
			assert.False(t, ix.Skipped(other.ID, "stubgen"),
				"a subject carrying no skip matches as ever")
		})
	})

	t.Run("Reader", func(t *testing.T) {
		t.Parallel()

		t.Run("mints a tracked handle under the index's scope", func(t *testing.T) {
			t.Parallel()

			g, inStore, inCache := twoPackages(t)
			ix := index(t, g, nil, storeOnly)

			var reads store.ReadSet
			r, err := ix.Reader(&reads)
			assert.NoError(t, err, "the handle mints over a frozen graph")

			_, held := r.Lookup(inStore.ID)
			assert.True(t, held, "the reader returns inside the scope")
			_, held = r.Lookup(inCache.ID)
			assert.False(t, held, "and refuses outside it")
			assert.True(t, reads.Len() > 0,
				"a read through the handle records an edge")
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
		if err := reg.ClaimNamespace("t", "the bench"); err != nil {
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
			if !ix.Skipped(hit, "stubgen") {
				b.Fatal("the skipped subject must return true")
			}
			if ix.Skipped(miss, "stubgen") {
				b.Fatal("the clean subject must return false")
			}
		}
	})
}
