---
adr: 0044
title: The lowering seams keep the departures of their implementation
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0011
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0044: The lowering seams keep the departures of their implementation

## Status

Accepted

## Context

RFC-0011 designs the lower seam and the respell seam of a backend, the coverage table of the emit
facts, and the settle that runs both hooks. The Go, TypeScript, Java and Rust backends implement
it. Later accepted designs decide 18 places where the code departs from the design. This record
keeps 16 other departures and fixes two defects. The throws lowering of Go and Rust changed the
results of a callable and left its body as written. The Java and Rust backends refused the
annotations of a parameter, which both languages spell. The decisions are recorded here, because
an accepted design is not amended.

## Decision

We keep the code as built where it departs from RFC-0011. The throws lowering refuses a callable
whose body it would have to change, and the Java and Rust backends write the annotations of a
parameter.

### The throws lowering refuses the body that it does not rewrite

`lowering.Callables` returns an error under the prefix of the language for a callable with
failures whose body has verbatim text or scaffolding statements, or a slot with a return
statement at any depth. The callable keeps its failures, and the settle withholds the
declaration. The default body has no statement and passes. A body that a template renders also
passes, because the template reads the lowered results.

RFC-0011 lets an idiom rewrite a structured body. The scaffolding return statement has one
expression, so the vocabulary cannot state the error value that Go appends. The lowering refuses a
structured body as it refuses a verbatim body.

### Departures that later designs decide

| Design | Code | Record |
|---|---|---|
| An enum becomes a defined type and a constant group in a Go plan | The Go lowering returns a defined type over the underlying type of the enum, `int` by default, and one constant for each variant | RFC-0049, RFC-0036 |
| Both hooks run in the settle | The lowering entry of a target calls the respell hook during annotation and stamps the name key of the target. The settle replaces that name with an override at directive authority or above | RFC-0030, RFC-0019, RFC-0020 |
| No code in the kernel converts case | `Identity.FlatName` upper-cases the first letter of each joined part, and so does `Emitter.JoinName` | RFC-0024, the decision log |
| The coverage table has three verdicts | A fourth verdict, `Rejects`, reports an Error and leaves the declaration out. `Optional`, `Computed` and `Underlying` have rows | RFC-0036 |
| Go refuses Level, Mutability, Override, HasDefault and Async | Level, Override, HasDefault and Async reject. The mutability of a field or a variable renders, and that of a method rejects | RFC-0036, ADR-0028 |
| The table places every fact, and it has no row of embeds | TypeScript, Java and Rust refuse the embeds of a struct | RFC-0031 |
| A Go plan refuses a sum | The Go lowering returns an interface with a marker method and one struct for each variant | RFC-0024 |
| The lower seam has five cells | Go and Rust also refuse a method name that a host declares twice | RFC-0031 |
| The model has no array, map or pointer structure, and composite spellings remain | A reference has a form and children. The settle and the Go speller rewrite a composite spelling whose child changes | RFC-0014, RFC-0019, RFC-0037, RFC-0056 |
| The dialect of the builtin types is the generator's | The type speller of each backend spells a canonical shape, builtins included | RFC-0014, RFC-0030 |
| No backend qualifies a bare resolved reference at render | After the settle, the layout sets the package of a reference into another package, and the speller qualifies it through the import set | RFC-0014, RFC-0019, RFC-0020, RFC-0056 |
| A backend without a hook renders declarations as emitted | `plugin.Backend` declares both hooks, and the settle runs for every backend | RFC-0030 |
| The settle runs once per plan over the whole graph | A warm run settles the units of its dirty groups against the names of the kept files | RFC-0020 |
| Review keeps a slot from a write after the schedule | The store seals each slot of a placed declaration, and a direct append panics until the settle starts | RFC-0030 |
| Nothing downstream joins two spellings of one fact | An export keys on emitted names and records the settled spelling | RFC-0019, ADR-0010 |
| The name tables map an origin to its settled name | Both tables key on the origin and the emitted name, and a resolved reference follows where its spelling equals that name | RFC-0020 |
| A hook returns the declarations that it rewrites | Go and Rust rewrite a callable in place | RFC-0030, RFC-0031 |
| Collisions are checked per package and per host | The records of a family that claims its files skip the check and the tables | ADR-0019 |

### Departures without an earlier record

