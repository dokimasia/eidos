---
rfc: 0092
title: The names of the length of an array
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-11-09
updated: 2026-11-09
discussion: https://github.com/dokimasia/dokimi/issues/84
supersedes: none
superseded-by: none
produces-adr: ADR-0126
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0092: The names of the length of an array

## Summary

A Go array type can state its length through a constant, as `[sizes.SaltSize]byte` does. The Go
frontend records the length of an array only as a literal, and keeps any other length in the
spelling alone. A generated file of another package then writes the spelling without the import of
`sizes`, and writes a constant of the package of the source without its qualifier, so the file does
not compile. We propose that the reference of an array lists the names of its length as its
children after the element, each with the package of its declaration. The shared walk of each
backend then binds their imports. The Go backend writes the length again where the walk writes a
name otherwise, and the rules check that a file can refer to each name.

## Motivation

- dokimi derives the input of `Store.Stage` from the field `Salt [sizes.SaltSize]byte` of its
  request. It writes `&api.StageRequest{Salt: [sizes.SaltSize]byte{1}}` into the suite in `apitest`,
  and the suite does not import `sizes`. `go vet` refuses the suite with `undefined: sizes`.
- A constant of the package of the request fails the same way. The suite writes `[SaltSize]byte{1}`,
  and `apitest` does not declare `SaltSize`.
- `TypeRef.Length` is the length of an array that the source writes as a literal, and 0 for an
  expression. The Go frontend sets it through `arrayLength`, and the reference has the element as
  its one child.
- `spellref.restatedStructural` keeps the spelling of a structural reference where the walk writes
  each child as the source wrote it. The element `byte` is such a child. The Go backend then writes
  `[sizes.SaltSize]byte` as the source wrote it, and it does not bind an import for the name of the
  length.
- `Rules.Importable` and `Rules.ImportableValue` walk the references of a type. The name of a length
  is no reference, so both functions admit a type that a file cannot write.
- The Rust frontend records a literal length alone too. The reference records a length expression
  such as `SALT_SIZE` in `[u8; SALT_SIZE]` only in its spelling, without the module of the constant.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Schema | `eidos-core/symbol/schema` | An Array lists the names of its length after its element |
| Kernel | `eidos-core/rules` | The fold of an Array folds its element alone |
| SDK | `eidos-sdk` | The regenerated facade |
| Go | `eidos-lang-go` | The names of a length in the frontend, the length that `Restate` writes again, and the refusal of a length that names a package without children |
| Rust | `eidos-lang-rust` | The names of a length in the frontend |

### Invariants

1. A backend binds the import of each name inside the length of an array that a generated file
   writes, as it binds the import of a type inside a composite.
2. The rules refuse an array type whose length names a declaration that the file cannot refer to.
3. A backend writes the length as the source wrote it where the walk writes each name as the source
   wrote it.

### The names of a length

A frontend lists the names that the length of an array refers to as the children of its reference,
after its element, in the order of the source:

- A Go name is an identifier or a qualified identifier of the length. A predeclared identifier, such
  as `len`, is no name, and neither is the field or the method that a selector of a value selects.
- A Rust name is an identifier or a path of the length expression.
- An integer literal has no names, and `Length` keeps its value.

Each name is a named reference whose spelling is the name as the source wrote it. Its package is the
package of its declaration: the import that its qualifier binds, or the package of the file for an
unqualified name. A Go file with a dot import, and a Rust file with a glob use, leave the package of
an unqualified name empty, because the name can be a declaration of the imported package. The
resolution step indexes the declarations that a type position can name. A name of a constant does
not resolve to a target. A name of a type, such as `T` in `unsafe.Sizeof(T{})`, resolves as the
reference of a type resolves.

`Length` keeps its meaning: the length of an array whose length is a literal, and 0 otherwise.

### The fold

The fold of an Array folds its element alone and drops the names of its length, so the `TypeShape`
of an array keeps one child and its length.

### The Go backend

The shared walk binds the import of each name through the qualification of a named reference, as it
does for a type. It keeps the spelling of the array where it writes each name as the source wrote
it. Otherwise `golang.Restate` writes the length again:

