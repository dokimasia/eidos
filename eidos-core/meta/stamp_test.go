// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"cmp"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// fakeNamespace is the namespace the stamp fixture's keys register
// under.
const fakeNamespace = "fake"

// rawFirstClaimAllocs is a raw stamp's first claim on a new subject:
// the bag, the boxed identity and the sync.Map entry. The value arrives
// boxed. The trie nodes sync.Map adds where two hashes share a prefix
// average below 0.4 per subject, which the mean over a check's calls
// rounds down.
const rawFirstClaimAllocs = 3

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

		t.Run("ranks below an earlier directive drop", func(t *testing.T) {
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

// A raw stamp allocates what the store keeps of a first claim, and
// nothing for an identical re-stamp, in the ordinary run, which runs no
// benchmark. Each counted first claim is on a subject of its own, which
// the setup takes outside the count. The check runs alone, because the
// count includes every goroutine's allocations.
func TestStampAllocs(t *testing.T) {
	_, f, _, _ := fixture(t)
	raw := meta.RawStamp{Key: "shape.role", Value: "writer"}
	assert.NoError(t, f.StampRaw(raw, on(sibling, "shape", 0)),
		"a claim outside the counted subjects creates the store's map and index")
	claims := firstClaims(allocSubjects)

	// Each count keeps the first error of its calls, which cmp.Or
	// returns without allocating.
	var err error
	at := 0
	next := func() meta.Claim {
		at++
		return claims[at-1]
	}
	assert.MaxAllocsWithSetup(t, next, func(claim meta.Claim) { err = cmp.Or(err, f.StampRaw(raw, claim)) },
		rawFirstClaimAllocs, "StampRaw allocates the bag, the boxed identity and the map entry of a first claim")
	assert.NoError(t, err, "every first claim is admitted")

	assert.MaxAllocs(t, func() { err = cmp.Or(err, f.StampRaw(raw, claims[0])) }, 0,
		"StampRaw allocates nothing for an identical re-stamp")
	assert.NoError(t, err, "every re-stamp is accepted")
}

// BenchmarkStamp measures the raw stamps a frontend's classification
// makes: a first claim on each of 200,000 subjects, the store built
// again outside the measurement when the subjects run out, and an
// identical re-stamp.
func BenchmarkStamp(b *testing.B) {
	raw := meta.RawStamp{Key: "shape.role", Value: "writer"}

	b.Run("StampRaw", func(b *testing.B) {
		b.Run("a first claim on a new subject", func(b *testing.B) {
			claims := firstClaims(benchSubjects)
			var (
				f    *meta.Facts
				next int
			)
			build := func() {
				_, f, _, _ = fixture(b)
				assert.NoError(b, f.StampRaw(raw, on(sibling, "shape", 0)),
					"a claim outside the subjects creates the store's map and index")
				next = 0
			}
			build()
			c := bench.Start(b).MaxAllocs(rawFirstClaimAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				if next == len(claims) {
					c.Excluding(build)
				}
				err = f.StampRaw(raw, claims[next])
				next++
			}
			assert.NoError(b, err, "every first claim is admitted")
		})

		b.Run("an identical re-stamp", func(b *testing.B) {
			_, f, _, _ := fixture(b)
			claim := by("shape", 1)
			assert.NoError(b, f.StampRaw(raw, claim), "the first stamp is admitted")
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var err error
			for c.Loop() {
				err = f.StampRaw(raw, claim)
			}
			assert.NoError(b, err, "the re-stamp is accepted")
		})
	})
}

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

// firstClaims returns a claim of the shape plugin on each of n distinct
// struct subjects, at the subject's index.
func firstClaims(n int) []meta.Claim {
	out := make([]meta.Claim, 0, n)
	for i, id := range benchIdentities(n) {
		out = append(out, on(id, "shape", i))
	}
	return out
}
