---
adr: 0097
title: A Go file has the format of gofumpt and golines
status: Accepted
date: 2026-10-21
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0097: A Go file has the format of gofumpt and golines

## Status

Accepted

## Context

The Go backend formatted each rendered file with `format.Source` of go/format, so a generated file was
a fixed point of gofmt. Issue 67 asked the same of TypeScript, and issue 66 asks it of the formatters
that a Go project runs beside gofmt.

- `gofmt -s` removes the type of each element of a composite literal, such as `Line{` inside
  `[]Line{`. The values of the samples have that type.
- gofumpt groups adjacent parameters of one type and writes an empty struct on one line. golines wraps
  each line that passes its width, such as a signature of 169 columns.
- golangci-lint 2.15.0 runs gofmt, gofumpt v0.12.0 and golangci/golines v0.16.0, with golines last.
  It skips a generated file unless its config turns that exclusion off. gofumpt formats a generated
  file that a command passes to it by path.
- gofumpt applies every simplification of `gofmt -s` itself. Its README states that a run of gofmt
  after gofumpt does not change a file. golines shortens only a line that passes its width, and it
  does not join lines.
- gofumpt splits long lines when the environment sets `GOFUMPT_SPLIT_LONG_LINES=on`. A composite
  literal of 150 columns then has another format without a width and at a width of 100 columns.

The reference tree has 30 generated Go files, and 32 with the stub of the interface of issue 66.
golangci-lint changed 7 or 8 of them under each of three configs: the config of the issue, the
baseline of ergon and the defaults of golangci-lint. After gofumpt with its three extra rules and
golines at 100 columns, it changed none of them under any of the three configs. A second pass of the
two formatters did not change a file either.

## Decision

We will format each rendered Go file with gofumpt and then with golines.

- gofumpt v0.12.0 formats the file with its three extra rules: grouped parameters, clothed returns and
  balanced calls. It formats for Go 1.13 and later, because it writes an octal literal with `0o`, and
  it formats without a module path, as golangci-lint does by default.
- golines v0.16.0 then wraps each line of code that passes the width of the plan policy `golang.width`:
  `80`, `100`, `120` or `none`. The default is `100`, the default width of golines and of
  golangci-lint. A tab counts four columns. golines keeps each comment and each struct tag as the
  backend writes it. The backend calls golines only where a line of code passes the width.
- A file that the backend writes at a width is then a fixed point of gofmt, of `gofmt -s`, of gofumpt
  with any of its extra rules and of golines at that width or a wider one.
- The backend refuses to format a file while the environment sets `GOFUMPT_SPLIT_LONG_LINES=on`.
  The render reports the refusal for each file, so the bytes of a file do not depend on the
  environment.
- The backend kit adds `Builder.FinaliseUnder`, a formatter that receives the policy of the plan, as
  the render reads the version of Python from the policy of a plan. A backend declares one formatter,
  through `Finalise` or through `FinaliseUnder`.

## Alternatives Considered

### A finaliser that always receives the policy

A new signature of `Finalise` would change seven backends, a fixture of the CLI and about 25 specs
whose formatters do not read the policy. An additional method changes the kernel and the Go backend
alone.

### gofumpt behind an option

The output of gofumpt with every extra rule is a fixed point of gofmt and of gofumpt with fewer rules,
so a project without gofumpt keeps a clean gate. An option would select between two outputs that both
pass that gate.

### A default width of 120

golines and golangci-lint wrap at 100 columns by default. A file wrapped at 120 columns changes under
that default, and a file wrapped at 100 columns does not change under a width of 120.

### Struct tags in columns

golines pads the keys of the struct tags of a struct into columns by default. The baseline of ergon
turns that off, because the padding moves every tag of a struct when one key changes. A project whose
golines pads its tags changes a generated struct whose tags have more than one key.

### The formatters of the project

Hermeticity refuses a formatter that the machine provides, because each machine would then write
other bytes.

### The split of long lines under the switch of gofumpt

A machine that sets the variable would write other bytes than a machine that does not. A warm run
would also keep the bytes of the run before it, because the fingerprint of a run does not contain
the environment.

## Consequences

**Positive:**

- A generated Go file passes a gate that runs gofmt, gofumpt or golines at a width of 100 columns or
  more.
- The values of the samples lose the types of their elements without a change of the value writer.

**Negative:**

- Every generated Go file with a construct that gofumpt rewrites, or with a line of more than 100
  columns, changes once.
- eidos-lang-go requires gofumpt, golines and dst, the syntax tree of golines.
- The render of Go files costs more. The render of a corpus of 200,000 declarations took 25% more
  time and 15% more allocations, because gofumpt applies its rules to the syntax tree of each file.
  No line of that corpus passes 100 columns. golines allocates about 2,100 times more for a file with
  a long line.
- A project whose golines wraps at more than 120 columns gets generated lines that are shorter than
  its own.
- A run whose environment sets `GOFUMPT_SPLIT_LONG_LINES=on` withholds every Go file.

## References

| What | Where |
|---|---|
| D215, the row of this decision | ../architecture/21-decisions.md |
| The Go satellite | ../architecture/11-languages.md |
| The render step that formats a file | ../architecture/07-rendering.md |
| Issue 66, a run fails after gofmt -s, gofumpt or golines formats the generated files | <https://github.com/dokimasia/eidos/issues/66> |
| gofumpt | <https://github.com/mvdan/gofumpt> |
| The fork of golines that golangci-lint uses | <https://github.com/golangci/golines> |
