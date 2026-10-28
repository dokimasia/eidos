---
rfc: 0087
title: Data files and line sets
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-28
updated: 2026-10-28
discussion: https://github.com/dokimasia/eidos/issues/98
supersedes: none
superseded-by: none
produces-adr: ADR-0113
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0087: Data files and line sets

## Summary

We propose a family of data files and a comparison of a set of lines under `run --check`.

- `plugin.Output.Name` declares a data family: one file per plan under a fixed name. Its template
  writes the whole file, and the run writes the bytes without a header, a frame or a format step.
- A sink recognises a data file of its brand by the digest that the manifest records for the path,
  because the file has no frame.
- `plugin.Output.LineSet` declares that the file of a data family is a set of lines. Under
  `--check`, the run passes each line that the committed file has and the fresh file lacks to the
  plugin through `plugin.LineSetReader`, and reports the finding that the plugin returns.

## Motivation

dokimi needs a committed list of the checks that a run generated, one line per check. A consumer
that removes a directive or renames a callable loses the checks that it licensed. In the generated
source files the loss is a deleted block among many changed lines: the move of the Go corpus of
dokimi to stub v0.2.0 regenerated 139 files. In a list of checks the same loss is one deleted line,
and `--check` can name it.

Three mechanisms of the run prevent such a file.

| Mechanism | Where | Effect on a data file |
|---|---|---|
| The speller of the target names each file from the word of its family, with the join and the extension of the language | `layout/route.go`, `fileNameOf` | A plugin cannot name a file `dokimi.golang.lock`. A plan can fix the name through `layout.Config`, but the plugin cannot state it |
| The render writes the header of each file, the kind templates and the format step of the language, and the contract stamps the frame | `backend/render`, `workspace/write.go` | A list of checks gets Go comments and a pass of gofmt |
| The disk sink classifies a file without the frame of the brand as foreign | `output/disk.go`, `ownership` | A data file without a frame is foreign on the next run, so the run withholds the update and `--check` compares nothing |

`run --check` reports each file that differs as `OutOfDate` (`workspace/close.go`, `outOfDate`). The
finding names the file and not its lines, so a removed check and an added check give the same
finding, and the plugin cannot report a removed check as its own error.

## Detailed design

### Components

| Component | Responsibility |
|---|---|
| `plugin.Output.Name` | Declares a data family and the name of its one file per plan |
| `plugin.Output.LineSet` | Declares that the file of a data family is a set of lines |
| `plugin.File.Data` | Marks a routed file of a data family for the render, the stamp and the package rule |
| `layout` | Validates a data family, names its file and gives it no package |
| `backend/render` | Writes the template of a data file as the whole file |
| `workspace` | Stages a data file without the frame, declares the recorded digest of its path to the sink, and compares a line set under `--check` |
| `output.Owner` | An optional role of a sink that takes the recorded digest of a data file |
| `output.ErrRecordedDigest` | The refusal of a commit over a data file whose bytes differ from the recorded digest |
| `plugin.LineSetReader` | An optional role of a generator that turns a removed line into a finding |
| `Builder.Removed` | The declaration of the function of `LineSetReader` for a plugin that the SDK builds |

### The data family

```go
// Name is the file name of a data family, such as "dokimi.golang.lock",
// and empty for a family of source files. A data family has the
// cardinality PerPlan and a template. The layout writes its one file
// under the directory of the family with the name as given. The
// template writes the whole file, and the run writes the bytes without
// a header, a frame or a format step.
Name string
```

- **Validation.** The layout refuses a data family whose `Per` is not `PerPlan`, whose `Template` is
  empty, or whose `Name` is not one path element. `Word` remains required, because the messages of
  the layout name a family by its word.
- **Name.** `fileNameOf` takes the file of the configuration first, then `Name`, then the speller. A
  plan can still rename the file through `layout.Config`. The route of the file records the handler
  as the input of a name that the family declares.
- **Directory.** A data family writes under a directory, as every `PerPlan` family does.
- **Package.** The layout gives a data file the zero package and calls no `Packager`, because the
  file declares no package.
- **Render.** The layout sets `plugin.File.Data`. The render executes the template of the family
  over the units of the file and writes its output as the body of the file. It writes no header,
  runs no kind template and runs no `Finalise` of the language.
- **Stamp.** The run stages the body of a data file as rendered, without `Contract.Stamp`. The
  manifest records the file as it records every file: the path, the plan, the digest of the bytes,
  the plugins and the sources.

### Ownership without a frame

A sink proves its ownership of a source file through the frame. A data file has none, so the run
proves it through the manifest of the previous generation.

```go
// Owner is implemented by a sink whose destination has files of its
// own. Own declares that the brand wrote the file at path with the
// digest hash, as the manifest of the previous generation records it.
// Prepare and Commit then classify a file at path without a frame
// through the digest: the same digest is the brand's intact output, and
// any other file without a frame is foreign. Own does not check the
// path, which Write and Delete check, and it cannot fail.
type Owner interface {
    Own(path, hash string)
}
```

- The run calls `Own` for each previous entry of a plan before it stages the files of the plan, and
  for each entry that the sweep removes. The frame decides the classification of a file with a
  frame, so a declared digest changes nothing for a source file.
