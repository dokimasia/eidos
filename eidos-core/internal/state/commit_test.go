// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"context"
	"crypto/sha256"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
)

// The ledger directories of the sealed state the cases inspect.
const (
	genPrefix = "state/gen/"
	segPrefix = "state/seg/"
)

// allocRuns is the number of calls [assert.MaxAllocs] makes: one to
// warm the function, and the 100 it counts.
const allocRuns = 101

// The ceilings of a commit's steps.
const (
	// newCommitAllocs is one commit of a cold run: the commit and its map
	// of live regions.
	newCommitAllocs = 2
	// newCommitParentAllocs is one commit over a parent of one live
	// segment: the cold commit's, and the first group of the map that
	// copies the parent's live regions.
	newCommitParentAllocs = newCommitAllocs + 1
	// addRegionsAllocs is the region segment of two blobs in a new commit:
	// the list of places, the segment's bytes, 3 for its name, which are
	// the hex digest's buffer, its string and the joined name, and the
	// first group of the live map.
	addRegionsAllocs = 6
	// putAllocs is a first row in a new commit: the copies of the key and
	// the row, the table's map, the map's first group and the key's
	// string.
	putAllocs = 5
	// deleteAllocs is a first tombstone in a new commit: the copy of the
	// key, the table's map, its first group and the key's string.
	deleteAllocs = 4
	// writeAllocs is the write of a commit of one row into an empty
	// ledger: the run segment and its name, the generation's encoding and
	// its name, CURRENT, the manifest's digests, and the listing the
	// collection walks.
	writeAllocs = 41
)

// refusing is a memory ledger that refuses every write of a name under
// one prefix.
type refusing struct {
	*ledger.Mem
	prefix string
}

// Write refuses a name under the prefix.
func (r refusing) Write(ctx context.Context, name string, b []byte) error {
	if strings.HasPrefix(name, r.prefix) {
		return errDevice
	}
	return r.Mem.Write(ctx, name, b)
}

// header returns a header anchored at an instant, with the composition
// the cases share.
func header(anchor time.Time) state.Header {
	return state.Header{Composition: sha256.Sum256([]byte("composition")), Anchor: anchor}
}

// past is an anchor before every blob a memory ledger writes, whose
// clock counts from the epoch: nothing is old enough to collect.
var past = time.Unix(0, 0)

// committed writes one commit of puts to the checks table, over a parent
// opened from the ledger where the ledger has one, and returns the live
// generation.
func committed(tb testing.TB, l ledger.Ledger, anchor time.Time, puts ...string) *state.Generation {
	tb.Helper()

	parent, err := state.Open(tb.Context(), l)
	if err != nil {
		parent = nil
	}
	c := state.NewCommit(parent, nil)
	for _, key := range puts {
		c.Put(state.TableChecks, []byte(key), []byte("row of "+key))
	}
	_, err = c.Write(tb.Context(), l, header(anchor), manifest.Manifest{Version: manifest.Version})
	assert.NoError(tb, err, "the commit writes")
	g, err := state.Open(tb.Context(), l)
	assert.NoError(tb, err, "and its generation opens")
	return g
}

// blobsUnder returns the names of the ledger's blobs under a prefix.
func blobsUnder(t *testing.T, l ledger.Ledger, prefix string) []string {
	t.Helper()

	blobs, err := l.List(t.Context(), "state")
	assert.NoError(t, err, "the state lists")
	var out []string
	for _, b := range blobs {
		if strings.HasPrefix(b.Name, prefix) {
			out = append(out, b.Name)
		}
	}
	return out
}

