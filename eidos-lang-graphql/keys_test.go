// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// namespace pins the spelling that every key opens with. rivalPlugin is a
// second registrant, and rivalKey is the key that it tries to register
// under the namespace of the satellite.
const (
	namespace                = "graphql"
	rivalPlugin              = "rival"
	rivalKey    meta.KeyName = "graphql.rival"
)

// keysAllocs is the count of a registration into a fresh registry. Keys
// allocates the nine kind lists of the keys. The registry allocates fifteen
// times for its namespace claim and for the growth of its lists and its map
// to twelve keys.
const keysAllocs = 9 + 15

// TestKeys pins the spellings of the keys, the value type of each, the
// kinds that each admits and the claim of the namespace, because a consumer
// reads the stamps of the frontend through them.
func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("Keys", func(t *testing.T) {
		t.Parallel()

		t.Run("registers each list key as a list of text", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, graphql.Keys(r), "the keys register")
			for _, k := range every() {
				_, held := meta.Lookup[[]string](r, k.name)
				expect.Equal(t, held, k.list, string(k.name)+" is a list of text exactly when its stamp is a list")
			}
		})

		t.Run("registers every other key as text", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, graphql.Keys(r), "the keys register")
			for _, k := range every() {
				_, held := meta.Lookup[string](r, k.name)
				expect.Equal(t, held, !k.list, string(k.name)+" is text exactly when its stamp is not a list")
			}
		})

		t.Run("returns no error for a second registration of one registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry().For(string(graphql.Name))
			assert.NoError(t, graphql.Keys(r), "the first registration succeeds")
			assert.NoError(t, graphql.Keys(r), "the registrant repeats its registration")
		})

		t.Run("returns an error for a registration of another registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, graphql.Keys(r.For(string(graphql.Name))), "the first registration succeeds")
			assert.HasError(t, graphql.Keys(r.For(rivalPlugin)), "the namespace has one registrant")
		})

		t.Run("returns an error for a key whose spelling a group took", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			rival := r.For(rivalPlugin)
			assert.NoError(t, rival.ClaimNamespace(rivalPlugin), "the rival claims its own namespace")
			_, err := meta.Register[bool](rival, meta.KeySpec{
				Name: rivalPlugin + ".grouped", Group: meta.GroupName(graphql.SchemaKey), Doc: "a key in a group",
			})
			assert.NoError(t, err, "the key of the rival registers into a group that spells a graphql key")
			err = graphql.Keys(r)
			assert.HasError(t, err, "a key and a group share no spelling")
			assert.Contains(t, err.Error(), string(graphql.SchemaKey), "the error names the key")
		})

		t.Run("returns an error for a list key whose spelling a group took", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			rival := r.For(rivalPlugin)
			assert.NoError(t, rival.ClaimNamespace(rivalPlugin), "the rival claims its own namespace")
			_, err := meta.Register[bool](rival, meta.KeySpec{
				Name: rivalPlugin + ".grouped", Group: meta.GroupName(graphql.SpreadsKey), Doc: "a key in a group",
			})
			assert.NoError(t, err, "the key of the rival registers into a group that spells a graphql key")
			err = graphql.Keys(r)
			assert.HasError(t, err, "a key and a group share no spelling")
			assert.Contains(t, err.Error(), string(graphql.SpreadsKey), "the error names the list key")
		})

		t.Run("registers every other key when a group took the spelling of one", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			rival := r.For(rivalPlugin)
			assert.NoError(t, rival.ClaimNamespace(rivalPlugin), "the rival claims its own namespace")
			_, err := meta.Register[bool](rival, meta.KeySpec{
				Name: rivalPlugin + ".grouped", Group: meta.GroupName(graphql.SchemaKey), Doc: "a key in a group",
			})
			assert.NoError(t, err, "the key of the rival registers into a group that spells a graphql key")
			assert.HasError(t, graphql.Keys(r), "the first key fails")
			_, held := meta.Lookup[[]string](r, graphql.SpreadsKey)
			assert.True(t, held, "the last key registers after the first one fails")
		})

		t.Run("registers every key under the graphql namespace", func(t *testing.T) {
			t.Parallel()

			for _, k := range every() {
				expect.Equal(t, k.name.Namespace(), namespace,
					string(k.name)+" is spelled under the namespace of the satellite")
			}
		})

		t.Run("claims the namespace for the registrant of the handle", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, graphql.Keys(r.For(string(graphql.Name))),
				"the keys register through the handle of the plugin")
			_, err := meta.Register[string](r.For(rivalPlugin), meta.KeySpec{
				Name: rivalKey, Doc: "a key under the namespace of another plugin",
			})
			assert.HasError(t, err, "another plugin registers no key under the namespace")
		})

		t.Run("registers each key for the kinds that the frontend stamps it on", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, graphql.Keys(r), "the keys register")
			for _, k := range every() {
				id, held := r.Resolve(k.name)
				assert.True(t, held, string(k.name)+" resolves")
				spec, held := r.Spec(id)
				assert.True(t, held, string(k.name)+" has a spec")
				expect.Equal(t, spec.Kinds, k.kinds, "the kinds of "+string(k.name))
			}
		})
	})

	t.Run("SchemaStatement", func(t *testing.T) {
		t.Parallel()

		t.Run("spells schema", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.SchemaStatement, "schema", "the value of the schema stamp of a schema definition")
		})
	})

	t.Run("ExtendStatement", func(t *testing.T) {
		t.Parallel()

		t.Run("spells extend", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.ExtendStatement, "extend", "the value of the schema stamp of an extension")
		})
	})
}

