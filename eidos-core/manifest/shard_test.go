// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package manifest_test

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/plugin"
)

// stubBucket pins the bucket of stubPath: the SHA-256 of
// "svc/store_stub.go" begins with the byte 0xea.
const stubBucket = "ea"

// benchShards is the number of documents the canonical record of
// benchFiles entries splits into: one per byte, every one filled.
const benchShards = 256

// The allocations of the canonical record's documents, which
// TestShardAllocs checks in the ordinary run and BenchmarkShard in a
// benchmark run.
const (
	// splitAllocs is one split of the canonical record: the list of each
	// entry's bucket, one list of entries per bucket, and the documents.
	splitAllocs = 1 + benchShards + 1
	// joinAllocs is one join of the documents: the joined list of
	// entries.
	joinAllocs = 1
	// encodeAllocs is one encoding of every document. Each document
	// allocates six times: the document handed to the encoder, the copied
	// list of entries, the options SetEscapeHTML joins, and the buffer
	// with its two growths. That is 1,536 with the collector off. A
	// collection empties the JSON encoder's pools, and the calls after it
	// allocate their values again: ten fresh processes with the default
	// collector counted up to 56 more. The ceiling allows 64 more.
	encodeAllocs = 6*benchShards + 64
	// decodeAllocs is one decoding of every document. Each entry, with
	// its two plugins and one source, allocates four times: its path and
	// the growths of its lists of plugins and sources. The documents
	// allocate 2,320 times, about nine each: the document, its bucket and
	// the growths of its list of entries. That is 42,320 with the
	// collector off. A collection empties the JSON decoder's pooled state,
	// and the next call allocates it again: ten fresh processes with the
	// default collector counted up to 15 more. The ceiling allows 16
	// more.
	decodeAllocs = 4*benchFiles + 2_320 + 16
)

