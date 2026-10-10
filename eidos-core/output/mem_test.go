// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/output"
)

// The allocations of a memory sink over a staging of one file, which
// TestMemAllocs checks in the ordinary run and BenchmarkMem in a
// benchmark run.
const (
	// newMemAllocs is the sink.
	newMemAllocs = 1
	// prepareOneAllocs is the preparation of one staged file: the path
	// list, the list of changes and the file's digest.
	prepareOneAllocs = 3
	// commitOneAllocs is the commit of one staged file into memory: the
	// path list, the list of records, the committed map and its storage,
	// and the file's digest.
	commitOneAllocs = 5
	// filesOneAllocs is the copy of one committed file: the map and its
	// storage, and the copy of the body.
	filesOneAllocs = 3
)

// The memory sink is what a dry run and a test write through, so
// it has to keep the staging invisible exactly as the disk sink
// does: nothing is readable until the commit.
func TestMem(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		var m output.Mem
		assert.NoError(t, m.Write(storeFile, []byte(firstBody)), "the zero sink stages")
		_, err := m.Commit()
		assert.NoError(t, err, "and commits")
		assert.Equal(t, string(m.Files()[storeFile]), firstBody, "the committed bytes read back")
	})

	t.Run("NewMem", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a sink without a file", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, output.NewMem().Files(), "nothing is committed")
		})
	})

	t.Run("Overwrite", func(t *testing.T) {
		t.Parallel()

		t.Run("leaves the commit into the empty destination a creation", func(t *testing.T) {
			t.Parallel()

			m := output.NewMem()
			assert.NoError(t, m.Overwrite(output.FoundDrifted, output.FoundForeign), "both verdicts are allowed")
			assert.NoError(t, m.Write(storeFile, []byte(firstBody)), "the file stages")
			got, err := m.Commit()
			assert.NoError(t, err, "the commit runs")
			assert.Length(t, got, 1, "one record for the file")
			assert.Equal(t, got[0].Action, output.ActionCreated, "the empty destination had no file")
		})

		t.Run("returns an error for FoundIntact", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, output.NewMem().Overwrite(output.FoundIntact), "the verdict is refused")
		})

		t.Run("returns ErrFinished after the commit", func(t *testing.T) {
			t.Parallel()

			m := output.NewMem()
			_, err := m.Commit()
			assert.NoError(t, err, "the commit runs")
			assert.ErrorIs(t, m.Overwrite(output.FoundDrifted), output.ErrFinished, "a sink serves one staging")
		})
	})

	t.Run("Commit", func(t *testing.T) {
		t.Parallel()

		t.Run("makes the staged bytes readable through Files", func(t *testing.T) {
			t.Parallel()

			m := stagedMem(t)
			_, err := m.Commit()
			assert.NoError(t, err, "the commit runs")
			assert.Equal(t, string(m.Files()[storeFile]), firstBody, "the committed bytes read back")
		})
	})

	t.Run("Discard", func(t *testing.T) {
		t.Parallel()

		t.Run("leaves nothing to read", func(t *testing.T) {
			t.Parallel()

			m := stagedMem(t)
			assert.NoError(t, m.Discard(), "the discard runs")
			assert.Empty(t, m.Files(), "a discarded sink has no file")
		})
	})

	t.Run("Files", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing before the commit", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, stagedMem(t).Files(), "staged means invisible, in memory as on disk")
		})

		t.Run("returns a copy of the map", func(t *testing.T) {
			t.Parallel()

			m := committedMem(t)
			files := m.Files()
			delete(files, storeFile)
			files["b.go"] = []byte("package b\n")
			assert.Equal(t, m.Files(), map[string][]byte{storeFile: []byte(firstBody)},
				"a caller editing the map it was handed edits nothing here")
		})

		t.Run("returns a copy of each body", func(t *testing.T) {
			t.Parallel()

			m := committedMem(t)
			m.Files()[storeFile][0] = 'P'
			assert.Equal(t, string(m.Files()[storeFile]), firstBody,
				"a caller editing a body it was handed edits nothing here")
		})
	})
}

