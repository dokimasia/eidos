# The shape catalog (eidos-plugin-shape)

*Builds on: [03](03-projection.md) (Callable and Resolve),
[05](05-directives.md) (param schemas). Feeds: consumers that
generate checks from stamps, where dokimi is the reference case.*

The classification vocabulary lives in its own satellite. It is
**one mechanism with three declared forms**: shape, mixin and
contract. A catalog changes often while its vocabulary settles,
which is exactly why this one lives outside the kernel, where its
cadence gates nobody.

## One mechanism, three forms

A classification is always the same mechanism: a named spec, typed
params, generated constants, stamped facts, and consumers gated with
`Where`. Every spec declares its **form**, and the form pins
everything that varies:

| | shape | mixin | contract |
|---|---|---|---|
| per callable | exactly one | any number | one per (instance, role) |
| arbitration | first claim wins ([04-metadata.md](04-metadata.md)) | accumulate: every stamp sticks | accumulate, per instance |
| inferred? | optionally, by a `Detect` function. `+gen:shape <name>` overrides at directive authority | never: directive only | never: directive only |
| directive | single-instance | `mixin <name>`, repeatable | `contract <proto> role=<r>`, repeatable, with `id=` separating instances |
| `precedence:` | required where signatures overlap | forbidden | forbidden |
| `roles:` | forbidden | forbidden | required, with arity |

The three forms are the occupied cells of a small grid: who asserts
the classification, inferred or declared, crossed with how many of
them stick. The empty cells are enforced. A shape spec carrying
`roles:`, or a mixin carrying `precedence:`, fails the schema before
review sees it.

This is not three systems. The form is the *type* of a
classification, and it decides things nothing downstream can work
out for itself: which arbitration applies, whether a second
annotation composes (mixin) or contradicts (shape), and what a
consumer generates, since a shape picks the check template, mixins
modify it, and a contract yields a harness across several callables.

One callable carries all three at once by construction. `Commit` is
`shape=writer`, it is role `commit` of contract `tx`, and it is
`atomic`. The key prefixes are disjoint, so they never collide.

A contract instance binds by protocol name within a package, and
`id=` separates two instances of one protocol in scope. Role arity,
such as "exactly one `commit`", is validated by the catalog's
validation-bucket annotator over the whole instance, and reported as
positioned Errors.

Generated name constants are **typed per form**: `shape.ShapeName`,
`shape.MixinName` and `shape.ContractName`. A consumer therefore
cannot pass a mixin where a shape is expected, which is the same
registered-name rule applied to the catalog's own vocabulary.

## Language-neutral by construction

Write a detector against one language's signature primitives and the
work multiplies: roughly 90 detectors across N languages is about
270 hand-written functions, each one re-deriving the same question
in another language's spelling.

So a detector reads **only `rules.Callable`** and the canonical
shapes ([03-projection.md](03-projection.md)), and resolves
parameters through `rules.Resolve`. One detector then serves every
language that returns Tier 1, which makes the cost 90 + N rather
than 90 × N. The module's CI enforces the Tier-3 import ban.

The contract a detector satisfies, pinned:

```go
type Detector interface {
    Name() Name                  // the shape it stamps; a generated constant
    Detect(c rules.Callable, r rules.Resolver) (Stamps, bool)
}
```

Detectors register as an **ordered list inside one annotator**, the
umbrella plugin, and the first claim wins per callable. They cannot
be separate rules, because the kernel's handler order-independence
rule ([06b-authoring.md](06b-authoring.md)) forbids ordering across
handlers. Precedence therefore has to live where order is data,
which is the list, and the list's order is generated from the specs'
precedence declarations.

## Specs come first

The catalog's source of truth is `spec/`, holding one spec per
contract, detector and mixin, written **before** the code.

The defect this prevents is the worst one a classification
vocabulary can have: a directive that does not pin down the check it
licenses, discovered by a consumer's conformance corpus after it
ships. The spec template turns that into something a reviewer can
check:

- **Claim**: the assertion, in neutral vocabulary, using no
  language's spelling.
- **Observation**: what a check has to be able to see for the claim
  to be checkable, and which param names that observation when the
  shape itself does not. A shape that allows an observation without
  naming it leaves every consumer guessing which sibling does the
  observing.
