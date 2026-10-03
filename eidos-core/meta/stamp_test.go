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

// fakeNamespace is the namespace the stamp fixture's keys register
// under.
const fakeNamespace = "fake"

// stampFixture returns a store with one registered bool key and
// its typed handle, so the raw path is checked against the typed
// reads that consume it.
func stampFixture(tb assert.TB) (*meta.Facts, meta.Key[bool]) {
	tb.Helper()

	r := meta.NewRegistry()
	assert.NoError(tb, r.ClaimNamespace(fakeNamespace), "the namespace is claimed")
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

// plugAt returns a plugin-authority claim on the fixture subject at one
// instance of its order.
func plugAt(instance int) meta.Claim {
	return meta.Claim{
		Subject:   subjectFile(),
		Authority: meta.AuthorityPlugin,
		Plugin:    "fakefront",
		Order:     meta.Order{Subject: subjectFile(), Instance: instance},
	}
}

// termKey registers one key of value type T and returns its handle,
// so a case naming several vocabulary terms states each one once.
func termKey[T meta.FactValue](tb assert.TB, r *meta.Registry, name meta.KeyName) meta.Key[T] {
	tb.Helper()

	key, err := meta.Register[T](r, meta.KeySpec{
		Name: name, Doc: "a fixture key for one term of the value vocabulary",
	})
	assert.NoError(tb, err, "the fixture key registers")
	return key
}

// The raw path is the classification stamp's: a pre-claim that
// crossed a phase as data, checked the way a typed write is.
func TestStamp(t *testing.T) {
	t.Parallel()

	t.Run("StampRaw", func(t *testing.T) {
		t.Parallel()

		t.Run("records a value the typed handle reads back", func(t *testing.T) {
			t.Parallel()

			f, key := stampFixture(t)
			raw := meta.RawStamp{Key: "fake.testFile", Value: true}
			assert.NoError(t, f.StampRaw(raw, plugAt(0)), "the raw stamp is recorded")

			got, held := meta.Get(f, subjectFile(), key)
			assert.True(t, held, "the typed read finds the fact")
			assert.True(t, got, "the typed read returns the stamped value")
		})

		t.Run("returns an error naming an unregistered key", func(t *testing.T) {
			t.Parallel()

			f, _ := stampFixture(t)
			err := f.StampRaw(meta.RawStamp{Key: "fake.ghost", Value: true}, plugAt(0))
			assert.HasError(t, err, "the stamp fails")
			assert.Contains(t, err.Error(), "fake.ghost", "the error names the key")
		})

		t.Run("returns an error for a false boolean", func(t *testing.T) {
			t.Parallel()

			f, _ := stampFixture(t)
			err := f.StampRaw(meta.RawStamp{Key: "fake.testFile", Value: false}, plugAt(0))
			assert.HasError(t, err, "the stamp fails, because absence is the negative")
		})

		t.Run("returns an error for a subject kind the key does not admit", func(t *testing.T) {
			t.Parallel()

			f, _ := stampFixture(t)
			wrongKind := plugAt(0)
			wrongKind.Subject.Kind = symbol.KindStruct
			err := f.StampRaw(meta.RawStamp{Key: "fake.testFile", Value: true}, wrongKind)
			assert.HasError(t, err, "the stamp fails")
		})

		t.Run("returns an error naming the type of a value outside the vocabulary", func(t *testing.T) {
			t.Parallel()

			f, _ := stampFixture(t)
			err := f.StampRaw(meta.RawStamp{Key: "fake.testFile", Value: 3.14}, plugAt(0))
			assert.HasError(t, err, "the stamp fails")
			assert.Contains(t, err.Error(), "float64", "the error names the type")
		})

		t.Run("returns an error naming the key's type for a value of another type", func(t *testing.T) {
			t.Parallel()

			f, _ := stampFixture(t)
			err := f.StampRaw(meta.RawStamp{Key: "fake.testFile", Value: "true"}, plugAt(0))
			assert.HasError(t, err, "the stamp fails")
			assert.Contains(t, err.Error(), "bool", "the error names the key's type")
		})

		t.Run("records nothing for a value of another type", func(t *testing.T) {
			t.Parallel()

			f, key := stampFixture(t)
			err := f.StampRaw(meta.RawStamp{Key: "fake.testFile", Value: "true"}, plugAt(0))
			assert.HasError(t, err, "the stamp fails")

			_, held := meta.Get(f, subjectFile(), key)
			assert.False(t, held, "the bag has no fact")
			assert.Empty(t, slices.Collect(f.ByKey(key.ID())), "the index lists no subject")
		})

		t.Run("loses to a directive-authority drop applied before it", func(t *testing.T) {
			t.Parallel()

			f, key := stampFixture(t)
			drop := meta.Claim{Subject: subjectFile(), Authority: meta.AuthorityDirective}
			assert.NoError(t, f.DropKey(key.ID(), drop), "the drop is recorded")
			assert.NoError(t,
				f.StampRaw(meta.RawStamp{Key: "fake.testFile", Value: true}, plugAt(0)),
				"the later raw stamp is recorded")

			_, held := meta.Get(f, subjectFile(), key)
			assert.False(t, held, "the drop outranks the stamp")
		})

		r := meta.NewRegistry()
		assert.NoError(t, r.ClaimNamespace(fakeNamespace), "the namespace is claimed")
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
				name: "records a string", key: text.Name(), value: "writer",
				read: func() (any, bool) { return meta.Get(f, subjectFile(), text) },
			},
			{
				name: "records an integer", key: number.Name(), value: int64(7),
				read: func() (any, bool) { return meta.Get(f, subjectFile(), number) },
			},
			{
				name: "records a boolean", key: flag.Name(), value: true,
				read: func() (any, bool) { return meta.Get(f, subjectFile(), flag) },
			},
			{
				name: "records a string list", key: list.Name(), value: []string{"a", "b"},
				read: func() (any, bool) { return meta.Get(f, subjectFile(), list) },
			},
			{
				name: "records an identity", key: named.Name(), value: subjectFile(),
				read: func() (any, bool) { return meta.Get(f, subjectFile(), named) },
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.NoError(t,
					f.StampRaw(meta.RawStamp{Key: tt.key, Value: tt.value}, plugAt(0)),
					"the term is recorded")
				got, held := tt.read()
				assert.True(t, held, "the typed handle finds the fact")
				assert.Equal(t, got, tt.value, "the typed handle returns the recorded value")
			})
		}
	})
}
