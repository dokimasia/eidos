---
rfc: 0060
title: The reference ensemble after its review
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-13
updated: 2026-10-13
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0040
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0060: The reference ensemble after its review

## Summary

The review of the reference ensemble left 46 items open: the composition and the canary, the
plugins of the handler set and the doubles, builder, enum, sentinel, defaults, the authored samples
and witnesses, the capabilities of the hosts and the shape catalog. We propose a verdict for each.
The composition registers the annotators of the authored samples and witnesses and the Go
annotator, as RFC-0014 and RFC-0026 state, so builder, enum and sentinel read the facts that they
are designed to read. The generated checks of builder, enum and sentinel each get the check that the
review found missing. defaults leaves an empty tag value out, the sample directive takes an alternate
without a value, and an empty sample or witness directive reports a finding. The other items are
out, each with the evidence that would reverse it. ADR-0040 records the five departures of the
implementation of RFC-0026.

## Motivation

- RFC-0014 states that the reference composition registers the annotators of the sample and the
  witness directives, and RFC-0026 states that the test file of builder takes its samples from the
  kernel's sample and witness facts. The composition registers neither.
- RFC-0026 states that sentinel selects error types through `golang.satisfiesError`, which the Go
  annotator stamps. The composition does not register the Go annotator, so the golden test file of
  the catalog has no check of `NotFoundError`.
- RFC-0014 states that a check that compares against one value passes whenever the subject already
  has that value. The check of a setter of builder compares against the first sample over a builder
  of `New<T>`, whose field can have that value already.
- The test file of sentinel calls `Error` on each sentinel, so a sentinel that is never assigned
  makes the test panic instead of failing with its name.
- defaults stamps an empty tag value as a default, and builder then assigns the zero value of the
  field over the value that the function of `defaults=` seeded.
- RFC-0014 states that each half of a sample reads independently, and the schema of the sample
  directive requires the value.
- A sample directive that states neither a value nor an alternate does not stamp or report
  anything, and neither does a witness directive that names no type parameter.
