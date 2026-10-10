// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// keySep is the byte that separates a present key's name from its
// subject's identity key, and the identity key's parts, pinned.
const keySep = 0

// The keys of the present rows that a case writes by hand beside the
// flag's: a key that sorts before the flag, and a key that sorts after it.
// A read of the flag leaves their rows alone.
const (
	alphaKey = "shape.alpha"
	otherKey = "shape.other"
)

// flagKey is the key of the flag, whose present rows the cases read.
const flagKey meta.KeyName = "shape.flag"

// runIndexReads is how many reads of a byte range a table's run makes
// before its first block: the footer, then the index.
const runIndexReads = 2

// errSegmentRead is the failure that the failing ledger returns.
var errSegmentRead = errors.New("the segment does not read")

// readFailing is a memory ledger that fails each read of a byte range
// after the first after of them. It is safe for concurrent use.
type readFailing struct {
	*ledger.Mem

	after, reads atomic.Int64
}

// ReadAt adds one to the count of reads, returns errSegmentRead for each
// read after the first after, and reads the memory ledger otherwise.
func (r *readFailing) ReadAt(ctx context.Context, name string, p []byte, off int64) (int, error) {
	if r.reads.Add(1) > r.after.Load() {
		return 0, errSegmentRead
	}
	return r.Mem.ReadAt(ctx, name, p, off)
}

// spanSubjects is how many subjects each key reads present on in the cases
// whose rows span more than one block. A row takes about 40 bytes, so 300
// rows fill about three blocks of 4,096 bytes.
const spanSubjects = 300

// claimsAllocs is one lookup of the recorded subject's seven claims in
// the generation of stampedFacts, after a lookup before it: the list of
// claims, the reads of beta's claim, the list of tags, and the boxes of
// the four values that Go boxes by allocating, which are the role, the
// count -7, the tags and the target. The key is on the stack, the run
// reader keeps the block that it decoded last, and every string is one
// that an earlier decode of the state made.
const claimsAllocs = 7

