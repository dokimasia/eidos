---
adr: 0047
title: The cross-language hub keeps the departures of its implementation
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0023
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0047: The cross-language hub keeps the departures of its implementation

## Status

Accepted

## Context

RFC-0023 designs the spokes of the four backends, the lowering policies, the TypeScript satellite
and the support matrix. Later accepted designs decide 13 places where the code departs from the
design. This record keeps eight other departures and fixes three defects:

- `meta set=typescript.int64 value=long` passes validation, because a string key takes any text.
  The TypeScript spoke then writes the type `long`.
- `NewPolicy` returns the first defect of each spec, and the design joins one error for each fault.
- The hub has no probe of the integers of 8 and 16 bits or of the float of 32 bits.

The decisions are recorded here, because an accepted design is not amended.

## Decision

We keep the code as built where it departs from RFC-0023, and fix the three defects.

### A key of the registry can close its values

`meta.KeySpec` has the new field `Choices`. A key of the type string with choices accepts only the
values that `Choices` lists. Registration refuses choices on a key of another type, an empty choice
and a choice listed twice. Validation reports `BadSpelling` with the choices when a meta directive
sets the key to another value. The workspace registers the key of each policy with the choices of
the policy, and the fingerprint of the composition folds the choices of each key.

RFC-0023 states that validation refuses a value outside the choices of a policy, and its design of
the target directive gives the params their choices. The meta set form of RFC-0039 writes the same
key without the target directive. The choices of the key close both paths, and a directive of a
plugin can close a key of its own the same way.

### The other fixes

- `NewPolicy` returns every defect of a spec, joined in the order of its checks. A key reports its
  first defect alone, because each check of a key assumes the one before it. The check of the
  default against the choices runs only for a spec with choices.
- The hub probes the integers and the unsigned integers of 8 and 16 bits and the float of 32 bits.
  Each spoke spells them as the table of RFC-0023 states.

### Departures that later designs decide

| Design | Code | Record |
|---|---|---|
| TypeScript declares four policies, and its spoke refuses a duration | TypeScript declares five. `typescript.duration` takes `string` or `number`, with the default `string` | RFC-0024 |
| The keys, the directive and the lowering entry exist where the backend respells or declares policies | The target of every enabled plan has a lowering entry, a name key and the `name` param | RFC-0030 |
| A backend can lack the type speller and the policy provider | `plugin.Backend` declares both roles, and the workspace always passes the backend as the type speller | RFC-0030 |
| A translated reference has the declared name of the referent | Each spoke spells the flat name of the referent | RFC-0024 |
| The table lists 30 forms | Each spoke also spells or refuses the top type, the empty value, the error value, and read-only lists, tuples and maps | RFC-0024, RFC-0042, RFC-0036 |
| Go spells an optional as `*T` and refuses an asynchronous stream | Go writes an optional of a list, bytes, a map, a function, a stream, the top type or a sum as the type itself, and an asynchronous stream as `iter.Seq2[T, error]` | RFC-0024 |
| TypeScript spells ten global types | It spells 23, among them `Readonly`, `ReadonlyMap` and eight error classes | ADR-0028, RFC-0042 |
| A reference folds under the rules of its owner's language | The fold of the type of a field adds the presence of the field, and the rules of the target decide a well-known type | RFC-0024, RFC-0037 |
| The TypeScript frontend registers `typescript.optional` | The satellite registers no such key | RFC-0036 |
| A nested type gets no name stamp | Rule 0 stamps the respell of the flat name of a nested type | RFC-0024 |
| The TypeScript rules have 17 methods of six interfaces | They have 16 methods of seven. The kernel substitutes, `DeriveSamples` replaces `SamplesOf`, `ErrorContract` is new, `any` and `unknown` fold to the top type, `Error` folds to the error value, and resolution follows `paths` and `baseUrl` | RFC-0031, RFC-0030, RFC-0042, RFC-0024, RFC-0054, RFC-0038 |
| No satellite has a Tier-3 package | The root package of each satellite is its Tier 3 | RFC-0059 |
| The support matrix has four tables | It has five, and the fifth is the projection side | RFC-0062 |

