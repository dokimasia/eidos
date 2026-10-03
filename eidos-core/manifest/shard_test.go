// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package manifest_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/plugin"
)

// stubBucket pins the bucket of stubPath: the SHA-256 of
// "svc/store_stub.go" begins with the byte 0xea.
const stubBucket = "ea"

// inBucket returns the first n paths of the form svc/fNNNN.go that
// belong to a bucket, in path order.
func inBucket(t *testing.T, bucket string, n int) []string {
	t.Helper()

	var out []string
	for i := 0; len(out) < n; i++ {
		if i == 100_000 {
			t.Fatalf("no %d paths of bucket %s among the first 100,000", n, bucket)
		}
		if p := fmt.Sprintf("svc/f%04d.go", i); manifest.BucketOf(p) == bucket {
			out = append(out, p)
		}
	}
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
			total := 0
			for i, s := range shards {
				assert.Equal(t, s.Version, manifest.Version, "each document states the version")
				assert.Equal(t, s.Workspace, workspaceName, "and the workspace")
				assert.True(t, i == 0 || s.Bucket > shards[i-1].Bucket, "the documents sort by bucket")
				for j, e := range s.Files {
					assert.Equal(t, manifest.BucketOf(e.Path), s.Bucket, "each entry belongs to its document")
					assert.True(t, j == 0 || e.Path > s.Files[j-1].Path, "and the entries sort by path")
				}
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
			assert.True(t, got.Equal(m), "the record they split from")
		})

		t.Run("returns the empty manifest for no documents", func(t *testing.T) {
			t.Parallel()

			got, err := manifest.Join(nil)
			assert.NoError(t, err, "no documents join")
			assert.True(t, got.Equal(manifest.Manifest{Version: manifest.Version}), "to the empty manifest")
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

		t.Run("returns equal bytes for a nil list and an empty one", func(t *testing.T) {
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
			assert.True(t, joined.Equal(record(want.Files...)), "the document reads back")
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

// The bucket a path belongs to is computed on every split and every
// check, and costs no allocation for the paths of a canonical tree. The
// check runs alone, because AllocsPerRun refuses to run beside
// parallel tests.
func TestShardZeroAlloc(t *testing.T) {
	assert.MaxAllocs(t, func() {
		if manifest.BucketOf(stubPath) != stubBucket {
			t.Fatal("BucketOf names another bucket")
		}
	}, 0, "BucketOf allocates nothing")
}

// BenchmarkShard measures the documents of the canonical record of
// 10,000 generated files: naming one path's bucket, splitting the record,
// joining its documents, and encoding and decoding every document, each
// under its allocation ceiling. BucketOf allocates nothing. Split
// allocates 258 times: the bucket of each entry, one list per bucket and
// the documents. Join allocates the joined list once. Encoding all 256
// documents measures 1,818 allocations, about seven a document, under a
// ceiling of 2,048 for the JSON encoder's pooled state, which a
// collection empties. Decoding them measures 42,325, about four an
// entry, under a ceiling of 44,000.
func BenchmarkShard(b *testing.B) {
	m := scaled()
	shards := manifest.Split(m)
	encoded := make([][]byte, len(shards))
	for i, s := range shards {
		var err error
		if encoded[i], err = manifest.EncodeShard(s); err != nil {
			b.Fatalf("EncodeShard: unexpected error: %v", err)
		}
	}

	b.Run("BucketOf", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = manifest.BucketOf(stubPath)
		}
		if got != stubBucket {
			b.Fatalf("BucketOf returned %s", got)
		}
	})

	b.Run("Split", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(258)
		defer c.End()
		var got []manifest.Shard
		for c.Loop() {
			got = manifest.Split(m)
		}
		if len(got) != len(shards) {
			b.Fatalf("Split returned %d documents", len(got))
		}
	})

	b.Run("Join", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got manifest.Manifest
		var err error
		for c.Loop() {
			got, err = manifest.Join(shards)
		}
		if err != nil || len(got.Files) != benchFiles {
			b.Fatalf("Join returned %d files and %v", len(got.Files), err)
		}
	})

	b.Run("EncodeShard", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(2_048)
		defer c.End()
		var got []byte
		var err error
		for c.Loop() {
			for _, s := range shards {
				got, err = manifest.EncodeShard(s)
			}
		}
		if err != nil || len(got) == 0 {
			b.Fatalf("EncodeShard returned %d bytes and %v", len(got), err)
		}
	})

	b.Run("DecodeShard", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(44_000)
		defer c.End()
		var got manifest.Shard
		var err error
		for c.Loop() {
			for _, e := range encoded {
				got, err = manifest.DecodeShard(e)
			}
		}
		if err != nil || got.Bucket == "" {
			b.Fatalf("DecodeShard returned bucket %q and %v", got.Bucket, err)
		}
	})
}
