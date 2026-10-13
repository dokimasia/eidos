---
rfc: 0056
title: The references that a generated file writes
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0037
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0056: The references that a generated file writes

## Summary

A generated file refers to declarations of other packages inside an inline type body and inside a
union constraint, and the backends do not bind the imports of those references. The Go backend
refuses both, and the TypeScript backend writes an inline object type without the imports that it
needs. We propose that an inline reference and a Go union list the types inside them as children,
so the shared walk of each backend binds their imports. We also propose four checks of the Go
backend: the import path rules of the go command, a parameter list in which some parameters have
names and others do not, the parentheses of a conversion, and a parameter of a test double that
hides a predeclared function. A package comment in a generated file is out.

## Motivation

Each defect in this list reproduces against the reference binary over a copy of its tree:

- The builder of a struct with the field `Retry struct{ Wait time.Duration }` reports that the
  inline body refers to a package whose imports the model does not record, and the Go plan writes
  nothing.
- The mock of `Clock[D ~int64 | time.Duration]` reports that `~int64 | time.Duration` is qualified
  and has no package. The Go frontend lowers the union to one named reference without a package.
- The TypeScript stub of `count(filter: { product: Product; limit: number }): number` uses
  `Product` and does not import it. tsc refuses the file, and the run does not report a finding.
- The stub of `Check(item string, append int) error` calls `append` to record the call, and
  `go vet` reports `cannot call append (variable of type int)`.

These defects and gaps do not show in a run of the reference binary:

- The go command refuses an import path that is not valid, and an import of another module's
  `internal` tree. The Go backend writes both, so the build of the consumer is the first step that
  reports them.
- Go gives a name to every parameter of a list or to none. A list in which some parameters have
  names does not parse, so the formatter withholds the whole file.
- The Go scaffold writes a conversion of `nil` to `*T` as `*T(nil)`, which Go parses as the
  dereference of a conversion to `T`.
- A generator cannot write the comment of a package.

## Detailed design

### Components

| Component | Change |
|---|---|
| The schema of `TypeRef` | An inline reference has the types of its body as children |
| `eidos-lang-go/frontend` | It records the children of an inline body, and lowers a constraint element to a union of its terms |
| `eidos-lang-typescript/frontend` | It records the children of an object type |
| `eidos-lang-go/backend` | It checks each package that it imports, keeps the spelling of an inline body with children, writes a union from its terms, refuses a list in which some members have names and others do not, and writes a conversion to a type that starts with an operator in parentheses |
| `eidos-reference/internal/double` | It renames a parameter or a result of a double that has the name of a predeclared identifier of Go |

### Invariants

1. A backend binds the import of each reference to a declaration that a generated file writes,
   inside an inline body and a union too.
2. The Go backend does not write an import that the go command refuses to build.
3. A reference that a backend cannot write refuses the declaration or the claimed file that
   contains it, and a finding gives the reason.

### The children of an inline reference

A frontend lists the types of an inline body as the children of its reference, in the order of the
body:

- A Go struct lists the type of each field, an embedded or blank field included.
- A Go interface lists the parameter types and the result types of each method, the type of each
  embed and each constraint element, in the order of its elements.
- A TypeScript object type lists the type of each property. For each method, construct signature
  and index signature, it lists the bounds and the defaults of the type parameters, the parameter
  types and the result type.

Java, Rust and protobuf have no inline type body. The members of the reference keep their own
types, so the rules read a field of an inline struct as before. The resolution step resolves the
children as it resolves the children of every other form, and the fold of the kernel gives the
shape of the reference the shapes of its children.

A backend writes an inline reference through the shared walk, which binds the import of each child.
The walk keeps the spelling of the body where it writes each child as the source wrote it. Where it
writes a child otherwise, the backend refuses the declaration, because the spelling of the body does
not follow from its children. The walk writes a child otherwise in two cases:

- The file imports the package of the child under another name, such as `time2` where a
  declaration of the file is named `time`.
- The child is a declaration of the package of its source, and the file is in another package, so
  the walk qualifies the child.

The Go backend still refuses an inline body without children that refers to another package. Such a
body comes from a plugin that wrote its spelling, and nothing records the package of a name inside
it.

### The terms of a Go union

The Go frontend lowers a union constraint, such as `~int64 | time.Duration`, to a `Union` reference
with one child for each term, in the order of the source. A term keeps its own reference, so
`time.Duration` records the package `time`. An approximation outside a union, such as `~int`, is a
union of one term. The spelling of the union keeps the approximation marks.

