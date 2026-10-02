// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package ledger reads and writes a run's record in the brand's state
// directory.
//
// A [Ledger] serves one run. [Ledger.BeginRun] returns the manifest
// the last committed run recorded, before the run loads.
// [Ledger.CommitRun] records this run's manifest strictly after every
// plan commit, so the record never lists a file the destination does
// not contain. [OpenDir] returns the ledger of a workspace root's state
// directory. [NewMem] returns one that records in memory.
//
// # The state directory
//
// The state directory belongs to the brand: .<brand>/ under the
// workspace root, so two tools built on the kernel never read each
// other's records. The manifest is [ManifestPath] inside it, and a
// commit leaves no other file there. The first commit creates the
// directory.
//
// # Atomicity
//
// [Dir.CommitRun] writes the manifest to a staging file beside it,
// syncs it, renames it over the manifest and syncs the directory, so a
// reader sees the old record or the new one and never half of either.
// A manifest whose bytes equal the recorded ones writes nothing, so a
// run that changed nothing touches no file of the state directory.
//
// # Failure semantics
//
// BeginRun returns the empty manifest and no error where no run has
// committed. It returns an error where a record exists and does not
// read, and the run then proceeds as if no run had committed. Every
// path resolves inside the workspace root through an [os.Root], so a
// symlink in the state directory does not lead a write outside it.
//
// # Dependency position
//
// core/ledger imports core/manifest, core/output,
// core/internal/stagefile and the Go stdlib.
package ledger
