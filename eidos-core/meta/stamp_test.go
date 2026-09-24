// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// stampFixture returns a store with one registered bool key and
// its typed handle, so the raw path is checked against the typed
// reads that consume it.
func stampFixture(tb assert.TB) (*meta.Facts, meta.Key[bool]) {
	tb.Helper()

	r := meta.NewRegistry()
	assert.NoError(tb, r.ClaimNamespace("fake", "fake"), "the namespace claims")
	key, err := meta.Register[bool](r, meta.KeySpec{
		Name:  "fake.testFile",
		Kinds: []symbol.Kind{symbol.KindFile},
		Doc:   "marks a file the language's own convention calls a test",
	})
	assert.NoError(tb, err, "the key registers")
	return meta.NewFacts(r), key
}

// subjectFile is the fixture's one stamped subject.
func subjectFile() symbol.Identity {
	return symbol.Identity{
		Lang: "fake", Package: "svc", Name: "svc/a_test.zz", Kind: symbol.KindFile,
	}
}

// plugAt returns a plugin-authority claim on the fixture subject.
func plugAt(seq int) meta.Claim {
	return meta.Claim{
		Subject:   subjectFile(),
		Authority: meta.AuthorityPlugin,
		Plugin:    "fakefront",
		Seq:       seq,
	}
}

// termKey registers one key of value type T and returns its handle,
// so a case naming several vocabulary terms states each one once.
func termKey[T meta.FactValue](tb assert.TB, r *meta.Registry, name meta.KeyName) meta.Key[T] {
	tb.Helper()

	key, err := meta.Register[T](r, meta.KeySpec{
		Name: name, Doc: "a fixture key holding one term of the value vocabulary",
	})
	assert.NoError(tb, err, "the fixture key registers")
	return key
}

// The raw path is the classification stamp's: a pre-claim that
// crossed a phase as data, held to the same checks as a typed
// write.
func TestStampRaw(t *testing.T) {
	t.Parallel()

	t.Run("resolves the name and typed readers see the value", func(t *testing.T) {
		t.Parallel()

		f, key := stampFixture(t)
		raw := meta.RawStamp{Key: "fake.testFile", Value: true}
		assert.NoError(t, f.StampRaw(raw, plugAt(0)), "the raw write admits")

		got, held := meta.Get(f, subjectFile(), key)
		assert.True(t, held && got, "the typed read returns the raw claim's value")
	})

	t.Run("refuses what the typed path refuses", func(t *testing.T) {
		t.Parallel()

		f, _ := stampFixture(t)
		err := f.StampRaw(meta.RawStamp{Key: "fake.ghost", Value: true}, plugAt(0))
		assert.HasError(t, err, "an unregistered key refuses")
		assert.Contains(t, err.Error(), "fake.ghost", "naming the key")

		err = f.StampRaw(meta.RawStamp{Key: "fake.testFile", Value: false}, plugAt(0))
		assert.HasError(t, err, "a false boolean refuses; absence is the negative")

		wrongKind := plugAt(0)
		wrongKind.Subject.Kind = symbol.KindStruct
		err = f.StampRaw(meta.RawStamp{Key: "fake.testFile", Value: true}, wrongKind)
		assert.HasError(t, err, "the kind restriction holds on the raw path")

		err = f.StampRaw(meta.RawStamp{Key: "fake.testFile", Value: 3.14}, plugAt(0))
		assert.HasError(t, err, "a value outside the vocabulary refuses")
		assert.Contains(t, err.Error(), "float64", "naming the type")
	})

	t.Run("carries every term of the value vocabulary", func(t *testing.T) {
		t.Parallel()

		r := meta.NewRegistry()
		assert.NoError(t, r.ClaimNamespace("fake", "fake"), "the namespace claims")
		text := termKey[string](t, r, "fake.text")
		number := termKey[int64](t, r, "fake.number")
		flag := termKey[bool](t, r, "fake.flag")
		list := termKey[[]string](t, r, "fake.list")
		named := termKey[symbol.Identity](t, r, "fake.named")
		f := meta.NewFacts(r)

		tests := []struct {
			name  string
			key   meta.KeyName
			value any
			read  func() (any, bool)
		}{
			{
				name: "a string", key: text.Name(), value: "writer",
				read: func() (any, bool) { return meta.Get(f, subjectFile(), text) },
			},
			{
				name: "an integer", key: number.Name(), value: int64(7),
				read: func() (any, bool) { return meta.Get(f, subjectFile(), number) },
			},
			{
				name: "a boolean", key: flag.Name(), value: true,
				read: func() (any, bool) { return meta.Get(f, subjectFile(), flag) },
			},
			{
				name: "a string list", key: list.Name(), value: []string{"a", "b"},
				read: func() (any, bool) { return meta.Get(f, subjectFile(), list) },
			},
			{
				name: "an identity", key: named.Name(), value: subjectFile(),
				read: func() (any, bool) { return meta.Get(f, subjectFile(), named) },
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.NoError(t,
					f.StampRaw(meta.RawStamp{Key: tt.key, Value: tt.value}, plugAt(0)),
					"a term of the vocabulary crosses the phase as data")
				got, held := tt.read()
				assert.True(t, held, "and the typed handle reads it back")
				assert.Equal(t, got, tt.value, "carrying the value the frontend recorded")
			})
		}
	})

	t.Run("refuses a value whose type differs from the key's", func(t *testing.T) {
		t.Parallel()

		f, key := stampFixture(t)
		err := f.StampRaw(meta.RawStamp{Key: "fake.testFile", Value: "true"}, plugAt(0))
		assert.HasError(t, err, "a string under a boolean key refuses")
		assert.Contains(t, err.Error(), "bool", "naming the key's type")

		_, held := meta.Get(f, subjectFile(), key)
		assert.False(t, held, "the bag is unchanged")
		assert.Empty(t, slices.Collect(f.ByKey(key.ID())),
			"and the index lists no subject")
	})

	t.Run("ranks like any other claim", func(t *testing.T) {
		t.Parallel()

		f, key := stampFixture(t)
		drop := meta.Claim{Subject: subjectFile(), Authority: meta.AuthorityDirective}
		assert.NoError(t, f.DropKey(key.ID(), drop), "a directive-authority drop admits")
		assert.NoError(t,
			f.StampRaw(meta.RawStamp{Key: "fake.testFile", Value: true}, plugAt(0)),
			"the later raw stamp admits")

		_, held := meta.Get(f, subjectFile(), key)
		assert.False(t, held, "the drop outranks the stamp whichever applied first")
	})
}