// A fact store's claims and the facts that read present record whole, so
// a store restored over the record reads, ranks and withdraws them as the
// recording store did.
func TestFacts(t *testing.T) {
	t.Parallel()

	t.Run("PhaseState", func(t *testing.T) {
		t.Parallel()

		t.Run("Claims", func(t *testing.T) {
			t.Parallel()

			t.Run("returns a subject's claims as the store yields them", func(t *testing.T) {
				t.Parallel()

				facts, _ := stampedFacts(t)
				g := recordedPhases(t, ledger.NewMem(), facts, nil, func(*state.Recorder) {})
				got, err := g.Phases(t.Context()).Claims(recordedSubject)
				assert.NoError(t, err, "the claims table reads")
				assert.Equal(t, got, bagOf(facts, recordedSubject), "every claim, each envelope and value whole")
			})

			t.Run("returns none for a subject nothing claimed", func(t *testing.T) {
				t.Parallel()

				facts, _ := stampedFacts(t)
				g := recordedPhases(t, ledger.NewMem(), facts, nil, func(*state.Recorder) {})
				unclaimed := siblingSubject
				unclaimed.Name = "Ledger"
				got, err := g.Phases(t.Context()).Claims(unclaimed)
				assert.NoError(t, err, "the claims table reads")
				assert.Empty(t, got, "a subject nothing stamped has no bag")
			})

			t.Run("restores a fact store that reads the recorded winners", func(t *testing.T) {
				t.Parallel()

				facts, keys := stampedFacts(t)
				g := recordedPhases(t, ledger.NewMem(), facts, nil, func(*state.Recorder) {})
				restored := meta.Restore(facts.Registry(), g.Phases(t.Context()))
				count, held := meta.Get(restored, recordedSubject, keys.count)
				assert.True(t, held, "the count is stamped")
				assert.Equal(t, count, -7, "the count reads its recorded winner")
				_, held = meta.Get(restored, recordedSubject, keys.role)
				assert.False(t, held, "and the group drop still covers the role")
				assert.NoError(t, restored.Damaged(), "nothing is damaged")
			})

			t.Run("returns ErrDamaged for a row that does not decode", func(t *testing.T) {
				t.Parallel()

				row := state.Row{Key: identityKeyOf(recordedSubject), Value: []byte{1, 0x80}}
				_, err := putRows(t, state.TableClaims, row).Phases(t.Context()).Claims(recordedSubject)
				assert.ErrorIs(t, err, state.ErrDamaged, "the claim's key is cut short")
			})
		})

		t.Run("Present", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the subjects on which a key read present in identity order", func(t *testing.T) {
				t.Parallel()

				facts, keys := stampedFacts(t)
				g := recordedPhases(t, ledger.NewMem(), facts, nil, func(*state.Recorder) {})
				got, err := g.Phases(t.Context()).Present(keys.flag.Name())
				assert.NoError(t, err, "the present table reads")
				assert.Equal(t, got, []symbol.Identity{siblingSubject, recordedSubject},
					"Account sorts before User")
			})

			t.Run("returns none for a key that read present nowhere", func(t *testing.T) {
				t.Parallel()

				facts, keys := stampedFacts(t)
				g := recordedPhases(t, ledger.NewMem(), facts, nil, func(*state.Recorder) {})
				got, err := g.Phases(t.Context()).Present(keys.role.Name())
				assert.NoError(t, err, "the present table reads")
				assert.Empty(t, got, "the group drop leaves the role absent everywhere")
			})

			t.Run("reads no row of another key", func(t *testing.T) {
				t.Parallel()

				other := state.Row{Key: append([]byte(otherKey), keySep, 'x'), Value: []byte{}}
				got, err := putRows(t, state.TablePresent, other).Phases(t.Context()).Present("shape.flag")
				assert.NoError(t, err, "the row that does not decode is under another key")
				assert.Empty(t, got, "the flag reads present nowhere")
			})

			t.Run("returns every subject of a key whose rows span more than one block", func(t *testing.T) {
				t.Parallel()

				rows, want := spanRows()
				got, err := putRows(t, state.TablePresent, rows...).Phases(t.Context()).Present(flagKey)
				assert.NoError(t, err, "the present table reads")
				assert.Equal(t, got, want, "every subject of the key, in identity order")
			})

			t.Run("returns ErrDamaged for a failed read of rows that span more than one block", func(t *testing.T) {
				t.Parallel()

				l := &readFailing{Mem: ledger.NewMem()}
				rows, _ := spanRows()
				c := state.NewCommit(nil, nil)
				for _, r := range rows {
					c.Put(state.TablePresent, r.Key, r.Value)
				}
				_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
				assert.NoError(t, err, "the commit writes")
				g, err := state.Open(t.Context(), l)
				assert.NoError(t, err, "the generation opens")
				l.reads.Store(0)
				l.after.Store(runIndexReads)
				_, err = g.Phases(t.Context()).Present(flagKey)
				expect.That(t, err).ErrorIs(state.ErrDamaged, "the failed read is damage").
					ErrorIs(errSegmentRead, "that wraps the ledger's failure")
			})

			t.Run("returns ErrDamaged for a damaged block among a key's rows", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				rows, _ := spanRows()
				c := state.NewCommit(nil, nil)
				for _, r := range rows {
					c.Put(state.TablePresent, r.Key, r.Value)
				}
				_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
				assert.NoError(t, err, "the commit writes")
				damageRun(t, l, func(n int) int { return n / 2 })
				g, err := state.Open(t.Context(), l)
				assert.NoError(t, err, "the generation opens")
				_, err = g.Phases(t.Context()).Present(flagKey)
				assert.ErrorIs(t, err, state.ErrDamaged, "the block in the middle of the run fails its CRC-32C")
			})

			tests := []struct {
				name string
				key  []byte
			}{
				{
					name: "returns ErrDamaged for a key of fewer than four parts",
					key:  append([]byte("shape.flag"), keySep, 'x'),
				},
				{
					name: "returns ErrDamaged for a kind without its separator",
					key: append([]byte("shape.flag\x00lang\x00pkg\x00owner\x00name\x00"),
						byte(symbol.KindStruct), 'x'),
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					g := putRows(t, state.TablePresent, state.Row{Key: tt.key, Value: []byte{}})
					_, err := g.Phases(t.Context()).Present("shape.flag")
					assert.ErrorIs(t, err, state.ErrDamaged, "the key does not spell a subject")
				})
			}

			t.Run("returns ErrDamaged for a table that does not read whole", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				facts, keys := stampedFacts(t)
				g := recordedPhases(t, l, facts, nil, func(*state.Recorder) {})
				damageRun(t, l, func(n int) int { return n - 1 })
				_, err := g.Phases(t.Context()).Present(keys.flag.Name())
				assert.ErrorIs(t, err, state.ErrDamaged, "the damaged footer is found")
			})
		})
	})
}

// A subject's claims allocate within their ceiling, and a key's present
// subjects after the first read allocate nothing, in the ordinary run,
// which runs no benchmark. The first of the 101 calls of each count is
// the read before it. Each count keeps the first error of its calls,
// which cmp.Or returns without allocating. The check runs alone, because
// the count includes every goroutine's allocations.
func TestFactsAllocs(t *testing.T) {
	facts, keys := stampedFacts(t)
	s := recordedPhases(t, ledger.NewMem(), facts, nil, func(*state.Recorder) {}).Phases(t.Context())
	var (
		n   int
		err error
	)
	assert.MaxAllocs(t, func() {
		got, cerr := s.Claims(recordedSubject)
		n, err = len(got), cmp.Or(err, cerr)
	}, claimsAllocs, "Claims allocates the decoded claims")
	assert.NoError(t, err, "the claims read")
	assert.InRange(t, n, 1, math.Inf(1), "the subject has claims")
	key := keys.flag.Name()
	assert.MaxAllocs(t, func() {
		got, perr := s.Present(key)
		n, err = len(got), cmp.Or(err, perr)
	}, 0, "a later Present allocates nothing")
	assert.NoError(t, err, "the present subjects read")
	assert.Equal(t, n, 2, "both stamped subjects")
}

