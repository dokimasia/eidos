// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/history"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// The ceilings of a restored store and of the enumeration of a store's
// bags.
const (
	// restoreAllocs is a store that restores from a recorded source: the
	// store, its key table, and the function that loads a key's recorded
	// presence.
	restoreAllocs = 3
	// bagsAllocs is a range over a store of bags of one claim without a
	// derivation: the slice every bag is yielded in.
	bagsAllocs = 1
)

// bagStore is how many bags the enumeration's ceilings are measured
// over.
const bagStore = 1_000

// errSegment is the failure a damaged recorded source returns.
var errSegment = errors.New("a segment is missing")

// recordedSource is a scripted bag source: the claims and the presence
// a previous run recorded, the failure it returns, and how often it
// read each subject's claims and each key's presence.
type recordedSource struct {
	claims     map[symbol.Identity][]meta.StoredClaim
	present    map[meta.KeyName][]symbol.Identity
	claimsErr  error
	presentErr error
	mu         sync.Mutex
	reads      map[symbol.Identity]int
	asked      map[meta.KeyName]int
}

// Claims counts the read and returns the subject's recorded claims.
func (s *recordedSource) Claims(subject symbol.Identity) ([]meta.StoredClaim, error) {
	s.mu.Lock()
	if s.reads == nil {
		s.reads = map[symbol.Identity]int{}
	}
	s.reads[subject]++
	s.mu.Unlock()
	if s.claimsErr != nil {
		return nil, s.claimsErr
	}
	return s.claims[subject], nil
}

// Present counts the read and returns the key's recorded presence.
func (s *recordedSource) Present(k meta.KeyName) ([]symbol.Identity, error) {
	s.mu.Lock()
	if s.asked == nil {
		s.asked = map[meta.KeyName]int{}
	}
	s.asked[k]++
	s.mu.Unlock()
	if s.presentErr != nil {
		return nil, s.presentErr
	}
	return s.present[k], nil
}

