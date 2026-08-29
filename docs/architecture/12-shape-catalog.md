# The shape catalog (eidos-plugin-shape)

*Builds on: [03](03-projection.md) (Callable and Resolve),
[05](05-directives.md) (param schemas). Feeds: consumers that
generate checks from stamps — dokimi is the reference case.*

The classification vocabulary as its own satellite: **one
mechanism with three declared forms** — shape, mixin, contract
(below). Catalogs churn as their vocabulary settles, which is
exactly why this one lives outside the kernel: its cadence gates
nobody.

## One mechanism, three forms

A classification is one mechanism — a named spec, typed params,
generated constants, stamped facts, `Where`-gated consumers — and
every spec declares its **form**, which pins everything that
varies:

| | shape | mixin | contract |
|---|---|---|---|
| per callable | exactly one | any number | one per (instance, role) |
| arbitration | first claim wins ([04-metadata.md](04-metadata.md)) | accumulate — every stamp sticks | accumulate, per instance |
| inferred? | optionally, by a `Detect` function; `+gen:shape <name>` overrides at directive authority | never — directive only | never — directive only |
| directive | single-instance | `mixin <name>`, repeatable | `contract <proto> role=<r>`, repeatable; `id=` separates instances |
| `precedence:` | required where signatures overlap | forbidden | forbidden |
| `roles:` | forbidden | forbidden | required, with arity |

The forms are the occupied cells of a small grid — who asserts
(inferred or declared) crossed with how many stick — and the
empty cells are enforcement: a shape spec carrying `roles:`, or a
mixin carrying `precedence:`, fails the schema before review sees
it. The split is not three systems; it is the *type* of a
classification, and it decides semantics nothing downstream can
re-derive: which arbitration applies, whether a second annotation
is composition (mixin) or contradiction (shape), and what a
consumer generates — a shape picks the check template, mixins
modify it, a contract yields a cross-callable harness. A callable
carries all three at once by construction: `Commit` is
`shape=writer`, role `commit` of contract `tx`, and `atomic`,
under disjoint key prefixes that never collide.

Contract instances bind by protocol name within a package;
`id=` separates two instances of one protocol in scope. Role
arity ("exactly one `commit`") is validated by the catalog's
validation-bucket annotator over the whole instance, reported as
positioned Errors.

Generated name constants are **typed per form** —
`shape.ShapeName`, `shape.MixinName`, `shape.ContractName` — so a
consumer cannot pass a mixin where a shape is expected: the
boundary-strings law applied to the catalog's own vocabulary.

## Language-neutral by construction

A detector written against one language's signature primitives
multiplies: ~90 detectors × N languages is ~270 hand-written
functions, each a re-derivation of the same question in another
language's spelling. So detectors read **only `rules.Callable`** and
the canonical shapes ([03-projection.md](03-projection.md));
parameter resolution goes through `rules.Resolve`. One detector
serves every language that answers Tier 1 — 90 + N, not 90 × N. The
module's CI enforces the Tier-3 import ban.

The contract a detector satisfies, pinned — and the composition
rule that makes precedence enforceable:

```go
type Detector interface {
    Name() Name                  // the shape it stamps; a generated constant
    Detect(c rules.Callable, r rules.Resolver) (Stamps, bool)
}
```

Detectors register as an **ordered list inside one annotator**
(the umbrella plugin), first claim wins per callable. They cannot
be separate rules: the kernel's handler order-independence law
([06b-authoring.md](06b-authoring.md)) forbids ordering across handlers,
so precedence must live where order is data — the list, whose
order is generated from the specs' precedence declarations.

## Spec-first (decided)

The catalog's source of truth is `spec/` — one spec per contract,
detector, and mixin, written **before** code. The defect class this
prevents is the worst one a classification vocabulary has: a
directive that under-determines the check it licenses, discovered by
a consumer's conformance corpus after shipping. The spec template
makes that a review-time checkbox:

- **Claim** — the assertion, in neutral vocabulary (no language's
  spelling).
- **Observation** — what a check must be able to *see* for the claim
  to be checkable; which param names the observation when the shape
  itself doesn't. A shape that permits observation without naming it
  leaves every consumer guessing which sibling is the observer.
- **Param schema** — each key with type, resolution kind, role
  scoping, required/optional, and counterexample marking
  ([05-directives.md](05-directives.md)); adversarial inputs a
  derivation cannot invent are declared, not discovered.
- **Falsifiability** — what goes red if the subject's handling is
  deleted. The hollow-law bug — a law binding only derived samples,
  inputs constructed to be accepted, which engages, stays green with
  the subject's handling deleted, and tests nothing — becomes a
  mandatory spec section instead of a shipped defect.
- **Counterexample obligations** — the invalid/unsafe/edge/refused
  family, stated per claim.
- **Precedence** (shape form only) — where signatures overlap (`Delete(v) error` is
  writer-shaped and deleter-shaped), which shape claims the
  callable and which yields. Detectors are an ordered list inside
  one annotator — the handler order-independence law
  ([06b-authoring.md](06b-authoring.md)) forbids hanging precedence on
  separate rules — so precedence is catalog data the spec
  declares, never an accident of registration.

