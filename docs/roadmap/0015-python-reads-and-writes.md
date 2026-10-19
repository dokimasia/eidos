---
milestone: 0015
title: Python reads and writes
status: Planned
depends-on: 0005, 0009
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: 0073
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Milestone 0015: Python reads and writes

## Goal

A Python project loads into the graph, and a plan writes Python modules and stub files from the
declarations of any language. The satellite reads every declaration construct of Python 3 and every
type form of the typing specification. It writes them for the supported versions 3.11 to 3.15.

## Done when

- [ ] The fork `dokimasia/tree-sitter-python` declares its own module path, and `eidos-lang` pins it
      in the grammar package `treesitter/python`.
- [ ] The model states how a caller passes the argument of a parameter and the delete accessor of a
      property, and every backend takes a stance on the new fact.
- [ ] The frontend loads a project through the source roots of its `pyproject.toml`.
      `frontendtest.RunFrontendSuite` passes over it, with its dependency rounds over typeshed and a
      virtual environment.
- [ ] The corpus grades every inventory row for Python, and the support matrix has a Python column.
- [ ] The fixture `shop` writes its Go model, its TypeScript client and its Python mirror in one
      run. The Go toolchain and tsc type-check the first two. The toolchain adapter compiles,
      type-checks and tests the mirror, and the frontend loads the mirror back with the stamps of its
      sources.
- [ ] `cmd/eidos` composes the Python frontend, the rules, the target and the plan `python`.

## Why now

dokimi, the reference consumer, reads Python, so its adoption waits on this satellite. The
satellite needs the run of 0005 and a second target from 0009 to check what it writes.

## Scope

The Python satellite of [11-languages.md](../architecture/11-languages.md), the grammar of the fork,
the embedded standard library of typeshed, and the two changes of the model that Python needs.

## Not in this milestone

- The types that a type checker infers for declarations without annotations. They are level 2 of
  the scale.
- Cython's `.pyx` files and the cells of notebooks.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| The release of the kernel with the new fact fails the ring for every released backend | the release of the satellite | Release the kernel and the backends in one train |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-10-19 | Added at position 15, with RFC-0073 | dokimi reads Python, and the satellite has a design |
