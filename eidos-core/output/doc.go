// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package output takes rendered files the last step, from values
// to bytes a person reviews and a run can prove it wrote.
//
// [NewContract] composes one brand with one language's comment
// forms. [Contract.Stamp] frames a [plugin.RenderedFile]:
// the marker an ecosystem recognises as generated code, one
// derivation line per emitting plugin and per source the file
// derives from, the body unchanged, and the trailer that closes
// it. [Contract.Verify] reads a stamped file back and checks it
// whole. [Read] parses any brand's frame and checks nothing, which
// is what tells another tool's file from a hand-written one.
//
// # The frame
//
// A stamped file is five parts, every line spelled through the
// language's own comment form:
//
//   - the marker line, naming the brand;
//   - one derivation line per plugin, then per source, sorted;
//   - one blank line, which is the boundary;
//   - the body, byte for byte as the renderer produced it;
//   - the trailer, which is the file's final line.
//
// The trailer's digest covers the body alone, so renaming the
// brand or changing the derivation leaves a file's integrity
// record untouched, and two tools can agree a body is intact
// while agreeing on nothing else. Because the trailer is the
// final line, bytes appended after it fail verification.
//
// The frame records no version, no date and no command line: two
// runs over one workspace produce identical bytes whatever phrasing
// started them. A record that grows a key still reads, because a
// reader skips the keys it does not know.
//
// # The sinks
//
// A [Sink] takes stamped bytes to their destination in two steps,
// stage then commit, so a failed or abandoned plan leaves the
// previous generation of files exactly in place and a dry run is
// a sink that never commits. [NewDisk] writes into a directory
// tree, [NewMem] into memory, and [NewTee] into several at once.
// A sink stages a file through [Sink.Write] and the removal of a
// file through [Sink.Delete].
//
// [Sink.Prepare] reads the destination before anything is written
// and reports, per staged path, a [Change]: the action the commit
// takes and what the path contains now, as a [Found] verdict. The
// verdict decides from the bytes alone, so a fresh clone without
// state gets the same one:
//
//   - [FoundNothing]: no file;
//   - [FoundSame]: the staged bytes, byte for byte;
//   - [FoundIntact]: the brand's frame over a body that hashes to its
//     trailer;
//   - [FoundDrifted]: the brand's frame over a body edited since its
//     stamp;
//   - [FoundForeign]: anything else, a hand-written file, another
//     brand's output, a file whose trailer was deleted or a directory.
//
// A commit writes if changed: identical bytes leave the file and
// its mtime untouched, because build systems key on mtimes.
// Anything else is written to a staging file and renamed over the
// target, so a reader sees the old file or the new one and never
// half of either.
//
// The disk sink writes as one brand. It overwrites an existing file
// only when the file is that brand's intact output, which is the
// check [Contract.Verify] makes. It refuses a hand-written file,
// another brand's output and an output edited since stamping, and
// leaves each as it is, so a file that changed between Prepare and
// Commit is refused at the commit. A sink that implements
// [Overwriter], as the disk, memory and tee sinks do, writes over the
// drifted files, the foreign files or both once [Overwriter.Overwrite]
// allowed their verdicts, and never over a directory. A staged removal
// deletes only the brand's intact output and leaves any other file in
// place. The sink
// resolves every path inside a root opened once, so a symlink
// pointing out of the tree does not escape it: the jail is the
// operating system's, and the path check at staging is only the
// first refusal.
//
// # Failure semantics
//
// Everything here returns errors and nothing panics. [Stamp]
// refuses what would break the frame: a body with a carriage
// return or without a final newline, and an empty or multi-line
// plugin or source name. The text policy is LF, and a formatter
// emitting CRLF is where that violation is fixed.
//
// A sink refuses a path that is invalid, climbs out of the root,
// ends in the reserved staging suffix, or was staged before, refuses
// a path after Prepare, and returns [ErrFinished] for every call
// after Commit or Discard. A commit keeps going past a file that
// fails and joins the errors, so one unwritable path does not
// withhold the rest.
//
// # Dependency position
//
// core/output imports core/plugin, core/internal/pathset,
// core/internal/stagefile and the Go stdlib. It never imports the
// root authoring package or the render pass: a renderer produces
// values, and stamping them is a separate step its consumer takes.
package output
