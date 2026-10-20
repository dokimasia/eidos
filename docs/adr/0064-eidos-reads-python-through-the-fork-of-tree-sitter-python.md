---
adr: 0064
title: eidos reads Python through the fork of tree-sitter-python
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: RFC-0073
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0064: eidos reads Python through the fork of tree-sitter-python

## Status

Accepted

## Context

A frontend parses through a pinned library, and tree-sitter is the platform for a language without a
proven pure-Go parser. gpython is the only Go library that parses Python, and it parses the grammar
of Python 3.4, which has no variable annotations, no `async def` and no positional-only parameters.

The grammar `tree-sitter/tree-sitter-python` released 0.25.0, and its master has not changed since
2025-09-15. Its pull requests for lazy imports and for PEP 798 have no review. It does not parse
four constructs that Python 3.13 to 3.15 add:

- the lazy import of PEP 810
- the unpacking in comprehensions of PEP 798
- the type parameter default of PEP 696
- the plus sign in a literal pattern of CPython 3.15

The fork `dokimasia/tree-sitter-python` adds each of the four on top of upstream's head `26855ea`,
one commit each. With the four commits, the fork parses 5,888 of the 5,891 example files of numpy,
django, flask and CPython main. CPython's `test_annotationlib.py` fails on a starred argument and an
extended slice in an annotation, and its `test_compile.py` on a dedent inside parentheses. The third
file, `badsyntax_3131.py`, is invalid syntax.

## Decision

We will parse Python through the grammar of the fork `dokimasia/tree-sitter-python`, which
`eidos-lang` pins in its package `treesitter/python`, because no other grammar or Go library parses
the Python of 3.15.

- The fork is the source of truth of the grammar. A change keeps the conventions of upstream: its
  corpus format and test names, its commit subjects, and the generated files of the CLI version
  that upstream committed.
- The fork declares the module path `github.com/dokimasia/tree-sitter-python`, and `eidos-lang`
  requires that module.
- Every satellite reads the grammar through `eidos-lang`, which is the only importer of the
  tree-sitter bindings.

## Alternatives Considered

### The upstream grammar

The frontend would pin `tree-sitter/tree-sitter-python` at 0.25.0. It would refuse every module that
imports lazily, and it would lose the default of every type parameter of PEP 696.

### A pure-Go parser

gpython parses Python 3.4. No other Go library parses Python 3.

### CPython's `ast` through an interpreter

The frontend would run the `python3` of the machine and read its syntax tree. Each interpreter
parses its own version of the language, so two machines would load two graphs from one workspace,
which the rule against toolchain frontends refuses.

### A replace directive

`eidos-lang` could require the upstream module and replace it with the fork. A replace directive
applies only in the main module, so a consumer of `eidos-lang` would build against upstream. `go
install` at a version refuses a module with a replace directive.

## Consequences

**Positive:**

- A module of Python 3.15 loads, with its lazy imports and the defaults of its type parameters.
- Every machine parses a workspace with one grammar.

**Negative:**

- The fork takes each change of upstream by hand, and keeps its own module path in `go.mod` and in
  the import of the Go binding's test.
- Every binary that links the satellite compiles the 3.5 MB of C of the parser and the 15 KB of C
  of the external scanner under cgo.
- A defect of the grammar is a change of the fork, which the checks of the fork verify: the tests
  of the grammar, a second generation of its files, the lint of `grammar.js`, the tests of the Rust
  and Go bindings, and a comparison of every tree with the previous parser.

## References

| What | Where |
|---|---|
| RFC-0073, the design of the satellite | ../rfc/0073-the-python-satellite.md |
| D176, the row of this decision | ../architecture/21-decisions.md |
| The fork of the grammar | <https://github.com/dokimasia/tree-sitter-python> |
| The upstream grammar | <https://github.com/tree-sitter/tree-sitter-python> |
| gpython | <https://github.com/go-python/gpython> |
| PEP 810, explicit lazy imports | <https://peps.python.org/pep-0810/> |
| PEP 696, type parameter defaults | <https://peps.python.org/pep-0696/> |
| Go Modules Reference, the replace directive and `go install` | <https://go.dev/ref/mod> |
