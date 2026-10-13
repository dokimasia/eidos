---
rfc: 0047
title: The directive registry and the output stream
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0047: The directive registry and the output stream

## Summary

A documentation tool cannot read the directive schemas of a workspace, because the sealed registry
is a private field of the workspace. A user cannot see the files of a run without writing them,
because the kernel has no sink that writes to a stream. We propose an accessor for the sealed
registry with an enumeration of its schemas, a stream sink in the output package, and the flag
`--stdout` of `run`. We refuse a span type, a byte offset in a position, an accessor for the loaded
packages, an attribution helper for templates and observers on the fact store.

## Motivation

- The directive reference of a release is generated from the registered schemas. A generator of the
  reference composes the binary and has no way to read the schemas that the composition registered.
- A user who wants to read the output of a binary before the binary writes it runs the binary in a
  copy of the repository. `run --dry-run` lists the paths and the digests of the files, and not their
  content.
- A program that composes a workspace can choose its output, and the output package offers a disk, a
  memory and a tee sink. Each of them writes to a directory, to memory or to other sinks, and not to
  an `io.Writer`.
- The parity audit proposes a span type, a byte offset, an accessor for the loaded packages, an
  attribution helper and observers on the fact store, each without a consumer.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/directive` | `Registry.Schemas`, and a second `Seal` that leaves the registry as it is |
| `eidos-core/workspace` | `Workspace.Directives` |
| `eidos-core/output` | `Stream`, `NewStream` and `Stream.Open` |
| `eidos-cli` | the flag `--stdout` of `run` |
| `docs/architecture` | the output document and the CLI document |

### Invariants

1. A sealed registry changes on no call, so a reader of it is safe beside a run.
2. A stream writes the files of each commit in path order, and the files of two commits never mix.
3. `run --stdout` does not write a file into the workspace and does not record a generation.

### The sealed registry

```go
// Schemas returns each registered schema in the order of its canonical
// spelling, the schemas of the kernel and of the targets included. A
// schema shares its lists with the declaration that registered it, so a
// caller does not change them.
func (r *Registry) Schemas() iter.Seq[Schema]

// Directives returns the sealed directive registry of the workspace: the
// schemas of the kernel, of each target and of each plugin. The registry
// refuses every registration, so it changes on no call.
func (w *Workspace) Directives() *directive.Registry
```

`Seal` on a sealed registry returns no error and leaves the registry as it is. The seal writes the
resolved constraints into the registry, so a second seal by the caller of `Directives` would write
into the registry of a running workspace.

### The output stream

```go
// Stream writes the files that its sinks commit to one writer. Each file
// is a header line, // === <path> ===, then the body and a newline.
type Stream struct { /* ... */ }

// NewStream returns a stream over w. prefix comes before the path of each
// file in its header, such as the name of a member of a list and a slash,
// and is empty for the paths as a workspace stages them.
func NewStream(w io.Writer, prefix string) *Stream

// Open returns a new sink that commits into the stream. It is safe for
// concurrent use, so a composition passes it to Builder.Output.
func (s *Stream) Open() (Sink, error)
```

A sink of a stream stages as every sink does. Its destination starts empty, so `Prepare` reports each
write as the creation of an absent file, and each removal as a removal that finds nothing. `Commit`
writes each staged file in path order under the lock of the stream, and returns the record of each.
It does not remove a file. A write that fails joins the error of `Commit`, and `Commit` continues
with the next file.

A run commits its plans one after another in commit order, so the output of a stream follows the
commit order of the plans and the path order inside each plan.

### `run --stdout`

`run --stdout` builds each workspace with a stream over standard output in place of the disk, and
without a ledger:

- The run writes each file of each plan that commits to standard output, and nothing into the
  workspace. A plan that fails commits nothing, as a plan that writes to the disk does.
- The run does not take the lock, read the sealed state or record a generation, so it runs cold.
- The header of a file of a member of a list starts with the name of the member, as a position of
  the member does in text output.
- Text output writes the findings and the errors to standard error as before, and leaves out the
  lines of the files and the summary line, so standard output contains the files alone.
- `--overwrite-drift` and `--adopt` do not change the run, because the stream has no file to write
  over.

```sh
acme run --stdout svc/... | less
```

`--stdout` with `--format=json`, `--dry-run` or `--check` is a usage error. JSON output writes its
events to standard output, and a dry run and a check commit nothing to write.

### Verdicts out

| Row | Capability | Reason | Reverses the verdict |
|---|---|---|---|
| K10 | A span type `position.Range` | A frontend records where a declaration starts, and a finding has one position and its related positions. No consumer reads a span | An editor integration that marks the span of a finding |
| K11 | A byte offset in `position.Pos` | A person and an editor find a position by its line and its column. An offset would grow every node of the model and the sealed state, and no consumer reads a source by offset | A consumer that rewrites the bytes of a source at a position |
| K13 | An accessor for the loaded packages | The report of a run lists its files and its plans, and the patterns that a run binds are the input of the caller | A consumer that needs the packages that a pattern selects |
| K19 | An attribution helper for one value in templates | The header of a file names the plugins of the file, and `explain` names the invocation behind each declaration | A consumer whose review needs the plugin of one value inside the file |
| K20 | Observers on the fact store | A handler reads facts through tracked reads in canonical order. An observer would run outside that order and outside the record of the reads | A capability that needs a reaction to a write that canonical reads cannot express |

### Failures

| Condition | Report |
|---|---|
| A write to the writer of a stream that fails | `Commit` joins the error and continues with the next file |
| `--stdout` with `--format=json`, `--dry-run` or `--check` | `StatusUsage` and a usage error that names both flags |

### Migration

1. Add `Registry.Schemas`, let a second `Seal` leave the registry as it is, and add
   `Workspace.Directives`.
2. Add the stream sink.
3. Add `--stdout` to `run`.
4. State the sink and the flag in the output document and the CLI document.

## Alternatives

### A copy of the schemas in the description

`Description` could list a copy of each schema. A documentation tool also resolves spellings and
candidates, which the registry answers, and a copy of each schema on each call of `Describe` serves
no command of the kernel.

### A preview in `run --dry-run`

`run --dry-run` could print the content of each file that it would change. The dry run discards each
staging after `Prepare`, and the report would keep every file of every plan in memory until it
renders.

### A header in the comment form of the target

A header in the comment form of the language of each file would need the language of the file, which
a sink does not receive. One header for every file also lets a reader split the output with one
pattern.

## Drawbacks

- The workspace exposes its registry, so the methods of the registry become part of the API that a
  consumer reads.
- `run --stdout` runs cold, so a large workspace renders every file.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan rows W5.K8, W5.K9, W5.K10, W5.K11, W5.K13, W5.K19 and W5.K20.
- The output contract and its sinks: RFC-0010.
- The command kernels: RFC-0021.
