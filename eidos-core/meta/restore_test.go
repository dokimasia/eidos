// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// restoreAllocs is a store that restores from a recorded source: the
// store, its key table, and the function that loads a key's recorded
// presence.
const restoreAllocs = 3

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

			f, role, flag := restored(t, &recordedSource{claims: map[symbol.Identity][]meta.StoredClaim{
				subject: {
					{Key: "shape.role", Claim: by("beta", 1), Value: "reader"},
					{Key: "shape.role", Claim: by("alpha", 1), Value: "writer"},
					{Key: "shape.comparable", Claim: by("alpha", 2), Value: true},
				},
			}})
			got, held := meta.Get(f, subject, role)
			assert.True(t, held, "the recorded fact reads present")
			assert.Equal(t, got, "writer", "under the claim that ranks first")
			flagged, held := meta.Get(f, subject, flag)
			assert.True(t, held && flagged, "and the second key reads present")
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

		t.Run("restores each bag once under concurrent readers", func(t *testing.T) {
			t.Parallel()

			src := &recordedSource{claims: map[symbol.Identity][]meta.StoredClaim{
				subject: {{Key: "shape.role", Claim: by("alpha", 1), Value: "writer"}},
			}}
			f, role, _ := restored(t, src)
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					_, held := meta.Get(f, subject, role)
					assert.True(t, held, "every reader finds the restored fact")
				})
			}
			wg.Wait()
			assert.Equal(t, src.reads[subject], 1, "the source is read once")
		})

		t.Run("reports Damaged for claims that do not read", func(t *testing.T) {
			t.Parallel()

			f, role, _ := restored(t, &recordedSource{claimsErr: errSegment})
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

			f, role, _ := restored(t, &recordedSource{claims: map[symbol.Identity][]meta.StoredClaim{
				subject: {
					{Key: "shape.gone", Claim: by("alpha", 1), Value: "writer"},
					{Key: "shape.role", Claim: by("alpha", 2), Value: "reader"},
				},
			}})
			got, held := meta.Get(f, subject, role)
			assert.True(t, held, "the claims that restore remain")
			assert.Equal(t, got, "reader", "with their value")
			assert.HasError(t, f.Damaged(), "the unknown key damages the store")
		})

		t.Run("reports Damaged for a value of another type than its key's", func(t *testing.T) {
			t.Parallel()

			f, _, flag := restored(t, &recordedSource{claims: map[symbol.Identity][]meta.StoredClaim{
				subject: {{Key: "shape.comparable", Claim: by("alpha", 1), Value: "true"}},
			}})
			_, held := meta.Get(f, subject, flag)
			assert.False(t, held, "the claim does not restore")
			assert.HasError(t, f.Damaged(), "and the store is damaged")
		})
	})
}

// A restored store is built without allocating more than an empty one
// and its presence loader in the ordinary run, which runs no benchmark.
// The check runs alone, because AllocsPerRun counts every goroutine's
// allocations and refuses to run beside parallel tests.
func TestRestoreAllocs(t *testing.T) {
	r, _, _, _ := fixture(t)
	src := &recordedSource{}
	var f *meta.Facts
	assert.MaxAllocs(t, func() { f = meta.Restore(r, src) }, restoreAllocs,
		"Restore allocates the store, its key table and its presence loader")
	assert.NoError(t, f.Damaged(), "Restore returns a store that is not damaged")
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
		assert.True(b, f.Registry() == r, "Restore returns a store over the registry")
	})
}

// restored returns a store restoring from src over the fixture's
// registry, and the fixture's keys.
func restored(t *testing.T, src meta.BagSource) (*meta.Facts, meta.Key[string], meta.Key[bool]) {
	t.Helper()

	r, _, role, flag := fixture(t)
	return meta.Restore(r, src), role, flag
}
