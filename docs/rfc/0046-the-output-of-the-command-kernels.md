---
rfc: 0046
title: The output of the command kernels
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0034
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0046: The output of the command kernels

## Summary

`explain` exits 0 for a target that the generation does not record. It shows the claims and the reads
of a declaration, and not the declaration itself. The commands render every Warning, and no flag
limits the output to the Errors. `version` leaves out the plugins of a plan that the config file
disables, and its text output leaves out the brand. The error of an output format has no sentinel. We
propose an exit status for an empty explanation, the declaration in an explanation, the flag
`--quiet`, the disabled plans in a description, the brand in `version` and a sentinel for the format.
We refuse a test host under cobra in the acceptance kit.

## Motivation

- A script that runs `explain` over a misspelled target gets the status 0 and an empty explanation.
- A user who asks how a generator saw the type of a field reads the source again, because the
  explanation of the declaration has no declaration.
- `--verbose` adds the Info findings, and no flag removes the Warnings.
- Build leaves a disabled plan out of the composition. `version` and `plan` then do not list its
  plugins, which the binary still contains.
- The start event of JSON output contains the brand, and the text output of `version` does not.
- `Format.Set` returns an error that a caller cannot test.
- The command document shows the kernels under cobra, and no test runs them there.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/workspace` | `Explanation.Declarations` and `Description.Disabled` |
| `eidos-cli` | the status of an empty explanation, the declaration event of `explain`, `Flags.Quiet`, the disabled plans in `plan` and `version`, the brand in `version`, and `ErrFormat` |
| `docs/architecture` | the CLI document |

### Invariants

1. `explain` exits 1 when the generation records nothing about its target in each workspace that
   contains the target.
2. An explanation of a declaration contains the declaration as the recorded load contains it.
3. `--quiet` renders the Error findings alone, and the summary still counts every finding.
4. `version` lists every plugin that the composition declares, the plugins of a disabled plan
   included.

### An empty explanation

`explain` returns `StatusFailed` and renders an error when each workspace that contains the target
explains it with empty lists of files, claims, executions and declarations:

```
error: cli: explain: the generation 3fa9… records nothing about svc/store_stubb.go
```

A workspace whose ledger has no generation fails with `ErrNoGeneration` as before.

### The declaration of an explanation

```go
// Declarations lists the declarations that the target names, as the
// recorded load contains them: each declaration that an identity
// matches, and each declaration at the position of a key. It is empty for
// a path and for a code at a position.
Declarations []node.Declaration
```

`explain` renders one event for each declaration. The JSON event `explain.declaration` has the fields
`subject`, `pos` and `declaration`, and `declaration` is the JSON form of the node, which
`node.EncodeJSON` writes for every kind of the model. Text output writes a line for the declaration
and then its JSON form, indented by two spaces:

```
declaration golang:svc.Store at svc/store.go:12:6
  {
    "kind": "Struct",
    ...
  }
```

A text form for each kind would restate the model and drift from it. The JSON form round-trips every
kind, and it is the form that the fixtures and the goldens read.

### Quiet output

```go
// Quiet is true under --quiet, which renders the Error findings alone.
Quiet bool
```

Under `--quiet`, a command renders a Warning or an Info finding not at all, and counts it in the
summary. `--quiet` and `--verbose` together are a usage error.

### The disabled plans of a description

```go
// Disabled lists the plans that the config disables, in composition
// order. Each has its name, the generators that it declares, its backend
// and its target. A generator of a disabled plan has no turn.
Disabled []PlanDescription
```

`version` lists the generators and the backend of each disabled plan beside the others, each plugin
once. `plan` writes one line for each disabled plan and one line for each of its generators:

```
plan bindings: disabled by the config file, target text, backend printer
  generator binder 3
```

The JSON event of a disabled plan is a `plan` event with the fields `name`, `disabled`, `generate`,
`backend` and `target`.

### The brand in version

The text output of `version` opens with the line `brand acme`, and the JSON event `version` gains the
field `brand`.

### The sentinel of a format

```go
// ErrFormat reports a value of --format that is neither text nor json.
var ErrFormat = errors.New("cli: unknown output format")
```

`Format.Set` returns an error that wraps `ErrFormat`. The flag package formats the error of a value
with the verb `%v`, so a parse of the flags reports the text of the error and not the sentinel. A host
that sets a `Format` itself tests the error with `errors.Is`.

### A host under cobra

Verdict: out. A cobra host in the test data of the acceptance kit needs cobra in the requirements of
the cli module, because `go mod tidy` drops a requirement that only a package under `testdata`
imports. Each consumer of the cli module would then load cobra into its module graph. The acceptance
kit exists so that each consumer runs the suite over its own binary, so a consumer with a cobra host
proves the host in its own tests. A defect of the kernels that only a cobra host shows would reverse
the verdict.

### Failures

| Condition | Report |
|---|---|
| A target that the generation records nothing about | `StatusFailed` and an error that names the generation and the target |
| `--quiet` with `--verbose` | `StatusUsage` and a usage error |
| A value of `--format` other than text and json | `StatusUsage` and an error that wraps `ErrFormat` |

### Migration

1. Add `Explanation.Declarations` and `Description.Disabled`.
2. Fail an empty explanation, and render the declarations in `explain`.
3. Add `--quiet`, the disabled plans in `plan` and `version`, the brand and `ErrFormat`.
4. State the flag, the status and the events in the CLI document.

## Alternatives

### A finding for an empty explanation

`explain` could report a finding with a code. A finding is a diagnostic of a run, positioned in the
source, and `explain` runs nothing. An error of the command is the report of every other command
failure.

### Disabled plans in the list of plans

`Description.Plans` could contain the disabled plans with a mark. Every caller that walks the plans
would then test the mark, and a caller that misses it would treat a disabled plan as one that runs.

### A text form of each kind

`explain` could write a field, a method or a variant in the spelling of the source. The model has
more than twenty kinds, and each text form would restate the facts of its kind.

## Drawbacks

- The output of `explain` for a declaration grows by the JSON form of the declaration.
- A script that relied on the status 0 of `explain` for an unknown target fails.
- `Description` gains a list that the composition does not run.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan rows W5.C3, W5.C4, W5.C6, W5.C7, W5.C10, W5.C12 and W5.C14.
- The command kernels and the cobra host: RFC-0021.
- The JSON codecs of the model: ADR-0003.
