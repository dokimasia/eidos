// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"cmp"
	"errors"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/output"
)

// The allocations of a tee over two memory sinks, which TestTeeAllocs
// checks in the ordinary run and BenchmarkTee in a benchmark run.
const (
	// newTeeAllocs is the tee and its list of sinks.
	newTeeAllocs = 2
	// teePrepareAllocs is the preparation of one staged file in two
	// memory sinks, and the copy of the first sink's changes.
	teePrepareAllocs = 2*prepareOneAllocs + 1
	// teeCommitAllocs is the commit of one staged file in two memory
	// sinks, and the copy of the first sink's records.
	teeCommitAllocs = 2*commitOneAllocs + 1
)

// errRefused is the failure the refusing sink returns.
var errRefused = errors.New("nowhere to write")

// refusing is a sink that takes nothing, for the fan-out's error
// path.
type refusing struct{}

// Write refuses the staging with errRefused.
func (refusing) Write(string, []byte) error { return errRefused }

// Delete refuses the removal with errRefused.
func (refusing) Delete(string) error { return errRefused }

// Prepare refuses the preparation with errRefused.
func (refusing) Prepare() ([]output.Change, error) { return nil, errRefused }

// Commit refuses the commit with errRefused.
func (refusing) Commit() ([]output.Written, error) { return nil, errRefused }

// Discard refuses the discard with errRefused.
func (refusing) Discard() error { return errRefused }

// The fan-out sink writes one staging to several destinations, so
// every one of them sees every call, and no failure is lost on the
// way.
func TestTee(t *testing.T) {
	t.Parallel()

	t.Run("NewTee", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a sink over every sink it was given", func(t *testing.T) {
			t.Parallel()

			sinks := []*output.Mem{output.NewMem(), output.NewMem(), output.NewMem()}
			tee := output.NewTee(sinks[0], sinks[1], sinks[2])
			assert.NoError(t, tee.Write(storeFile, []byte(firstBody)), "the file stages")
			_, err := tee.Commit()
			assert.NoError(t, err, "the commit runs")
			for _, m := range sinks {
				assert.Equal(t, string(m.Files()[storeFile]), firstBody, "every sink has the bytes")
			}
		})
	})

	t.Run("Overwrite", func(t *testing.T) {
		t.Parallel()

		t.Run("lets every sink's commit write over a drifted file", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{storeFile: files.Text(drifted(t))})
			m := output.NewMem()
			tee := output.NewTee(opened(t, root, nil), m)
			assert.NoError(t, tee.Overwrite(output.FoundDrifted), "the verdict is allowed in every sink")
			next := stampedAs(t, diskBrand, secondBody)
			assert.NoError(t, tee.Write(storeFile, []byte(next)), "the file stages")
			_, err := tee.Commit()
			assert.NoError(t, err, "the commit writes over the drifted file")
			files.HasContent(t, filepath.Join(root, storeFile), next, "the disk has the staged bytes")
			assert.Equal(t, string(m.Files()[storeFile]), next, "and so does the memory sink")
		})

		t.Run("returns an error for a sink that does not overwrite", func(t *testing.T) {
			t.Parallel()

			tee := output.NewTee(output.NewMem(), refusing{})
			assert.HasError(t, tee.Overwrite(output.FoundDrifted), "the refusing sink would refuse the file")
		})
	})

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("stages into every sink", func(t *testing.T) {
			t.Parallel()

			first, second := output.NewMem(), output.NewMem()
			tee := output.NewTee(first, second)
			assert.NoError(t, tee.Write(storeFile, []byte(firstBody)), "the staging takes")
			_, err := tee.Commit()
			assert.NoError(t, err, "the commit runs")
			assert.Equal(t, string(first.Files()[storeFile]), firstBody, "the first sink has the bytes")
			assert.Equal(t, string(second.Files()[storeFile]), firstBody, "and so does the second")
		})

		t.Run("returns the error of a failing sink", func(t *testing.T) {
			t.Parallel()

			tee := output.NewTee(output.NewMem(), refusing{})
			assert.ErrorIs(t, tee.Write(storeFile, []byte(firstBody)), errRefused,
				"a staging that failed anywhere failed")
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()

		t.Run("stages a removal into every sink", func(t *testing.T) {
			t.Parallel()

			first, second := output.NewMem(), output.NewMem()
			tee := output.NewTee(first, second)
			assert.NoError(t, tee.Delete("gone.go"), "the removal stages")
			assert.HasError(t, first.Delete("gone.go"), "the first sink has the removal")
			assert.HasError(t, second.Delete("gone.go"), "and so does the second")
		})

		t.Run("returns the error of a failing sink", func(t *testing.T) {
			t.Parallel()

			tee := output.NewTee(output.NewMem(), refusing{})
			assert.ErrorIs(t, tee.Delete("gone.go"), errRefused, "a removal that failed anywhere failed")
		})
	})

	t.Run("Prepare", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the first sink's changes", func(t *testing.T) {
			t.Parallel()

			tee := output.NewTee(output.NewMem(), output.NewMem())
			assert.NoError(t, tee.Write(storeFile, []byte(firstBody)), "the staging takes")
			got, err := tee.Prepare()
			assert.NoError(t, err, "every sink prepares")
			assert.Length(t, got, 1, "one change per staged path")
			assert.Equal(t, got[0].Action, output.ActionCreated, "the first sink's verdict")
		})

		t.Run("returns the error of a failing sink", func(t *testing.T) {
			t.Parallel()

			_, err := output.NewTee(output.NewMem(), refusing{}).Prepare()
			assert.ErrorIs(t, err, errRefused, "a preparation that failed anywhere failed")
		})
	})

	t.Run("Commit", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the first sink's records", func(t *testing.T) {
			t.Parallel()

			tee := output.NewTee(output.NewMem(), output.NewMem())
			assert.NoError(t, tee.Write(storeFile, []byte(firstBody)), "the staging takes")
			got, err := tee.Commit()
			assert.NoError(t, err, "the commit runs")
			assert.Length(t, got, 1, "one record per staged file")
			assert.Equal(t, got[0].Path, storeFile, "read off the first sink, which is the one of record")
		})

		t.Run("returns the error of a failing sink", func(t *testing.T) {
			t.Parallel()

			_, err := output.NewTee(output.NewMem(), refusing{}).Commit()
			assert.ErrorIs(t, err, errRefused, "a commit that failed anywhere failed")
		})
	})

	t.Run("Discard", func(t *testing.T) {
		t.Parallel()

		t.Run("discards every sink", func(t *testing.T) {
			t.Parallel()

			first, second := output.NewMem(), output.NewMem()
			tee := output.NewTee(first, second)
			assert.NoError(t, tee.Write(storeFile, []byte(firstBody)), "the staging takes")
			assert.NoError(t, tee.Discard(), "the discard runs")
			assert.ErrorIs(t, first.Discard(), output.ErrFinished, "the first sink is discarded")
			assert.ErrorIs(t, second.Discard(), output.ErrFinished, "and so is the second")
		})

		t.Run("returns the error of a failing sink", func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, output.NewTee(output.NewMem(), refusing{}).Discard(), errRefused,
				"a discard that failed anywhere failed")
		})
	})
}

