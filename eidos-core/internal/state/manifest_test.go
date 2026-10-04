// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
)

// The fixture record's workspace, plan and digest, and the scale of the
// record a case splits into documents.
const (
	workspaceName = "platform"
	planName      = "go-services"
	files         = 1_000
)

// errDevice is the refusal a failing ledger returns.
var errDevice = errors.New("the device is gone")

// The ceilings of the record's read and write over a record of 1,000
// files, each with the JSON codec's state pooled by an earlier call.
const (
	// readManifestAllocs is one read: the listing, each document's read
	// and decode, and the joined manifest. Three fresh processes with the
	// collector off counted 2,560 each.
	readManifestAllocs = 2_560
	// writeManifestAllocs is one write of a manifest equal to the record:
	// each document's encoding and digest, compared with the record's,
	// and nothing written. Three fresh processes with the collector off
	// counted 1,791 each.
	writeManifestAllocs = 1_791
	// manifestStateAllocs is the JSON codec's state, which it takes from a
	// pool per processor. A collection empties the pool, and 75 of 100
	// runs of one iteration wrote the equal manifest with 9 allocations
	// more. Each ceiling allows it once.
	manifestStateAllocs = 9
)

// failing is a memory ledger that refuses one operation: a list, a read,
// a write of one name, or every unsynced write.
type failing struct {
	*ledger.Mem
	list    bool
	read    bool
	put     bool
	writeAt string
}

// Put refuses where the ledger fails unsynced writes.
func (f failing) Put(ctx context.Context, name string, b []byte) error {
	if f.put {
		return errDevice
	}
	return f.Mem.Put(ctx, name, b)
}

// List refuses where the ledger fails lists.
func (f failing) List(ctx context.Context, dir string) ([]ledger.Blob, error) {
	if f.list {
		return nil, errDevice
	}
	return f.Mem.List(ctx, dir)
}

// Read refuses where the ledger fails reads.
func (f failing) Read(ctx context.Context, name string) ([]byte, error) {
	if f.read {
		return nil, errDevice
	}
	return f.Mem.Read(ctx, name)
}

// Write refuses the one name the ledger fails writes of.
func (f failing) Write(ctx context.Context, name string, b []byte) error {
	if name == f.writeAt {
		return errDevice
	}
	return f.Mem.Write(ctx, name, b)
}

// entry returns a well-formed entry for one path.
func entry(path string) manifest.Entry {
	return manifest.Entry{Path: path, Plan: planName, Hash: "sha256:" + strings.Repeat("ab", 32)}
}

// scaled returns a manifest of files entries, sorted by path.
func scaled() manifest.Manifest {
	m := manifest.Manifest{Version: manifest.Version, Workspace: workspaceName}
	for i := range files {
		m.Files = append(m.Files, entry(fmt.Sprintf("p%03d/f%d.go", i/10, i%10)))
	}
	return m
}

// recorded returns a memory ledger that contains the documents of m,
// and their digests.
func recorded(tb testing.TB, m manifest.Manifest) (*ledger.Mem, state.Digests) {
	tb.Helper()

	l := ledger.NewMem()
	digests, err := state.WriteManifest(tb.Context(), l, m, nil)
	assert.NoError(tb, err, "the record is written")
	return l, digests
}

// documentOf returns the ledger name of a path's document.
func documentOf(path string) string { return "manifest/" + manifest.BucketOf(path) + ".json" }

