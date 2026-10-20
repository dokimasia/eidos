---
adr: 0087
title: eidos reads TypeScript through the fork of tree-sitter-typescript
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: RFC-0082
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0087: eidos reads TypeScript through the fork of tree-sitter-typescript

## Status

Accepted

## Context

A frontend parses through a pinned library, and tree-sitter is the platform for TypeScript. The
native compiler of TypeScript 7 is written in Go, but all its Go packages are internal.

The grammar `tree-sitter/tree-sitter-typescript` released v0.23.2 on 2024-11-11. Its master has
five commits since then, the last on 2025-01-30, and none of them changes the grammar. The
repository has 23 open pull requests. The grammar reads seven constructs differently from
TypeScript:

- TypeScript reads `readonly string[] | null` as a union of `readonly string[]` and `null`. The
  grammar nests the union inside `readonly`. The frontend marks a list or a tuple immutable, so it
  loaded the list of such a union as mutable.
- `lib.es2015.symbol.wellknown.d.ts` of TypeScript 7.0.2 contains `[K in keyof readonly any[]]?:
  boolean` at line 106. The grammar reads a primary type after `keyof` alone, so it parsed the type
  as an ERROR.
- The grammar does not parse an import type such as `import('m').X` before `[]` or an indexed
  access, as the name of a generic type, or after `keyof`.
- A static precedence makes the grammar read `f<typeof import('m')>()` as two comparisons.
- The scanner ends JSX text at every `&`, so `<p>a & b</p>` is an ERROR.
- Where a keyword can start a type or a member, the grammar does not read it as a name. So
  `{ abstract: string }`, `i < unique.length` and `[symbol: string]` are ERRORs.
- The grammar reads a `<` after a line break as type arguments, so a call signature with type
  parameters cannot start the next member of an object type.

The fork `dokimasia/tree-sitter-typescript` fixes the seven on top of upstream's head `75b3874`,
one commit each. A comparison of the trees ran over the 108 files of the library and the files of
desktop, redux and vscode, 16,498 files in all. The CI of upstream parses the same three projects.

- Upstream's grammar fails on 57 of the files. The fork fails on 11, and each of them is a fixture
  that is invalid on purpose.
- No fix adds an error to a file.
- The fix of `readonly` changes 503 trees. The `readonly` nodes that contain a whole union drop
  from 1,249 to none.
- The fix of `typeof import()` also changes 33 trees without an error. Each of them read such a
  call with arguments as two comparisons.

## Decision

We will parse TypeScript and TSX through the grammars of the fork
`dokimasia/tree-sitter-typescript`, which `eidos-lang` pins in its package `treesitter/typescript`,
because the upstream grammar has not changed since its last release and reads seven constructs
differently from TypeScript.

- The fork is the source of truth of the grammar. A change keeps the conventions of upstream: its
  corpus format and test names, its commit subjects and its generated files. tree-sitter 0.24.4
  generates the files, because the lock file of upstream names that release.
- The fork declares the module path `github.com/dokimasia/tree-sitter-typescript`, and
  `eidos-lang` requires that module at the pseudo-version of a commit.
- depguard refuses an import of `github.com/dokimasia/tree-sitter` outside `lang/treesitter`, as it
  refuses an import of `github.com/tree-sitter`.
- A spec of the satellite requires every file of the embedded library to parse without a syntax
  error.

## Alternatives Considered

### The upstream grammar

The frontend would pin `tree-sitter/tree-sitter-typescript` at v0.23.2 until a release of upstream
fixes the seven constructs. A list or a tuple under `readonly` would load as mutable in a union or
an intersection. The library would load with a syntax error, which a spec of the satellite would
list. A file with any of the other five constructs would report a syntax error.

### A replace directive

`eidos-lang` could require the upstream module and replace it with the fork. A replace directive
applies only in the main module, so a consumer of `eidos-lang` would build against upstream. `go
install` at a version refuses a module with a replace directive.

## Consequences

**Positive:**

- A list or a tuple under `readonly` keeps its immutability in a union or an intersection, as in
  `readonly T[] | null`.
- Every file of the embedded library parses without a syntax error.
- Every valid file of desktop, redux and vscode parses without a syntax error.
- Every machine parses a workspace with one grammar.

**Negative:**

- The fork takes each change of upstream by hand, and keeps its own module path in `go.mod` and in
  the import of the Go binding's test.
- A change of the grammar regenerates the parsers of both grammars, 17.5 MB of C, so each change of
  the fork has a large diff.
- A defect of the grammar is a change of the fork, which the checks of the fork verify: the corpus
  tests, a second generation of its files, the lint of the grammar, the tests of the Rust and Go
  bindings, and a comparison of every tree with the previous parser.

**Neutral:**

- The fork does not parse 11 of the 16,498 files, and each of them is invalid on purpose.

## References

| What | Where |
|---|---|
| RFC-0082, the design of the library | ../rfc/0082-the-declaration-files-of-the-typescript-library.md |
| D205, the row of this decision | ../architecture/21-decisions.md |
| The fork of the grammar | <https://github.com/dokimasia/tree-sitter-typescript> |
| The upstream grammar | <https://github.com/tree-sitter/tree-sitter-typescript> |
| Go Modules Reference, the replace directive and `go install` | <https://go.dev/ref/mod> |