1. It parses the length from the spelling of the array.
2. It finds each name in the length by the spelling that its child records, in the order of the
   source, and replaces it with the spelling that the walk wrote for it.
3. It writes the element after the length.

`Restate` keeps the text of a length without names. A literal length then writes again where the
walk writes the element otherwise, as `[0]time.Duration` does under the import name `time2`. An
array without a spelling writes its `Length`. `Restate` refuses a spelling that does not parse, and a
count of names that differs from the count of children.

The Go speller refuses an array without names whose length qualifies a package, as it refuses an
inline body without children that refers to another package. Such an array comes from a plugin that
wrote its spelling.

### The Rust backend

The walk binds each name through its use, as it binds a path of a type. The Rust backend keeps the
written spelling of each structural reference. It refuses a structural reference whose child has a
use under another name, so it refuses an array whose name the walk writes otherwise.

### The rules

`Rules.Importable` and `Rules.ImportableValue` walk the children of a reference. They check each name
of a length by its package and its unqualified spelling, as they check a type. Go refuses a name
without a package and without a target unless it predeclares the name, as for an unresolved
reference of a type.

### The frontend version

Each array whose length is an expression gains children in the record of its unit, so
`golang.FrontendVersion` and the version of the Rust frontend rise.

### Failures

| Condition | Effect |
|---|---|
| A name of a declaration that the file cannot refer to, such as an unexported constant of another package | `Rules.Importable` refuses the type |
| A Go file with a dot import, or a Rust file with a glob use, that states an unqualified name in a length | The name has no package, and `Rules.Importable` refuses the type in a file of another package |
| A Go length whose spelling does not parse | The reference is refused |
| A Rust array whose name the walk writes otherwise | The reference is refused, as is each structural reference of Rust whose child spells otherwise |
| A Go array without names whose length qualifies a package | The reference is refused |

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| The names of a length as children of the array | in | The shared walk binds the import of each child, as for an inline body, and the generated file keeps the constant that the source names | none known |
| The evaluated length in `Length` | out | A frontend parses each unit apart from the others, so it cannot evaluate a constant of another package, and the Rust frontend has no evaluator of constant expressions | A frontend that evaluates the constants of other units |
| A new field of `TypeRef` for the names | out | Every walk visits the children with the package of each reference, and a new field would need another walk in each consumer | A consumer that must tell the names of a length from its element without the form |
| The rewrite of a Rust array whose name the walk writes otherwise | out | The Rust backend restates no structural reference, because it has no parser of a Rust type | A Rust backend that restates its composites |

## Alternatives

### The evaluated length

The frontend could record the value of the length in `Length`. The backend would then write
`[32]byte`, as go/types prints the type. The Go frontend evaluates the constants of one package at a
time with its imports stubbed. `sizes.SaltSize` has no value at the parse of `api`. The value would
need a step after the load, and the Rust frontend has no evaluator. The generated file would also
lose the constant that the source names.

### A field for the names

`TypeRef` could gain a field with the names of the length. Each walk of the model, the resolution
step and the rules would need to visit the field, and the schema would gain a field for one form.

### A refusal alone

The rules could refuse every array whose length names a declaration, and the backend could refuse to
write one. dokimi would then refuse each check whose input has such a field. It refuses a sample
that a file cannot refer to in the same way.

## Drawbacks

- An Array with a length expression has more than one child. A consumer that reads the element of an
  array reads index 0, as each consumer in the repository does.
- The fold drops the names, so the shape of an array does not record them.
- The Go frontend version and the Rust frontend version rise, so each unit of both languages parses
  once after the release.
- The Go backend version rises, because a file that writes such an array now imports the package of
  its names.

## Open questions

None.

## References

| What | Where |
|---|---|
| dokimi issue 84, a derived input of an array field whose length is a constant of another package | <https://github.com/dokimasia/dokimi/issues/84> |
| RFC-0056, the references that a generated file writes | 0056-the-references-that-a-generated-file-writes.md |
| The Go specification, array types | <https://go.dev/ref/spec#Array_types> |
| The Rust reference, array types | <https://doc.rust-lang.org/reference/types/array.html> |
