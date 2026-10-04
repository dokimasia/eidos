// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

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
// built before the count. The check runs alone, because AllocsPerRun
// counts every goroutine's allocations and refuses to run beside
// parallel tests.
func TestTeeAllocs(t *testing.T) {
	body := []byte(firstBody)
	first, second := output.NewMem(), output.NewMem()
	var built *output.Tee
	assert.MaxAllocs(t, func() { built = output.NewTee(first, second) }, newTeeAllocs,
		"NewTee allocates the tee and its list of sinks")
	assert.NoError(t, built.Discard(), "NewTee returns a sink over both")

	at, empty := 0, tees(t, nil)
	assert.MaxAllocs(t, func() {
		if err := empty[at].Write(storeFile, body); err != nil {
			t.Fatalf("Write: unexpected error: %v", err)
		}
		at++
	}, 2*firstWriteAllocs, "Write allocates each sink's first write")

	at, empty = 0, tees(t, nil)
	assert.MaxAllocs(t, func() {
		if err := empty[at].Delete(storeFile); err != nil {
			t.Fatalf("Delete: unexpected error: %v", err)
		}
		at++
	}, 2*firstDeleteAllocs, "Delete allocates each sink's first removal")

	at, one := 0, tees(t, body)
	assert.MaxAllocs(t, func() {
		if _, err := one[at].Prepare(); err != nil {
			t.Fatalf("Prepare: unexpected error: %v", err)
		}
		at++
	}, teePrepareAllocs, "Prepare allocates each sink's preparation and a copy of the first one's")

	at, one = 0, tees(t, body)
	assert.MaxAllocs(t, func() {
		if _, err := one[at].Commit(); err != nil {
			t.Fatalf("Commit: unexpected error: %v", err)
		}
		at++
	}, teeCommitAllocs, "Commit allocates each sink's commit and a copy of the first one's")

	at, one = 0, tees(t, body)
	assert.MaxAllocs(t, func() {
		if err := one[at].Discard(); err != nil {
			t.Fatalf("Discard: unexpected error: %v", err)
		}
		at++
	}, 0, "Discard allocates nothing")
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

	b.Run("Write/the first file of a staging", func(b *testing.B) {
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

	b.Run("Delete/the first removal of a staging", func(b *testing.B) {
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

	b.Run("Prepare/a staging of one file", func(b *testing.B) {
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

	b.Run("Commit/a staging of one file", func(b *testing.B) {
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

	b.Run("Discard/a staging of one file", func(b *testing.B) {
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
}

// stagedTee returns a tee over two memory sinks with one file staged.
func stagedTee(tb assert.TB) *output.Tee {
	tb.Helper()

	tee := output.NewTee(output.NewMem(), output.NewMem())
	assert.NoError(tb, tee.Write(storeFile, []byte(firstBody)), "the file stages")
	return tee
}

// tees returns allocRuns tees over two memory sinks each, with body
// staged at storeFile, or with nothing staged for a nil body: one tee
// for each call of an allocation check that consumes its sinks.
func tees(t *testing.T, body []byte) []*output.Tee {
	t.Helper()

	out := make([]*output.Tee, allocRuns)
	for i := range out {
		out[i] = output.NewTee(output.NewMem(), output.NewMem())
		if body != nil {
			assert.NoError(t, out[i].Write(storeFile, body), "the file stages")
		}
	}
	return out
}
