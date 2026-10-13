---
adr: 0052
title: A run locates the stores of the frontends with input files
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

# ADR-0052: A run locates the stores of the frontends with input files

## Status

Accepted

## Context

A frontend that reads dependency units implements a store locator, such as the module cache of Go or
the `ct.sym` of a JDK. The command kernels called `Workspace.Stores` before each run, which called the
locator of every frontend of the composition. The Java locator returns an error without `JAVA_HOME`.
The Go locator returns an error without `GOROOT` or a `go` binary on `PATH`. A binary that composes
the Java frontend failed for every user without a JDK, also over a tree without a Java file.
Every run also read the `ct.sym` of the JDK into memory.

## Decision

We will locate the stores of a frontend in the load of a run, only when the tree has an input file of
the frontend, because a binary with every frontend otherwise needs the toolchain of every language.

- `workspace.Input.Getenv` and `load.Config.Getenv` pass the environment to the load.
- After the selection, the load calls in composition order the locator of each frontend that has an
  input file. An input file is a file that the selection of the frontend claims and that is not an
  output of the brand.
- The load returns the error of a locator behind the name of its frontend. It refuses a store name
  that two locators return, or that a locator and `Input.Stores` both provide. The run does not run
  again cold for such an error.
- The command kernels pass the environment in place of the stores. `Workspace.Stores` remains for a
  caller that needs every store before a run, such as a test fixture.

## Alternatives Considered

### Locators that return no store without a toolchain

The Java and Go locators could return an empty map where the environment has no toolchain. The load
would then report every import of the standard library as a need without a place. A Java user without
`JAVA_HOME` would lose the error that contains the name of the variable.

### Eager location with errors for the frontends with input files

The command kernels could locate every store before the run and pass the errors to the run. The run
would report the error of a frontend with an input file. A run over a tree without a Java file would
still read the `ct.sym` of the JDK into memory.

### A binary without the Java frontend

The binary could leave out the frontends whose locators need a toolchain. eidos composes the full
surface of each language that it has. Every consumer binary that composes the Java frontend has the
same failure.

## Consequences

**Positive:**

- A tree without a Java file runs without a JDK, and a tree without a Go file runs without a Go
  toolchain.
- A run over a tree without a Java file does not read the `ct.sym` of the JDK.
- A tree with a Java file still gets the error that contains the name `JAVA_HOME`.

**Negative:**

- The load returns a locator error after the walk of the tree.
- `watch` renders the error of a locator in a pass and runs the next pass. It no longer exits with the
  error.
- A caller that passes both `Input.Stores` and `Input.Getenv` gets an error for a store name that both
  provide.

**Neutral:**

- The message of a locator error gains the prefix `load:`.

## References

| What | Where |
|---|---|
| RFC-0065, the design of the binary eidos | ../rfc/0065-the-eidos-binary.md |
| RFC-0021, the store locators and `Workspace.Stores` | ../rfc/0021-the-command-kernels.md |
| D162, the row of this decision | ../architecture/21-decisions.md |