// The record's documents round-trip through a ledger, and a commit
// writes only the documents whose entries changed.
func TestManifest(t *testing.T) {
	t.Parallel()

	t.Run("ReadManifest", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the empty manifest for a ledger without documents", func(t *testing.T) {
			t.Parallel()

			got, digests, err := state.ReadManifest(t.Context(), ledger.NewMem())
			assert.NoError(t, err, "no record is no error")
			assert.True(t, got.Equal(manifest.Manifest{Version: manifest.Version}), "the empty manifest")
			assert.Empty(t, digests, "and no digests")
		})

		t.Run("returns the manifest WriteManifest recorded with each document's digest", func(t *testing.T) {
			t.Parallel()

			m := scaled()
			l, want := recorded(t, m)
			got, digests, err := state.ReadManifest(t.Context(), l)
			assert.NoError(t, err, "the record reads")
			assert.True(t, got.Equal(m), "the recorded manifest")
			assert.Equal(t, digests, want, "and the recorded digests")
		})

		t.Run("skips a blob whose name is no bucket's document", func(t *testing.T) {
			t.Parallel()

			m := scaled()
			l, _ := recorded(t, m)
			for _, name := range []string{"manifest/ea.json.8000000000000000.stage", "manifest/EA.json", "manifest/notes"} {
				assert.NoError(t, l.Write(t.Context(), name, []byte("not a document")), "a stray blob is written")
			}
			got, _, err := state.ReadManifest(t.Context(), l)
			assert.NoError(t, err, "the stray blobs are skipped")
			assert.True(t, got.Equal(m), "the recorded manifest")
		})

		t.Run("returns ErrUnsupported for a document that does not decode", func(t *testing.T) {
			t.Parallel()

			l, _ := recorded(t, scaled())
			name := documentOf("p000/f0.go")
			assert.NoError(t, l.Write(t.Context(), name, []byte("not json")), "the document breaks")
			got, _, err := state.ReadManifest(t.Context(), l)
			assert.ErrorIs(t, err, manifest.ErrUnsupported, "the record does not read")
			assert.Contains(t, err.Error(), name, "the error names the document")
			assert.Empty(t, got.Files, "beside the empty manifest")
		})

		t.Run("returns ErrUnsupported for a document that records another bucket than its name", func(t *testing.T) {
			t.Parallel()

			m := manifest.Manifest{Version: manifest.Version, Workspace: workspaceName, Files: []manifest.Entry{
				entry("p000/f0.go"),
			}}
			l, _ := recorded(t, m)
			body, err := l.Read(t.Context(), documentOf("p000/f0.go"))
			assert.NoError(t, err, "the document reads")
			other := "manifest/00.json"
			if documentOf("p000/f0.go") == other {
				other = "manifest/01.json"
			}
			assert.NoError(t, l.Write(t.Context(), other, body), "the document is copied under another bucket")
			_, _, err = state.ReadManifest(t.Context(), l)
			assert.ErrorIs(t, err, manifest.ErrUnsupported, "the record does not read")
		})

		t.Run("returns ErrUnsupported for documents that do not join", func(t *testing.T) {
			t.Parallel()

			m := scaled()
			l, _ := recorded(t, m)
			s := manifest.Split(m)[0]
			s.Workspace = "other"
			body, err := manifest.EncodeShard(s)
			assert.NoError(t, err, "the document encodes")
			assert.NoError(t, l.Write(t.Context(), "manifest/"+s.Bucket+".json", body),
				"one document changes workspace")
			_, _, err = state.ReadManifest(t.Context(), l)
			assert.ErrorIs(t, err, manifest.ErrUnsupported, "the record does not read")
		})

		t.Run("returns an error for a ledger that does not list", func(t *testing.T) {
			t.Parallel()

			_, _, err := state.ReadManifest(t.Context(), failing{Mem: ledger.NewMem(), list: true})
			assert.ErrorIs(t, err, errDevice, "the list's refusal")
		})

		t.Run("returns an error for a document that does not read", func(t *testing.T) {
			t.Parallel()

			l, _ := recorded(t, scaled())
			_, _, err := state.ReadManifest(t.Context(), failing{Mem: l, read: true})
			assert.ErrorIs(t, err, errDevice, "the read's refusal")
		})
	})

	t.Run("WriteManifest", func(t *testing.T) {
		t.Parallel()

		t.Run("writes every document of a new record", func(t *testing.T) {
			t.Parallel()

			m := scaled()
			l, digests := recorded(t, m)
			assert.Equal(t, l.Writes(), len(manifest.Split(m)), "one write per document")
			assert.Length(t, digests, len(manifest.Split(m)), "and one digest per document")
		})

		t.Run("writes nothing for a manifest equal to the record", func(t *testing.T) {
			t.Parallel()

			m := scaled()
			l, digests := recorded(t, m)
			before := l.Writes()
			again, err := state.WriteManifest(t.Context(), l, m, digests)
			assert.NoError(t, err, "the equal record commits")
			assert.Equal(t, l.Writes(), before, "nothing is written")
			assert.Equal(t, again, digests, "and the digests are the recorded ones")
		})

		t.Run("writes only the document whose entry changed", func(t *testing.T) {
			t.Parallel()

			m := scaled()
			l, digests := recorded(t, m)
			before := l.Writes()
			m.Files[7].Hash = "sha256:" + strings.Repeat("cd", 32)
			_, err := state.WriteManifest(t.Context(), l, m, digests)
			assert.NoError(t, err, "the changed record commits")
			assert.Equal(t, l.Writes(), before+1, "one document is written")
			got, _, err := state.ReadManifest(t.Context(), l)
			assert.NoError(t, err, "the record reads")
			assert.True(t, got.Equal(m), "the changed manifest")
		})

		t.Run("removes the document whose bucket the manifest leaves empty", func(t *testing.T) {
			t.Parallel()

			first := entry("p000/f0.go")
			m := manifest.Manifest{Version: manifest.Version, Workspace: workspaceName, Files: []manifest.Entry{first}}
			l, digests := recorded(t, m)
			m.Files = nil
			kept, err := state.WriteManifest(t.Context(), l, m, digests)
			assert.NoError(t, err, "the emptied record commits")
			assert.Empty(t, kept, "no document remains")
			listed, err := l.List(t.Context(), "manifest")
			assert.NoError(t, err, "the documents list")
			assert.Empty(t, listed, "the document is removed")
		})

		t.Run("removes a document the recorded digests do not name", func(t *testing.T) {
			t.Parallel()

			m := scaled()
			l, _ := recorded(t, m)
			m.Files = m.Files[:1]
			_, err := state.WriteManifest(t.Context(), l, m, nil)
			assert.NoError(t, err, "the record commits without digests")
			got, digests, err := state.ReadManifest(t.Context(), l)
			assert.NoError(t, err, "the record reads")
			assert.True(t, got.Equal(m), "the documents of the other buckets are removed")
			assert.Length(t, digests, 1, "one document remains")
		})

		t.Run("returns an error for a manifest that breaks the format", func(t *testing.T) {
			t.Parallel()

			m := scaled()
			m.Files[1].Plan = ""
			l := ledger.NewMem()
			_, err := state.WriteManifest(t.Context(), l, m, nil)
			assert.HasError(t, err, "the record is refused")
			assert.Equal(t, l.Writes(), 0, "and nothing is written")
		})

		t.Run("returns the error of a document that fails to write", func(t *testing.T) {
			t.Parallel()

			m := scaled()
			l := failing{Mem: ledger.NewMem(), writeAt: documentOf(m.Files[0].Path)}
			_, err := state.WriteManifest(t.Context(), l, m, nil)
			assert.ErrorIs(t, err, errDevice, "the failed write is returned")
		})

		t.Run("writes every other document beside one that fails", func(t *testing.T) {
			t.Parallel()

			m := scaled()
			l := failing{Mem: ledger.NewMem(), writeAt: documentOf(m.Files[0].Path)}
			_, err := state.WriteManifest(t.Context(), l, m, nil)
			assert.HasError(t, err, "the failed write is returned")
			assert.Equal(t, l.Writes(), len(manifest.Split(m))-1, "every other document is written")
		})

		t.Run("returns an error for a ledger that does not list", func(t *testing.T) {
			t.Parallel()

			l := failing{Mem: ledger.NewMem(), list: true}
			_, err := state.WriteManifest(t.Context(), l, scaled(), nil)
			assert.ErrorIs(t, err, errDevice, "the list's refusal")
			assert.Equal(t, l.Writes(), 0, "and nothing is written")
		})
	})
}

