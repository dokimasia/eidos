---
adr: 0118
title: The imports of a file keep off the names of its package
status: Accepted
date: 2026-11-01
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0118: The imports of a file keep off the names of its package

## Status

Accepted

## Context

The conformance suites of dokimi failed in two ways in a package that another file of the package
already used.

- A kit declared the specification of its linearizability law as `var spec = …`, and the suite
  in the same package imported `go.dokimi.dev/spec`. Go refused the package, because its package
  block contains the declarations of every file.
- The suite of a contract in a package named `spec` referred to `spec.Reader` before its template
  stated the import of `go.dokimi.dev/spec` through `use`. The render refused the file, because the
  reference had bound the name `spec` to another package.

The render reserved the names of the declarations of the file under render and no other name. The
builtin `use` binds the name that the template states and returns no name, so a template could not
refer to an import by a name that the render chooses.

## Decision

We will keep the imports of a file off the names that the other files of its package declare, and
let a template import a path under a name that the render chooses.

- A language whose files of one package share one scope declares `render.Language.PackageScope`
  through `backend.Builder.PackageScope`. The hook yields the names that a file-level declaration of
  a source file declares in the scope of its package.
- The render computes the scope of each package of its routed files once per render, on the first
  file of the package that a worker renders. The scope contains each name that the hook yields for a
  declaration that `plugin.RenderContext.Declarations` yields for a source file of the package, the
  declared names of the routed files of a package of two or more routed files, and the settled names
  of the kept files of its namespace in `plugin.RenderContext.Others`. A source file at the path of a
  routed file does not count, because the run writes it again. A record of a family that claims its
  files does not count, because its template declares names that the render does not know.
- `plugin.RenderContext.Declarations` passes the path and the file-level declarations of each
  source file as `symbol.Symbol` values. The render then reads the sealed graph without the types of
  the package that builds it.
- `render.ImportSet.SetPackageScope` gives each file the scope of its package. `Bind` and `BindItem`
  take the lowest free suffix of a name of the scope. `Claim` reports false for such a name of an
  import from another package. `Use` returns an error, because the template refers to the import by
  the stated name.
- The Go backend declares the hook. It yields the name of every file-level declaration but a method,
  and the name of each constant of an enumeration.
- The builtin `import` binds a path through the language's `TemplateImport` under a free name and
  returns the name. A whole package binds under the name of its package clause where the template
  states no name, as a reference of the file binds it. The file's own package returns the empty
  name.
- A rendered file of a package that binds an import reports `plugin.RenderedFile.PackageScope`. Its
  group records the package edge of its package and `state.PackageScopeEdge` of the plan and the
  package. A warm round makes that edge dirty for each package that a file-level name of the plan
  enters or leaves.

## Alternatives Considered

### Names reserved by the layout

The layout knows the names of the routed and the kept files, and could reserve them on each routed
file. It does not know the names that a source declaration declares in the scope of its package,
such as the constants of a Go enumeration, so the rule of each language would move into the layout.

### Scopes computed before the workers start

The render could compute every scope before its workers start. That serial step added 4.5% to the
time of the render of the scaled Go corpus of 200,000 declarations. With the scopes computed on the
workers, the benchstat comparison does not find a difference in time at the 0.05 level.

### A loaded package in the render context

The render context could return the loaded package at a path. The backend packages would then
depend on the types of the package that builds the graph, and the import rules of the write side
refuse that dependency.

### A name that use returns

`use` could return the name that it binds. Every template that calls `use` for its effect would then
print the name.

### A suffix for a collision of use

`use` could take a suffix where another binding takes the stated name. The template refers to the
import by the stated name, so the suffix would break the reference.

### A package scope for every language

The render could keep the imports of every language off the names of the other files of a package. A
module of TypeScript, Python or Rust is a namespace of its own. A single-type import of Java shadows a
type of its package within its file alone, so Java would write fully qualified names for no reason.

## Consequences

**Positive:**

- A generated Go file imports a package under a free name where a kit of its package declares the
  name of the package, and a template refers to an import by the name that the render returns.
- A warm run writes the same imports as a cold run after a declaration enters or leaves the package.

**Negative:**

- An import of another file can take a name that the template of a claimed file declares, because
  the render does not know those names.
- A Go file that binds an import renders again after any edit of a declaration of its package.
- The render of the scaled Go corpus of 200,000 declarations allocates 1.67% more memory and 0.04%
  more objects. Its median time moves from 744.0 ms to 766.7 ms over 12 interleaved runs of each
  side. The benchstat comparison does not find a difference at the 0.05 level (p=0.060).
- The builtin adds one entry to the function maps of each template. A build of the Go backend
  allocates 1,662 times, 48 more.

## References

| What | Where |
|---|---|
| D236, the row of this decision | ../architecture/21-decisions.md |
| The import set and the builtins of a template | ../architecture/07-rendering.md |
| The reads of a group | ../architecture/09-incrementality.md |
| The Go specification, declarations and scope | <https://go.dev/ref/spec#Declarations_and_scope> |
| `GeneratedFile.QualifiedGoIdent` of protogen | <https://pkg.go.dev/google.golang.org/protobuf/compiler/protogen#GeneratedFile.QualifiedGoIdent> |
