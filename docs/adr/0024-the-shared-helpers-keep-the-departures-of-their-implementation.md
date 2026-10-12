---
adr: 0024
title: The shared helpers keep the departures of their implementation
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0031
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0024: The shared helpers keep the departures of their implementation

## Status

Accepted

## Context

The accepted design moves the duplicated mechanics of the satellites into the kits of the kernel and
into eidos-lang. It states every function that the satellites need from those packages, so each
package changes once. It also states four invariants:

1. A language fact belongs to its satellite, and a shared helper takes the fact as a parameter.
2. A satellite keeps its exported API, except the `TypeName` method that the SPI drops.
3. Every helper states its allocation contract, and a helper on a hot path allocates no more than
   the copies that it replaces.
4. Behaviour that a user sees changes only where the design specifies the change.

The implementation met a function of the design that did not fit the code in 32 places. A
signature crossed the direction of the imports, a helper allocated more than the copies that it
replaced, or the copies differed in a rule that the design took for a shared one. Each such place is
listed here, because an accepted design is not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departures are these.

### Helpers that the satellites do not get

| Design | Code | Reason |
|---|---|---|
| `frontend.Generated`, a classifier for the marker of a foreign generator | Each frontend stamps its `GeneratedKey` in its parse | Each parse reads the marker in the tree that it holds, and the classifier would read every file again. Java recognises two annotations, which a marker function of one result cannot report |
| `numeric.Syntax` and `numeric.Parse` | Each frontend keeps its decoder | TypeScript reads a double, Java a two's complement int or long, and Rust an exact integer. A `Parse` that returns a float64 loses the precision of the last two |
| `backend.Builder.Statements` | Each backend keeps its exported `Scaffold` | The option takes the types of `eidos-lang/scaffold`, and the kernel imports nothing of eidos-lang |
| `scaffold.Base` | Each backend keeps its two methods of one line | A target that embeds `Base` has two fields, and Go allocates when it converts such a struct to an interface. The current target is one pointer, which converts without an allocation |
| `lowering.VariantName` | Each lowering joins the names | The body is one expression |
| `rules.EnumBuilder` | Each rules package keeps its `EnumOf` | The three languages differ in the zero value, in the reading of a numeric value and in the conversion of a value out of range. A builder would take each step as a callback, and each language would keep its loop |
| The protobuf rules through `rules.Resolve` | The protobuf rules keep their resolution walk | Protobuf resolves oneof fields, services and rpcs, and it has no package variables |
| `Emitter.Fields` in the reference ensemble | No reference generator changes | No generator of the ensemble translates fields through the Emitter |

### Helpers with another signature

| Design | Code | Reason |
|---|---|---|
| `plugin.UserHomeDir(getenv)` | `plugin.UserHomeVar()` returns the name of the variable | Each store locator reads the variable through its own getenv, and its error contains the name of the variable that is unset |
| `lowering.Callables(lang, s, fold) (bool, error)` with a fold of the returns and the failures | `Callables` returns an error, and its fold also takes the name of the callable and returns an error | The Rust fold refuses a failure through the error, and the message of the refusal contains the name of the callable |
| `rules.Resolve(r, b Bound, scope, name, kind)` | `Resolve(r, src SourceRules, v View, scope, name, kind)` | A `Bound` allocates its memo on each call, and the resolution walk needs no memo |
| `Resolver.Usable` with the own type of the subject | `own` is the zero identity unless the walk searches the own type | A member that only the own type can use is then usable on the own type alone |
| `toolchain.Run` under ctx and env | env lists the variables that the run sets on top of the environment of the process, and nil keeps the environment | The Go and Rust adapters add variables to the environment, and the TypeScript and Java adapters keep it |
| `Emitter.Fields(fields, keep)` | A nil keep keeps every field | Three of the four generators translate every field |

### Contracts that the code states otherwise

- `Tree.Problems` returns nil for a clean tree, so a frontend skips the syntax report of a clean
  file. A caller ranges over the result only where it is not nil.
- `Node.Bytes` returns the source of a node without a copy, so a comparison of a token allocates
  nothing. The design has no such method.
- `SourceUnit.ReportSyntax` allocates four objects for its loop state, because the range over the
  iterator moves the loop body and its counters to the heap.
- `GraphBuilder.StampModule` boxes a module and its root once for the packages of one module.
- `plugin.FuncType` copies the parameters into a new list where the function type has results, so
  it never writes into the spare room of the list that the caller passed. The copy costs one
  allocation where that list had room.
- `frontend.TestFiles` allocates the list of packages and the list of stamps, two objects for a
  unit.
- The toolchain helpers state no allocation contract. Each call starts a process or writes files on
  the cold path of a test.
- `toolchain.Run` reports the end of its context and the end of `RunTimeout` through one error,
  which wraps the cause of the context. The cause of the bound wraps `context.DeadlineExceeded`.
- `toolchain.Sources` matches the end of a name, so a suffix such as `.test.js` or `Test.class`
  selects files. It passes over a directory of skip below dir, and never over dir itself.
- `spellref.SpellRestated` leaves the Go backend its walk that checks inline imports, and the
  qualifier of the backend keeps its two refusals of a name that is not a Go type name. The
  constants of the Go root are unexported, so the backend declares its own pointer and variadic
  marks.

### Behaviour that changes beyond the design

- `treesitter.Comments.Leading` passes a sibling that `Passes` reports also where it shares the line
  of the comment. A TypeScript decorator and a Rust visibility on the line of a comment change.
- `render.Coverage.RefusesEmbeds` warns `RefusedFact` for an embed and its facts, and still renders
  the declaration without them.
- The TypeScript frontend reports a `package.json` that it cannot read under a new code,
  BadPackage.
- The Go rules resolve a declaration that shadows a predeclared type before the placeholder of the
  predeclared type.
- `toolchain.WriteFile` refuses every path that is not local to its directory, an absolute path
  included. The adapters wrote an absolute path of a fixture under the scratch directory before.
- The Rust satisfaction check returns the error of a failed restore of its crate root.
- Rust stamps the module of a package at its first file, and Java stamps it after the lowering.
- Messages change where the shared helper writes them: the arity message of a map no longer contains
  the key and the value, `lowering.NamedPayload` writes one message for every language, the resolution walk
  writes one message for the kinds of a type, which adds Sum to the TypeScript message, and the
  messages of the Go restatement, the Java store and dependency reads and the Rust module checks
  are new.

## Alternatives Considered

### Build the design as written

Each function would exist with the stated signature. It lost for each row for the reason in the
row. Two of the rows, `Statements` and `Base`, would break the direction of the imports or add an
allocation to every conversion of a target. Two others, `numeric.Parse` and `EnumBuilder`, would make
a language fact a shared rule, against the first invariant.

### Write a second design

A new design would state the functions as built. It lost because the departures change no
contract between modules beyond the ones listed, and the code already states each of them in its
docblocks.

## Consequences

**Positive:**

- The satellites share the mechanics without a cycle in the imports and without an allocation that
  the copies did not have.
- Each language keeps the facts that differ between languages, such as the precision of a numeric
  literal and the rules of an enumeration.

**Negative:**

- A reader of the design finds 8 helpers that the code does not have, and 6 that have another
  signature.
- Eight behaviours that a user sees change beyond the ones that the design specifies.

**Neutral:**

- The protobuf rules keep their own resolution walk beside the shared one.

## References

| What | Where |
|---|---|
| RFC-0031, the shared helpers of the satellites | [0031-the-shared-helpers-of-the-satellites.md](../rfc/0031-the-shared-helpers-of-the-satellites.md) |
