// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// namespace pins the spelling that every key opens with. rivalPlugin is a
// second registrant, and rivalKey is the key that it tries to register
// under the namespace of the satellite.
const (
	namespace                = "python"
	rivalPlugin              = "rival"
	rivalKey    meta.KeyName = "python.rival"
)

// keysAllocs is the count of a registration into a fresh registry. Keys
// allocates the eight kind lists of the keys. The registry allocates
// fifteen times for its namespace claim and for the growth of its lists and
// its map to fourteen keys.
const keysAllocs = 8 + 15

// value is the type of the value that the frontend stamps under a key.
type value uint8

// The value types of the keys.
const (
	// mark is a boolean mark.
	mark value = iota
	// text is one text.
	text
	// list is a list of text.
	list
)

// key is one key of the satellite. value is the type of its stamps, and
// kinds are the kinds that the key admits.
type key struct {
	name  meta.KeyName
	value value
	kinds []symbol.Kind
}

// TestKeys pins the spellings of the keys, the value type of each, the
// kinds that each admits and the claim of the namespace, because a consumer
// reads the stamps of the frontend through them.
func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("Keys", func(t *testing.T) {
		t.Parallel()

		t.Run("registers each mark as a bool", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, python.Keys(r), "the keys register")
			for _, k := range every() {
				_, held := meta.Lookup[bool](r, k.name)
				expect.Equal(t, held, k.value == mark, string(k.name)+" is a bool exactly when its stamp is a mark")
			}
		})

		t.Run("registers the key of a type variable as text", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, python.Keys(r), "the keys register")
			for _, k := range every() {
				_, held := meta.Lookup[string](r, k.name)
				expect.Equal(t, held, k.value == text, string(k.name)+" is text exactly when its stamp is text")
			}
		})

		t.Run("registers each list key as a list of text", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, python.Keys(r), "the keys register")
			for _, k := range every() {
				_, held := meta.Lookup[[]string](r, k.name)
				expect.Equal(t, held, k.value == list,
					string(k.name)+" is a list of text exactly when its stamp is a list")
			}
		})

		t.Run("returns no error for a second registration of one registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry().For(string(python.Name))
			assert.NoError(t, python.Keys(r), "the first registration succeeds")
			assert.NoError(t, python.Keys(r), "the registrant repeats its registration")
		})

		t.Run("returns an error for a registration of another registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, python.Keys(r.For(string(python.Name))), "the first registration succeeds")
			assert.HasError(t, python.Keys(r.For(rivalPlugin)), "the namespace has one registrant")
		})

		tests := []struct {
			name string
			give meta.KeyName
		}{
			{name: "returns an error for a mark whose spelling a group took", give: python.TestFileKey},
			{
				name: "returns an error for the key of a type variable whose spelling a group took",
				give: python.TypeVarKey,
			},
			{name: "returns an error for a list key whose spelling a group took", give: python.ConditionKey},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := meta.NewRegistry()
				rival := r.For(rivalPlugin)
				assert.NoError(t, rival.ClaimNamespace(rivalPlugin), "the rival claims its own namespace")
				_, err := meta.Register[bool](rival, meta.KeySpec{
					Name: rivalPlugin + ".grouped", Group: meta.GroupName(tt.give), Doc: "a key in a group",
				})
				assert.NoError(t, err, "the key of the rival registers into a group that spells a python key")
				err = python.Keys(r)
				assert.HasError(t, err, "a key and a group share no spelling")
				assert.Contains(t, err.Error(), string(tt.give), "the error names the key")
			})
		}

		t.Run("registers every other key when a group took the spelling of one", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			rival := r.For(rivalPlugin)
			assert.NoError(t, rival.ClaimNamespace(rivalPlugin), "the rival claims its own namespace")
			_, err := meta.Register[bool](rival, meta.KeySpec{
				Name: rivalPlugin + ".grouped", Group: meta.GroupName(python.TestFileKey), Doc: "a key in a group",
			})
			assert.NoError(t, err, "the key of the rival registers into a group that spells a python key")
			assert.HasError(t, python.Keys(r), "the first key fails")
			_, held := meta.Lookup[[]string](r, python.ConditionKey)
			assert.True(t, held, "the last key registers after the first one fails")
		})

		t.Run("registers every key under the python namespace", func(t *testing.T) {
			t.Parallel()

			for _, k := range every() {
				expect.Equal(t, k.name.Namespace(), namespace,
					string(k.name)+" is spelled under the namespace of the satellite")
			}
		})

		t.Run("claims the namespace for the registrant of the handle", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, python.Keys(r.For(string(python.Name))),
				"the keys register through the handle of the plugin")
			_, err := meta.Register[string](r.For(rivalPlugin), meta.KeySpec{
				Name: rivalKey, Doc: "a key under the namespace of another plugin",
			})
			assert.HasError(t, err, "another plugin registers no key under the namespace")
		})

		t.Run("registers each key for the kinds that the frontend stamps it on", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, python.Keys(r), "the keys register")
			for _, k := range every() {
				id, held := r.Resolve(k.name)
				assert.True(t, held, string(k.name)+" resolves")
				spec, held := r.Spec(id)
				assert.True(t, held, string(k.name)+" has a spec")
				expect.Equal(t, spec.Kinds, k.kinds, "the kinds of "+string(k.name))
			}
		})
	})

	t.Run("TypeVar", func(t *testing.T) {
		t.Parallel()

		t.Run("spells TypeVar", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, python.TypeVar, "TypeVar", "the stamp of a variable of TypeVar")
		})
	})

	t.Run("ParamSpec", func(t *testing.T) {
		t.Parallel()

		t.Run("spells ParamSpec", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, python.ParamSpec, "ParamSpec", "the stamp of a variable of ParamSpec")
		})
	})

	t.Run("TypeVarTuple", func(t *testing.T) {
		t.Parallel()

		t.Run("spells TypeVarTuple", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, python.TypeVarTuple, "TypeVarTuple", "the stamp of a variable of TypeVarTuple")
		})
	})
}

