// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
)

// The ledger directory of the manifest's documents, and the extension
// each document's name ends in.
const (
	manifestDir = "manifest"
	documentExt = ".json"
)

// writers is how many documents a commit writes at once: a synced write
// waits on the disk, and 16 writers measured 143 µs a write against
// 745 µs on one.
const writers = 16

// Digests are the SHA-256 digests of a record's documents, keyed by
// bucket: what a commit compares a new document with before it writes
// it.
type Digests map[string][sha256.Size]byte

// ReadManifest reads every document under the ledger's manifest
// directory and joins them, and returns the manifest beside the digest
// of each document it read. A ledger without documents returns the
// empty manifest. A blob of the directory whose name is not a bucket's
// document, such as a staging file a killed writer left, is skipped.
//
// Error modes: a document that does not read, does not decode, or
// records another bucket than its name states, and documents that do
// not join, each name the document and return the empty manifest.
func ReadManifest(ctx context.Context, l ledger.Ledger) (manifest.Manifest, Digests, error) {
	empty := manifest.Manifest{Version: manifest.Version}
	blobs, err := l.List(ctx, manifestDir)
	if err != nil {
		return empty, nil, fmt.Errorf("state: read the manifest: %w", err)
	}
	digests := Digests{}
	var shards []manifest.Shard
	for _, b := range blobs {
		bucket, named := bucketOf(b.Name)
		if !named {
			continue
		}
		s, sum, rerr := readDocument(ctx, l, b.Name, bucket)
		if rerr != nil {
			return empty, nil, fmt.Errorf("state: read the manifest: %w", rerr)
		}
		digests[bucket] = sum
		shards = append(shards, s)
	}
	m, err := manifest.Join(shards)
	if err != nil {
		return empty, nil, fmt.Errorf("state: read the manifest: %w", err)
	}
	return m, digests, nil
}

// readDocument reads and decodes one bucket's document, and returns it
// with the digest of its bytes.
//
// Error modes: a document that does not read, does not decode, or
// records another bucket than its name states, each naming the
// document, the last two wrapping [manifest.ErrUnsupported].
func readDocument(ctx context.Context, l ledger.Ledger, name, bucket string) (
	manifest.Shard, [sha256.Size]byte, error,
) {
	data, err := l.Read(ctx, name)
	if err != nil {
		return manifest.Shard{}, [sha256.Size]byte{}, err
	}
	s, err := manifest.DecodeShard(data)
	if err != nil {
		return manifest.Shard{}, [sha256.Size]byte{}, fmt.Errorf("%s: %w", name, err)
	}
	if s.Bucket != bucket {
		return manifest.Shard{}, [sha256.Size]byte{}, fmt.Errorf("%s records bucket %s: %w",
			name, s.Bucket, manifest.ErrUnsupported)
	}
	return s, sha256.Sum256(data), nil
}

// WriteManifest records a manifest as its documents: it writes every
// document whose digest differs from recorded, up to 16 at once and each
// durably, then removes every document of the directory whose bucket the
// manifest leaves empty, and returns the digest of every document the
// manifest has. A document the ledger contains with the bytes recorded
// states is not written again, so a manifest equal to the recorded one
// writes nothing.
//
// Error modes: a manifest that breaks the format's invariants writes
// nothing and returns the encoding's error. A document that fails to
// write or to remove is joined into the returned error, and the other
// documents still write.
func WriteManifest(
	ctx context.Context, l ledger.Ledger, m manifest.Manifest, recorded Digests,
) (Digests, error) {
	docs, err := documentsOf(m)
	if err != nil {
		return nil, err
	}
	if _, err := docs.write(ctx, l, recorded); err != nil {
		return docs.digests, err
	}
	return docs.digests, nil
}

// documents are a manifest's documents ready to write: each bucket's
// shard, its encoding and the encoding's digest.
type documents struct {
	shards  []manifest.Shard
	bodies  [][]byte
	digests Digests
}

// documentsOf encodes a manifest's documents.
//
// Error modes: the encoding's error for a manifest that breaks the
// format's invariants.
func documentsOf(m manifest.Manifest) (documents, error) {
	shards := manifest.Split(m)
	docs := documents{shards: shards, bodies: make([][]byte, len(shards)), digests: make(Digests, len(shards))}
	for i, s := range shards {
		b, err := manifest.EncodeShard(s)
		if err != nil {
			return documents{}, fmt.Errorf("state: write the manifest: %w", err)
		}
		docs.bodies[i] = b
		docs.digests[s.Bucket] = sha256.Sum256(b)
	}
	return docs, nil
}

// write writes every document whose digest differs from recorded, up to
// 16 at once and each durably, then removes every document of the
// directory whose bucket the documents leave empty. It returns how many
// bytes it wrote.
//
// Error modes: a document that fails to write or to remove is joined
// into the returned error, and the other documents still write.
func (docs documents) write(ctx context.Context, l ledger.Ledger, recorded Digests) (int64, error) {
	var stale []int
	var written int64
	for i, s := range docs.shards {
		if held, kept := recorded[s.Bucket]; !kept || held != docs.digests[s.Bucket] {
			stale = append(stale, i)
			written += int64(len(docs.bodies[i]))
		}
	}
	listed, err := l.List(ctx, manifestDir)
	if err != nil {
		return 0, fmt.Errorf("state: write the manifest: %w", err)
	}
	errs := writeAll(ctx, l, len(stale), func(k int) (string, []byte) {
		i := stale[k]
		return documentName(docs.shards[i].Bucket), docs.bodies[i]
	})
	for _, b := range listed {
		if bucket, named := bucketOf(b.Name); named {
			if _, kept := docs.digests[bucket]; !kept {
				errs = append(errs, l.Remove(ctx, b.Name))
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		return written, fmt.Errorf("state: write the manifest: %w", err)
	}
	return written, nil
}

// writeAll writes n blobs, which blob(k) names and returns, through
// [ledger.Ledger.Write] on up to [writers] goroutines, and returns every
// write's error in the order of k.
func writeAll(ctx context.Context, l ledger.Ledger, n int, blob func(k int) (string, []byte)) []error {
	errs := make([]error, n)
	var next atomic.Int64
	var wg sync.WaitGroup
	for range min(writers, n) {
		wg.Go(func() {
			for {
				k := int(next.Add(1)) - 1
				if k >= n {
					return
				}
				name, b := blob(k)
				errs[k] = l.Write(ctx, name, b)
			}
		})
	}
	wg.Wait()
	return errs
}

// documentName returns the ledger name of one bucket's document:
// manifest/<bucket>.json.
func documentName(bucket string) string { return manifestDir + "/" + bucket + documentExt }

// bucketOf returns the bucket a ledger name is the document of, and
// reports false for a name that is no bucket's document.
func bucketOf(name string) (string, bool) {
	rest, under := strings.CutPrefix(name, manifestDir+"/")
	bucket, document := strings.CutSuffix(rest, documentExt)
	if !under || !document || len(bucket) != 2 {
		return "", false
	}
	for i := range 2 {
		if c := bucket[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", false
		}
	}
	return bucket, true
}