// BenchmarkFacts measures a lookup of the recorded subject's claims and a
// key's present subjects in the generation of stampedFacts, each after
// one lookup.
func BenchmarkFacts(b *testing.B) {
	facts, keys := stampedFacts(b)
	s := recordedPhases(b, ledger.NewMem(), facts, nil, func(*state.Recorder) {}).Phases(b.Context())

	b.Run("PhaseState", func(b *testing.B) {
		b.Run("Claims", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(claimsAllocs)
			defer c.End()
			var (
				got []meta.StoredClaim
				err error
			)
			for c.Loop() {
				got, err = s.Claims(recordedSubject)
			}
			assert.NoError(b, err, "the claims read")
			assert.Equal(b, got, bagOf(facts, recordedSubject), "Claims returns the subject's claims")
		})

		b.Run("Present", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(0)
			defer c.End()
			var (
				got []symbol.Identity
				err error
			)
			for c.Loop() {
				got, err = s.Present(keys.flag.Name())
			}
			assert.NoError(b, err, "the present table reads")
			assert.Length(b, got, 2, "the flag reads present on both subjects")
		})
	})
}

// stampedFacts returns a fact store over the recorded keys with a claim of
// every vocabulary type on the recorded subject, two of them on the count,
// a directive's group drop over the role, and a flag on the sibling, and
// the keys.
func stampedFacts(tb testing.TB) (*meta.Facts, phaseKeys) {
	tb.Helper()

	r, keys := phaseRegistry(tb)
	facts := meta.NewFacts(r)
	derived := claimBy("beta", 2)
	derived.Derived = []meta.Read{{Subject: siblingSubject}, {Subject: siblingSubject, Key: keys.flag.Name()}}
	drop := claimBy("defaults", 1)
	drop.Authority = meta.AuthorityDirective
	drop.Bucket = 3
	assert.NoError(tb, meta.Stamp(facts, keys.role, "reader", claimBy("alpha", 1)), "the role is stamped")
	assert.NoError(tb, meta.Stamp(facts, keys.count, int64(-7), claimBy("alpha", 1)), "the count is stamped")
	assert.NoError(tb, meta.Stamp(facts, keys.count, int64(9), derived), "and again by beta")
	assert.NoError(tb, meta.Stamp(facts, keys.flag, true, claimBy("alpha", 1)), "the flag is stamped")
	assert.NoError(tb, meta.Stamp(facts, keys.tags, []string{"a", "b"}, claimBy("alpha", 1)), "the tags")
	assert.NoError(tb, meta.Stamp(facts, keys.target, siblingSubject, claimBy("alpha", 1)), "the target")
	assert.NoError(tb, facts.DropGroup("shape.writer", drop), "a directive drops the writer group")
	sibling := claimBy("alpha", 1)
	sibling.Subject = siblingSubject
	assert.NoError(tb, meta.Stamp(facts, keys.flag, true, sibling), "the sibling's flag is stamped")
	return facts, keys
}

// bagOf returns the claims a store yields for one subject.
func bagOf(f *meta.Facts, subject symbol.Identity) []meta.StoredClaim {
	var out []meta.StoredClaim
	for id, claims := range f.Bags() {
		if id == subject {
			out = append(out, claims...)
		}
	}
	return out
}

// spanRows returns the present rows of the cases whose rows span more than
// one block, spanSubjects subjects under each of three keys with the flag's
// rows in the middle, and the flag's subjects in identity order.
func spanRows() ([]state.Row, []symbol.Identity) {
	keys := []string{alphaKey, string(flagKey), otherKey}
	rows := make([]state.Row, 0, len(keys)*spanSubjects)
	flagged := make([]symbol.Identity, 0, spanSubjects)
	for _, key := range keys {
		for i := range spanSubjects {
			id := symbol.Identity{
				Lang: recordedSubject.Lang, Package: recordedSubject.Package,
				Name: fmt.Sprintf("Subject%04d", i), Kind: symbol.KindStruct,
			}
			rows = append(rows, state.Row{Key: append(append([]byte(key), keySep), identityKeyOf(id)...)})
			if key == string(flagKey) {
				flagged = append(flagged, id)
			}
		}
	}
	return rows, flagged
}

// identityKeyOf returns an identity's key as the claims table spells it,
// pinned: the language, the package, the owner and the name, each followed
// by the separator, the kind's byte and the separator, then the
// discriminator.
func identityKeyOf(id symbol.Identity) []byte {
	var key []byte
	for _, part := range []string{string(id.Lang), id.Package, id.Owner, id.Name} {
		key = append(append(key, part...), keySep)
	}
	key = append(key, byte(id.Kind), keySep)
	return append(key, id.Disc...)
}