| Design | Code | Reason |
|---|---|---|
| A Java sum becomes a sealed interface and its permitted records | The Java lowering returns a sealed interface and one final class for each variant | A class keeps each field of the payload as the payload states it, with its visibility, its mutability and its annotations. A record makes every component private and final |
| A refusal is at the declaration's position | Every finding of the settle is at the routing key of its unit, and a refused member name withholds the whole declaration of the file | An emit declaration has no position. A declaration without one of its members states another contract than the generator emitted |
| The kernel learns no language | The settle finds a settled name inside a spelling by the qualifier marks of Go, Java, TypeScript and Rust | A composite spelling contains the names of its children, and the settle rewrites each name that it settled |
| The coverage table places every fact of the emit model | The table of RFC-0011 lists the facts of its time. Each backend states a verdict for every fact of the generated list, which later designs extended | A coverage table that a generated list drives cannot miss a fact |
| Go declares no annotations | The Go backend writes each annotation as a `//go:` line | The Go frontend records each `//go:` directive as an annotation, so the backend writes it back |
| The cells of TypeScript, Java and Rust | TypeScript refuses annotations outside a class and its members, and on a parameter. TypeScript and Java refuse a method outside a type. Java writes interface properties as constants and the types of an interface as member types. Rust refuses Implements, and the level of a field or a struct. TypeScript refuses the level of a struct | A standard TypeScript decorator applies to a class and its members alone. Each other refusal is a construct that the target does not spell, and each rendering is the form that the target has |
| The lower seam has five cells | The Go lowering also sets the receiver of each method of a struct. TypeScript refuses two members of one name that the language does not allow. Java refuses a file-level type that is private, protected or static, and Rust refuses a method without a receiving type | The Go template attaches a method through its receiver. TypeScript allows one property, one getter and one setter of each name, and one implementation of a method of a class. Java declares those modifiers on member types alone. Rust declares a method inside the impl block of its type |
| Every returned declaration has the origin of the input | A nil list keeps the declaration, rewritten in place or not, and the origin check runs on a returned list | A rewrite in place keeps the declaration and its origin |
| A companion is named in the neutral form, and the respell spells every name once | The TypeScript lowering names each variant interface in its final form, and the respell spells it again | The union of the variants is a composite spelling, which the settle does not rewrite. The respell of a final TypeScript name returns the same name |
| A suite assertion pins that a second settle changes nothing | `Settle` returns at once on a settled store, and the suite has no check of a second settle. The tests of each lowering check that it consumes the fact that it lowers | A store settles once, so no hook can run twice on it |
| An error means that the target cannot spell a name at a visibility, and each hook uses its own initialisms | The four hooks use the shared caser `naming.Default`, and each also refuses a name that is not an identifier or that is a reserved word | One set of initialisms gives one name the same words in every target. A reserved word or a name outside the grammar does not compile |
| The settle passes every declaration to the lowering | The settle passes the declarations of a file alone, and the lowering of a declaration covers its members | A member lowers with its host, because a hook reads one declaration |
| Collisions are checked per package and per host | A method with a receiver collides within its receiver alone | A Go method with a receiver is in the method set of its receiver type, and not in the scope of the package |
| The kernel enumerates no field list of its own | The settle and the backend suite walk the statements and the expressions of a body with code of their own | A name in a body resolves against the parameters, the results and the earlier declaring assignments, in statement order. A walk of the generated field lists does not track that scope |
| One fixture renders `FetchRow` in four cases | Every canonical name of the fixture is one word. Each satellite tests the case of a joined name in the tests of its respell | A name of one word renders the same in each case, so the bytes of the fixture do not depend on the caser |
| Structured bodies may change, and verbatim bodies refuse | Both refuse, as the preceding section states | The scaffolding return statement has one expression |

## Alternatives Considered

### Rewrite the structured body

The Rust lowering could wrap each returned value in `Ok`, and Go could append `nil` to each return
statement. A Go return of two values needs a statement with two expressions, which the vocabulary
does not have. A rewrite in Rust alone would give the two targets different answers to one emit
graph.

### Implement each design as written

eidos could remove the rows that later designs added, write Java records, and refuse Go
annotations. Each change removes a construct that a target spells and that a later design
decided.

## Consequences

- A generator that declares failures on a callable writes its body through a template, or emits
  the lowered results itself.
- A Java or Rust plan writes the annotations of a parameter before the parameter.
- RFC-0011 has its departures in this record.

## References

- [RFC-0011: Neutral emit and the target lowering seams](../rfc/0011-neutral-emit-and-the-target-lowering-seams.md)
- [RFC-0014: The projection vocabulary and the rules seam](../rfc/0014-the-projection-vocabulary.md)
- [RFC-0019: Source scopes, plan exports and workspace checks](../rfc/0019-source-scopes-exports-and-checks.md)
- [RFC-0020: Warm runs, the sealed state and re-execution by artifact](../rfc/0020-warm-runs-and-the-sealed-state.md)
- [RFC-0024: Protobuf editions and a service in Go and TypeScript](../rfc/0024-protobuf-editions-and-services.md)
- [RFC-0030: The contracts of the SPI and the authoring API](../rfc/0030-the-contracts-of-the-spi-and-the-authoring-api.md)
- [RFC-0031: The shared helpers of the satellites](../rfc/0031-the-shared-helpers-of-the-satellites.md)
- [RFC-0036: The facts that the model drops](../rfc/0036-the-facts-that-the-model-drops.md)
- [RFC-0037: Five contracts that the reviewed defects need](../rfc/0037-five-contracts-that-the-reviewed-defects-need.md)
- [RFC-0049: Go enumerations](../rfc/0049-go-enumerations.md)
- [RFC-0056: The references that a generated file writes](../rfc/0056-the-references-that-a-generated-file-writes.md)
- [ADR-0010: An export entry has a five-part key and no signature](0010-export-entries-have-a-five-part-key.md)
- [ADR-0019: A family claims its files through a file template](0019-a-family-claims-its-files-through-a-template.md)
- [ADR-0028: The facts of the model keep the departures of their implementation](0028-the-facts-of-the-model-keep-the-departures-of-their-implementation.md)
- [The decision log](../architecture/21-decisions.md), for the join of a name in the authoring API
