// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package annotate_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/annotate"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// id spells a fixture identity in the corpus package.
func id(owner, name string, kind symbol.Kind) symbol.Identity {
	return symbol.Identity{
		Lang: "golang", Package: "fix", Owner: owner, Name: name, Kind: kind,
	}
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

	g := store.New()
	assert.NoError(tb, g.AddPackage(&node.Package{
		ID:   symbol.Identity{Lang: "golang", Package: "fix", Kind: symbol.KindPackage},
		Path: []string{"fix"},
		Files: []*node.File{{
			ID:   symbol.Identity{Lang: "golang", Package: "fix", Name: "fix/a.go", Kind: symbol.KindFile},
			Path: "fix/a.go",
			Decls: node.Symbols{
				stringer, failer, errMethod, wrapper, flat, slippery,
				leaky, holder, nested,
			},
		}},
	}), "the fixture loads")
	g.Freeze()
	return g
}

// The annotator stamps what the sealed graph proves, so each proof
// and each refusal-to-guess is pinned over a hand-built graph.
func TestNew(t *testing.T) {
	t.Parallel()

	registry := meta.NewRegistry()
	annotator := annotate.New()
	keyed, is := annotator.(plugin.KeyProvider)
	assert.True(t, is, "the annotator declares its keys")
	assert.NoError(t, keyed.Keys(registry), "and they register")

	g := fixture(t)
	facts := meta.NewFacts(registry)
	index, err := plugin.NewIndex(g, facts, nil, nil)
	assert.NoError(t, err, "the graph indexes")
	reader, err := index.Reader(store.NewReadSet())
	assert.NoError(t, err, "a tracked reader mints")
	sink := diag.NewSink()
	assert.NoError(t, annotator.Annotate(&plugin.AnnotatorContext{
		Index: index, Reader: reader, Facts: facts,
		Sink: sink, Plugin: golang.Name, Bucket: 0,
	}), "the annotator runs clean")
	for d := range sink.All() {
		t.Errorf("the annotator reported %v, and a proof pass states nothing", d)
	}

	stamped := func(key meta.KeyName) map[string]bool {
		out := map[string]bool{}
		keyID, held := registry.Resolve(key)
		assert.True(t, held, string(key)+" is registered")
		for subject := range facts.ByKey(keyID) {
			out[subject.Name] = true
		}
		return out
	}

	t.Run("proves satisfaction through attachment and embeds", func(t *testing.T) {
		t.Parallel()

		errors := stamped(golang.SatisfiesErrorKey)
		assert.True(t, errors["Failer"], "a package-level method attaches")
		assert.False(t, errors["Wrapper"], "String is not Error")

		stringers := stamped(golang.SatisfiesStringerKey)
		assert.True(t, stringers["Stringish"], "the interface satisfies itself")
		assert.True(t, stringers["Wrapper"], "and the embed carries it across")
	})

	t.Run("marks interface embedding on structs alone", func(t *testing.T) {
		t.Parallel()

		embedded := stamped(golang.EmbedsInterfaceKey)
		assert.True(t, embedded["Wrapper"],
			"the graph holds the embed as an interface")
		assert.False(t, embedded["Nested"],
			"an interface embedding an interface is the norm, not a fact")
	})

	t.Run("proves comparability and refuses to guess it", func(t *testing.T) {
		t.Parallel()

		provenComparable := stamped(golang.ComparableKey)
		assert.True(t, provenComparable["Flat"], "ints and a comparable struct compare")
		assert.False(t, provenComparable["Slippery"], "a slice field never does")
		assert.False(t, provenComparable["Leaky"],
			"an embedded field takes part in comparison, and its slice refuses")
		assert.False(t, provenComparable["Holder"],
			"an identifier merely starting with interface stays unproven")
	})
}