// Each method of the tee allocates what its sinks allocate, and a copy
// of the first sink's list, in the ordinary run, which runs no
// benchmark. Each call that consumes its sinks takes a tee of its own,
// built outside the count, and each count keeps the first error of its
// calls, which cmp.Or returns without allocating. The check runs alone,
// because the count includes every goroutine's allocations.
func TestTeeAllocs(t *testing.T) {
	body := []byte(firstBody)
	first, second := output.NewMem(), output.NewMem()
	var built *output.Tee
	assert.MaxAllocs(t, func() { built = output.NewTee(first, second) }, newTeeAllocs,
		"NewTee allocates the tee and its list of sinks")
	assert.NoError(t, built.Discard(), "NewTee returns a sink over both")

	var err error
	assert.MaxAllocsWithSetup(t, teeOf(t, nil), func(tee *output.Tee) { err = cmp.Or(err, tee.Write(storeFile, body)) },
		2*firstWriteAllocs, "Write allocates each sink's first write")
	assert.NoError(t, err, "every first write stages")

	assert.MaxAllocsWithSetup(t, teeOf(t, nil), func(tee *output.Tee) { err = cmp.Or(err, tee.Delete(storeFile)) },
		2*firstDeleteAllocs, "Delete allocates each sink's first removal")
	assert.NoError(t, err, "every first removal stages")

	assert.MaxAllocsWithSetup(t, teeOf(t, body), func(tee *output.Tee) {
		_, perr := tee.Prepare()
		err = cmp.Or(err, perr)
	}, teePrepareAllocs, "Prepare allocates each sink's preparation and a copy of the first one's")
	assert.NoError(t, err, "every staging prepares")

	assert.MaxAllocsWithSetup(t, teeOf(t, body), func(tee *output.Tee) {
		_, cerr := tee.Commit()
		err = cmp.Or(err, cerr)
	}, teeCommitAllocs, "Commit allocates each sink's commit and a copy of the first one's")
	assert.NoError(t, err, "every staging commits")

	assert.MaxAllocsWithSetup(t, teeOf(t, body), func(tee *output.Tee) { err = cmp.Or(err, tee.Discard()) },
		0, "Discard allocates nothing")
	assert.NoError(t, err, "every staging discards")
}