// The record's documents are versioned public API: each is one bucket of
// the record, its bytes are canonical, it reads back what it wrote, and
// the documents join back into the record they split from.
func TestShard(t *testing.T) {
	t.Parallel()

	t.Run("BucketOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the first byte of the path's SHA-256 as two lowercase hex digits", func(t *testing.T) {
			t.Parallel()

			for _, p := range []string{stubPath, "a.go", "deeply/nested/dir/with/a/long/name_stub_test.go", ""} {
				sum := sha256.Sum256([]byte(p))
				assert.Equal(t, manifest.BucketOf(p), hex.EncodeToString(sum[:1]), "the digest's first byte")
			}
			assert.Equal(t, manifest.BucketOf(stubPath), stubBucket, "the pinned bucket")
		})
	})

	t.Run("Split", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one document per filled bucket in bucket order", func(t *testing.T) {
			t.Parallel()

			m := scaled()
			shards := manifest.Split(m)
			assert.Pairwise(t, shards, func(earlier, later manifest.Shard) bool {
				return earlier.Bucket < later.Bucket
			}, "the documents sort by bucket")
			total := 0
			for _, s := range shards {
				expect.Equal(t, s.Version, manifest.Version, "each document states the version")
				expect.Equal(t, s.Workspace, workspaceName, "and the workspace")
				for _, e := range s.Files {
					expect.Equal(t, manifest.BucketOf(e.Path), s.Bucket, "each entry belongs to its document")
				}
				expect.Pairwise(t, s.Files, func(earlier, later manifest.Entry) bool {
					return earlier.Path < later.Path
				}, "and the entries sort by path")
				total += len(s.Files)
			}
			assert.Equal(t, total, benchFiles, "every entry is in one document")
		})

		t.Run("returns no document for an empty manifest", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, manifest.Split(record()), "no bucket is filled")
		})
	})

	t.Run("Join", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the manifest the documents split from", func(t *testing.T) {
			t.Parallel()

			m := scaled()
			got, err := manifest.Join(manifest.Split(m))
			assert.NoError(t, err, "the documents join")
			assert.Equal(t, got, m, "the record they split from", assert.EquateEmpty())
		})

		t.Run("returns the empty manifest for no documents", func(t *testing.T) {
			t.Parallel()

			got, err := manifest.Join(nil)
			assert.NoError(t, err, "no documents join")
			assert.Equal(t, got, manifest.Manifest{Version: manifest.Version}, "to the empty manifest",
				assert.EquateEmpty())
		})

		t.Run("returns ErrUnsupported for two documents of one bucket", func(t *testing.T) {
			t.Parallel()

			paths := inBucket(t, stubBucket, 2)
			_, err := manifest.Join([]manifest.Shard{shard(entry(paths[0])), shard(entry(paths[1]))})
			assert.ErrorIs(t, err, manifest.ErrUnsupported, "the bucket is recorded twice")
		})

		t.Run("returns ErrUnsupported for documents of two workspaces", func(t *testing.T) {
			t.Parallel()

			shards := manifest.Split(record(entry(inBucket(t, "00", 1)[0]), entry(stubPath)))
			shards[1].Workspace = "other"
			_, err := manifest.Join(shards)
			assert.ErrorIs(t, err, manifest.ErrUnsupported, "two workspaces are refused")
		})

		t.Run("returns ErrUnsupported for a document that breaks an invariant", func(t *testing.T) {
			t.Parallel()

			broken := shard(entry(stubPath))
			broken.Version = 1
			_, err := manifest.Join([]manifest.Shard{broken})
			assert.ErrorIs(t, err, manifest.ErrUnsupported, "the document is refused")
		})
	})

	t.Run("EncodeShard", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the document as JSON indented by two spaces with a final newline", func(t *testing.T) {
			t.Parallel()

			got, err := manifest.EncodeShard(shard(entry(stubPath)))
			assert.NoError(t, err, "the document encodes")
			assert.Equal(t, string(got), `{
  "version": 2,
  "workspace": "platform",
  "bucket": "ea",
  "files": [
    {
      "path": "svc/store_stub.go",
      "plan": "go-services",
      "hash": "`+hash("ab")+`",
      "plugins": [
        "acme-audit",
        "stubgen"
      ],
      "sources": [
        "golang:svc/store.Store"
      ]
    }
  ]
}
`, "the canonical bytes")
		})

		t.Run("returns an empty list for every nil list", func(t *testing.T) {
			t.Parallel()

			bare := entry(stubPath)
			bare.Plugins, bare.Sources = nil, nil
			got, err := manifest.EncodeShard(shard(bare))
			assert.NoError(t, err, "the document encodes")
			assert.Contains(t, string(got), `"plugins": [],`, "the plugins are an empty list")
			assert.Contains(t, string(got), `"sources": []`, "the sources are an empty list")

			empty, err := manifest.EncodeShard(shard())
			assert.NoError(t, err, "the empty document encodes")
			assert.Contains(t, string(empty), `"files": []`, "the files are an empty list")
		})

		t.Run("returns the bytes of an empty list for a nil list", func(t *testing.T) {
			t.Parallel()

			nilFiles, err := manifest.EncodeShard(shard())
			assert.NoError(t, err, "the nil list encodes")
			emptyFiles, err := manifest.EncodeShard(shard([]manifest.Entry{}...))
			assert.NoError(t, err, "the empty list encodes")
			assert.Equal(t, nilFiles, emptyFiles, "one document, one encoding")
		})

		t.Run("returns a path without escaping its HTML characters", func(t *testing.T) {
			t.Parallel()

			p := inBucket(t, stubBucket, 1)[0]
			s := shard(entry(p))
			s.Files[0].Sources = []string{"golang:svc/a&b.Store"}
			got, err := manifest.EncodeShard(s)
			assert.NoError(t, err, "the document encodes")
			assert.Contains(t, string(got), `"golang:svc/a&b.Store"`, "the source is as written")
		})

		t.Run("leaves the caller's nil lists nil", func(t *testing.T) {
			t.Parallel()

			s := shard(changed(func(e *manifest.Entry) { e.Plugins = nil }))
			_, err := manifest.EncodeShard(s)
			assert.NoError(t, err, "the document encodes")
			assert.Nil(t, s.Files[0].Plugins, "the caller's entry is unchanged")
		})

		pair := inBucket(t, stubBucket, 2)
		outside := inBucket(t, "00", 1)[0]
		tests := []struct {
			name string
			give manifest.Shard
		}{
			{name: "returns an error for a document of another version", give: versioned(1)},
			{name: "returns an error for a document without a version", give: versioned(0)},
			{name: "returns an error for a bucket in uppercase digits", give: bucketed("EA")},
			{name: "returns an error for a bucket with a digit outside hex", give: bucketed("eg")},
			{name: "returns an error for a bucket of one digit", give: bucketed("e")},
			{name: "returns an error for an entry outside the document's bucket", give: shard(entry(outside))},
			{name: "returns an error for files out of path order", give: shard(entry(pair[1]), entry(pair[0]))},
			{name: "returns an error for a path listed twice", give: shard(entry(stubPath), entry(stubPath))},
			{name: "returns an error for an empty path", give: shard(entry(""))},
			{name: "returns an error for an absolute path", give: shard(entry("/svc/a.go"))},
			{name: "returns an error for a path that climbs out of the root", give: shard(entry("../a.go"))},
			{name: "returns an error for the root's own path", give: shard(entry("."))},
			{name: "returns an error for a path with a backslash", give: shard(entry(`svc\a.go`))},
			{
				name: "returns an error for an entry without a plan",
				give: shard(changed(func(e *manifest.Entry) { e.Plan = "" })),
			},
			{
				name: "returns an error for a hash without its digest's name",
				give: shard(changed(func(e *manifest.Entry) { e.Hash = strings.Repeat("ab", 32) })),
			},
			{
				name: "returns an error for a hash of the wrong width",
				give: shard(changed(func(e *manifest.Entry) { e.Hash = "sha256:abab" })),
			},
			{
				name: "returns an error for a hash in uppercase digits",
				give: shard(changed(func(e *manifest.Entry) { e.Hash = hash("AB") })),
			},
			{
				name: "returns an error for a hash with a digit outside hex",
				give: shard(changed(func(e *manifest.Entry) { e.Hash = hash("zz") })),
			},
			{
				name: "returns an error for plugins out of order",
				give: shard(changed(func(e *manifest.Entry) { e.Plugins = []plugin.ID{stubgen, audit} })),
			},
			{
				name: "returns an error for a plugin listed twice",
				give: shard(changed(func(e *manifest.Entry) { e.Plugins = []plugin.ID{stubgen, stubgen} })),
			},
			{
				name: "returns an error for sources out of order",
				give: shard(changed(func(e *manifest.Entry) {
					e.Sources = []string{"golang:svc/store.Z", "golang:svc/store.A"}
				})),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := manifest.EncodeShard(tt.give)
				assert.HasError(t, err, "the document is refused")
				assert.Contains(t, err.Error(), "manifest: encode:", "the error names the package and the step")
			})
		}
	})

	t.Run("DecodeShard", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the document EncodeShard wrote", func(t *testing.T) {
			t.Parallel()

			p := inBucket(t, stubBucket, 2)
			want := shard(entry(p[0]), entry(p[1]))
			encoded, err := manifest.EncodeShard(want)
			assert.NoError(t, err, "the document encodes")
			got, err := manifest.DecodeShard(encoded)
			assert.NoError(t, err, "the bytes decode")
			joined, err := manifest.Join([]manifest.Shard{got})
			assert.NoError(t, err, "the document joins")
			assert.Equal(t, joined, record(want.Files...), "the document reads back", assert.EquateEmpty())
		})

		t.Run("returns the document without a key the format does not know", func(t *testing.T) {
			t.Parallel()

			got, err := manifest.DecodeShard([]byte(
				`{"version":2,"workspace":"platform","bucket":"ea","files":[],"cache":{}}`))
			assert.NoError(t, err, "an unknown key is skipped")
			assert.Equal(t, got.Bucket, stubBucket, "the document is the known keys")
		})

		tests := []struct {
			name string
			give string
		}{
			{
				name: "returns ErrUnsupported for a document of another version",
				give: `{"version":1,"bucket":"ea","files":[]}`,
			},
			{name: "returns ErrUnsupported for bytes that are not JSON", give: `version: 2`},
			{name: "returns ErrUnsupported for a JSON list", give: `[]`},
			{
				name: "returns ErrUnsupported for an entry outside the document's bucket",
				give: `{"version":2,"bucket":"00","files":[` +
					`{"path":"` + stubPath + `","plan":"p","hash":"` + hash("ab") + `"}]}`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := manifest.DecodeShard([]byte(tt.give))
				assert.ErrorIs(t, err, manifest.ErrUnsupported, "the document is not read")
			})
		}
	})
}

