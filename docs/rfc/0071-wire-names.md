---
rfc: 0071
title: Wire names
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-19
updated: 2026-10-19
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0062
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0071: Wire names

## Summary

A generator that writes the types of a wire format writes field names that
a consumer matches by string, such as the keys of a JSON response. The
settle respells every field name in the convention of the target, so a key
such as `__typename` reaches a TypeScript file as `typename`. This RFC
gives an emit field the flag `Wire`. The settle keeps the name of a wire
field as the generator wrote it, and a target writes the name bare or
quoted, or refuses it where the target cannot write it.

## Motivation

The TypeScript respell, `spell.Name` in `eidos-lang-typescript/spell/name.go`,
spells a field in camel case. It passes a name that is not an identifier
through unchanged, such as `content-type`, because a consumer matches such
a key by string. A key that is an identifier has no such protection:

| Key of the wire format | The name that the settle writes |
|---|---|
| `__typename` | `typename` |
| `first_name` | `firstName` |
| `ID` | `id` |

The GraphQL client of the conformance fixture `catalog` writes the
TypeScript types of the JSON of each response. Its function `keepKey`, in
`eidos-conformance/lang/graphql/client.go`, predicts the respell with the
camel case of `naming.New()`, the conversion that the TypeScript respell
uses. Where the prediction changes the name, `keepKey` turns the
field into a computed property whose name is the key as a string literal,
which the respell passes through. The result is valid TypeScript, with
three costs:

- The generator copies the convention of the target, so it breaks when the
  convention changes.
- The file quotes a key that is an identifier, as `"__typename": "Product"`.
  graphql-codegen writes the key bare, as `__typename: 'Test'` in the
  specification of its plugin for operations.
- Every generator of the types of a JSON format repeats the workaround.

One GraphQL object type can render as the TypeScript interface of a
response, whose keys are wire names. A plugin of the same plan can render
it as a domain class, whose properties follow the convention. An override
on the origin applies to both declarations, so the generator that emits a
declaration marks its wire names.

## Detailed design

### The flag

The schema of the field, `eidos-core/symbol/schema/member.go`, gains one
emit field with a fact:

```go
// Wire marks a name that is a key of a wire format, which a consumer
// matches by string, such as a key of a JSON object. The settle keeps
// the name as the generator wrote it, and the target writes it as it
// is or refuses it.
Wire bool `eidos:"emit,fact=Wire"` // the name is a key of a wire format
```

The generator regenerates `emit.Field` with the field, `symbol.FactWire`,
and the walk that reports the fact. The node side has no such field,
because a source states a key through its own syntax: a protobuf
`json_name`, a Go struct tag, a GraphQL field name.

### The settle

The recording callback of `planNames` reads the flag on the carrier. A
wire field takes its emitted name as its settled name. It skips the
respell hook and the override of its origin:

| Name | Settled name |
|---|---|
| a field with `Wire` | the emitted name |
| any other name | the spelling of the respell hook, or the override of the origin |

The override does not apply, because the key is what the consumer matches,
and a renamed key breaks the consumer. A wire field joins the member check
of its host as any field does.

### The targets

Each backend declares a verdict for `FactWire` in its coverage table:

| Target | Verdict | The name in the file |
|---|---|---|
| TypeScript | Renders | bare where it is an identifier, and quoted otherwise, as `memberKey` writes a property key |
| GraphQL | Renders | the name, where it is a GraphQL name that does not begin with `__`. The lowering refuses any other name, because a schema cannot declare it |
| protobuf | Renders | the name, where it is an identifier. The lowering refuses any other name |
| Go | Rejects | a Go file writes a key in a struct tag, and the case of a Go name states its visibility |
| Java | Rejects | a Java file writes a key in an annotation |
| Rust | Rejects | a Rust file writes a key in a serde attribute |

A rejection is a positioned Error of the coverage guard, `RejectedFact`,
and the backend withholds the declaration. A refusal of the GraphQL or the
protobuf lowering is a positioned Error under `RefusedConstruct`, which
withholds the declaration too.

### The client of the conformance fixture

`properties` marks every property that it returns as a wire field, in
place of `keepKey`:

```go
for _, f := range out {
	f.Wire = true
}
```

`clientSum` finds the key `__typename` by its name. The golden
`client.ts` of the fixture `catalog` writes `__typename: "Product"`.

### Migration

- The schema, the generated models, the SDK facade and the fact vocabulary
  regenerate.
- Every coverage table gains the verdict, and `backendtest` checks that
  every table is total, as it checks every fact.
- The GraphQL and protobuf backends check a wire name in their lowerings,
  which run in the settle before the respell.

## Alternatives considered

### The workaround in each generator

A generator keeps writing a quoted computed name where the respell would
change a key.

**Why not:** the generator copies the convention of the target, and it
writes a quoted key where TypeScript admits a bare one.

### An override that the generator stamps

The settle could read a name that a generator stamps on the origin.

**Why not:** an override is a fact of the origin. One origin renders more
than one declaration, so a stamp on it cannot mark the interface of a
response alone. The settle also does not read a name at plugin authority,
because the lowering stamps a name on every declaration that a plan
renders.

### A respell that keeps every field name of TypeScript

The TypeScript respell could keep every field name.

**Why not:** a plan from Go into a TypeScript domain model expects camel
case. The convention is right for most fields, and a wire key is the
exception that the generator knows.

### A wire name of the source

A frontend could record the key that a source states, such as a protobuf
`json_name`. The settle would keep that key.

**Why not:** the source states the key, and the generator marks the
declarations that are types of a wire format. A generator that needs the
key of the source reads it and writes it as the name of a wire field.

## Drawbacks

- One emit field and one fact. The coverage table of every backend, the
  fact vocabulary and the SDK facade change.
- A wire name skips the check of reserved words in the respell hook. The
  backend writes a reserved word as a quoted key where its grammar requires
  one.
- A generator that marks a field of a Go, Java or Rust declaration gets a
  refusal and writes the key in a tag or an annotation instead.

## References

| What | Where |
|---|---|
| graphql-codegen, the specification of its plugin for operations | <https://github.com/dotansimha/graphql-code-generator/blob/master/packages/plugins/typescript/operations/tests/ts-documents.spec.ts> |
| The JSON mapping of protobuf, which writes `json_name` | <https://protobuf.dev/programming-guides/json/> |
