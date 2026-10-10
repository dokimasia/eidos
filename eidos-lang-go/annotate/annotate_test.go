// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package annotate_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/annotate"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages the fixture graph declares: the corpus package, and
// another whose method names a corpus type as its receiver.
const (
	fixPackage   = "fix"
	otherPackage = "other"
)

// newAllocs is the annotator New returns: its four handlers, the proof
// they share, the handles they stamp under and the key registration,
// and the kit's 24 that build the plugin.
const newAllocs = 4 + 1 + 1 + 1 + 24

// The annotator stamps what the sealed graph proves, so each proof and
// each fact it leaves unstated is pinned over a hand-built graph.
func TestAnnotate(t *testing.T) {
	t.Parallel()

	registry := meta.NewRegistry()
	annotator := annotate.New()
	keyed, is := annotator.(plugin.KeyProvider)
	assert.True(t, is, "the annotator declares its keys")
	assert.NoError(t, keyed.Keys(registry), "the keys register")

	facts := meta.NewFacts(registry)
	index, err := plugin.NewIndex(fixture(t), facts, nil, nil)
	assert.NoError(t, err, "the graph indexes")
	reader, err := index.Reader(store.NewReadSet())
	assert.NoError(t, err, "a tracked reader mints")
	sink := diag.NewSink()
	assert.NoError(t, annotator.Annotate(&plugin.AnnotatorContext{
		Index: index, Reader: reader, Facts: facts,
		Sink: sink, Plugin: golang.Name, Bucket: 0,
	}), "the annotator runs clean")

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			key     meta.KeyName
			subject string
			want    bool
		}{
			{
				name: "stamps satisfies-error on a type a package-level Error method attaches to",
				key:  golang.SatisfiesErrorKey, subject: "Failer", want: true,
			},
			{
				name: "stamps no satisfies-error on a type whose only method is String",
				key:  golang.SatisfiesErrorKey, subject: "Wrapper",
			},
			{
				name: "stamps satisfies-error on a defined type over a builtin",
				key:  golang.SatisfiesErrorKey, subject: "Errno", want: true,
			},
			{
				name: "stamps no satisfies-error through a method another package declares",
				key:  golang.SatisfiesErrorKey, subject: "Flat",
			},
			{
				name: "stamps satisfies-stringer on an interface that declares String",
				key:  golang.SatisfiesStringerKey, subject: "Stringish", want: true,
			},
			{
				name: "stamps satisfies-stringer through an embedded interface",
				key:  golang.SatisfiesStringerKey, subject: "Wrapper", want: true,
			},
			{
				name: "stamps embeds-interface on a struct that embeds an interface",
				key:  golang.EmbedsInterfaceKey, subject: "Wrapper", want: true,
			},
			{
				name: "stamps no embeds-interface on an interface that embeds an interface",
				key:  golang.EmbedsInterfaceKey, subject: "Nested",
			},
			{
				name: "stamps comparable on a struct of comparable fields",
				key:  golang.ComparableKey, subject: "Flat", want: true,
			},
			{
				name: "stamps no comparable on a struct with a slice field",
				key:  golang.ComparableKey, subject: "Slippery",
			},
			{
				name: "stamps no comparable on a struct that embeds an incomparable struct",
				key:  golang.ComparableKey, subject: "Leaky",
			},
			{
				name: "stamps no comparable on a field type that only starts with interface",
				key:  golang.ComparableKey, subject: "Holder",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, stamped(t, registry, facts, tt.key)[tt.subject], tt.want, "the stamp the graph proves")
			})
		}

		t.Run("reports nothing for a graph it proves facts over", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, slices.Collect(sink.All()), "a proof pass states no finding")
		})
	})
}

// The construction allocates the handlers and the plugin the kit
// builds. The ordinary run, which runs no benchmark, checks that
// ceiling here.
func TestAnnotateAllocs(t *testing.T) {
	var a plugin.Annotator
	assert.MaxAllocs(t, func() { a = annotate.New() }, newAllocs, "New allocates within its ceiling")
	assert.NotNil(t, a, "New returns the annotator")
}

// BenchmarkAnnotate measures the construction a composition makes once
// per run. The construction runs once before the contract starts, so
// what the first call initialises stays out of the count.
func BenchmarkAnnotate(b *testing.B) {
	b.Run("New", func(b *testing.B) {
		a := annotate.New()
		c := bench.Start(b).MaxAllocs(newAllocs)
		defer c.End()
		for c.Loop() {
			a = annotate.New()
		}
		assert.NotNil(b, a, "New returns the annotator")
	})
}

// stamped returns the names of the subjects a run stamped under one
// key.
func stamped(tb assert.TB, registry *meta.Registry, facts *meta.Facts, key meta.KeyName) map[string]bool {
	tb.Helper()

	keyID, held := registry.Resolve(key)
	assert.True(tb, held, string(key)+" is registered")
	out := map[string]bool{}
	for subject := range facts.ByKey(keyID) {
		out[subject.Name] = true
	}
	return out
}

// id spells a fixture identity in the corpus package.
func id(owner, name string, kind symbol.Kind) symbol.Identity {
	return symbol.Identity{Lang: golang.Lang, Package: fixPackage, Owner: owner, Name: name, Kind: kind}
}