// A commit is the one write path of the sealed state, so what it makes
// live, what it leaves, and what it removes are each pinned.
func TestCommit(t *testing.T) {
	t.Parallel()

	t.Run("AddRegions", func(t *testing.T) {
		t.Parallel()

		t.Run("returns where each blob is in one segment", func(t *testing.T) {
			t.Parallel()

			c := state.NewCommit(nil, nil)
			refs, err := c.AddRegions([][]byte{[]byte("ab"), []byte("cde")})
			assert.NoError(t, err, "the regions add")
			assert.Equal(t, refs[0].Segment, refs[1].Segment, "both blobs are in one segment")
			assert.Equal(t, []int64{refs[0].Offset, refs[0].Length, refs[1].Offset, refs[1].Length},
				[]int64{0, 2, 2, 3}, "each at its place")
			assert.HasPrefix(t, refs[0].Segment, segPrefix, "under the segment directory")
		})

		t.Run("returns nothing for no blob", func(t *testing.T) {
			t.Parallel()

			refs, err := state.NewCommit(nil, nil).AddRegions(nil)
			assert.NoError(t, err, "nothing adds")
			assert.Empty(t, refs, "and nothing is placed")
		})

		t.Run("returns an error for a second segment", func(t *testing.T) {
			t.Parallel()

			c := state.NewCommit(nil, nil)
			_, err := c.AddRegions([][]byte{[]byte("ab")})
			assert.NoError(t, err, "the first segment adds")
			_, err = c.AddRegions([][]byte{[]byte("cd")})
			assert.HasError(t, err, "a commit writes one region segment")
		})
	})

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("makes the generation live", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			g := committed(t, l, past, "alpha")
			assert.Equal(t, g.Header.Composition, header(past).Composition, "the header is the commit's")
			assert.Equal(t, g.Header.Sequence, uint64(1), "a cold run's generation is the first")
			assert.Equal(t, g.Header.Parent, "", "and has no parent")
			assert.Equal(t, g.Header.Format, uint64(state.Format), "in the kernel's format")
		})

		t.Run("numbers a generation after its parent", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			first := committed(t, l, past, "alpha")
			second := committed(t, l, past, "beta")
			assert.Equal(t, second.Header.Sequence, uint64(2), "the child counts one more")
			assert.Equal(t, second.Header.Parent, first.Name, "and names its parent")
		})

		t.Run("writes nothing for a commit that changes nothing", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			first := committed(t, l, past, "alpha")
			writes := l.Writes()
			result, err := state.NewCommit(first, nil).Write(t.Context(), l, header(past),
				manifest.Manifest{Version: manifest.Version})
			assert.NoError(t, err, "the empty commit succeeds")
			assert.Equal(t, l.Writes(), writes, "without a write")
			assert.Equal(t, result, state.Result{Generation: first.Name}, "and the parent stays live")
		})

		t.Run("writes the manifest's documents", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			_, err := state.NewCommit(nil, nil).Write(t.Context(), l, header(past), scaled())
			assert.NoError(t, err, "the commit writes")
			got, _, err := state.ReadManifest(t.Context(), l)
			assert.NoError(t, err, "the documents read")
			assert.True(t, got.Equal(scaled()), "and join to the manifest the commit recorded")
		})

		t.Run("records the digest of each document in the generation", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			_, err := state.NewCommit(nil, nil).Write(t.Context(), l, header(past), scaled())
			assert.NoError(t, err, "the commit writes")
			g, err := state.Open(t.Context(), l)
			assert.NoError(t, err, "the generation opens")
			_, digests, err := state.ReadManifest(t.Context(), l)
			assert.NoError(t, err, "the documents read")
			assert.Equal(t, g.Manifest, digests, "the generation records the documents' digests")
		})

		t.Run("writes a document the ledger changed after the parent's commit", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			_, err := state.NewCommit(nil, nil).Write(t.Context(), l, header(past), scaled())
			assert.NoError(t, err, "the first commit writes")
			parent, err := state.Open(t.Context(), l)
			assert.NoError(t, err, "and opens")
			empty := manifest.Manifest{Version: manifest.Version}
			emptied, err := state.WriteManifest(t.Context(), l, empty, parent.Manifest)
			assert.NoError(t, err, "a run without a generation empties the record")
			_, err = state.NewCommit(parent, emptied).Write(t.Context(), l, header(past), scaled())
			assert.NoError(t, err, "the second commit writes")
			got, _, err := state.ReadManifest(t.Context(), l)
			assert.NoError(t, err, "the documents read")
			assert.True(t, got.Equal(scaled()), "the ledger records the manifest the parent records")
		})

		t.Run("records the ledger's documents where the parent records others", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			_, err := state.NewCommit(nil, nil).Write(t.Context(), l, header(past), scaled())
			assert.NoError(t, err, "the first commit writes")
			parent, err := state.Open(t.Context(), l)
			assert.NoError(t, err, "and opens")
			empty := manifest.Manifest{Version: manifest.Version}
			emptied, err := state.WriteManifest(t.Context(), l, empty, parent.Manifest)
			assert.NoError(t, err, "a run without a generation empties the record")
			_, err = state.NewCommit(parent, emptied).Write(t.Context(), l, header(past), empty)
			assert.NoError(t, err, "the second commit writes")
			g, err := state.Open(t.Context(), l)
			assert.NoError(t, err, "and opens")
			assert.NotEqual(t, g.Name, parent.Name, "a successor is live")
			assert.Equal(t, g.Manifest, emptied, "and records the ledger's documents")
		})

		t.Run("counts the bytes it wrote", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			c := state.NewCommit(nil, nil)
			c.Put(state.TableChecks, []byte("alpha"), []byte("row"))
			result, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
			assert.NoError(t, err, "the commit writes")
			assert.True(t, result.Written > 0, "it wrote bytes")
		})

		t.Run("counts the bytes of the live generation", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			c := state.NewCommit(nil, nil)
			c.Put(state.TableChecks, []byte("alpha"), []byte("row"))
			result, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
			assert.NoError(t, err, "the commit writes")
			assert.True(t, result.Size > 0, "the live generation has bytes")
		})

		t.Run("removes every blob older than the anchor the live generation does not reference", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			committed(t, l, time.Now(), "alpha")
			g := committed(t, l, time.Now(), "beta")
			assert.Equal(t, blobsUnder(t, l, genPrefix), []string{g.Name}, "one generation remains")
		})

		t.Run("keeps a blob newer than the anchor", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			committed(t, l, past, "alpha")
			committed(t, l, past, "beta")
			assert.Length(t, blobsUnder(t, l, genPrefix), 2, "both generations are newer than the anchor")
		})

		t.Run("releases a segment no unit references", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			c := state.NewCommit(nil, nil)
			refs, err := c.AddRegions([][]byte{[]byte("ab")})
			assert.NoError(t, err, "the region adds")
			_, err = c.Write(t.Context(), l, header(time.Now()), manifest.Manifest{Version: manifest.Version})
			assert.NoError(t, err, "the first commit writes")
			first, err := state.Open(t.Context(), l)
			assert.NoError(t, err, "and opens")
			assert.Equal(t, first.Live(refs[0].Segment), 1, "the segment has one live region")

			next := state.NewCommit(first, nil)
			next.Release(refs[0])
			next.Put(state.TableChecks, []byte("alpha"), []byte("row"))
			_, err = next.Write(t.Context(), l, header(time.Now()), manifest.Manifest{Version: manifest.Version})
			assert.NoError(t, err, "the second commit writes")
			second, err := state.Open(t.Context(), l)
			assert.NoError(t, err, "and opens")
			assert.Equal(t, second.Live(refs[0].Segment), 0, "no region of the segment is live")
			assert.NotContains(t, blobsUnder(t, l, segPrefix), refs[0].Segment, "and the segment is removed")
		})

		failures := []struct {
			name   string
			prefix string
		}{
			{name: "returns the error of a segment that does not write", prefix: segPrefix},
			{name: "returns the error of a generation that does not write", prefix: genPrefix},
			{name: "returns the error of a CURRENT that does not write", prefix: "state/CURRENT"},
		}
		for _, tt := range failures {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				mem := ledger.NewMem()
				first := committed(t, mem, past, "alpha")
				l := refusing{Mem: mem, prefix: tt.prefix}
				c := state.NewCommit(first, nil)
				_, err := c.AddRegions([][]byte{[]byte("ab")})
				assert.NoError(t, err, "the region adds")
				c.Put(state.TableChecks, []byte("beta"), []byte("row"))
				_, err = c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
				assert.ErrorIs(t, err, errDevice, "the ledger's own error returns")
				live, err := state.Open(t.Context(), mem)
				assert.NoError(t, err, "the state still opens")
				assert.Equal(t, live.Name, first.Name, "and the parent is live")
			})
		}

		t.Run("returns the error of a document that does not write after CURRENT", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			l := refusing{Mem: mem, prefix: "manifest/"}
			_, err := state.NewCommit(nil, nil).Write(t.Context(), l, header(past), scaled())
			assert.ErrorIs(t, err, errDevice, "the ledger's own error returns")
			_, err = state.Open(t.Context(), mem)
			assert.NoError(t, err, "and the generation is live")
		})
	})

	t.Run("Put", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps the last row put under one key", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			c := state.NewCommit(nil, nil)
			c.Put(state.TableChecks, []byte("alpha"), []byte("first"))
			c.Put(state.TableChecks, []byte("alpha"), []byte("second"))
			_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
			assert.NoError(t, err, "the commit writes")
			g, err := state.Open(t.Context(), l)
			assert.NoError(t, err, "and opens")
			row, held, err := g.Get(t.Context(), state.TableChecks, []byte("alpha"))
			assert.NoError(t, err, "the row reads")
			assert.True(t, held, "the key has a row")
			assert.Equal(t, string(row), "second", "the last put decides")
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()

		t.Run("deletes a row an older run wrote", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			first := committed(t, l, past, "alpha", "beta")
			c := state.NewCommit(first, nil)
			c.Delete(state.TableChecks, []byte("alpha"))
			_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
			assert.NoError(t, err, "the commit writes")
			g, err := state.Open(t.Context(), l)
			assert.NoError(t, err, "and opens")
			_, held, err := g.Get(t.Context(), state.TableChecks, []byte("alpha"))
			assert.NoError(t, err, "the lookup reads")
			assert.False(t, held, "the tombstone deletes the older row")
		})
	})

	t.Run("merges", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps every row through a merge of more than eight runs", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			committed(t, l, past, strings.Repeat("seed", 1000))
			var g *state.Generation
			for i := range 12 {
				g = committed(t, l, past, "key"+strconv.Itoa(i))
			}
			rows, err := g.All(t.Context(), state.TableChecks)
			assert.NoError(t, err, "the table reads")
			assert.Length(t, rows, 13, "every row survives the merges")
		})

		t.Run("drops a deleted row at a merge", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			first := committed(t, l, past, "alpha", "beta")
			c := state.NewCommit(first, nil)
			c.Delete(state.TableChecks, []byte("alpha"))
			c.Put(state.TableChecks, []byte("gamma"), []byte(strings.Repeat("x", 4096)))
			_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
			assert.NoError(t, err, "the merging commit writes")
			g, err := state.Open(t.Context(), l)
			assert.NoError(t, err, "and opens")
			rows, err := g.All(t.Context(), state.TableChecks)
			assert.NoError(t, err, "the table reads")
			var keys []string
			for _, r := range rows {
				keys = append(keys, string(r.Key))
			}
			assert.Equal(t, keys, []string{"beta", "gamma"}, "the deleted row is gone")
		})
	})
}