- **Param schema**: each key with its type, resolution kind, role
  scoping, required or optional, and counterexample marking
  ([05-directives.md](05-directives.md)). Adversarial inputs that no
  derivation could invent are declared rather than discovered.
- **Falsifiability**: what turns red if you delete the subject's
  handling. This catches the hollow rule, meaning a rule that binds
  only derived samples, built from inputs constructed to be
  accepted, which runs, stays green after you delete the subject's
  handling, and tests nothing. Making it a mandatory spec section
  catches it before it ships.
- **Counterexample obligations**: the invalid, unsafe, edge and
  refused family, stated per claim.
- **Precedence**, for the shape form only. Where signatures overlap,
  such as `Delete(v) error` being both writer-shaped and
  deleter-shaped, it says which shape claims the callable and which
  yields. Since detectors are an ordered list inside one annotator,
  and the handler order-independence rule
  ([06b-authoring.md](06b-authoring.md)) forbids hanging precedence
  on separate rules, precedence is catalog data the spec declares
  rather than an accident of registration order.

The format is settled rather than deferred. Specs are YAML files
under a published JSON Schema whose sections are exactly the
template above. A spec is therefore machine-checkable for structural
completeness, so a missing falsifiability section fails CI before
review sees it, the generators consume it directly, and the
documentation site renders it without transforming anything.

One worked spec, so the template reads as a shape rather than a
list:

```yaml
# spec/shapes/writer.yaml
name: writer
form: shape
claim: >
  The callable persists exactly one input value and returns
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
  refused: a callable returning a value beside the error —
    that is answeringwriter, not writer
precedence:
  yields_to: [deleter]    # same signature under a removal name
```

The other two forms, complete, with every mandatory section present
and only the sections their form forbids absent, meaning
`precedence:` here and `roles:` on the mixin:

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

## Generated from the specs

Registry code, name constants, param constants and directive schemas
are **generated from the specs** by a real eidos workspace. This
satellite, rather than the kernel, is where the framework builds
itself with itself, because here the dependency direction is legal.

The generator lives in a tools module with its own `go.mod`, which
depends on the kernel and on `eidos-lang-go`. The catalog module
itself never requires a language satellite, which is the same Tier-3
discipline its detectors live under. Nothing is hand-written, so
nothing can drift. The spec renders to the documentation site
verbatim, which makes the spec the documentation.

The flow, in order. First, specs validate against the published JSON
Schema, so structural incompleteness fails CI before review. Second,
the tools module's eidos workspace reads `spec/` through a
purpose-built YAML frontend and generates the registries: name
constants, param constants, directive schemas and the detector
precedence order. Third, the generated output is committed, and a
mirror guard reruns the workspace and diffs the tree, the same
discipline the kernel's models use
([02-symbol-model.md](02-symbol-model.md)). Fourth, the
documentation site renders the specs verbatim.

No code ever lives in the YAML. The frontend maps each spec onto the
ordinary node model: one spec is a `node.Struct`, its params are
`node.Field`s, and form and resolution kinds ride on `shape.spec.*`
metadata. YAML positions are real positions, so a bad spec fails at
`spec/shapes/writer.yaml:14:3` like any other source.

The join to behaviour is generated, so the compiler checks it. The
wiring map, one line per inferable spec such as `ids.Writer:
writer.Detector()`, makes a spec without a `Detect` implementation a
compile error, and a generated completeness test finds the orphan in
the other direction.

## Division of labour

Shape **stamps**. Consumers **check**. The catalog classifies
callables and asserts nothing about behaviour. Generating checks
from classifications is the consumer's business, and dokimi, the
reference consumer, is the proving case.

Stamps are the standard three facts, meaning the name, the role
bindings and the param resolutions, in the `shape.*` namespace. They
are overridable like all metadata, so a wrong inference is a
one-line directive at the declaration rather than a fork.

## Scope

Every name enters through the spec template or not at all. Specs
group by form, under `spec/shapes/`, `spec/mixins/` and
`spec/contracts/`, and the directory has to match the declared form.
Name constants ship generated and typed per form. A `Detect`
function exists only for shape-form specs, which is the only
inferable form. Mixins and contracts have no detector by
construction.
