---
adr: 0065
title: A Python unit is one module
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

# ADR-0065: A Python unit is one module

## Status

Accepted

## Context

A unit is the input of one parse, and the language decides it. Python scopes a name to its module:
two modules of one directory both declare `User` under two identities, and a name of one module is
in the scope of another only through an import. A module is a `.py` file, a `.pyi` stub file, or
both files of one name in one directory. A type checker reads the stub of a module and not its
source.

The import system names a module by its path below a source root, as `shop.models` for
`src/shop/models.py` below `src`. The source root is a fact of the project. The build backend that
`pyproject.toml` names states it in its own table, and each build backend defaults to its own root
where the table states none.

## Decision

We will make each Python module one unit, with the package of its dotted name, below the source
root that the `pyproject.toml` of its project states, because Python scopes a name to its module.

- Where a stub file and a source file of one name share a directory, the unit lowers the stub, and
  the source file is a member without declarations with the stamp `python.stubbed`.
- The package of a module is its dotted name with slashes, as `shop/models` for `shop.models`, and
  the `__init__.py` of a package directory is the module of the package.
- The partition finds the nearest `pyproject.toml` above each file, and the file is a shared input
  of the unit. The source root is the root that the table of its build backend states:
  setuptools, hatchling, poetry-core, uv_build, pdm-backend or maturin.
- A project that states no root has the root `src` for the files below `src`. A file outside every
  source root, a file outside every project, and a file of a project whose `pyproject.toml` does not
  read take the root above the run of package directories that contain them, as pytest and mypy
  name such a module.

## Alternatives Considered

### A unit per package directory

A unit would be one directory, as a Go package is. Two modules of one directory would then be one
package, and the `User` of each module would collide in it.

### The source root `src` where it exists

A project without a declared root would take `src` where the directory exists and its own
directory otherwise. The existence of a directory would be an input of every unit of the project,
which the partition reads through no file. The run of package directories reads the `__init__.py`
and `__init__.pyi` files above a module, which the partition declares as shared inputs.

## Consequences

**Positive:**

- Two modules of one directory declare one name without a collision.
- A module has the package that its import names, so a reference through an import resolves to it.

**Negative:**

- A change of a `pyproject.toml` makes every unit of its project dirty.
- A build backend that is not in the table of the satellite names its root through a
  `pyproject.toml` that the partition does not read, and its modules take the root of the run of
  package directories.
- A namespace package of PEP 420 outside `src` has no `__init__.py`, so its modules take the root of
  their own directory, and their packages leave out the name of the namespace package.

## References

| What | Where |
|---|---|
| RFC-0073, the design of this decision | ../rfc/0073-the-python-satellite.md |
| D177, the row of this decision | ../architecture/21-decisions.md |
| The typing specification, the module resolution order | <https://typing.python.org/en/latest/spec/distributing.html> |
| setuptools, package discovery | <https://setuptools.pypa.io/en/latest/userguide/package_discovery.html> |
| uv, the build backend | <https://docs.astral.sh/uv/concepts/build-backend/> |
