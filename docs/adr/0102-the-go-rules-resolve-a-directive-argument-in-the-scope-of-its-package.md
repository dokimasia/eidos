---
adr: 0102
title: The Go rules resolve a directive argument in the scope of its package
status: Accepted
date: 2026-10-23
supersedes: ADR-0098
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0102: The Go rules resolve a directive argument in the scope of its package

## Status

Accepted

## Context

A directive argument of Go refers to a declaration by a spelling, such as `errors=volume.ErrTooLarge`
or `new=newSubject`. The Go rules resolved the spelling in the scope of the file of its subject
alone. A qualifier resolved through the imports of the file. A bare name resolved in the package of
the subject and in the dot imports of the file.

- A directive is a comment, so Go does not require its file to import a package that the comment
  mentions. Issue 73 reports a method in `hooks.go` whose directive refers to `volume.ErrTooLarge`.
  Only `schema.go` of the same package imports `volume`. Validation reported EID-0040 for the
  qualified name. The full import path `example.com/shop/volume.ErrTooLarge` resolved.
- Issue 74 reports a type of the package `store` whose directive refers to `newSubject`, a function
  of the external test package `store_test` in the same directory. The frontend loads both packages
  in one unit, but the bare name did not resolve. The full import path
  `example.com/shop/store_test.newSubject` resolved.
- go/doc resolves a doc link of a file through the imports of every file of its package. It reads
  the name that each import binds and deletes a name that two files bind to different paths. It
  gives the imports of the file of the link no precedence. Its package `go/doc/comment` also
  resolves a name of a standard package of one path element, such as `io.EOF`, without an import.
  It reads a list of those packages that a generator writes for each release of Go.
- The Go frontend recorded a use for each candidate of a directive argument in the bindings of its
  file. The dependency round places only the used packages, and a signature-only file keeps only
  the imports of the packages that it uses.
- A cold run validates the directives before the stamps of the load apply, and a warm run validates
  them with an empty fact store. The rules cannot read the `golang.constraint` stamp of a file
  during validation.

## Decision

We will resolve a directive argument of Go in the scope of its package where the scope of its file
does not declare the name.

- A qualifier that the imports of the subject's file do not bind resolves through the imports of
  the other files of its package. The imports of the file take precedence.
- The scope of the package reads the qualifier that each import binds: the alias of the import, or
  the name that its path assumes. It does not probe the unaliased imports for the clauses of their
  packages, as the scope of a file does.
- A qualifier that two files bind to different import paths resolves to no declaration. The finding
  EID-0040 lists both paths.
- The scope of the package reads every file of the package, internal test files and files outside
  the build included. A resolution does not depend on the build tags of the load.
- A bare name that neither the package of the subject nor a dot import of its file declares
  resolves in the external test package of the directory. That package has the path of the package
  with the suffix `_test`. A declaration of the package takes precedence, and a subject of the
  external test package has no such fallback.
- A dot import brings its names into its own file alone, as in Go.
- A stand-in for a package outside the view takes the import of another file in the same way. A
  subject without a file resolves through the imports of its package.
- The Go frontend records the uses of a directive argument in the same scope, after every file of
  the package has parsed. A signature-only file keeps an import that a directive of another file of
  its package uses.
- A name of a standard package of one path element does not resolve where the files of the package
  do not import the package, such as `io.EOF`.
- The frontend version is 0.10.0, because a unit of an earlier version records the uses of a
  directive argument in the scope of its file alone.

## Alternatives Considered

### The scope of the file, with the full import path for every other name

A directive would need the full import path for each package that the file of the directive does
not import. Go code and the doc links of Go refer to a declaration through its qualifier. Nothing in
Go ties the qualifier of a comment to the imports of its file.

### The rule of go/doc without the precedence of the file

go/doc deletes a name that two files bind to different paths, also where the file of the link binds
the name itself. A file that imports `alt "example.com/one"` would then refuse `alt.Err` because
another file binds `alt` to `example.com/two`. Go code in that file resolves `alt.Err` through its
own import.

### The files inside the build alone

The rules would leave out each file whose build constraint excludes it from the load. Validation
cannot read the stamps that mark such a file. A scope that followed the build tags would also
resolve a name on one platform and refuse it on another.

### A probe of the unaliased imports of the package

The scope of a file probes each unaliased import for a qualifier that the imports of the file do not
bind, because the clause of a package can declare a name other than the one that its path assumes.
Over a package, the probe would give such a qualifier one candidate for each unaliased import of
every file. The frontend would record a use for each candidate and load every imported package.

### The names of the standard packages of one element

A list of the standard packages that a generator writes follows the release of Go that built the
tool, not the toolchain of the workspace. The rules have no such list. Both issues refer to packages
of the module.

## Consequences

**Positive:**

- The trees of issues 73 and 74 load without a finding. The full import paths resolve as before.
- A directive in a file without imports can refer to a declaration of a package that another file of
  its package imports, as a doc link of Go can.
- The scope of a package reads the imports of its files on each lookup and allocates only the list
  of its candidates.
- The frontend allocates the lists of the files of a package once for each package. The parse of
  200,000 declarations allocates 4,155,000 times, against 4,162,000 before, and its time does not
  change.

**Negative:**

- A directive can refer to a declaration through the import of a file that the go command does not
  compile with the file of the directive, such as a file outside the build or an internal test file.
- A qualifier that only the clause of an imported package declares does not resolve through another
  file. The file needs its own import, or the directive needs the full import path.
- A name of a standard package of one element resolves only where a file of the package imports the
  package. Its path has no slash, so the form of a full import path does not apply to it.
- A lookup through the scope of a package reads every import of the package, because the scope does
  not keep an index between lookups.

## References

| What | Where |
|---|---|
| D220, the row of this decision | ../architecture/21-decisions.md |
| ADR-0098, the uses of a Go load, whose scope of a directive argument this decision replaces | 0098-the-go-frontend-loads-the-packages-that-a-load-uses.md |
| The Go satellite | ../architecture/11-languages.md |
| `importByName` of go/doc in Go 1.27.3 | <https://github.com/golang/go/blob/go1.27.3/src/go/doc/reader.go> |
| Issue 73, a qualified name resolves only through the imports of its own file | <https://github.com/dokimasia/eidos/issues/73> |
| Issue 74, a directive cannot refer to a function of the external test package by its short name | <https://github.com/dokimasia/eidos/issues/74> |
