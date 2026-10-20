---
adr: 0066
title: The Python frontend reads the standard library from typeshed
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

# ADR-0066: The Python frontend reads the standard library from typeshed

## Status

Accepted

## Context

Every module of Python reads the builtins, and most modules import the standard library: `typing`,
`collections.abc`, `dataclasses`, `enum`, `datetime`. The rules classify a builtin through its
class, and the frontend resolves an import of the standard library to its declaration. A load
therefore needs the declarations of the standard library, with their types.

The source of the standard library that an interpreter installs has few annotations, and the
modules that CPython implements in C, such as `math` and `_datetime`, have no source at all.
typeshed annotates every module of the standard library in stub files, and pyright and mypy read
those stubs. Its file `VERSIONS` lists the minor versions of Python in which each module exists.
typeshed is licensed under the Apache License 2.0, with parts under the MIT licence.

## Decision

We will embed the stubs of the standard library of typeshed at one commit in the package
`frontend/typeshed` of `eidos-lang-python`, and the frontend will read the standard library from
them, because every machine then loads one standard library with its types.

- The package embeds the stub of each module of `stdlib`, the stub of each package directory and
  `VERSIONS`, in the layout of the typeshed repository. The licence file of typeshed, which covers
  both licences, and the commit are beside the tree. At `e75a39fe` the tree has 752 stubs and
  `VERSIONS`, 2,868,009 bytes.
- The frontend opens the tree as its store `typeshed`. A dependency round places a module that
  `VERSIONS` lists for the version of the options of the load, and it loads the module at the
  signature depth.
- The installed distributions load from the second store, `venv`: the site-packages directory of
  the virtual environment that `VIRTUAL_ENV` names.
- A refresh copies the stubs and `VERSIONS` of a newer commit and changes the constant `Commit`.

## Alternatives Considered

### The standard library of the interpreter

The load would read the standard library from the `lib` directory of the interpreter of a
`pyvenv.cfg`. Little of that source has annotations, and the modules in C have no source.

### typeshed from the machine

The composition would locate the typeshed that a type checker installs, such as the copy of mypy or
of pyright. Its version would follow the installed checker, so two machines would load two
standard libraries from one workspace.

## Consequences

**Positive:**

- A reference to a class of the standard library resolves to its declaration on every machine.
- The rules read the shapes of the builtins and of the generic classes of `typing` and
  `collections.abc` from their declarations.

**Negative:**

- The repository contains 2.9 MB of stubs that another project maintains, under two licences.
- A fix of typeshed reaches eidos only through a refresh of the embedded tree.
- The third-party stubs of typeshed are not embedded, so a distribution without its own stubs loads
  from its source in site-packages, where it has one.

## References

| What | Where |
|---|---|
| RFC-0073, the design of this decision | ../rfc/0073-the-python-satellite.md |
| D178, the row of this decision | ../architecture/21-decisions.md |
| typeshed at `e75a39fe` | <https://github.com/python/typeshed/tree/e75a39fec6f375f5002d54c5607b44bedc025a4b> |
| The typing specification, distributing type information | <https://typing.python.org/en/latest/spec/distributing.html> |