// TestKeysAllocs checks the ceiling of a registration in the ordinary run,
// which does not run benchmarks. Each call registers into a registry of its
// own, and the setup builds the registry outside the count. The count keeps
// the first error of its calls, and cmp.Or returns it without allocating.
func TestKeysAllocs(t *testing.T) {
	var err error
	assert.MaxAllocsWithSetup(t, meta.NewRegistry, func(r *meta.Registry) { err = cmp.Or(err, python.Keys(r)) },
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
			err = python.Keys(r)
		}
		assert.NoError(b, err, "Keys registers the vocabulary")
	})
}

// every returns the whole key set of the satellite, which a composition
// registers as one and a corpus fixture declares as one.
func every() []key {
	file := []symbol.Kind{symbol.KindFile}
	callable := []symbol.Kind{symbol.KindFunction, symbol.KindMethod}
	return []key{
		{name: python.TestFileKey, kinds: file},
		{name: python.StubbedKey, kinds: file},
		{name: python.TypedDictKey, kinds: []symbol.Kind{symbol.KindInterface}},
		{name: python.InitVarKey, kinds: []symbol.Kind{symbol.KindField}},
		{name: python.ImplementationKey, kinds: callable},
		{name: python.GeneratorKey, kinds: callable},
		{name: python.TypeVarKey, value: text, kinds: []symbol.Kind{symbol.KindVariable}},
		{name: python.BasesKey, value: list, kinds: []symbol.Kind{symbol.KindEnum}},
		{
			name: python.KeywordsKey, value: list,
			kinds: []symbol.Kind{symbol.KindStruct, symbol.KindInterface, symbol.KindEnum},
		},
		{
			name: python.AnnotatedKey, value: list,
			kinds: []symbol.Kind{
				symbol.KindField, symbol.KindParam, symbol.KindReturn, symbol.KindVariable, symbol.KindConstant,
				symbol.KindAlias,
			},
		},
		{name: python.LazyKey, value: list, kinds: file},
		{name: python.FutureKey, value: list, kinds: file},
		{name: python.HeadKey, value: list, kinds: file},
		{name: python.ConditionKey, value: list, kinds: file},
	}
}