// A read and an unchanged write of the record allocate within their
// ceilings in the ordinary run, which runs no benchmark. The check runs
// alone, because AllocsPerRun counts every goroutine's allocations and
// refuses to run beside parallel tests.
func TestManifestAllocs(t *testing.T) {
	m := scaled()
	l, digests := recorded(t, m)
	assert.MaxAllocs(t, func() {
		if _, _, err := state.ReadManifest(t.Context(), l); err != nil {
			t.Fatalf("ReadManifest: unexpected error: %v", err)
		}
	}, readManifestAllocs+manifestStateAllocs, "ReadManifest allocates each document's read and decode")
	assert.MaxAllocs(t, func() {
		if _, err := state.WriteManifest(t.Context(), l, m, digests); err != nil {
			t.Fatalf("WriteManifest: unexpected error: %v", err)
		}
	}, writeManifestAllocs+manifestStateAllocs, "WriteManifest allocates each document's encoding and digest")
}

// BenchmarkManifest measures a read of a record of 1,000 files, and a
// write of a manifest equal to it, which a run without changes makes.
// Each case runs once before the measurement.
func BenchmarkManifest(b *testing.B) {
	m := scaled()
	l, digests := recorded(b, m)

	b.Run("ReadManifest/a record of 1,000 files", func(b *testing.B) {
		_, _, err := state.ReadManifest(b.Context(), l)
		assert.NoError(b, err, "the record reads before the measurement")
		c := bench.Start(b).MaxAllocs(readManifestAllocs + manifestStateAllocs)
		defer c.End()
		var got manifest.Manifest
		for c.Loop() {
			got, _, err = state.ReadManifest(b.Context(), l)
		}
		assert.NoError(b, err, "the record reads")
		assert.Length(b, got.Files, files, "with every file")
	})

	b.Run("WriteManifest/a manifest equal to the record", func(b *testing.B) {
		_, err := state.WriteManifest(b.Context(), l, m, digests)
		assert.NoError(b, err, "the record writes before the measurement")
		writes := l.Writes()
		c := bench.Start(b).MaxAllocs(writeManifestAllocs + manifestStateAllocs)
		defer c.End()
		var got state.Digests
		for c.Loop() {
			got, err = state.WriteManifest(b.Context(), l, m, digests)
		}
		assert.NoError(b, err, "the record writes")
		assert.Equal(b, got, digests, "WriteManifest returns the recorded digests")
		assert.Equal(b, l.Writes(), writes, "and writes nothing")
	})
}