- A file without a frame at another digest is foreign, not drifted. The digest alone cannot tell an
  edited data file from a file of a person at the path of a stamped output, and the run keeps a
  foreign stale file and leaves its entry, as it does for a source file that lost its frame.
- A path that the manifest does not record is foreign, so a first run does not write over a file of
  the same name that a person wrote.
- A commit refuses to write over a data file at another digest with an error that wraps
  `output.ErrRecordedDigest`, unless the sink writes over foreign files.
- `Disk` implements `Owner`. `Mem` has no files of its own and does not implement it.
- `Owner` is an optional role, so the implementations of `Sink` outside the kernel compile
  unchanged.

### The line set

```go
// LineSet reports that the file of a data family is a set of lines,
// which run --check compares line by line. A line-set family is a data
// family.
LineSet bool
```

```go
// RemovedLine is one line that the committed file of a line-set family
// has and the fresh file lacks.
type RemovedLine struct {
    Family string // the tag of the family
    Path   string // the workspace-relative path of the file
    Line   int    // the line in the committed file, from 1
    Text   string // the line without its line ending
}

// LineSetReader is implemented by a generator with a line-set family.
// Removed returns the finding that run --check reports for a removed
// line, and false for a line whose removal is no finding. The run sets
// the position of the finding to the line, and its origin to the
// generator, where the finding leaves them zero.
type LineSetReader interface {
    Removed(r RemovedLine) (diag.Diag, bool)
}
```

- **Comparison.** Under `--check`, for each line-set file, the run reads the committed file from the
  tree of the run. It splits both files at `\n`, drops the `\r` of a `\r\n` line ending, and starts no
  empty line after a final line ending. Each line of the committed file that the fresh file does not
  contain is a removed line, in the order of the committed file. The order and the repetition of
  lines do not matter.
- **Findings.** The run passes each removed line to `Removed` of the generator that declares the
  family, and reports each finding that it returns. A generator that does not implement `LineSetReader`
  reports nothing for a removed line. A plugin that the SDK builds implements the role, and declares
  its function through `Builder.Removed`. Without a declaration it reports nothing.
- **The stale file.** The run keeps `OutOfDate` for every line-set file that differs, also where the
  fresh file only removes lines. A file that differs is out of date, and the consumer commits the
  fresh file in both cases.
- **Cost.** The comparison builds a set of the fresh lines and scans the committed lines once:
  O(n + m) time and one string key per fresh line. It runs only under `--check`, and only for a
  line-set file.

### Failure handling

| Failure | Behaviour |
|---|---|
| A data family that is not `PerPlan`, has no template or has a name that is not one path element | The composition fails to build with the error of the layout |
| A file at the path of a data file that the manifest does not record | The sink classifies it as foreign. The run withholds the write unless the consumer allows foreign files |
| A data file that a person edited since the last run | The sink classifies it as foreign, and the commit refuses it under `output.ErrRecordedDigest` |
| A committed line-set file that the tree of the run does not contain | No line is removed, and `OutOfDate` reports the file |

### What does not change

- The frame, the stamp and the classification of every source file.
- The routing of every family without a name.
- `run` without `--check`. It writes the fresh line-set file as it writes every file.

### Verdicts

- `Name` keeps `Word` required. A data family without a word would need a second spelling in every
  message of the layout.
- Ownership through the manifest, not through a frame in a comment. A list of checks that a tool
  other than eidos reads has no comment syntax of its own.
- The run reports `OutOfDate` also for a file that only lost lines. The issue asks to drop it
  there. A file that differs from the run is out of date whatever the plugin says about its lines,
  and `--check` would otherwise pass over a stale file whose removals the plugin accepts.
- `RemovedLine` in place of the family and the text alone. The plugin needs the path and the line to
  place a finding that a reviewer can open.
- `Owner` and `LineSetReader` are optional roles. `plugin.Generator` and `output.Sink` keep their
  methods.

## Alternatives considered

### The file name through the configuration of the plan

`layout.Config` can already fix the file of a family. The plugin cannot state the name, so every
composition of dokimi would repeat it, and the header, the frame and the format step would remain.

### A frame in a data file

The contract could stamp a data file with `#` lines. A list of checks then carries the comment
syntax of one consumer, and a reader of the list must strip the frame.

### Line findings in the kernel

The run could report each removed line under a kernel code. The plugin knows the meaning of a line,
such as the check and the directive that licensed it. Only the plugin can decide that a removal
needs no approval.

## Drawbacks

- A data file has no frame, so a copy of the file outside the workspace does not carry the proof of
  its origin. The manifest is the only record.
- A first run over a workspace that already contains a file at the path of a data file withholds the
  write until the consumer allows foreign files.

## Open questions

None.

## References

| What | Where |
|---|---|
| Issue 98, a generator cannot write a data file under a name of its own | <https://github.com/dokimasia/eidos/issues/98> |
| Issue 99, `--check` reports a changed line-set file as a stale file | <https://github.com/dokimasia/eidos/issues/99> |
| Routing and layout | ../architecture/18-routing-and-layout.md |
| The workspace and the plans | ../architecture/08-workspace-and-plans.md |