The format is settled, not deferred: specs are YAML files under a
published JSON Schema whose sections are exactly the template above
— so a spec is machine-checkable for structural completeness (a
missing falsifiability section fails CI before review sees it), the
generators consume it directly, and the docs site renders it without
transformation.

One spec, worked, so the template is a shape rather than a list:

```yaml
# spec/shapes/writer.yaml
name: writer
form: shape
claim: >
  The callable persists exactly one input value and answers
  success solely through its error model.
observation: >
  A check must see the persisted value through a sibling reader;
  the `reads` param names it when no reader shape is stamped on
  the host.
params:
  - {key: reads,  type: reference, resolve: callable-in-scope}
  - {key: sample, type: string, counterexample: true}
falsifiability: >
  Deleting the subject's write path turns the generated
  round-trip check red: the sample written through the subject
  is no longer observable through the reader.
counterexamples:
  invalid: a value the subject must refuse
  unsafe: a value that must never reach the reader unescaped
  refused: a callable answering a value beside the error —
    that is answeringwriter, not writer
precedence:
  yields_to: [deleter]    # same signature under a removal name
```

The other two forms, complete — every mandatory section present,
and *only* the sections the form forbids absent (`precedence:`
here, `roles:` on the mixin):

```yaml
# spec/contracts/tx.yaml
name: tx
form: contract
claim: >
  The group provides transactional begin / commit / rollback over
  one resource: effects between begin and commit are observable
  only after commit, and never after rollback.
observation: >
  A check must watch the resource from outside the transaction;
  the `observer` param names the reader it looks through.
params:
  - {key: observer, type: reference, resolve: callable-in-scope}
roles:
  begin:    {arity: one}
  commit:   {arity: one}
  rollback: {arity: one}
falsifiability: >
  commit without begin fails; work done before rollback is not
  observable through the observer after it.
counterexamples:
  invalid: an operation sequenced outside begin/commit
  edge: an empty transaction — begin then commit with no work —
    must succeed and observe nothing
  refused: two callables claiming one role in one instance

# spec/mixins/atomic.yaml
name: atomic
form: mixin
claim: the callable's effect is all-or-nothing under failure
observation: >
  Partial state must be observable to be refutable; the
  `observer` param names the reader the check inspects
  mid-failure.
params:
  - {key: observer, type: reference, resolve: callable-in-scope}
falsifiability: >
  inject failure mid-effect; any partial state visible through
  the observer turns the check red
counterexamples:
  edge: a zero-effect call — atomicity over nothing must pass
  unsafe: a failure injected between the effect and its
    acknowledgement
```

## Generated from specs

Registry code, name constants, param constants, and directive schemas
are **generated from the specs** by a real eidos workspace — this
satellite, not the kernel, is where the framework builds itself with
itself, because here the dependency direction is legal. The generator
lives in a tools module (its own `go.mod`) that depends on the kernel
and `eidos-lang-go`; the catalog module itself never requires a language
satellite, which is the same Tier-3 discipline its detectors live
under. There is nothing hand-written to drift. Spec renders to the
docs site verbatim — the spec *is* the documentation.

The flow, in order: (1) specs validate against the published JSON
Schema — structural incompleteness fails CI before review; (2)
the tools module's eidos workspace reads `spec/` through a
purpose-built YAML frontend and generates the registries — name
constants, param constants, directive schemas, the detector
precedence order; (3) generated output is committed, and a mirror
guard reruns the workspace and diffs the tree, the same
discipline as the kernel's models
([02-symbol-model.md](02-symbol-model.md)); (4) the docs site
renders the specs verbatim.

No code ever lives in the YAML. The frontend maps each spec onto
the ordinary node model — one spec is a `node.Struct`, its params
are `node.Field`s, form and resolution kinds ride `shape.spec.*`
metadata, and YAML positions are real positions, so a bad spec
fails at `spec/shapes/writer.yaml:14:3` like any other source.
The join to behavior is generated and therefore compiler-checked:
the wiring map (`ids.Writer: writer.Detector()`, one line per
inferable spec) makes a spec without a `Detect` implementation a
compile error, and a generated completeness test finds the
orphan in the other direction.

## Division of labor

Shape **stamps**; consumers **check**. The catalog classifies
callables and asserts nothing about behavior — generating checks from
classifications is the consumer's business (dokimi, the reference
consumer, is the proving case). Stamps are the standard three facts
(name, role bindings, param resolutions) in the `shape.*` namespace,
overridable like all metadata — a wrong inference is a one-line
directive at the declaration, never a fork.

## Scope

Every name enters through the spec template or not at all. Specs
group by form (`spec/shapes/`, `spec/mixins/`, `spec/contracts/` —
the directory must match the declared form); name constants ship
generated, typed per form. A `Detect` function exists only for
shape-form specs — the inferable form; mixins and contracts have
no detector by construction.
