// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package manifest encodes the record of every file a workspace
// generated: versioned, deterministic, and public.
//
// A [Manifest] names its workspace and lists one [Entry] per generated
// file: the path, the plan that wrote it, the digest of its bytes, the
// plugins whose units assembled it and the declarations it derives
// from. A run records the manifest after its last plan commits, and
// the next run reads it to name the plan of a drifted file and to
// remove the outputs no plan produces any more. Ownership of a file is
// never read from here: the provenance trailer in the file's own bytes
// decides it, so a fresh clone without a manifest gets the same
// verdicts.
//
// # Documents
//
// The record is stored as up to 256 documents, one [Shard] per bucket:
// a file belongs to the bucket [BucketOf] names, the first byte of the
// SHA-256 of its path. [Split] returns a manifest's documents and
// [Join] returns the manifest its documents record, so a commit
// rewrites only the documents whose entries changed, and a reader
// joins every document of the directory.
//
// # Encoding
//
// [EncodeShard] writes JSON indented by two spaces, with a final
// newline, and an empty list as [] wherever a slice is nil, so two
// equal documents encode to equal bytes. A document contains no clock,
// no command line and no absolute path, so two runs over one tree
// record the same bytes. [DecodeShard] reads what EncodeShard writes
// and skips a key it does not know.
//
// # Invariants
//
// Encoding, decoding and joining check one set of invariants:
//
//   - the version is [Version];
//   - the bucket is two lowercase hex digits, and every entry's path
//     belongs to it;
//   - the files are sorted by path, with one entry per path;
//   - every path is workspace-relative and slash-separated;
//   - every entry names its plan;
//   - every hash is "sha256:" and 64 lowercase hex digits;
//   - every entry's plugins and sources are sorted, with no repeat.
//
// EncodeShard refuses a document that breaks one. DecodeShard returns an
// error wrapping [ErrUnsupported] for it, as it does for another
// version and for bytes that are not JSON, and so does Join, which also
// refuses two documents of one bucket and documents of two workspaces.
//
// # Dependency position
//
// core/manifest imports core/diag and the Go stdlib. core/plugin
// imports it for the records a workspace check reads, so it names a
// plugin by its origin, [diag.Origin], which a plugin's ID is an alias
// of.
package manifest
