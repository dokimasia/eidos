// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package ledger stores a workspace's record as named blobs in the
// brand's state directory.
//
// A [Ledger] stores bytes under slash-separated names. The kernel
// encodes every blob it stores: the manifest's documents, the
// generations of the sealed state and their segments, and the parse
// memo's entries. [OpenDir] returns the ledger of a workspace root's
// state directory, [OpenAt] the ledger of a directory itself, and
// [NewMem] one that stores in memory.
//
// # The state directory
//
// The state directory belongs to the brand: .<brand>/ under the
// workspace root, so two tools built on the kernel never read each
// other's records. A name such as manifest/ab.json is the file
// .<brand>/manifest/ab.json, and [ManifestPath] is the directory of the
// manifest's documents. The first write creates the directory.
//
// # The lock
//
// A [Locker] admits one holder at a time. [Dir.Lock] takes the operating
// system's lock on the directory's lock file, which ends with the
// holder's process, and [Mem.Lock] takes a lock within the process. The
// holder records itself as a [Holder] in lock.json beside the lock file
// until its release, and a contender that finds the lock taken returns a
// [LockedError] that names the holder. [LockPath] is the lock file of a
// brand's state directory.
//
// # Durability
//
// [Ledger.Write] replaces a blob atomically and durably: the bytes go
// to a staging file of their own, which is synced and renamed over the
// name, and the directory is synced after the rename, so a reader sees
// the old blob or the new one and a crash of the machine keeps the new
// one once Write returns. [Ledger.Put] replaces a blob atomically
// without a sync, for entries that check their own bytes. A removal is
// not synced.
//
// # Failure semantics
//
// A name nothing wrote reads as an error wrapping [fs.ErrNotExist], and
// a name that is not slash-separated and relative, or that has an
// empty, "." or ".." element, is an error wrapping [fs.ErrInvalid].
// Every path of a [Dir] resolves inside its directory through an
// [os.Root], so a symlink in the state directory does not lead a read
// or a write outside it. A cancelled context returns its error before
// the call touches a blob.
//
// # Dependency position
//
// core/ledger imports core/output, core/internal/stagefile and the Go
// stdlib.
package ledger
