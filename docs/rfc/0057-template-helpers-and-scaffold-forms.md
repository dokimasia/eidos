---
rfc: 0057
title: Template helpers and scaffold forms
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-13
updated: 2026-10-13
discussion: none
supersedes: none
superseded-by: none
produces-adr: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0057: Template helpers and scaffold forms

## Summary

The comparison with the previous version lists nine helpers and forms that a plugin of eidos writes
by other means: template helpers for argument lists, values, qualified names and assertions,
constructors of type references, a selector, three forms of the scaffold, shared builders of test
doubles and a check of the result names of a mirrored method. Each consumer of the reference
ensemble writes its output without them. We propose the verdict out for each, with the consumer
that would reverse it. We also correct the rendering document, which describes a fluent builder
that eidos does not have.

## Motivation

- The improvement plan lists each item as a capability without a counterpart.
- The proposed decision D-4 of the plan, a selector expression in the scaffold, waits on a record.
- The rendering document describes the construction of emit values as "a typed, fluent Go API".
  A plugin constructs emit values as composite literals of the generated kinds, and no method
  chains them.

## Detailed design

### Verdicts

| Row | Capability | Verdict | Reason | Reverses the verdict |
|---|---|---|---|---|
| W5.G31 | Helpers that write an argument list, the variadic spread included, and the fields of a recorded call | out | mockgen and stubgen claim their files through one template of each target, and each template writes the arguments in the syntax of its target: `x...` in Go and `...x` in TypeScript | A template that serves two or more targets and passes the parameters of a callable on |
| W5.G32 | Shared builders of the delegate bodies, the recorded calls and the satisfaction checks of a test double | closed | The package `internal/double` of the reference ensemble builds the methods of both doubles. The bodies, the call records and the satisfaction checks differ in each target: a Go stub appends to a slice and asserts `var _ I = (*T)(nil)`, and a TypeScript stub pushes into an array and assigns to a constant of the interface | A third generator of doubles that writes the same templates |
| W5.G33 | A template helper that writes a value | out | The builder places each sample through the body of a method of its test record, which the settle respells together with the builder, and the scaffold writes the value with its imports. A helper would write the value and still leave the names to the record | A template that places a value outside a body, such as an entry of a table of samples |
| W5.A5 | Constructors of pointer, list, map, function and external type references that leave the spelling to the target | out | The plugins that write such references, handlergen, repogen, validategen and builder, write Go declarations, such as a handler that takes `*http.Request`, and the ensemble runs them in its Go plan alone. Each states the Go spelling beside the structure. A plugin of two or more targets translates a type through `Emitter.Type`, whose spoke chooses the spelling under the policy of the plan, because a list is an array or a collection type in another target | A plugin of two or more targets that builds a composite type that no source declares |
| W5.A6 | A selector expression in the scaffold (D-4) | out | The plugins that write a member access in a scaffold body, builder, handlergen and the plugins of the package `internal/callee`, write Go declarations and run in the Go plan alone, and each target of the ensemble writes a member access with a dot. The names that they join are declarations that the settle does not rename: a field of the source and a field that the plugin names | A plugin of two or more targets that writes a member access in a scaffold body |
| W5.A7 | A return of several values, content after a named slot and a neutral function type in the scaffold | out | repogen returns its zero values through the template `repo.tmpl`, middlewaregen places its slots before `return h` through `middleware.tmpl`, and mockgen and stubgen write their function fields through a template of each target. The scaffold vocabulary stays small, so logic belongs to a template or a hand-written file. The fourth form, a conversion to a pointer type, is a defect of the Go scaffold that RFC-0056 fixes | A body that a target writes from the scaffold alone and that needs one of the forms |
| W5.A8 | A helper that qualifies a reference in a claimed file template | closed for registrygen, out across families | registrygen imports the package of its callee through the builtin `use`, which binds the name in the name assignment of the file and refuses a name that the file binds otherwise. A reference from one emitted declaration to another in a file that the layout routes into another package needs the package of the routed file, and no identity of an emitted declaration has it. stubgen and validategen state the limit in their docs | A plan that routes two families of one plugin into different packages |
| W5.A10 | An assertion dialect for generated tests in the vocabulary of each language | out | The generated Go tests assert through the package `testing` of Go, which needs no import beyond it, and the TypeScript test of a stub is a check of tsc. No consumer runs its generated tests under another assertion library | A consumer whose generated tests use another assertion library |
| W5.A15 | A check that the result names of a mirrored method are free in its body | out | The bodies of the mirrored methods of the ensemble declare no name: a double calls its function field, and a weaver appends calls and assignments without a declaration | A weaver that declares a name in the body of a mirrored method |

### The rendering document

The section on the builder API of the rendering document states that a plugin constructs emit
values as composite literals of the generated kinds, which the compiler checks, and that templates
and composite literals produce the same values.

### Migration

1. Correct the section on the builder API of the rendering document.

## Alternatives

### The helpers and forms of the previous version

eidos could carry each helper and form of the previous version. Each would be public API that the
plugins of the ensemble do not call, and the vocabulary of each of the four backends would grow by
it.

## Drawbacks

- A plugin of two or more targets that needs one of the items waits for its record and its code.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan rows W5.G31, W5.G32, W5.G33, W5.A5, W5.A6, W5.A7, W5.A8, W5.A10 and
  W5.A15, and the proposed decision D-4.
- D36 in [21](../architecture/21-decisions.md), the scaffold vocabulary.
- [RFC-0037](0037-five-contracts-that-the-reviewed-defects-need.md), the binding of a template
  import.
- [RFC-0056](0056-the-references-that-a-generated-file-writes.md), the conversion of the Go
  scaffold.
- [07](../architecture/07-rendering.md), the rendering document.
