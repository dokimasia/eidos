---
adr: 0031
title: The required options keep the departures of their implementation
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0043
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0031: The required options keep the departures of their implementation

## Status

Accepted

## Context

The accepted design adds the mark `schema:"required"`, the function `MissingOptions` and the text
form of a duration. Build refuses a composition whose config leaves a required option at the zero
value of its type. The components of the design are the plugin package, the options step of the
workspace, the plugin suite and the callee options of the reference ensemble.

`AssertDiscovery` of the acceptance kit and `AssertCycleRefused` of the workspace suite ran a
composition without the config file of its tree. A composition with a required option fails Build
there, so both checks failed for the reference ensemble. Each change is listed here, because an
accepted design is not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departures are these.

| Design | Code | Reason |
|---|---|---|
| The design changes no check of the acceptance kit. `AssertDiscovery` expects plan to exit 0 inside a repository below a config file that does not decode | `AssertDiscovery` runs plan in a repository without a config file first. Plan inside the marked repository exits with the same status, and its standard error does not contain the unknown key of the config file above the marker. The case of `--config` passes the config file of the tree of the fixture | Plan without a config file exits 64 for a composition with a required option. A run that reads the config file above the marker exits 64 too, so the status alone does not show that the marker stops the search. The error of the decoder contains the unknown key |
| The design changes no check of the workspace suite. `AssertCycleRefused` builds the fixture over an empty directory | `AssertCycleRefused` copies the tree of the fixture into the directory before it builds | The config file of the tree sets the required options. A build without the file stops on the missing options before the check reaches the cycle |

## Alternatives Considered

### State the status without a config file in the fixture

A field of the fixture could state the status of plan in a repository without a config file. The
check derives the same status from a run of the binary, and a field can disagree with the binary.

### Check the standard error alone

The check could leave out the status and check only that the run reports nothing about the config
file above the marker. A host that discards its standard error would then pass while it reads that
file. The status catches such a host for a composition without a required option.

### Build the cycle without the build of the fixture

`AssertCycleRefused` could build the plans with the cycle and leave out the build of the plans of
the fixture. The check would then pass a composition that fails for another reason, where the error
of that composition also contains the names of the two plans.

## Consequences

- The acceptance kit has two more test hosts. Both search past every version control marker, and
  one of them exits 64 without a config file. The tests of the kit reject each host.
- A fixture of the workspace suite passes `AssertCycleRefused` where its composition reads the
  config file of its tree.
- `AssertCycleRefused` takes an empty directory, as the other checks of the suite do.

## References

- [RFC-0043](../rfc/0043-required-options-and-option-values-from-text.md)