// A commit's steps allocate within their ceilings in the ordinary run,
// which runs no benchmark. Each step that changes a commit takes one
// built before the count. The check runs alone, because AllocsPerRun
// counts every goroutine's allocations and refuses to run beside
// parallel tests.
func TestCommitAllocs(t *testing.T) {
	parent, ref := parentWithRegion(t)
	var c *state.Commit
	assert.MaxAllocs(t, func() { c = state.NewCommit(nil, nil) }, newCommitAllocs,
		"NewCommit allocates the commit and its live map")
	assert.MaxAllocs(t, func() { c = state.NewCommit(parent, nil) }, newCommitParentAllocs,
		"NewCommit copies the parent's live regions")
	assert.MaxAllocs(t, func() { c.Release(ref) }, 0, "Release allocates nothing")

	blobs := [][]byte{[]byte("ab"), []byte("cde")}
	key, row := []byte("alpha"), []byte("row")
	fresh, at := newCommits(allocRuns), 0
	assert.MaxAllocs(t, func() {
		if _, err := fresh[at].AddRegions(blobs); err != nil {
			t.Fatalf("AddRegions: unexpected error: %v", err)
		}
		at++
	}, addRegionsAllocs, "AddRegions allocates the places, the segment and its name")
	fresh, at = newCommits(allocRuns), 0
	assert.MaxAllocs(t, func() {
		fresh[at].Put(state.TableChecks, key, row)
		at++
	}, putAllocs, "Put allocates the copies and the table's map")
	fresh, at = newCommits(allocRuns), 0
	assert.MaxAllocs(t, func() {
		fresh[at].Delete(state.TableChecks, key)
		at++
	}, deleteAllocs, "Delete allocates the copy and the table's map")

	writes := make([]*state.Commit, allocRuns)
	ledgers := make([]*ledger.Mem, allocRuns)
	for i := range writes {
		writes[i], ledgers[i] = state.NewCommit(nil, nil), ledger.NewMem()
		writes[i].Put(state.TableChecks, key, row)
	}
	h, m := header(past), manifest.Manifest{Version: manifest.Version}
	at = 0
	assert.MaxAllocs(t, func() {
		if _, err := writes[at].Write(t.Context(), ledgers[at], h, m); err != nil {
			t.Fatalf("Write: unexpected error: %v", err)
		}
		at++
	}, writeAllocs, "Write allocates the segments, the generation and CURRENT")
}

