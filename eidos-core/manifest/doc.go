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
// # Encoding
//
// [Encode] writes JSON indented by two spaces, with a final newline,
// and an empty list as [] wherever a slice is nil, so two equal
// manifests encode to equal bytes. The record contains no clock, no
// command line and no absolute path, so two runs over one tree record
// the same bytes. [Decode] reads what Encode writes and skips a key it
// does not know.
//
// # Invariants
//
// Both directions check one set of invariants:
//
//   - the version is [Version];
//   - the files are sorted by path, with one entry per path;
//   - every path is workspace-relative and slash-separated;
//   - every entry names its plan;
//   - every hash is "sha256:" and 64 lowercase hex digits;
//   - every entry's plugins and sources are sorted, with no repeat.
//
// Encode refuses a manifest that breaks one. Decode returns an error
// wrapping [ErrUnsupported] for it, as it does for another version and
// for bytes that are not JSON.
//
// # Dependency position
//
// core/manifest imports core/diag and the Go stdlib. core/plugin
// imports it for the records a workspace check reads, so it names a
// plugin by its origin, [diag.Origin], which a plugin's ID is an alias
// of.
package manifest