// TestKeysAllocs checks the ceiling of a registration in the ordinary run,
// which does not run benchmarks. Each call registers into a registry of its
// own, and the setup builds the registry outside the count. The count keeps
// the first error of its calls, and cmp.Or returns it without allocating.
func TestKeysAllocs(t *testing.T) {
	var err error
	assert.MaxAllocsWithSetup(t, meta.NewRegistry, func(r *meta.Registry) { err = cmp.Or(err, graphql.Keys(r)) },
		keysAllocs, "Keys allocates the kind lists and the growth of the registry")
	assert.NoError(t, err, "Keys registers the vocabulary")
}

// BenchmarkKeys measures the registration that a composition makes once,
// each into a registry of its own.
func BenchmarkKeys(b *testing.B) {
	b.Run("Keys", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(keysAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			var r *meta.Registry
			c.Excluding(func() { r = meta.NewRegistry() })
			err = graphql.Keys(r)
		}
		assert.NoError(b, err, "Keys registers the vocabulary")
	})
}

// key is one key of the satellite. list reports whether the frontend stamps
// the key as a list of text, and kinds are the kinds that the key admits.
type key struct {
	name  meta.KeyName
	list  bool
	kinds []symbol.Kind
}

// every returns the whole key set of the satellite, which a composition
// registers as one and a corpus fixture declares as one.
func every() []key {
	file := []symbol.Kind{symbol.KindFile}
	function := []symbol.Kind{symbol.KindFunction}
	return []key{
		{name: graphql.SchemaKey, kinds: file},
		{name: graphql.RootsKey, list: true, kinds: file},
		{name: graphql.OperationKey, kinds: []symbol.Kind{symbol.KindStruct, symbol.KindFunction}},
		{name: graphql.InputKey, kinds: []symbol.Kind{symbol.KindStruct}},
		{name: graphql.DirectiveKey, list: true, kinds: []symbol.Kind{symbol.KindInterface}},
		{name: graphql.RepeatableKey, kinds: []symbol.Kind{symbol.KindInterface}},
		{
			name: graphql.ExtensionKey,
			kinds: []symbol.Kind{
				symbol.KindStruct, symbol.KindInterface, symbol.KindEnum, symbol.KindAlias,
			},
		},
		{name: graphql.AnonymousKey, kinds: function},
		{name: graphql.FragmentKey, kinds: function},
		{name: graphql.SelectionKey, kinds: function},
		{name: graphql.CoordinateKey, kinds: []symbol.Kind{symbol.KindField}},
		{name: graphql.SpreadsKey, list: true, kinds: []symbol.Kind{symbol.KindStruct, symbol.KindSumVariant}},
	}
}