// A restored store reads, ranks and withdraws a previous run's claims
// as if the run had made them, loading each bag and each key's presence
// on first use.
func TestRestore(t *testing.T) {
	t.Parallel()

	t.Run("Restore", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the recorded winner of each key of a restored bag", func(t *testing.T) {
			t.Parallel()

			f, role, flag := restored(t, &recordedSource{
				claims: map[symbol.Identity][]meta.StoredClaim{
					subject: {
						{Key: "shape.role", Claim: by("beta", 1), Value: "reader"},
						{Key: "shape.role", Claim: by("alpha", 1), Value: "writer"},
						{Key: "shape.comparable", Claim: by("alpha", 2), Value: true},
					},
				},
				present: map[meta.KeyName][]symbol.Identity{"shape.role": {subject}, "shape.comparable": {subject}},
			})
			got, held := meta.Get(f, subject, role)
			assert.True(t, held, "the recorded fact reads present")
			assert.Equal(t, got, "writer", "under the claim that ranks first")
			flagged, held := meta.Get(f, subject, flag)
			assert.True(t, held, "and the second key reads present")
			assert.True(t, flagged, "with its recorded value")
			assert.NoError(t, f.Damaged(), "nothing is damaged")
		})

		t.Run("ranks a claim of the run against the restored claims", func(t *testing.T) {
			t.Parallel()

			f, role, _ := restored(t, &recordedSource{claims: map[symbol.Identity][]meta.StoredClaim{
				subject: {{Key: "shape.role", Claim: dropBy("defaults", 1), Drop: true}},
			}})
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "a plugin stamps in the run")
			_, held := meta.Get(f, subject, role)
			assert.False(t, held, "the restored directive drop outranks the new stamp")
		})

		t.Run("restores a group drop that covers a member stamped in the run", func(t *testing.T) {
			t.Parallel()

			f, role, _ := restored(t, &recordedSource{claims: map[symbol.Identity][]meta.StoredClaim{
				subject: {{Group: "shape.writer", Claim: dropBy("defaults", 1), Drop: true}},
			}})
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "a member stamps in the run")
			_, held := meta.Get(f, subject, role)
			assert.False(t, held, "the restored group drop covers it")
		})

		t.Run("withdraws a restored claim", func(t *testing.T) {
			t.Parallel()

			f, role, _ := restored(t, &recordedSource{
				claims: map[symbol.Identity][]meta.StoredClaim{
					subject: {{Key: "shape.role", Claim: by("alpha", 1), Value: "writer"}},
				},
				present: map[meta.KeyName][]symbol.Identity{"shape.role": {subject}},
			})
			assert.NoError(t, f.Withdraw(role.ID(), by("alpha", 1)), "the restored claim is withdrawn")
			_, held := meta.Get(f, subject, role)
			assert.False(t, held, "the fact reads absent")
			assert.Empty(t, slices.Collect(f.ByKey(role.ID())), "and leaves the recorded presence")
		})

		t.Run("lists the recorded presence without restoring a bag", func(t *testing.T) {
			t.Parallel()

			src := &recordedSource{present: map[meta.KeyName][]symbol.Identity{"shape.role": {subject}}}
			f, role, _ := restored(t, src)
			assert.Equal(t, slices.Collect(f.ByKey(role.ID())), []symbol.Identity{subject},
				"the recorded subject has the key")
			assert.Empty(t, src.reads, "and no bag is read to list it")
		})

		t.Run("merges a transition recorded before the presence loaded", func(t *testing.T) {
			t.Parallel()

			f, role, _ := restored(t, &recordedSource{
				present: map[meta.KeyName][]symbol.Identity{"shape.role": {subject}},
			})
			assert.NoError(t, meta.Stamp(f, role, "writer", on(sibling, "alpha", 1)),
				"another subject stamps in the run")
			assert.Equal(t, slices.Collect(f.ByKey(role.ID())), []symbol.Identity{sibling, subject},
				"the recorded subject and the new one, in identity order")
		})

		t.Run("asks the source nothing for a key nothing registered", func(t *testing.T) {
			t.Parallel()

			src := &recordedSource{}
			f, _, _ := restored(t, src)
			assert.Empty(t, slices.Collect(f.ByKey(meta.KeyID(999))), "the key lists nothing")
			assert.Empty(t, src.asked, "and the source is not asked for its presence")
		})

		t.Run("reads a fact absent without restoring a bag that the presence does not list", func(t *testing.T) {
			t.Parallel()

			src := &recordedSource{claims: map[symbol.Identity][]meta.StoredClaim{
				subject: {{Key: "shape.role", Claim: dropBy("defaults", 1), Drop: true}},
			}}
			f, role, _ := restored(t, src)
			_, held := meta.Get(f, subject, role)
			assert.False(t, held, "the dropped fact reads absent")
			assert.Empty(t, src.reads, "and no bag is read to read it")
		})

		t.Run("restores each bag once under concurrent readers", func(t *testing.T) {
			t.Parallel()

			src := &recordedSource{
				claims: map[symbol.Identity][]meta.StoredClaim{
					subject: {{Key: "shape.role", Claim: by("alpha", 1), Value: "writer"}},
				},
				present: map[meta.KeyName][]symbol.Identity{"shape.role": {subject}},
			}
			f, role, _ := restored(t, src)
			outcomes := history.Concurrently(8, time.Minute, func(int) (any, error) {
				_, held := meta.Get(f, subject, role)
				return held, nil
			})
			for _, o := range outcomes {
				expect.True(t, o.Finished, "every reader finishes")
				expect.Equal(t, o.Output, any(true), "every reader finds the restored fact")
			}
			assert.Equal(t, src.reads[subject], 1, "the source is read once")
		})

		t.Run("reports Damaged for claims that do not read", func(t *testing.T) {
			t.Parallel()

			f, role, _ := restored(t, &recordedSource{
				claimsErr: errSegment,
				present:   map[meta.KeyName][]symbol.Identity{"shape.role": {subject}},
			})
			_, held := meta.Get(f, subject, role)
			assert.False(t, held, "the bag reads empty")
			assert.ErrorIs(t, f.Damaged(), errSegment, "and the failure is kept")
		})

		t.Run("reports Damaged for a presence that does not read", func(t *testing.T) {
			t.Parallel()

			f, role, _ := restored(t, &recordedSource{presentErr: errSegment})
			assert.Empty(t, slices.Collect(f.ByKey(role.ID())), "the key lists nothing")
			assert.ErrorIs(t, f.Damaged(), errSegment, "and the failure is kept")
		})

		t.Run("reports Damaged for a claim under a key nothing registered", func(t *testing.T) {
			t.Parallel()

			f, role, _ := restored(t, &recordedSource{
				claims: map[symbol.Identity][]meta.StoredClaim{
					subject: {
						{Key: "shape.gone", Claim: by("alpha", 1), Value: "writer"},
						{Key: "shape.role", Claim: by("alpha", 2), Value: "reader"},
					},
				},
				present: map[meta.KeyName][]symbol.Identity{"shape.role": {subject}},
			})
			got, held := meta.Get(f, subject, role)
			assert.True(t, held, "the claims that restore remain")
			assert.Equal(t, got, "reader", "with their value")
			assert.HasError(t, f.Damaged(), "the unknown key damages the store")
		})

		t.Run("reports Damaged for a value of another type than its key's", func(t *testing.T) {
			t.Parallel()

			f, _, flag := restored(t, &recordedSource{
				claims: map[symbol.Identity][]meta.StoredClaim{
					subject: {{Key: "shape.comparable", Claim: by("alpha", 1), Value: "true"}},
				},
				present: map[meta.KeyName][]symbol.Identity{"shape.comparable": {subject}},
			})
			_, held := meta.Get(f, subject, flag)
			assert.False(t, held, "the claim does not restore")
			assert.HasError(t, f.Damaged(), "and the store is damaged")
		})
	})

	t.Run("Bags", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each key's claims in key order and in rank order", func(t *testing.T) {
			t.Parallel()

			_, f, role, flag := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "reader", by("beta", 1)), "beta stamps the role")
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "alpha stamps the role")
			assert.NoError(t, meta.Stamp(f, flag, true, by("alpha", 2)), "alpha stamps the flag")
			assert.Equal(t, bagsOf(f), map[symbol.Identity][]meta.StoredClaim{
				subject: {
					{Key: "shape.comparable", Claim: by("alpha", 2), Value: true},
					{Key: "shape.role", Claim: by("alpha", 1), Value: "writer"},
					{Key: "shape.role", Claim: by("beta", 1), Value: "reader"},
				},
			}, "shape.comparable sorts first, and alpha outranks beta")
		})

		t.Run("returns a group's drops after the keys", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, f.DropGroup("shape.writer", dropBy("defaults", 1)), "the group drop arrives")
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "a member stamps")
			assert.Equal(t, bagsOf(f), map[symbol.Identity][]meta.StoredClaim{
				subject: {
					{Key: "shape.role", Claim: by("alpha", 1), Value: "writer"},
					{Group: "shape.writer", Claim: dropBy("defaults", 1), Drop: true},
				},
			}, "the group drop follows the key's claim")
		})

		t.Run("returns a key's drop without a value", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, f.DropKey(role.ID(), dropBy("defaults", 1)), "the drop arrives")
			assert.Equal(t, bagsOf(f), map[symbol.Identity][]meta.StoredClaim{
				subject: {{Key: "shape.role", Claim: dropBy("defaults", 1), Drop: true}},
			}, "the drop has no value")
		})

		t.Run("returns a bag for each subject", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "the subject is stamped")
			assert.NoError(t, meta.Stamp(f, role, "reader", on(sibling, "alpha", 1)), "and so is the sibling")
			assert.Equal(t, bagsOf(f), map[symbol.Identity][]meta.StoredClaim{
				subject: {{Key: "shape.role", Claim: by("alpha", 1), Value: "writer"}},
				sibling: {{Key: "shape.role", Claim: on(sibling, "alpha", 1), Value: "reader"}},
			}, "each subject's bag arrives once")
		})

		t.Run("returns a bag of more keys than fit in place", func(t *testing.T) {
			t.Parallel()

			r, _, _, _ := fixture(t)
			names := []meta.KeyName{"shape.e", "shape.d", "shape.c", "shape.b", "shape.a"}
			flags := make([]meta.Key[bool], 0, len(names))
			for _, name := range names {
				k, err := meta.Register[bool](r, meta.KeySpec{Name: name, Doc: "one of five flags"})
				assert.NoError(t, err, "the flag registers")
				flags = append(flags, k)
			}
			f := meta.NewFacts(r)
			for _, k := range flags {
				assert.NoError(t, meta.Stamp(f, k, true, by("alpha", 1)), "each flag is stamped")
			}
			claims := bagsOf(f)[subject]
			got := make([]meta.KeyName, 0, len(claims))
			for _, c := range claims {
				got = append(got, c.Key)
			}
			assert.Equal(t, got, []meta.KeyName{"shape.a", "shape.b", "shape.c", "shape.d", "shape.e"},
				"the five keys arrive in name order")
		})

		t.Run("leaves out a bag whose every claim was withdrawn", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "the subject is stamped")
			assert.NoError(t, f.Withdraw(role.ID(), by("alpha", 1)), "and the claim is withdrawn")
			assert.Empty(t, bagsOf(f), "the empty bag is not yielded")
		})

		t.Run("returns copies of a list value and of a derivation", func(t *testing.T) {
			t.Parallel()

			r, _, _, _ := fixture(t)
			tags, err := meta.Register[[]string](r, meta.KeySpec{Name: "shape.tags", Doc: "the subject's tags"})
			assert.NoError(t, err, "the list key registers")
			f := meta.NewFacts(r)
			claim := by("alpha", 1)
			claim.Derived = []meta.Read{{Subject: sibling}}
			assert.NoError(t, meta.Stamp(f, tags, []string{"a"}, claim), "the list is stamped")
			for _, bag := range f.Bags() {
				bag[0].Value.([]string)[0] = "b"
				bag[0].Claim.Derived[0].Key = "shape.role"
			}
			got, _ := meta.Get(f, subject, tags)
			assert.Equal(t, got, []string{"a"}, "the store keeps its value")
			views := slices.Collect(f.Claims(subject, tags.ID()))
			assert.Equal(t, views[0].Claim.Derived, []meta.Read{{Subject: sibling}},
				"and its derivation")
		})

		t.Run("stops when the range's body breaks", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "the subject is stamped")
			assert.NoError(t, meta.Stamp(f, role, "reader", on(sibling, "alpha", 1)), "and so is the sibling")
			seen := 0
			for range f.Bags() {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the range ends at the first bag")
		})

		t.Run("returns claims a source restores into an equal store", func(t *testing.T) {
			t.Parallel()

			r, f, role, flag := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "alpha stamps the role")
			assert.NoError(t, f.DropKey(flag.ID(), dropBy("defaults", 2)), "a directive drops the flag")
			assert.NoError(t, f.DropGroup("shape.writer", dropBy("defaults", 3)), "and the group")
			src := &recordedSource{claims: bagsOf(f), present: map[meta.KeyName][]symbol.Identity{}}
			assert.Empty(t, bagsOf(meta.Restore(r, src)), "a restored store yields no bag before a touch")
			again := meta.Restore(r, src)
			assert.NotEmpty(t, slices.Collect(again.Claims(subject, role.ID())), "the role's claims restore the bag")
			assert.Equal(t, bagsOf(again), bagsOf(f), "a touched bag yields the claims it restored")
		})
	})
}

