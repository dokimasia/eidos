// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
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
