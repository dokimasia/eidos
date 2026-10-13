---
rfc: 0041
title: The gates of a rule and the order of a plugin
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0041: The gates of a rule and the order of a plugin

## Summary

The gates of a rule admit a subject by a directive, by the presence of a fact, or by the value of a
fact. They do not select a subject on which a fact is absent, and they do not read the value that
triggers the rule. So eight plugins of the reference ensemble filter their matches inside the
handler, which the authoring rules forbid. We propose a negated gate and two value gates, which read
the value of the trigger. debugweaver and auditweaver trigger on emitted structs instead of emitted
methods. We also order errorgen and auditgen through the capability graph, and keep the priorities
without named values.

## Motivation

The authoring rules forbid a gate inside a handler. A handler that starts with a filter hides the
selectivity of its rule from the dispatch, and the dispatch then invokes the handler for every match
that the handler ignores.

The gates cannot state three selections of the reference ensemble:

- errorgen, auditgen and validategen run on each emitted method of a handled struct. Their handlers
  return unless the method is `ServeHTTP`.
- authgen, metricgen and tracegen run on each emitted function of a handled struct. Their handlers
  return unless the body of the function declares the named slot `chain`.
- debugweaver and auditweaver run on each emitted method of their subjects. Their handlers return
  unless the method has a receiver.

A rule also cannot select the subjects on which a fact is absent. Nested wrappers conjoin their gates
and two rules form a disjunction, so a plugin that needs the absence of a fact tests it in the
handler.

errorgen and auditgen set the priorities 1 and 2, so the call of errorgen precedes the call of
auditgen in the epilogue of `ServeHTTP`. A priority places a plugin against every plugin of its role,
and the two plugins need an order between the two of them alone. The kernel does not declare named
priority values, and a plugin that does not declare a priority runs at 0.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core` root package | `Not`, `NameEquals` and `HasSlot`. The test of a predicate reads the value of the trigger, and the dispatch enumerates a rule through its first gate that the fact index serves |
| `eidos-core/emit` | the generated function `NameOf` |
| `eidos-core/internal/gen/model` | the names template writes `NameOf` |
| `eidos-core/plugin` | the docblock of `CapabilityProvider` states the priority of a plugin that does not declare one |
| `eidos-sdk` | the facade re-exports the new functions |
| `eidos-reference` | `callee` returns rules with their value gates, the weavers trigger on emitted structs, and errorgen and auditgen order through `After` |
| `docs/architecture` | the authoring document and the plugin document name the gates and the order |

### Invariants

1. The matches of a rule are the subjects that its gates admit. A handler does not filter them.
2. The dispatch evaluates every gate before the invocation, untracked, because the evaluation of a
   gate is routing.
3. The dispatch enumerates a rule through the narrowest index that its gates admit: its directive,
   then its first key gate that `Not` did not negate, then its kind.
4. A negated key gate reads its key, so the subscription record of the rule contains the key. Build
   refuses an annotator that selects by the absence of a key of another annotator and does not run
   after that annotator, as it refuses a selection by the presence of the key.

### The negated gate

```go
// Not admits a subject that p does not admit. For HasKey, that is a
// subject on which the key reads absent. For KeyEquals, it is also a
// subject with another value. The fact index cannot enumerate a negated
// gate, so a rule whose key gates are all negated enumerates every subject
// of its kind. The zero Pred remains zero under Not, and a typed build
// refuses it. Not allocates the closure of the test, one allocation.
func Not(p Pred) Pred
```

Nested wrappers conjoin gates, two rules form a disjunction, and `Not` negates a gate. So a plugin
states any selection over keys and values as rules, one rule for each conjunction.

### Value gates

```go
// NameEquals admits a subject whose value has the name: the declaration
// that a node trigger runs on, or the emitted value that an OnEmit trigger
// runs on. For the empty name it returns the zero Pred, which a typed
// build refuses. NameEquals allocates the closure of the test over the
// name, one allocation.
func NameEquals(name string) Pred

// HasSlot admits an emitted function or method whose body declares the
// named slot. It does not admit a node declaration, because the node
// model does not record a body. The prologue and the epilogue are on every
// body, and an owner cannot declare a slot under their names. So for
// either name, and for the empty name, HasSlot returns the zero Pred,
// which a typed build refuses. HasSlot allocates the closure of the test
// over the name, one allocation.
func HasSlot(name string) Pred
```

On an `OnEmit` rule a key gate reads the facts of the origin, and a value gate reads the emitted
value. Nested wrappers conjoin the two:

```go
sdk.Where(sdk.HasKey(meta.Named[bool](handlergen.KeyHandler)),
    sdk.Where(sdk.NameEquals(handlergen.ServeHTTP),
        sdk.OnEmit(symbol.KindMethod, epilogue)))