// A restored store is built without allocating more than an empty one
// and its presence loader, and a read of a fact that the recorded
// presence leaves out allocates nothing, in the ordinary run, which runs
// no benchmark. The check runs alone, because the count includes every
// goroutine's allocations.
func TestRestoreAllocs(t *testing.T) {
	r, _, role, _ := fixture(t)
	src := &recordedSource{}
	var f *meta.Facts
	assert.MaxAllocs(t, func() { f = meta.Restore(r, src) }, restoreAllocs,
		"Restore allocates the store, its key table and its presence loader")
	assert.NoError(t, f.Damaged(), "Restore returns a store that is not damaged")
	held := true
	assert.MaxAllocs(t, func() { _, held = meta.Get(f, subject, role) }, 0,
		"Get allocates nothing for a fact that the recorded presence leaves out")
	assert.False(t, held, "the fact reads absent")
	assert.Empty(t, src.reads, "and no bag is read to read it")

	stamped := storeOfBags(t, bagStore)
	n := 0
	assert.MaxAllocs(t, func() {
		n = 0
		for range stamped.Bags() {
			n++
		}
	}, bagsAllocs, "a range over Bags allocates the slice every bag is yielded in")
	assert.Equal(t, n, bagStore, "the range yields every bag")
}

// BenchmarkRestore measures the store a warm run builds over the
// previous run's sealed state, once per run.
func BenchmarkRestore(b *testing.B) {
	b.Run("Restore", func(b *testing.B) {
		r, _, _, _ := fixture(b)
		src := &recordedSource{}
		c := bench.Start(b).MaxAllocs(restoreAllocs)
		defer c.End()
		var f *meta.Facts
		for c.Loop() {
			f = meta.Restore(r, src)
		}
		assert.Equal(b, f.Registry(), r, "Restore returns a store over the registry", assert.ByIdentity())
	})

	b.Run("Bags", func(b *testing.B) {
		b.Run("a store of a thousand bags of one claim", func(b *testing.B) {
			f := storeOfBags(b, bagStore)
			c := bench.Start(b).MaxAllocs(bagsAllocs)
			defer c.End()
			n := 0
			for c.Loop() {
				n = 0
				for range f.Bags() {
					n++
				}
			}
			assert.Equal(b, n, bagStore, "the range yields every bag")
		})
	})
}

// bagsOf collects a store's bags by subject.
func bagsOf(f *meta.Facts) map[symbol.Identity][]meta.StoredClaim {
	out := map[symbol.Identity][]meta.StoredClaim{}
	for id, claims := range f.Bags() {
		out[id] = slices.Clone(claims)
	}
	return out
}

// storeOfBags returns a store of n bags, each of one claim of the role
// key without a derivation.
func storeOfBags(tb assert.TB, n int) *meta.Facts {
	tb.Helper()

	_, f, role, _ := fixture(tb)
	for _, id := range benchIdentities(n) {
		assert.NoError(tb, meta.Stamp(f, role, "writer", on(id, "alpha", 1)), "the subject is stamped")
	}
	return f
}

// restored returns a store restoring from src over the fixture's
// registry, and the fixture's keys.
func restored(t *testing.T, src meta.BagSource) (*meta.Facts, meta.Key[string], meta.Key[bool]) {
	t.Helper()

	r, _, role, flag := fixture(t)
	return meta.Restore(r, src), role, flag
}