The Go backend keeps the spelling of a union where it writes each term as the source wrote it.
Otherwise it writes the terms again, joined by `|`, and puts the approximation mark before each term
that the spelling of the union marks. A union without a spelling does not record the marks, and the
backend refuses it. The type-set elements of a declared interface keep their stamp
`golang.typeSet`, because the interface has no reference for them.

### The packages that the Go backend imports

The Go backend imports the package of a type reference, of a call and of a name value through one
step. Before the step binds a package other than the package of the file, it checks the path by
the rules of the go command:

- The path is a valid import path, as `module.CheckImportPath` of `golang.org/x/mod` checks it.
- Where the path has an element `internal`, the package of the file is the parent of the last such
  element, or a package below it. The parent of a path that starts with `internal` is the standard
  library, which contains no generated file. The suffix `_test` of an external test package belongs
  to the package that it tests.

A path that fails a check refuses the declaration, with the reason of the check. The check runs on
each binding, and allocates nothing for a path that passes. The builtin `use` imports the path that
a template states, and the backend does not check it.

### Parameter lists

The Go backend refuses a parameter list or a result list in which some members have names and
others do not, because Go gives a name to every member of a list or to none. The backend refuses
such a list before the formatter runs, so the refusal skips one declaration and not the whole file.

### Conversions

Go requires parentheses around the type of a conversion where the type starts with `*` or `<-`, or
with the keyword `func`. The Go scaffold writes such a conversion as `(*T)(nil)`.

### Parameters of a double

Go lets a parameter take the name of a predeclared identifier, such as `append` or `len`. A body
that calls the identifier then calls the parameter. The stub of a method appends to its list of
calls, so a parameter named `append` breaks it.

The double package gives such a parameter, and such a named result, the name followed by the
smallest number from 0 that the method does not have. It takes the predeclared identifiers from the
universe scope of `go/types`. The rename applies to the doubles of TypeScript too, where it changes
no type.

### Verdicts

| Capability | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| A package comment in a generated file | out | The ensemble writes each family into the package of its source, and the files of the source document the package. The Go backend skips an `emit.Package` in a unit under `UnspeltKind` | A generator or a route that fills a package with generated files alone |
| A respelling or a refusal of a predeclared name at the settle | out | Go permits a declaration and a parameter to take such a name. A refusal at the settle would refuse valid Go, and the settle does not read the bodies that templates write. The doubles rename their parameters | A generated body that calls a predeclared function in the scope of a name that its generator does not choose |

### Failures

| Condition | Report |
|---|---|
| The walk writes a child of an inline body otherwise than the source | The declaration is refused under the code of its template |
| A Go inline body without children refers to another package | The declaration is refused |
| A Go union without a spelling has a term that the walk writes otherwise | The declaration is refused |
| An import path that is not valid | The declaration is refused with the reason of `CheckImportPath` |
| An import of an `internal` tree that does not contain the file | The declaration is refused |
| A Go list in which some members have names and others do not | The declaration is refused |

### Migration

1. State the children of an inline reference in the schema of `TypeRef`, and generate the models.
2. Record the children of an inline body in the Go and the TypeScript frontends, and the terms of a
   union in the Go frontend.
3. Check each import in the Go backend, keep an inline body with children, write a union from its
   terms, refuse a list in which some members have names and others do not, and write a
   conversion in parentheses.
4. Rename the parameters of a double that hide a predeclared identifier.

## Alternatives

### The members of an inline body in the emit model

The emit form of a reference could carry the fields and the methods of an inline body, and a
backend could write the body from them. A Go struct does not record an embedded or a blank field
among its fields, so the written body would lose them. The children bind the imports and keep the
body as the source wrote it.

### A check of the import block in the formatter

The Go formatter could check each path of the import block of a file. A failure would withhold the
whole file, and the formatter does not know the package of the file for the rule of `internal`.

### A mark of approximation on a reference

The model could mark a reference as an approximation. Only Go constraints have the mark, and the
spelling of the union already contains it.

## Drawbacks

- The Go backend refuses an inline body when the walk writes a child otherwise, where it could
  write the body again from its children.
- The backend does not check a path that a template imports through `use`.
- An approximation that a type parameter substitution rewrites loses its mark, so the backend
  refuses the union.

## Open questions

None.

## Unresolved and future work

None.

## References

- D87 in [21](../architecture/21-decisions.md), the trailing comments and the annotations of the
  model.
- [RFC-0001](0001-symbol-model-contract.md), the contract of a type reference.
- [The import path rules of the go command](https://go.dev/ref/mod#go-mod-file-ident) and
  [internal directories](https://go.dev/cmd/go/#hdr-Internal_Directories).