// BenchmarkTee measures each method of a tee over two memory sinks and
// a staging of one file, each call that consumes its sinks on a tee
// built outside the measurement.
func BenchmarkTee(b *testing.B) {
	body := []byte(firstBody)

	b.Run("NewTee", func(b *testing.B) {
		first, second := output.NewMem(), output.NewMem()
		c := bench.Start(b).MaxAllocs(newTeeAllocs)
		defer c.End()
		var tee *output.Tee
		for c.Loop() {
			tee = output.NewTee(first, second)
		}
		assert.NoError(b, tee.Discard(), "NewTee returns a sink over both")
	})

	b.Run("Write", func(b *testing.B) {
		b.Run("the first file of a staging", func(b *testing.B) {
			var tee *output.Tee
			fresh := func() { tee = output.NewTee(output.NewMem(), output.NewMem()) }
			c := bench.Start(b).MaxAllocs(2 * firstWriteAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(fresh)
				err = tee.Write(storeFile, body)
			}
			assert.NoError(b, err, "the file stages")
		})
	})

	b.Run("Delete", func(b *testing.B) {
		b.Run("the first removal of a staging", func(b *testing.B) {
			var tee *output.Tee
			fresh := func() { tee = output.NewTee(output.NewMem(), output.NewMem()) }
			c := bench.Start(b).MaxAllocs(2 * firstDeleteAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(fresh)
				err = tee.Delete(storeFile)
			}
			assert.NoError(b, err, "the removal stages")
		})
	})

	b.Run("Prepare", func(b *testing.B) {
		b.Run("a staging of one file", func(b *testing.B) {
			var tee *output.Tee
			fresh := func() { tee = stagedTee(b) }
			c := bench.Start(b).MaxAllocs(teePrepareAllocs)
			defer c.End()
			var (
				got []output.Change
				err error
			)
			for c.Loop() {
				c.Excluding(fresh)
				got, err = tee.Prepare()
			}
			assert.NoError(b, err, "the staging prepares")
			assert.Length(b, got, 1, "one change for the staged file")
		})
	})

	b.Run("Commit", func(b *testing.B) {
		b.Run("a staging of one file", func(b *testing.B) {
			var tee *output.Tee
			fresh := func() { tee = stagedTee(b) }
			c := bench.Start(b).MaxAllocs(teeCommitAllocs)
			defer c.End()
			var (
				got []output.Written
				err error
			)
			for c.Loop() {
				c.Excluding(fresh)
				got, err = tee.Commit()
			}
			assert.NoError(b, err, "the staging commits")
			assert.Length(b, got, 1, "one record for the staged file")
		})
	})

	b.Run("Discard", func(b *testing.B) {
		b.Run("a staging of one file", func(b *testing.B) {
			var tee *output.Tee
			fresh := func() { tee = stagedTee(b) }
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(fresh)
				err = tee.Discard()
			}
			assert.NoError(b, err, "the staging is discarded")
		})
	})
}

// stagedTee returns a tee over two memory sinks with one file staged.
func stagedTee(tb assert.TB) *output.Tee {
	tb.Helper()

	tee := output.NewTee(output.NewMem(), output.NewMem())
	assert.NoError(tb, tee.Write(storeFile, []byte(firstBody)), "the file stages")
	return tee
}

// teeOf returns the setup of an allocation check that consumes its
// sinks: each call builds a tee over two memory sinks with body staged
// at storeFile, or with nothing staged for a nil body.
func teeOf(tb assert.TB, body []byte) func() *output.Tee {
	return func() *output.Tee {
		tee := output.NewTee(output.NewMem(), output.NewMem())
		if body != nil {
			assert.NoError(tb, tee.Write(storeFile, body), "the file stages")
		}
		return tee
	}
}
