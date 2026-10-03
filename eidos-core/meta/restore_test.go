// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// errSegment is the failure a damaged recorded source returns.
var errSegment = errors.New("a segment is missing")

// recordedSource is a scripted bag source: the claims and the presence
// a previous run recorded, the failure it returns, and how often it
// read each subject's claims.
type recordedSource struct {
	claims     map[symbol.Identity][]meta.StoredClaim
	present    map[meta.KeyName][]symbol.Identity
	claimsErr  error
	presentErr error
	mu         sync.Mutex
	reads      map[symbol.Identity]int
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

// Present returns the key's recorded presence.
func (s *recordedSource) Present(k meta.KeyName) ([]symbol.Identity, error) {
	if s.presentErr != nil {
		return nil, s.presentErr
	}
	return s.present[k], nil
}

// restored returns a store restoring from src over the fixture's
// registry, and the fixture's keys.
func restored(t *testing.T, src meta.BagSource) (*meta.Facts, meta.Key[string], meta.Key[bool]) {
	t.Helper()

	r, _, role, flag := fixture(t)
	return meta.Restore(r, src), role, flag
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

			drop := by("defaults", 1)
			drop.Authority = meta.AuthorityDirective
			f, role, _ := restored(t, &recordedSource{claims: map[symbol.Identity][]meta.StoredClaim{
				subject: {{Key: "shape.role", Claim: drop, Drop: true}},
			}})
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "a plugin stamps in the run")
			_, held := meta.Get(f, subject, role)
			assert.False(t, held, "the restored directive drop outranks the new stamp")
		})

		t.Run("restores a group drop that covers a member stamped in the run", func(t *testing.T) {
			t.Parallel()

			drop := by("defaults", 1)
			drop.Authority = meta.AuthorityDirective
			f, role, _ := restored(t, &recordedSource{claims: map[symbol.Identity][]meta.StoredClaim{
				subject: {{Group: "shape.writer", Claim: drop, Drop: true}},
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
				"the recorded subject carries the key")
			assert.Empty(t, src.reads, "and no bag is read to list it")
		})

		t.Run("merges a transition recorded before the presence loaded", func(t *testing.T) {
			t.Parallel()

			other := subject
			other.Name = "Cache"
			onOther := by("alpha", 1)
			onOther.Subject = other
			f, role, _ := restored(t, &recordedSource{
				present: map[meta.KeyName][]symbol.Identity{"shape.role": {subject}},
			})
			assert.NoError(t, meta.Stamp(f, role, "writer", onOther), "another subject stamps in the run")
			assert.Equal(t, slices.Collect(f.ByKey(role.ID())), []symbol.Identity{other, subject},
				"the recorded subject and the new one, in identity order")
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