// fixture builds a sealed graph the proofs can read: a type
// satisfying error through a package-level method, a struct
// embedding an interface and inheriting its String, a comparable
// struct, and one made incomparable by a slice field.
func fixture(tb assert.TB) *store.Graph {
	tb.Helper()

	stringer := &node.Interface{
		ID: id("", "Stringish", symbol.KindInterface), Name: "Stringish",
		Methods: []*node.Method{{
			ID: id("Stringish", "String", symbol.KindMethod), Name: "String", Abstract: true,
			Returns: []*node.Return{{Type: &node.TypeRef{Spelling: "string"}}},
		}},
	}
	failer := &node.Struct{
		ID: id("", "Failer", symbol.KindStruct), Name: "Failer",
	}
	errMethod := &node.Method{
		ID: id("Failer", "Error", symbol.KindMethod), Name: "Error",
		Receives: &node.TypeRef{Spelling: "Failer"},
		Returns:  []*node.Return{{Type: &node.TypeRef{Spelling: "string"}}},
	}
	wrapper := &node.Struct{
		ID: id("", "Wrapper", symbol.KindStruct), Name: "Wrapper",
		Embeds: []*node.Embed{{
			Ref: &node.TypeRef{Spelling: "Stringish", Target: stringer.ID},
		}},
	}
	flat := &node.Struct{
		ID: id("", "Flat", symbol.KindStruct), Name: "Flat",
		Fields: []*node.Field{
			{ID: id("Flat", "a", symbol.KindField), Name: "a", Type: &node.TypeRef{Spelling: "int"}},
			{
				ID: id("Flat", "w", symbol.KindField), Name: "w",
				Type: &node.TypeRef{Spelling: "Wrapper", Target: wrapper.ID},
			},
		},
	}
	slippery := &node.Struct{
		ID: id("", "Slippery", symbol.KindStruct), Name: "Slippery",
		Fields: []*node.Field{{
			ID: id("Slippery", "xs", symbol.KindField), Name: "xs",
			Type: &node.TypeRef{Spelling: "[]int"},
		}},
	}
	leaky := &node.Struct{
		ID: id("", "Leaky", symbol.KindStruct), Name: "Leaky",
		Embeds: []*node.Embed{{
			Ref: &node.TypeRef{Spelling: "Slippery", Target: slippery.ID},
		}},
	}
	holder := &node.Struct{
		ID: id("", "Holder", symbol.KindStruct), Name: "Holder",
		Fields: []*node.Field{{
			ID: id("Holder", "h", symbol.KindField), Name: "h",
			Type: &node.TypeRef{Spelling: "interfaceHolder"},
		}},
	}
	nested := &node.Interface{
		ID: id("", "Nested", symbol.KindInterface), Name: "Nested",
		Embeds: []*node.Embed{{
			Ref: &node.TypeRef{Spelling: "Stringish", Target: stringer.ID},
		}},
	}
	errno := &node.Alias{
		ID: id("", "Errno", symbol.KindAlias), Name: "Errno", Defined: true,
		Target: &node.TypeRef{Spelling: "uintptr"},
	}
	errnoMethod := &node.Method{
		ID: id("Errno", "Error", symbol.KindMethod), Name: "Error",
		Receives: &node.TypeRef{Spelling: "Errno"},
		Returns:  []*node.Return{{Type: &node.TypeRef{Spelling: "string"}}},
	}
	// A method in another package whose receiver shares a fixture
	// type's name attaches to nothing here.
	foreign := &node.Method{
		ID: symbol.Identity{
			Lang: golang.Lang, Package: otherPackage, Owner: "Flat", Name: "Error", Kind: symbol.KindMethod,
		},
		Name:     "Error",
		Receives: &node.TypeRef{Spelling: "Flat"},
		Returns:  []*node.Return{{Type: &node.TypeRef{Spelling: "string"}}},
	}

	g := store.New()
	assert.NoError(tb, g.AddPackage(&node.Package{
		ID:   symbol.Identity{Lang: golang.Lang, Package: fixPackage, Kind: symbol.KindPackage},
		Path: []string{fixPackage},
		Files: []*node.File{{
			ID:   symbol.Identity{Lang: golang.Lang, Package: fixPackage, Name: "fix/a.go", Kind: symbol.KindFile},
			Path: "fix/a.go",
			Decls: node.Symbols{
				stringer, failer, errMethod, wrapper, flat, slippery,
				leaky, holder, nested, errno, errnoMethod,
			},
		}},
	}), "the fixture loads")
	assert.NoError(tb, g.AddPackage(&node.Package{
		ID:   symbol.Identity{Lang: golang.Lang, Package: otherPackage, Kind: symbol.KindPackage},
		Path: []string{otherPackage},
		Files: []*node.File{{
			ID:    symbol.Identity{Lang: golang.Lang, Package: otherPackage, Name: "other/b.go", Kind: symbol.KindFile},
			Path:  "other/b.go",
			Decls: node.Symbols{foreign},
		}},
	}), "the second package loads")
	g.Freeze()
	return g
}
