---
adr: 0105
title: The layout refuses a declaration whose file cannot import one of its references
status: Accepted
date: 2026-10-25
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0105: The layout refuses a declaration whose file cannot import one of its references

## Status

Accepted

## Context

A generator records declarations into the file of a family, and the layout routes each file to its
directory and its package. The file template of a family that claims its files runs once over every
record of a file.

- The suite generator of dokimi writes the interface of the methods of a struct. A struct that
  embeds `*assert.Recorder` of go.dokimi.dev/assert v0.6.0 promotes `Clock() matcher.Clock`, and
  the type of the result is in `go.dokimi.dev/assert/internal/matcher`. Issue 80 reports that the
  Go backend refused the import in the suite file of a consumer. The render then withheld the whole
  file under EID-0023. The run wrote none of the records of that file.
- The Go backend checked the import rules of the go command when a template spelled a reference: a
  valid import path and the internal rule. A refusal fails the file template. The render then
  withholds the whole file.
- The layout refused one declaration and kept the rest of its file where the plan does not emit the
  referent of a reference or where the file of the referent has no package. It did not check
  whether the file of a declaration can import the package of a reference.
- The SDK did not state the import rule of a target. A plugin had to copy the rule of each language
  to decline a declaration before it records the declaration.
- Go refuses two more references that the backend did not check. The go command compiles an
  external test package into the test binary of its directory alone, and an import of such a
  package from another package fails with `cannot find module providing package`. Go code refers to
  the exported names of another package alone.

## Decision

We will check the import rule of the target for each declaration in the layout. A plugin reads the
same rule through the rules of its language.

- `golang.CheckReference(from, pkg, name)` returns the reason that Go code of the package `from`
  cannot refer to the declaration `name` of the package `pkg`, and nil where it can. It refuses a
  path that is not a valid import path, an external test package of another package, an internal
  package of a tree that does not contain `from`, and an unexported name of another package. An
  empty name checks the import alone.
- `plugin.ImportChecker` is the role of a backend whose target restricts the references of a file.
  The kit declares the rule through `backend.Builder.ImportRule`, and the built backend serves it.
  The Go backend declares `spell.Importable`, which reads the package of a reference as the backend
  imports it.
- The layout calls the role for each reference of each routed declaration, after it qualifies the
  references that cross into another package. It refuses a declaration with a reference that its
  file cannot refer to under `UnimportableReference`, EID-0082, at the origin of the declaration.
  The file keeps its other declarations. The layout checks no file whose package derives none, and
  no declaration that another finding refused.
- `rules.ImportRules` is the optional capability of a language with the same rule over a reference
  of the graph, and `Bound.Importable(pkg, ref)` calls it. The Go rules implement it through
  `golang.CheckReference`, for each reference inside `ref`. A generator calls it before it records a
  declaration into a file of `pkg`. For a reference whose target is a function, the result states
  whether the code can call the function.
- The `bind` of the Go backend checks the import of a package through `golang.CheckReference`
  without a name. A template that spells a reference into an external test package fails, as a
  reference into an internal package of another tree fails.

## Alternatives Considered

### A render of each record apart

The file template of a family that claims its files loops over its records itself, so the render
cannot run the template for one record and keep the output of the others.

### The rule in each plugin

Each generator would restate the import rules of each target that it writes. Issue 80 requests the
opposite.

### The check in the emitter

The layout routes the files after every generator has run, so the emitter does not know the package
of the file of a declaration.

### The check of a name at the render

The render spells the references inside a body that the layout does not walk. A check of the name
there refuses the bodies of the shared fixture of the backend suite. Those bodies call a function
with a lower-case name of another package. The other languages of the fixture write that call as it
is.

## Consequences

**Positive:**

- The rules refuse `Log.Clock` and `Log.WithClock` of the tree of issue 80, whose signatures refer
  to `matcher.Clock`, through `Bound.Importable`. They admit the other 15 methods of `Log` and the 47
  methods of `Clock`. A generator that calls `Bound.Importable` leaves out such a method and writes
  the rest of the interface.
- With the suite generator of dokimi as it is, the layout refuses the stub and the interface of
  `Log` under EID-0082 at `Log`. The file templates of the suite and of the test of the stub still
  spell `matcher.Clock` from the data of their records. The render withholds those two files under
  EID-0023.
- A Go plan refuses a declaration that refers to an external test package or to an unexported name
  of another package. Go does not compile such a reference.

**Negative:**

- The layout walks every reference of every routed declaration of a plan whose backend declares the
  rule. The Go rule allocates nothing for a reference that the code can refer to.
- The rule takes an import path that ends in `_test` for an external test package, as the frontend
  loads one. A directory whose name ends in `_test` and whose files declare another package name has
  such a path too.
- A plugin passes the package of the file that it writes into. Under a route, the layout derives
  that package after the generators ran. A plugin cannot call the rule with that package before it
  records.

## References

| What | Where |
|---|---|
| D223, the row of this decision | ../architecture/21-decisions.md |
| ADR-0019, a family claims its files through a file template | 0019-a-family-claims-its-files-through-a-template.md |
| ADR-0088, the layout qualifies an emitted reference | 0088-the-layout-qualifies-an-emitted-reference.md |
| The routing and the layout | ../architecture/18-routing-and-layout.md |
| `go help importpath`, internal packages, Go 1.27.3 | <https://go.dev/s/go14internal> |
| `go help test`, the test files that declare a package with the suffix `_test`, Go 1.27.3 | <https://pkg.go.dev/cmd/go#hdr-Test_packages> |
| The Go specification, exported identifiers | <https://go.dev/ref/spec#Exported_identifiers> |
| Issue 80, a reference that the file cannot import discards every record of the file | <https://github.com/dokimasia/eidos/issues/80> |