```

`NameEquals` reads the name of a node declaration from its identity. It reads the name of an emitted
value through a function that the model generator writes beside `RespellNames`:

```go
// NameOf returns the name that s declares, and false for a kind without a
// name. It does not allocate.
func NameOf(s symbol.Symbol) (string, bool)
```

The dispatch changes in three places:

- The test of a predicate takes the value of the trigger beside the fact store and the subject.
  `HasKey` and `KeyEquals` do not read the value.
- `flatten` records the position of the first key gate that `Not` did not negate, and
  `enumerateFacts` enumerates the subjects of that key. A rule without such a gate enumerates every
  subject of its kind, as a bare rule does. An `OnEmit` rule enumerates the emitted values of its
  kind, as it does now.
- `subscriptionsFor` writes one record for each key gate, negated or not. A rule whose gates read no
  key gets one record without a key, as a bare rule does.

### The weavers

A method of a Go interface does not render a body, so debugweaver and auditweaver skip it by its
receiver. The emit model sets a receiver where the language writes one, as Go and Rust do. A
TypeScript or Java method has no receiver in the model, so a receiver gate would put a convention of
two languages into the neutral gates. The weavers trigger on the emitted struct instead, and weave
each of its methods:

```go
// Weave returns the rule of a weaver. On an emitted struct, it appends
// <package>.<func>(<format>, "<T>.<M>") into the prologue of each method,
// where T is the name of the origin of the struct and M the name of the
// method.
func Weave(o *Log) sdk.Rule
```

A weaver invocation covers one struct and checks its options once, so a weaver with empty options
reports `Missing` once for each struct instead of once for each method. The woven methods are the
same. debugweaver weaves the methods of `<T>Repo` and of the mock of `<T>Repository`, and auditweaver
weaves the methods of each emitted struct of a subject under `audit`.

`callee.Chain` and `callee.Epilogue` also return the rule with its trigger and its value gate, and
each plugin wraps the rule in its gate on the origin:

```go
// Chain returns the rule of a plugin of the middleware chain. On each
// emitted function whose body declares the slot of the chain, it appends
// h = <package>.<func>(h) into the slot.
func Chain(o *Options) sdk.Rule

// Epilogue returns the rule of a plugin that calls a function of the
// request. On each emitted method ServeHTTP, it appends
// <package>.<func>(w, r) into the epilogue.
func Epilogue(o *Options) sdk.Rule
```

The rule of validategen on emitted methods takes the gate `NameEquals(handlergen.ServeHTTP)` in the
same way.

### The order of a plugin

The kernel does not declare named priority values. A plugin that does not implement
`CapabilityProvider` runs at priority 0 in each role. Priorities are signed integers, so a negative priority runs before such
plugins and a positive one after them. The capability graph orders two plugins: `Requires` where the
provider must be present, and `After` where it may be absent.

errorgen and auditgen do not declare a priority. errorgen requires `http.handler`. auditgen requires
`http.handler` and runs after `http.error`. So the call of errorgen precedes the call of auditgen
where both plugins run, and auditgen still runs without errorgen.

### Failures

- A typed build panics on the zero predicate. `Not` returns it for the zero predicate, `NameEquals`
  for the empty name, and `HasSlot` for the empty name and the names of the two standard slots.
- A gate does not report a finding.

### Migration

1. Generate `emit.NameOf` and its test.
2. Add `Not`, `NameEquals` and `HasSlot`, pass the value to the predicates, record the enumeration
   gate and write the subscription records. Regenerate the facade.
3. Return rules from `callee`, gate validategen on the name, trigger the weavers on structs, and
   order errorgen and auditgen through `After`.
4. Name the gates and the default priority in the authoring document and the plugin document.

## Alternatives

### Named priority values

Named values, such as bands for foundation, composition, cross-cutting and finishing generators,
give the authors of independent plugins one scale. They also state a taxonomy whose order the
capability graph states already: an extension requires its host. Build refuses a plugin that
requires a capability whose provider has a later priority. A band that an author chooses by the
taxonomy instead of by the dependency makes that refusal more likely, and two plugins of one band
still run in the order of their names.

### A gate with a function over the value

One constructor, such as `Value(func(*emit.Method) bool)`, can test any field of the emit model. The
dispatch runs it before the invocation, but nothing about it is data. A tool cannot print it, and an
index cannot serve it. It moves the filter of the handler into the wrapper, which the authoring rules
call a scan.

### A receiver gate

`HasReceiver()` serves the weavers with a third value gate. The emit model sets a receiver for a Go
or Rust method alone, so the gate selects by the convention of two languages. A struct renders the
bodies of its methods in every target.

### An index of emitted values by name

The emit store could index its values by kind and name, so that `NameEquals` narrows the enumeration
of an `OnEmit` rule to its matches. Units arrive in the store in every turn of every plan, so the
store would maintain the index for every unit, for three rules that select by name. A value gate
costs one comparison for each emitted value of the kind, as a key gate on the origin does now.

### And and Or

Combinators over gates read as boolean algebra. Nested wrappers conjoin and two rules form a
disjunction, so the negation is the one missing operator.

## Drawbacks

- `Pred` gains two flags, and the test of every predicate takes one argument more.
- A rule whose key gates are all negated, or whose gates read only the value, enumerates every
  subject of its kind.
- A weaver invocation covers a whole struct, so a warm run that reruns it rebuilds the calls of every
  method of the struct.
- The root package gains three functions and the emit package one generated function. Eight
  reference plugins and the package `callee` change.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan rows W5.K12, W5.K14 and W5.K15.
- The reference ensemble, with the priorities of errorgen and auditgen and the triggers of the
  weavers: RFC-0026.
- The order of a plugin apart from the presence of its provider: RFC-0030.
