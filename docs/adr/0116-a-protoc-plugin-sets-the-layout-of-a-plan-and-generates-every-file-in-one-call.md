---
adr: 0116
title: A protoc plugin sets the layout of a plan and generates every file in one call
status: Accepted
date: 2026-10-28
supersedes: none
superseded-by: none
rfc: RFC-0088
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0116: A protoc plugin sets the layout of a plan and generates every file in one call

## Status

Accepted

## Context

We ran a mock project through `buf generate`. It had a generators repository with a binary
`protoc-gen-acme` and an application with seven proto files in four packages. Two defects of
RFC-0088 showed up.

- The Go backend derives the import path of a generated package from the `importBase` of its plan.
  A request carries no Go module, and the parameter of RFC-0088 sets plugin options and policies
  only. The generator had to hard-code the import path of the application.
- buf calls a local plugin once for each directory by default. In such a call, every reference to a
  message of another directory failed with EID-0070, because the plan does not emit that message.
  RFC-0088 required `strategy: all` only for a plan that writes one file per plan, so the default
  configuration failed for any schema that imports across directories.

## Decision

We will map the fields of a plan in the parameter, and we will require `strategy: all` with a
finding that names it.

- `plan.<plan>.<field>=<value>` sets `importBase`, `dir`, `policy` or `disabled` of a plan, as
  `workspace.PlanConfig` holds them and as a config file of eidos names them. A field that the
  parameter does not set, a policy that the layout does not know and a `disabled` that is no bool
  return `ErrParameter`.
- The prefix `plan.` names no plugin, as the prefix `policy.` does not.
- A request whose run refuses a reference under EID-0070 or EID-0081, and that imports files that
  it does not generate, opens its `error` field with `protoc.ErrImportedReference`. The error names
  `strategy: all`. The findings of the run follow it.
- The documentation of the host shows `strategy: all` and `clean: true` in every `buf.gen.yaml`.

## Alternatives Considered

### A reference to the route of an imported message

protoc-gen-go generates one directory per call and refers to the messages of other directories
through their `go_package`. A plan could refer to a dependency declaration at the route where the
same plan generates it. This changes the import rule of the kernel under ADR-0105, and needs an RFC
of its own.

### The `go_package` of a file as the import path

The Go backend could read `go_package` from the options of a file. That covers Go alone, and a Go
plan of `eidos run` reads its import path from the module of its sources already.

## Consequences

**Positive:**

- The mock project generates with the import path in `buf.gen.yaml`, and its code vets and tests.
- A run in the default strategy names the setting that fixes it.

**Negative:**

- buf cannot call an eidos plugin per directory in parallel.

## References

| What | Where |
|---|---|
| D234, the row of this decision | ../architecture/21-decisions.md |
| The protoc plugin binary | ../architecture/14-distribution-and-cli.md |
| RFC-0088, a protoc plugin host | ../rfc/0088-a-protoc-plugin-host.md |
| ADR-0115, the host of a protoc plugin | 0115-a-protoc-plugin-runs-an-eidos-composition.md |
