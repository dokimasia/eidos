// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package state encodes the kernel's record in a [ledger.Ledger]: the
// sealed state a warm run reads, and the manifest's documents.
//
// # The sealed state
//
// A [Generation] is one blob named by the SHA-256 of its bytes: a
// [Header], the digest of each manifest document, the segments it
// references, and for each [Table] the table's runs. A run is a sorted
// list of rows in blocks of about 4 KiB, each followed by its CRC-32C,
// and ends with a sparse index of each block's first key. A lookup
// searches the index and reads one block through
// [ledger.Ledger.ReadAt]. A row deleted in a newer run is a tombstone
// there. A segment is immutable once written, so a generation shares
// every segment its parent wrote that it still references. The ledger
// names the live generation in state/CURRENT, which a [Commit] replaces
// last.
//
// A [Commit] adds the run's regions as one segment, and the rows each
// table changes as one run of each table in a second segment, so its
// bytes follow what the run changed and not the size of the record. It
// merges a table's runs into one where the table would keep more than
// eight runs, or where its newer runs would contain more bytes than its
// oldest. It then writes the generation, the manifest's documents whose
// bytes differ from the ledger's, and CURRENT, and removes every
// generation and segment the new generation does not reference and that
// is older than its anchor. A commit that changes nothing writes
// nothing. A generation of another [Format] does not open, with an error
// wrapping [ErrFormat].
//
// [AppendRegion] and [DecodeRegion] encode one unit's region with a
// string table of its own, so a region decodes without any other blob,
// in a generation or in the parse memo. [Generation.Load] returns the
// record of the load, which is the [go.dokimi.dev/eidos/core/frontend/load.Prior]
// a warm load reads, and [RecordLoad] records the next load in a commit.
//
// # The parse memo
//
// A [Memo] keeps the region of every unit a run parsed under memo/, each
// entry named by the SHA-256 of the unit's key and the executable's
// digest, so an entry serves the build that wrote it. A load restores a
// unit from it, and the run's commit writes what the load parsed, without
// a sync, touches what the load restored, removes what did not decode,
// and keeps the memo under its cap by removing the least recently used
// entries. memo/total states the memo's size, and memo/trimmed when a
// commit last listed the memo whole, which it does at least daily.
//
// # The manifest
//
// [ReadManifest] lists the manifest directory, decodes every document
// and joins them, and returns the digest of each document beside the
// manifest. [WriteManifest] writes only the documents whose digest
// differs from the recorded one, several at once and each durably, and
// removes the documents whose bucket the new manifest leaves empty.
//
// # Failure semantics
//
// A stored record that does not read whole returns an error wrapping
// [ErrDamaged]: a CRC-32C that does not match, a blob that is missing or
// shorter than its record, or bytes that do not decode. A run that meets
// one discards what it derived and runs cold. A manifest document that
// does not read or does not decode fails the whole read, because a
// record missing a bucket would hide that bucket's files from the sweep.
// A write that fails leaves the record the ledger named before it.
//
// # Dependency position
//
// core/internal/state imports core/ledger, core/manifest, core/node,
// core/store, core/frontend/load, core/plugin, core/directive,
// core/meta, core/diag, core/position, core/symbol, core/internal/wire
// and the Go stdlib.
package state