// BenchmarkCommit measures each step of a commit: its construction, a
// region segment, a release, a row, a tombstone, and the write that
// makes it live.
func BenchmarkCommit(b *testing.B) {
	parent, ref := parentWithRegion(b)
	key, row := []byte("alpha"), []byte("row")

	b.Run("NewCommit/a cold run", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newCommitAllocs)
		defer c.End()
		var got *state.Commit
		for c.Loop() {
			got = state.NewCommit(nil, nil)
		}
		assert.NotNil(b, got, "NewCommit returns the commit")
	})

	b.Run("NewCommit/over a parent of one live segment", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newCommitParentAllocs)
		defer c.End()
		var got *state.Commit
		for c.Loop() {
			got = state.NewCommit(parent, nil)
		}
		assert.NotNil(b, got, "NewCommit returns the commit")
	})

	b.Run("AddRegions/two blobs into a new commit", func(b *testing.B) {
		blobs := [][]byte{[]byte("ab"), []byte("cde")}
		c := bench.Start(b).MaxAllocs(addRegionsAllocs)
		defer c.End()
		var (
			commit *state.Commit
			refs   []state.RegionRef
			err    error
		)
		for c.Loop() {
			c.Excluding(func() { commit = state.NewCommit(nil, nil) })
			refs, err = commit.AddRegions(blobs)
		}
		assert.NoError(b, err, "the regions add")
		assert.Length(b, refs, 2, "one place for each blob")
	})

	b.Run("Release", func(b *testing.B) {
		commit := state.NewCommit(parent, nil)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		for c.Loop() {
			commit.Release(ref)
		}
		assert.NotNil(b, commit, "the release is recorded")
	})

	b.Run("Put/a first row into a new commit", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(putAllocs)
		defer c.End()
		var commit *state.Commit
		for c.Loop() {
			c.Excluding(func() { commit = state.NewCommit(nil, nil) })
			commit.Put(state.TableChecks, key, row)
		}
		assert.NotNil(b, commit, "the row is recorded")
	})

	b.Run("Delete/a first tombstone into a new commit", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(deleteAllocs)
		defer c.End()
		var commit *state.Commit
		for c.Loop() {
			c.Excluding(func() { commit = state.NewCommit(nil, nil) })
			commit.Delete(state.TableChecks, key)
		}
		assert.NotNil(b, commit, "the tombstone is recorded")
	})

	b.Run("Write/a commit of one row into an empty ledger", func(b *testing.B) {
		h, m := header(past), manifest.Manifest{Version: manifest.Version}
		c := bench.Start(b).MaxAllocs(writeAllocs)
		defer c.End()
		var (
			commit *state.Commit
			l      *ledger.Mem
			result state.Result
			err    error
		)
		for c.Loop() {
			c.Excluding(func() {
				commit, l = state.NewCommit(nil, nil), ledger.NewMem()
				commit.Put(state.TableChecks, key, row)
			})
			result, err = commit.Write(b.Context(), l, h, m)
		}
		assert.NoError(b, err, "the commit writes")
		assert.NotEqual(b, result.Generation, "", "and makes a generation live")
	})
}

// parentWithRegion returns a live generation whose one region segment
// has one live region, and the region's place.
func parentWithRegion(tb testing.TB) (*state.Generation, state.RegionRef) {
	tb.Helper()

	l := ledger.NewMem()
	c := state.NewCommit(nil, nil)
	refs, err := c.AddRegions([][]byte{[]byte("ab")})
	assert.NoError(tb, err, "the region adds")
	_, err = c.Write(tb.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
	assert.NoError(tb, err, "the commit writes")
	g, err := state.Open(tb.Context(), l)
	assert.NoError(tb, err, "and opens")
	return g, refs[0]
}

// newCommits returns n commits of a cold run.
func newCommits(n int) []*state.Commit {
	out := make([]*state.Commit, n)
	for i := range out {
		out[i] = state.NewCommit(nil, nil)
	}
	return out
}
