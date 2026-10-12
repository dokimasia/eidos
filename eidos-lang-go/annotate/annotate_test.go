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
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/nodetest"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/store"
)

// The packages the fixture graph declares: the corpus package, and
// another whose method names a corpus type as its receiver.
const (
	fixPackage   = "fix"
	otherPackage = "other"
)

// The builtins of the types of the fixture.
const (
	stringType = "string"
	intType    = "int"
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
	assert.NoError(t, golang.Keys(registry.For(string(golang.Lang))), "the keys of the Go frontend register")
	annotator := annotate.New()
	keyed, is := annotator.(plugin.KeyProvider)
	assert.True(t, is, "the annotator declares its keys")
	assert.NoError(t, keyed.Keys(registry.For(string(golang.AnnotatorName))), "the annotator finds its keys")

	facts := meta.NewFacts(registry)
	index, err := plugin.NewIndex(fixture(t), facts, nil, nil)
	assert.NoError(t, err, "the graph indexes")
	reader, err := index.Reader(store.NewReadSet())
	assert.NoError(t, err, "a tracked reader mints")
	sink := diag.NewSink()
	assert.NoError(t, annotator.Annotate(&plugin.AnnotatorContext{
		Index: index, Reader: reader, Facts: facts,
		Sink: sink, Plugin: golang.AnnotatorName, Turn: 0,
	}), "the annotator runs clean")

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an annotator under a name other than the name of the backend", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, annotator.Name(), golang.AnnotatorName, "the annotator has a name of its own")
		})

		t.Run("returns from Keys the error of a registry without the keys of the Go frontend", func(t *testing.T) {
			t.Parallel()

			keys, provides := annotate.New().(plugin.KeyProvider)
			assert.True(t, provides, "the annotator declares its keys")
			assert.HasError(t, keys.Keys(meta.NewRegistry()), "the annotator registers no key of its own")
		})

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
				name: "stamps no satisfies-stringer on a struct that embeds two String methods at one depth",
				key:  golang.SatisfiesStringerKey, subject: "Ambiguous",
			},
			{
				name: "stamps satisfies-stringer on a type whose String returns an alias of string",
				key:  golang.SatisfiesStringerKey, subject: "Labelled", want: true,
			},
			{
				name: "stamps no satisfies-stringer on a type whose String returns a defined type over string",
				key:  golang.SatisfiesStringerKey, subject: "Tagged",
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

// fixture builds a sealed graph the proofs can read: a type
// satisfying error through a package-level method, a struct
// embedding an interface and inheriting its String, a comparable
// struct, and one made incomparable by a slice field.
func fixture(tb assert.TB) *store.Graph {
	tb.Helper()

	b := nodetest.New(gotesting.Language{})
	stringer := &node.Interface{
		Name: "Stringish",
		Methods: []*node.Method{{
			Name: "String", Abstract: true,
			Returns: []*node.Return{{Type: nodetest.Named(stringType)}},
		}},
	}
	failer := &node.Struct{Name: "Failer"}
	errMethod := &node.Method{
		Name:     "Error",
		Receives: b.Ref(failer),
		Returns:  []*node.Return{{Type: nodetest.Named(stringType)}},
	}
	wrapper := &node.Struct{Name: "Wrapper", Embeds: []*node.Embed{{Ref: b.Ref(stringer)}}}
	// Ambiguous embeds two interfaces that both declare String, so the
	// selector is ambiguous at one depth and Go promotes neither method.
	printer := &node.Interface{
		Name: "Printer",
		Methods: []*node.Method{{
			Name: "String", Abstract: true,
			Returns: []*node.Return{{Type: nodetest.Named(stringType)}},
		}},
	}
	ambiguous := &node.Struct{
		Name:   "Ambiguous",
		Embeds: []*node.Embed{{Ref: b.Ref(stringer)}, {Ref: b.Ref(printer)}},
	}
	flat := &node.Struct{
		Name: "Flat",
		Fields: []*node.Field{
			{Name: "a", Type: nodetest.Named(intType)},
			{Name: "w", Type: b.Ref(wrapper)},
		},
	}
	slippery := &node.Struct{
		Name:   "Slippery",
		Fields: []*node.Field{{Name: "xs", Type: nodetest.List(nodetest.Named(intType))}},
	}
	leaky := &node.Struct{Name: "Leaky", Embeds: []*node.Embed{{Ref: b.Ref(slippery)}}}
	holder := &node.Struct{
		Name:   "Holder",
		Fields: []*node.Field{{Name: "h", Type: nodetest.Named("interfaceHolder")}},
	}
	nested := &node.Interface{Name: "Nested", Embeds: []*node.Embed{{Ref: b.Ref(stringer)}}}
	errno := &node.Alias{Name: "Errno", Defined: true, Target: nodetest.Named("uintptr")}
	errnoMethod := &node.Method{
		Name:     "Error",
		Receives: b.Ref(errno),
		Returns:  []*node.Return{{Type: nodetest.Named(stringType)}},
	}
	// Label is an alias of string, so a String that returns it returns
	// string. Tag is a defined type over string, and a String that returns
	// it does not.
	label := &node.Alias{Name: "Label", Target: nodetest.Named(stringType)}
	tag := &node.Alias{Name: "Tag", Defined: true, Target: nodetest.Named(stringType)}
	labelled := &node.Struct{
		Name:    "Labelled",
		Methods: []*node.Method{{Name: "String", Returns: []*node.Return{{Type: b.Ref(label)}}}},
	}
	tagged := &node.Struct{
		Name:    "Tagged",
		Methods: []*node.Method{{Name: "String", Returns: []*node.Return{{Type: b.Ref(tag)}}}},
	}
	// A method in another package whose receiver shares a fixture
	// type's name attaches to nothing here.
	foreign := &node.Method{
		Name:     "Error",
		Receives: nodetest.Named("Flat"),
		Returns:  []*node.Return{{Type: nodetest.Named(stringType)}},
	}
	b.File(fixPackage, "fix/a.go",
		stringer, failer, errMethod, wrapper, printer, ambiguous, flat, slippery,
		leaky, holder, nested, errno, errnoMethod,
		label, tag, labelled, tagged,
	)
	b.File(otherPackage, "other/b.go", foreign)

	g := store.New()
	assert.Total(tb, g.AddPackage, b.Build(tb), "the fixture package is admitted")
	g.Freeze()
	return g
}
