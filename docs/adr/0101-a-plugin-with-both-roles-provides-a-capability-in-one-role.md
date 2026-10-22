---
adr: 0101
title: A plugin with both roles provides a capability in one role
status: Accepted
date: 2026-10-22
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0101: A plugin with both roles provides a capability in one role

## Status

Accepted

## Context

A plugin with both roles runs an annotator half and a generator half. Each half is a member of its
own role, and the workspace orders the members of a role by priority and by capabilities. Every
annotator runs before every generator. The workspace gave both halves of such a plugin every
capability of the plugin.

The weave of ADR-0100 failed in a composition with the owner when the owner required the capability
of the weaver.

- The weaver declares `After(owner)`, so the workspace leaves its gate unbound in a composition
  without the owner.
- The annotator half of the owner stamps the key that the gate tests. The generator half of the
  owner requires the capability of the weaver.
- Both halves of the owner provided its capability, so the weaver ran after the generator half of
  the owner. That generator half required the weaver, and the workspace refused the composition
  with a capability cycle in the generator role.
- The weaver does not need an order against the generator half, because its gate tests a key of the
  annotator half.
- Issue 72 reports the failure for a generator of test doubles and a generator of conformance
  suites.

`Priority` already takes a role, because one priority cannot place the two halves of a plugin
independently.

## Decision

We will let a plugin with both roles provide a capability in one role.

- `plugin.CapabilityProvider.Provides` takes the role whose capabilities it returns, as `Priority`
  does.
- `Builder.Provides` declares a capability of both roles. `Builder.ProvidesIn` declares a
  capability of one role. A typed build panics on a capability of a role that its plugin lacks, and
  on a role other than the annotator role and the generator role.
- A member of a role provides the capabilities of that role alone. A capability of the annotator
  role orders no generator.
- The check that a label has one provider, the check that a required label has a provider, and the
  bind of ADR-0100 read the capabilities of the roles that each plugin implements. A capability of a
  role that a plugin lacks has no provider.
- The advice of a gate on a key of a later annotator lists the capabilities of the annotator role of
  the registrant.
- `Requires` and `After` keep one list for both halves. Each half of a plugin with both roles
  already runs after the providers of its own role.

## Alternatives Considered

### A marker of the weaver without an order

The weaver would declare a capability that it reads without an order, such as `Optional(owner)`.
The bind of ADR-0100 would read the marker beside `After`. The marker adds a third verb for
capabilities, and the bind is its only reader. The weaver would also need to know which half of the
owner produces what it reads. The owner knows that, and states it once for every consumer.

### An order that yields to a requirement

The workspace would drop an `After` edge that closes a cycle with a `Requires` edge. A weaver that
also writes into the units of the generator half of the owner would then lose its order without a
finding.

### A second interface for the capabilities of a role

A plugin would declare the capabilities of each role through a second interface, and `Provides`
would keep its form. Both interfaces would then state the capabilities of one plugin, and a
hand-rolled provider could make them disagree. In this repository, only the typed build and the test doubles of
the kernel implement `CapabilityProvider`.

## Consequences

**Positive:**

- The weave of issue 72 builds with the owner and without it. With the owner, the weaver runs before
  the generator half of the owner, and its gate binds and refuses a misspelled key.
- A plugin with both roles can provide one capability for its stamps and another for its units.
- A plugin that declares its capabilities through `Builder.Provides` keeps its order.

**Negative:**

- Each hand-rolled `plugin.CapabilityProvider` adds the role parameter to `Provides`.
- A typed build of a plugin with capabilities allocates a map and a list for each role, where it
  allocated one list.
- An owner whose generator half requires the weaver declares its capability with
  `Builder.ProvidesIn`. Under `Builder.Provides`, the composition still fails with a capability
  cycle.

## References

| What | Where |
|---|---|
| D219, the row of this decision | ../architecture/21-decisions.md |
| ADR-0100, the gate on a key of an absent owner | 0100-the-workspace-leaves-a-gate-on-a-key-of-an-absent-owner-unbound.md |
| The capabilities and the priorities of a plugin | ../architecture/06-plugins.md |
| The two-package weave | ../architecture/06b-authoring.md |
| RFC-0030, the capabilities of `plugin.CapabilityProvider` | ../rfc/0030-the-contracts-of-the-spi-and-the-authoring-api.md |
| Issue 72, the capability cycle of the weave of ADR-0100 | <https://github.com/dokimasia/eidos/issues/72> |
