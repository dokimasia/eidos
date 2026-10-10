// Copyright Dokimasia B.V. 2026
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
// # The record of the phases
//
// A [Recorder] collects what the phases after the load executed: each
// validation, invocation, check and group with the edges it read, and
// each file that a plan generated, through one [Lane] for each goroutine
// that records, and the audit's findings. A lane leaves out a pure
// invocation, which read nothing and touched and reported nothing. Such
// an invocation produced nothing, and a change to its subject is the one
// change that can make it produce something. An [EdgeHash] is the first
// eight bytes of the SHA-256 of an edge's spelling, and a [RecordRef]
// names a record by the same hash of its kind and its key fields.
// [RecordPhases] reads the prior record whole before any plan commits,
// and [PhaseRecord.Commit] completes the record into the commit once the
// plans have committed. It sorts the records by ID and the reads by edge,
// and records only the rows that differ from the prior record's. A plan
// that does not commit keeps its prior records and files, and the plans
// table lists it. [Generation.Phases] returns a generation's record,
// which looks up a record by its key and the records that read an edge.
//
// A file that a plan generated is an [Artifact] in the artifacts table,
// under its path and under its path in lower case, which finds the paths
// that one tree cannot contain beside a new path. The names table keeps
// each file-level name that a plan's files declare, under its collision
// scope and under its origin. [PhaseState.Names] reads it as the
// [go.dokimi.dev/eidos/core/plugin.Names] of the files a warm run keeps.
//
// A warm run calls [Recorder.Keep], drops the record of each validation
// and annotator invocation that it executes again or removes, and names
// each subject whose claims it withdrew. The commit then keeps the other
// shared records and bags of claims of the generation. A warm run that
// executes part of a plan calls [Recorder.KeepPlan], and drops each of
// the plan's invocations, groups and files that it executes again, so the
// commit keeps the plan's other records. A record that reported a finding
// lists [FindingsEdge] among its reads, so the readers table lists every
// record whose findings a warm run reports again.
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
// core/meta, core/diag, core/position, core/symbol, core/internal/grow,
// core/internal/wire and the Go stdlib.
package state
