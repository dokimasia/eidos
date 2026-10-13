---
adr: 0050
title: eidos publishes the binary eidos
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0065
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0050: eidos publishes the binary eidos

## Status

Accepted

## Context

eidos did not publish a binary. The rule assumed that the binaries of consumers are the products and
that a consumer serves the users who do not write Go. The declarative host changed that assumption:

- A declarative plugin is a directory of a YAML manifest and template trees. A binary runs it in a
  plan that the binary marks extensible.
- A team that writes declarative plugins still composed a binary in Go before its first run. The
  declarative fixture of `eidos-conformance` is such a composition in 60 lines.
- A binary with typed plugins remains the product of its consumer. A binary without plugins does not
  replace a consumer binary.

## Decision

We will publish the binary `eidos`, the main package `cmd/eidos` of the root module
`go.dokimi.dev/eidos`, with every frontend and one extensible plan for each target, because the
declarative host lets a team run its plugins without a Go binary of its own.

The binary has these parts:

- The frontends of Go, TypeScript, Java, Rust and protobuf, and the rules of Go, TypeScript and
  protobuf. Java and Rust run under the absent rules.
- The plans `golang`, `typescript`, `java` and `rust`, each extensible and without a generator. Each
  plan reads the packages of the language of its target below the workspace root.
- Dispatch on `runtime.GOMAXPROCS(0)` workers, and no annotator.
- The brand `eidos`, which names `.eidos.yaml`, `.eidos/`, the carriers `+eidos:` and the variables
  `EIDOS_*`.

`go install go.dokimi.dev/eidos/cmd/eidos@latest` installs the binary. The root module has no replace
directive. Each release of the root module also publishes archives for darwin, Linux and Windows and
a Homebrew cask. The job `binaries` of `release.yml` builds them with cgo on macOS, as the release of
techne does, because ergon builds the commands of `.ergon.yaml` with cgo off.

## Alternatives Considered

### The binary in eidos-cli

`eidos-cli` could contain the command. Every consumer of the command kernels would then require the
grammars of tree-sitter in its module graph.

### The binary in eidos-reference

`eidos-reference` already contains a binary. Its plugins are the canary of the ring. Its `go.mod` has
seven replace directives, and `go install` refuses such a module.

### A module of its own

A module such as `go.dokimi.dev/eidos/bin` could contain the command under the path
`go.dokimi.dev/eidos/bin/cmd/eidos`. The root module already has the path `go.dokimi.dev/eidos`.
ergon and techne keep their commands in the root module.

### Plans that the config file declares

A config file could declare a plan with a name, a target and sources. The config file then assembles
a composition, which the command kernels refuse. A plugin whose rules require a directive already runs
only for the subjects that set the directive.

### A binary without the Java and Rust frontends

The binary could leave out the frontends whose locators need a toolchain or whose languages have no
rules. The binary composes every frontend and backend that eidos has.

## Consequences

**Positive:**

- A team that does not write Go installs the binary from an archive or with Homebrew, and runs its
  declarative plugins with a config file.
- The binary has the commands, options and output of every binary built with eidos.

**Negative:**

- The root module joins `go.work`. ergon's targets then run in 17 modules instead of 16. The tests of
  the root module build the grammars of tree-sitter with cgo.
- `go install` needs a C compiler on every platform.
- Each release of the root module runs a job of up to 60 minutes on a macOS runner with zig and UPX.
  The repository maintains `.goreleaser.yml` and that job beside the baseline of ergon.
- A declarative rule that lists the members of a Java or Rust subject reports `DECLARATIVE-0001`,
  because those languages have no rules package.
- A run over a tree with a Go file needs a Go toolchain, also when every plan reads another language.
- `.eidos.yaml` and `.eidos/` become the files of the binary in a repository that runs it.
- A config file that does not extend a plan runs without output and without a finding.

**Neutral:**

- A consumer binary composes as before and reads its own `.<brand>.yaml`.

## References

| What | Where |
|---|---|
| RFC-0065, the design of the binary | ../rfc/0065-the-eidos-binary.md |
| D2, the rule that this decision supersedes, and D160, the row of this decision | ../architecture/21-decisions.md |
| The distribution of eidos | ../architecture/14-distribution-and-cli.md |
| ergon RFC-0007, binary releases and cgo | <https://github.com/dokimasia/ergon/blob/main/docs/rfc/0007-binary-releases.md> |
| The release of techne with cgo | <https://github.com/dokimasia/techne/blob/main/.goreleaser.yml> |