### Departures without an earlier record

| Design | Code | Reason |
|---|---|---|
| `Spelling` is the whole spelling in the target, such as `Record<string, bigint>` | A named reference has the bare name in `Spelling` and its arguments in `Args` | The emit model requires that form, so the settle and the import set read each argument |
| The Go frontend and the Go annotator both register the Go keys, and the annotator has the name `golang` | The annotator is `golang-facts`. It looks the keys up and registers none, so Build fails for an annotator without the Go frontend | The frontend registers the keys that its units stamp. A plugin name differs from the name of the backend, so neither registers the namespace of the other |
| Go spells an optional as `*T` | Go writes an optional of the error value as the type itself | An error value is nil where it is absent |
| TypeScript spells ten global types | `AsyncIterableIterator`, `AsyncGenerator` and `AbortSignal` are also global types of the speller | Each is a global of the TypeScript libraries that a module can hide, so the speller refuses it where a declaration of the module hides it |
| The policy step runs after the plans compile | Build resolves the policies before the lowering entries, the registries and the plan step | The lowering entry leaves out the policies of a backend whose specs are defective, so it needs the result of the policy step |
| A nil subject with a marker of the brand makes `AttachSugar` panic | It panics for a marker that attaches. A refused marker reports and returns before the attach | A refused marker attaches nothing, so it needs no subject |
| The policy specs and their constants are in `lang.go`, `keys.go` and `literal.go` | They are in `policies.go` | A file contains one concept |
| `Properties` pairs each getter with its setter, and `Constructors` returns the constructor signatures of a class | `Properties` reads the getters of an instance alone. `Constructors` returns the public signatures alone, and none where every constructor is private or protected or the base is not a class of the view | A static getter is not a property of an instance, and generated code can call only a public constructor |

## Alternatives Considered

### Check the choice in the policy

`Policy.Choice` could compare the override with the choices of its spec and fall back to the
resolved choice. The spoke would then write the resolved choice, and the author would not get a
finding for a value that the run ignores.

### Refuse `meta set` of a policy key

Validation could refuse a set of a policy key and leave the target directive as the one way to
override a policy. The meta set form writes every key that its registrant does not derive, and a
rule for one family of keys would make it irregular.

## Consequences

- A value outside the choices of a policy reports `BadSpelling` at the carrier through either
  directive.
- A run after a change of the choices of a key runs cold.
- RFC-0023 has its departures in this record.

## References

- [RFC-0023: The cross-language hub and the TypeScript satellite](../rfc/0023-the-cross-language-hub.md)
- [RFC-0024: Protobuf editions and a service in Go and TypeScript](../rfc/0024-protobuf-editions-and-services.md)
- [RFC-0030: The contracts of the SPI and the authoring API](../rfc/0030-the-contracts-of-the-spi-and-the-authoring-api.md)
- [RFC-0031: The shared helpers of the satellites](../rfc/0031-the-shared-helpers-of-the-satellites.md)
- [RFC-0036: The facts that the model drops](../rfc/0036-the-facts-that-the-model-drops.md)
- [RFC-0037: Five contracts that the reviewed defects need](../rfc/0037-five-contracts-that-the-reviewed-defects-need.md)
- [RFC-0038: The modules of an import specifier](../rfc/0038-the-modules-of-an-import-specifier.md)
- [RFC-0039: The meta set form, parameter defaults and name suggestions](../rfc/0039-the-meta-set-form-parameter-defaults-and-name-suggestions.md)
- [RFC-0042: The error contract of a type](../rfc/0042-the-error-contract-of-a-type.md)
- [RFC-0054: TypeScript declarations that the model loses](../rfc/0054-typescript-declarations-that-the-model-loses.md)
- [RFC-0059: Package structure and the facade](../rfc/0059-package-structure-and-the-facade.md)
- [RFC-0062: The projection side of the support matrix](../rfc/0062-the-projection-side-of-the-support-matrix.md)
- [ADR-0028: The facts of the model keep the departures of their implementation](0028-the-facts-of-the-model-keep-the-departures-of-their-implementation.md)