- RFC-0026 has five departures that its implementation made, and its front matter lists its ADR as
  `tbd`.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-reference` | `Base` registers the sample, the witness and the Go annotators. The tree has an authored sample and a witness |
| `eidos-reference/builder` | The check of a setter sets the alternate sample over the seed of `New<T>From` |
| `eidos-reference/enum` | The test file checks that `IsValid` reports false for the value that no variant has |
| `eidos-reference/sentinel` | The test file reports a sentinel that is never assigned, and runs its other checks over the assigned ones |
| `eidos-reference/defaults` | An empty tag value is no default |
| `eidos-core/directive` | The value of the sample directive is optional |
| `eidos-core/authored` | The sample annotator reports `EmptySample` and the witness annotator `EmptyWitness` |
| ADR-0040 | The departures of the implementation of RFC-0026 |

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| The sample and the witness annotators in the composition | in | RFC-0014 and RFC-0026 state both | none |
| The Go annotator in the composition | in | RFC-0026 states that sentinel selects error types through its fact | none |
| The protobuf frontend in the composition | out | No generator of the ensemble reads a protobuf declaration: each reads the declarations of the language of its plan, and RFC-0026 composes a Go plan and a TypeScript plan. The conformance corpus loads and projects the protobuf sources | A generator of the ensemble that reads a protobuf declaration |
| The monkey harness in the repository and in the nightly workflow | out | The two campaigns of 900 rounds found no defect of eidos, and the ten flagged rounds were sources that the harness broke. Fuzzing of the project runs as `go test -fuzz` targets | A defect that a random edit of the tree finds and no suite finds |
| The returned values in the call records of a stub | out | A call record states the arguments of a call, as RFC-0026 states, and the test that installs the function field of a stub knows what the field returns | A consumer test that reads the results of a call from its record |
| No stub for an interface without methods | out | The stub of such an interface compiles and satisfies the interface, and its test file checks the satisfaction | A target that refuses a type without members |
| A registry that passes a name beside the value | out | The callee of the options takes one value, as `gob.Register` does | A consumer whose registry function takes a name |
| The options of the previous version for the words of repogen, mockgen, stubgen and middlewaregen, and the visibility of a repository | out | Each plugin joins the words that it declares as constants, and the golden files of the tree and the hand-written code that calls the generated declarations depend on them. A plugin that needs other words or another visibility is a plugin of its own. ADR-0040 records that the config example of RFC-0021 sets an option that stubgen does not have | A consumer that runs a plugin of the ensemble and needs other words |
| The identity of the method that makes a struct a writer | out | No plugin reads which method made a struct a writer | A plugin that reads the method of a writer |
| The doubles in the external test package | out | A double in the package of its interface can implement an unexported interface, and the tests of the package call it. An external test package sees the exported declarations alone | A consumer whose doubles serve the tests of another package |
| The owner of the method in the log of debugweaver | out | RFC-0026 states the name of the origin, which the repository and its mock share | A consumer that reads the log to tell a double from its repository |
| `Clone` and its check, `Mutate`, the tag `builder:"-"`, setters for embedded members and for exported fields alone | out | RFC-0026 lists the declarations of a builder: the builder, `New<T>`, `New<T>From`, the setters of the declared fields and `Build`. A builder lives in the package of its struct, so a setter of an unexported field sets it there. A promoted field through an embedded pointer panics on a nil pointer, so a setter of it needs the value of the embed first | A consumer that copies built values, excludes a field from its builder, or sets the members of an embed through the builder |
| A finding for a struct without a settable member | out | Such a builder still has `New<T>`, `New<T>From` and `Build`, which compile | A consumer that reports such a struct as a defect of its source |
| A seed from a `<T>Defaults` function by convention | out | The param `defaults=` names the function, as RFC-0026 states, and a convention would be a second form | A consumer that cannot write the param |
| The options of the previous version for the words, the tests and the tags of builder, enum and defaults | out | Each plugin joins the words that it declares as constants, as repogen, mockgen, stubgen and middlewaregen do | A consumer that runs a plugin of the ensemble and needs other words |
| The check of a setter with two distinct samples | in | RFC-0014 states that a check against one value passes where the subject has the value already | none |
| A note in the test file of a generic struct without witnesses | out | The builder writes no check for such a struct, and a witness directive states the witnesses | A consumer that reads why a check is missing |
| The test of a set by the spelling `struct{}` | closed | builder writes Go alone, and a map of empty structs is a set by the convention of Go, which its spelling states. RFC-0059 decides that the root package of a satellite is the Tier-3 vocabulary and that no helper package exists | none |
| The directive `text` of RFC-0026 | out | An enum variant is not a subject kind, so no rule of the authoring API runs on it, and a directive on a variant reaches no plugin. ADR-0040 records the departure | An enum variant as a subject kind of the schema |
| A decoder that calls the parser of an author who keeps `String` | out | RFC-0026 states that enum writes `Parse<T>` and the encoding pair only where it writes `String`, and an author who writes `String` writes its parser too | An author who writes `String` and needs the decoder of enum |
| A finding for `enum` on a type without variants | closed | RFC-0027 refuses the directive on a declaration of another kind under `MisplacedDirective`, and the reference binary reports it | none |
| A finding for a variant outside the unit of its type | closed | RFC-0049 promotes the constants of every file of a package, and the constants of another package are out by its verdict | none |
| The checks of the value outside the variants, the zero value, the text of the encoder and the refusal of the decoder | in for the value outside the variants | The test file checks the text of each variant through `String` and `MarshalText`, and the refusal of `Parse<T>`. `IsValid` has no check of a value outside the variants, which `EnumInfo.OutOfRange` returns. The zero value is a variant where one has it, and the test checks each variant | none |
| A check that every sentinel is assigned | in | A sentinel that is never assigned makes the test panic instead of failing with its name | none |
| A prefix for each package | out | The authoring API has no rule on a package, so a directive on a package reaches no plugin, and the option `prefix` sets the prefix of a run | A rule on a package subject |
| Checks of error types beyond those of RFC-0026 | out | RFC-0026 states the checks of an error type: a message for its zero value, and the cause that it was given | A consumer that needs the checks of the previous version |
| A warning for a type that a run did not resolve | out | The error contract of the Go rules reads the declarations of the view, and a type that the view does not contain has no contract | A consumer whose error types embed types of another module |
| The match of the spelling `"error"` | closed | sentinel selects error types through `golang.satisfiesError` and reads their cause from the error contract of RFC-0042. It matches no spelling | none |
| A default on an alias | out | RFC-0026 states the directive `default` on a field, and builder reads defaults of fields | A reader of a default of an alias |
| An empty tag value as no default | in | builder assigns the zero value over the value that the function of `defaults=` seeded | none |
| The validation of a default in defaults | out | builder, the one reader of the defaults, reads the text in the scope of its struct and reports `BadDefault` at the field | A second reader of the defaults |
| A sample that states the alternate alone | in | RFC-0014 states that each half reads independently, and `rules.Complete` pairs a stated alternate with a derived value | none |
| A finding for a witness directive without a key | in | Such a directive stamps nothing, and the author sees no effect | none |
| A capability of builder, enum and sentinel | out | No plugin extends their output. A plugin that extends one requires a capability, which the host then provides | A plugin that extends the output of builder, enum or sentinel |
| The overrides `key=`, `value=` and `kind=` of a part of a shape | out | The specs of the catalog bind each part by its position, and the draft design of the catalog has no override | A shape whose parts a position does not bind |
| Named constants of the vocabularies of `fidelity` and `mode` | out | No plugin reads the values of the two params, and the spec of each param states its vocabulary | A plugin that reads `fidelity` or `mode` |

### The composition and the tree

`Base` registers `authored.Sample()`, `authored.Witness()` and the Go annotator beside shapewriter
and defaults. In the tree, `City` of `catalog/address.go` gets a sample with a value and an
alternate, and `Country` a sample with the alternate alone, so the checks of their setters use the
authored values. The generic interface `Cache` of `catalog/cache.go` gets the witness `T=string`,
so its stub test runs at `string`. The golden test file of the catalog checks the zero value of
`NotFoundError` and its cause.

### The checks

- builder: the check of a setter starts from `New<T>From(seed)`, whose seed has the first sample of
  each field, and sets the alternate sample. It compares the field with the alternate. A field
  without an alternate keeps the check from `New<T>()` with the first sample.
- enum: where enum writes `IsValid` and `EnumInfo.OutOfRange` has a value, the test file checks
  that `IsValid` reports false for the type converted from that value.
- sentinel: the test file checks that each sentinel is assigned, and runs the checks of the
  messages and of `errors.Is` over the assigned sentinels.

### The authored directives

The value of the sample directive is optional. The sample annotator reports `EmptySample` (EID-0076)
at an instance without a value and without an alternate. The witness annotator reports
`EmptyWitness` (EID-0077) at an instance without a key. Both are Errors at the carrier.

### Failures

| Condition | Report |
|---|---|
| A sample directive without a value and without an alternate | `EmptySample`, an Error at the carrier |
| A witness directive without a key | `EmptyWitness`, an Error at the carrier |

### Migration

1. Make the value of the sample directive optional, and report the two empty directives.
2. Leave an empty tag value out of defaults.
3. Register the three annotators in `Base`, and add the sample and the witness to the tree.
4. Change the checks of builder, enum and sentinel, and update the golden files.
5. Write ADR-0040.

## Alternatives

### The directive `text` through a param of the directive `enum`

The directive `enum` could take the text of each variant as a param, such as
`text=StatusDraft:draft`. The text of a variant would then be far from the variant, and one param
would contain a list in a format of its own.

### Every option of the previous version

The plugins could take the options of the previous version. Each option would be public
configuration that the tree and the golden files do not use, and the ensemble demonstrates the
composition of plugins.

## Drawbacks

- The tree and the golden files grow by the authored values and the checks of `NotFoundError`.
- A consumer of the previous version rewrites its options for the words of the plugins.

## Open questions

None.

## Unresolved and future work

None.

## References

- [RFC-0014](0014-the-projection-vocabulary.md), the authored values and the two values of a check.
- [RFC-0021](0021-the-command-kernels.md), the config example.
- [RFC-0026](0026-the-reference-ensemble-and-the-canary-ring.md), the ensemble.
- [RFC-0027](0027-the-subject-kinds-of-a-directive.md), the misplaced directive.
- [RFC-0042](0042-the-error-contract-of-a-type.md), the error contract.
- [RFC-0049](0049-go-enumerations.md), the promotion of constants.
- [RFC-0059](0059-package-structure-and-the-facade.md), the Tier-3 vocabulary.