// The bucket a path belongs to costs no allocation, and the split, the
// join and the codec of the canonical record's documents allocate within
// their ceilings, in the ordinary run, which runs no benchmark. Each
// count of the codec keeps the first error of its calls, which cmp.Or
// returns without allocating. The check runs alone, because the count
// includes every goroutine's allocations.
func TestShardAllocs(t *testing.T) {
	var bucket string
	assert.MaxAllocs(t, func() { bucket = manifest.BucketOf(stubPath) }, 0, "BucketOf allocates nothing")
	assert.Equal(t, bucket, stubBucket, "BucketOf names the path's bucket")

	m := scaled()
	var shards []manifest.Shard
	assert.MaxAllocs(t, func() { shards = manifest.Split(m) }, splitAllocs,
		"Split allocates the buckets, one list per bucket and the documents")
	assert.Length(t, shards, benchShards, "Split fills every bucket")

	var joined manifest.Manifest
	var err error
	assert.MaxAllocs(t, func() { joined, err = manifest.Join(shards) }, joinAllocs,
		"Join allocates the joined list")
	assert.NoError(t, err, "the documents join")
	assert.Length(t, joined.Files, benchFiles, "Join returns every entry")

	encoded := make([][]byte, len(shards))
	assert.MaxAllocs(t, func() {
		for i, s := range shards {
			var eerr error
			encoded[i], eerr = manifest.EncodeShard(s)
			err = cmp.Or(err, eerr)
		}
	}, encodeAllocs, "EncodeShard allocates six times per document")
	assert.NoError(t, err, "every document encodes")

	var decoded manifest.Shard
	assert.MaxAllocs(t, func() {
		for _, e := range encoded {
			var derr error
			decoded, derr = manifest.DecodeShard(e)
			err = cmp.Or(err, derr)
		}
	}, decodeAllocs, "DecodeShard allocates four times per entry")
	assert.NoError(t, err, "every document decodes")
	assert.Equal(t, decoded.Bucket, shards[len(shards)-1].Bucket, "the last document reads back")
}