// Each method of the memory sink allocates what it keeps or returns in
// the ordinary run, which runs no benchmark. Each call that consumes
// its sink takes a sink of its own, built outside the count, and each
// count keeps the first error of its calls, which cmp.Or returns
// without allocating. The check runs alone, because the count includes
// every goroutine's allocations.
func TestMemAllocs(t *testing.T) {
	body := []byte(firstBody)
	var built *output.Mem
	assert.MaxAllocs(t, func() { built = output.NewMem() }, newMemAllocs, "NewMem allocates the sink")
	assert.Empty(t, built.Files(), "NewMem returns a sink without a file")

	var err error
	assert.MaxAllocsWithSetup(t, memOf(t, nil), func(m *output.Mem) { err = cmp.Or(err, m.Write(storeFile, body)) },
		firstWriteAllocs, "Write allocates the staging's maps on the first write")
	assert.NoError(t, err, "every first write stages")

	assert.MaxAllocsWithSetup(t, memOf(t, nil), func(m *output.Mem) { err = cmp.Or(err, m.Delete(storeFile)) },
		firstDeleteAllocs, "Delete allocates the removal set on the first removal")
	assert.NoError(t, err, "every first removal stages")

	assert.MaxAllocsWithSetup(t, memOf(t, body), func(m *output.Mem) {
		_, perr := m.Prepare()
		err = cmp.Or(err, perr)
	}, prepareOneAllocs, "Prepare allocates the path list, the changes and the digest")
	assert.NoError(t, err, "every staging prepares")

	assert.MaxAllocsWithSetup(t, memOf(t, body), func(m *output.Mem) {
		_, cerr := m.Commit()
		err = cmp.Or(err, cerr)
	}, commitOneAllocs, "Commit allocates the path list, the records, the committed map and the digest")
	assert.NoError(t, err, "every staging commits")

	assert.MaxAllocsWithSetup(t, memOf(t, body), func(m *output.Mem) { err = cmp.Or(err, m.Discard()) },
		0, "Discard allocates nothing")
	assert.NoError(t, err, "every staging discards")

	m := committedMem(t)
	var files map[string][]byte
	assert.MaxAllocs(t, func() { files = m.Files() }, filesOneAllocs, "Files allocates the map and the body's copy")
	assert.Length(t, files, 1, "Files returns the committed file")
}

// BenchmarkMem measures each method of the memory sink over a staging
// of one file, each call that consumes its sink on a sink built outside
// the measurement.
func BenchmarkMem(b *testing.B) {
	body := []byte(firstBody)

	b.Run("NewMem", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newMemAllocs)
		defer c.End()
		var m *output.Mem
		for c.Loop() {
			m = output.NewMem()
		}
		assert.Empty(b, m.Files(), "NewMem returns a sink without a file")
	})

	b.Run("Write", func(b *testing.B) {
		b.Run("the first file of a staging", func(b *testing.B) {
			var m *output.Mem
			fresh := func() { m = output.NewMem() }
			c := bench.Start(b).MaxAllocs(firstWriteAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(fresh)
				err = m.Write(storeFile, body)
			}
			assert.NoError(b, err, "the file stages")
		})
	})

	b.Run("Delete", func(b *testing.B) {
		b.Run("the first removal of a staging", func(b *testing.B) {
			var m *output.Mem
			fresh := func() { m = output.NewMem() }
			c := bench.Start(b).MaxAllocs(firstDeleteAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(fresh)
				err = m.Delete(storeFile)
			}
			assert.NoError(b, err, "the removal stages")
		})
	})

	b.Run("Prepare", func(b *testing.B) {
		b.Run("a staging of one file", func(b *testing.B) {
			var m *output.Mem
			fresh := func() { m = stagedMem(b) }
			c := bench.Start(b).MaxAllocs(prepareOneAllocs)
			defer c.End()
			var (
				got []output.Change
				err error
			)
			for c.Loop() {
				c.Excluding(fresh)
				got, err = m.Prepare()
			}
			assert.NoError(b, err, "the staging prepares")
			assert.Length(b, got, 1, "one change for the staged file")
		})
	})

	b.Run("Commit", func(b *testing.B) {
		b.Run("a staging of one file", func(b *testing.B) {
			var m *output.Mem
			fresh := func() { m = stagedMem(b) }
			c := bench.Start(b).MaxAllocs(commitOneAllocs)
			defer c.End()
			var (
				got []output.Written
				err error
			)
			for c.Loop() {
				c.Excluding(fresh)
				got, err = m.Commit()
			}
			assert.NoError(b, err, "the staging commits")
			assert.Length(b, got, 1, "one record for the staged file")
		})
	})

	b.Run("Discard", func(b *testing.B) {
		b.Run("a staging of one file", func(b *testing.B) {
			var m *output.Mem
			fresh := func() { m = stagedMem(b) }
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(fresh)
				err = m.Discard()
			}
			assert.NoError(b, err, "the staging is discarded")
		})
	})

	b.Run("Files", func(b *testing.B) {
		b.Run("one committed file", func(b *testing.B) {
			m := committedMem(b)
			c := bench.Start(b).MaxAllocs(filesOneAllocs)
			defer c.End()
			var files map[string][]byte
			for c.Loop() {
				files = m.Files()
			}
			assert.Length(b, files, 1, "Files returns the committed file")
		})
	})
}

// stagedMem returns a memory sink with one file staged.
func stagedMem(tb assert.TB) *output.Mem {
	tb.Helper()

	m := output.NewMem()
	assert.NoError(tb, m.Write(storeFile, []byte(firstBody)), "the file stages")
	return m
}

// committedMem returns a memory sink that committed one file.
func committedMem(tb assert.TB) *output.Mem {
	tb.Helper()

	m := stagedMem(tb)
	_, err := m.Commit()
	assert.NoError(tb, err, "the file commits")
	return m
}

// memOf returns the setup of an allocation check that consumes its
// sink: each call builds a memory sink with body staged at storeFile,
// or with nothing staged for a nil body.
func memOf(tb assert.TB, body []byte) func() *output.Mem {
	return func() *output.Mem {
		m := output.NewMem()
		if body != nil {
			assert.NoError(tb, m.Write(storeFile, body), "the file stages")
		}
		return m
	}
}