// BenchmarkShard measures the documents of the canonical record of
// 10,000 generated files: naming one path's bucket, splitting the record,
// joining its documents, and encoding and decoding every document, each
// under its allocation ceiling. The encode and the decode run one
// iteration before the measurement, so the JSON codec's pooled state is
// in place in the sub-benchmark's own goroutine.
func BenchmarkShard(b *testing.B) {
	m := scaled()
	shards := manifest.Split(m)
	encoded := make([][]byte, len(shards))
	for i, s := range shards {
		var err error
		encoded[i], err = manifest.EncodeShard(s)
		assert.NoError(b, err, "every document encodes")
	}

	b.Run("BucketOf", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = manifest.BucketOf(stubPath)
		}
		assert.Equal(b, got, stubBucket, "BucketOf names the path's bucket")
	})

	b.Run("Split", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(splitAllocs)
		defer c.End()
		var got []manifest.Shard
		for c.Loop() {
			got = manifest.Split(m)
		}
		assert.Length(b, got, len(shards), "Split fills every bucket")
	})

	b.Run("Join", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(joinAllocs)
		defer c.End()
		var got manifest.Manifest
		var err error
		for c.Loop() {
			got, err = manifest.Join(shards)
		}
		assert.NoError(b, err, "the documents join")
		assert.Length(b, got.Files, benchFiles, "Join returns every entry")
	})

	b.Run("EncodeShard", func(b *testing.B) {
		var got []byte
		var err error
		c := bench.Start(b).Warmup(1).MaxAllocs(encodeAllocs)
		defer c.End()
		for c.Loop() {
			for _, s := range shards {
				got, err = manifest.EncodeShard(s)
			}
		}
		assert.NoError(b, err, "every document encodes")
		assert.NotEmpty(b, got, "into its bytes")
	})

	b.Run("DecodeShard", func(b *testing.B) {
		var got manifest.Shard
		var err error
		c := bench.Start(b).Warmup(1).MaxAllocs(decodeAllocs)
		defer c.End()
		for c.Loop() {
			for _, e := range encoded {
				got, err = manifest.DecodeShard(e)
			}
		}
		assert.NoError(b, err, "every document decodes")
		assert.Equal(b, got.Bucket, shards[len(shards)-1].Bucket, "the last document reads back")
	})
}

// inBucket returns the first n paths of the form svc/fNNNN.go that
// belong to a bucket, in path order, among the first 100,000.
func inBucket(t *testing.T, bucket string, n int) []string {
	t.Helper()

	var out []string
	for i := range 100_000 {
		if len(out) == n {
			break
		}
		if p := fmt.Sprintf("svc/f%04d.go", i); manifest.BucketOf(p) == bucket {
			out = append(out, p)
		}
	}
	assert.Length(t, out, n, "the first 100,000 paths contain enough of bucket "+bucket)
	return out
}

// shard returns the well-formed document of the given entries, which
// share stubBucket.
func shard(files ...manifest.Entry) manifest.Shard {
	return manifest.Shard{Version: manifest.Version, Workspace: workspaceName, Bucket: stubBucket, Files: files}
}

// changed returns stubPath's entry with one change applied.
func changed(change func(*manifest.Entry)) manifest.Entry {
	e := entry(stubPath)
	change(&e)
	return e
}

// versioned returns the empty document stating a version.
func versioned(v int) manifest.Shard {
	s := shard()
	s.Version = v
	return s
}

// bucketed returns the empty document naming a bucket.
func bucketed(bucket string) manifest.Shard {
	s := shard()
	s.Bucket = bucket
	return s
}
