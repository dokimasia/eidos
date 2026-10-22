---
adr: 0100
title: The workspace leaves a gate on a key of an absent owner unbound
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

# ADR-0100: The workspace leaves a gate on a key of an absent owner unbound

## Status

Accepted

## Context

A rule can have a gate on a key that another plugin registers. The gate takes a handle from
`meta.Named`, which has the name of the key and no id. The workspace binds each such gate after the
key registry seals. It refused the composition for each name that the registry did not contain.

A plugin orders itself after the provider of a capability through `Requires` or `After`. `Requires`
refuses a composition without a provider of the capability. `After` orders the plugin only where the
composition has a provider.

The authoring guide described a weaver with a gate on a key of its owner. The weaver declares
`After(owner.Cap)` to serve compositions without the owner. Such a composition failed to build.

- In a composition without the owner, nothing registers the key. The bind of the gate returned the
  error "nothing registers the key", and the workspace collected it as a fault of the composition.
- debugweaver of the reference ensemble declares `After` for repogen and mockgen. Its gate tests the
  key `repogen.repository`. A composition of debugweaver without repogen failed to build.
- Issue 71 reports a generator of test doubles and a generator of suites. The generator of suites
  stamps a fact on each interface under its directive. A rule of the generator of doubles has a gate
  on that fact, so a project that generates doubles alone could not build its composition.
- The bind returned the same error for a misspelled key as for a key of an absent owner.
- `meta.Register` refuses a key of an unclaimed namespace. Every registered key has a namespace with
  a claimant.

## Decision

We will leave a gate on a key of an absent owner unbound where its plugin runs after a capability
without a provider. An unbound gate does not select a subject.

- The bind returns an error that wraps `plugin.ErrUnclaimedNamespace` for a name of an unclaimed
  namespace. For an unregistered name of a claimed namespace, it returns the error "nothing
  registers the key".
- For a plugin that runs after a capability that no plugin of the composition provides, the
  workspace drops each error that wraps `plugin.ErrUnclaimedNamespace`. Every other error of the
  bind remains a fault of the composition.
- The dispatch does not find a fact under an unbound gate. `HasKey` and `KeyEquals` on such a key do
  not select a subject. `Not` over either selects every subject of its trigger.
- A misspelled key still fails the build where a plugin claims the namespace of the key. It also
  fails the build where each capability of `After` has a provider.
- `plugintest.Fixture` binds a plugin by the same rule. The composition of its bind is the plugin and
  the plugins of the source that `plugintest.Loaded` loads.
- The signature of `plugin.KeyBinder` does not change.

## Alternatives Considered

### A handle that marks its key as optional

A plugin would mark a gate as optional with a handle such as `meta.Optional[bool](name)`. The bind
would leave such a gate unbound wherever the registry lacks the key. No composition would then
refuse a misspelled optional key, also a composition with the owner. `After` already states that the
plugin serves compositions without the provider. The handle would state that fact a second time.

### An unbound gate for every unregistered name

The bind would leave each gate on an unregistered name unbound. No composition would then refuse a
misspelled key.

### A namespace for each capability

The workspace would drop the error only for a namespace that belongs to the absent capability. A
capability and a namespace are separate names. repogen provides the capability `repository` and
claims the namespace `repogen`. In a composition without the provider, nothing relates the two
names.

### A variant of the weaver without the gate

The weaver would come in two variants, and the root of each composition would choose one. Each root
would then need to know the owner whose key the gate reads. The weaver already states that
dependency once through `After`.

## Consequences

**Positive:**

- debugweaver builds in a composition without repogen. Its gate does not select a subject there. The
  generator of doubles of issue 71 builds in a composition without the generator of suites.
- A key with a misspelled local part still fails each composition that contains its owner.
- The kernel gains one sentinel, and no plugin changes its declarations.

**Negative:**

- Where a capability of `After` has no provider, a run does not report a finding for a key with a
  misspelled namespace. Each composition in which every capability of `After` has a provider still
  refuses the key.
- A negated gate on a key of an absent owner selects every subject of its trigger.
- A fixture of `plugintest.New` has no source, so its composition is the plugin alone. A spec over
  such a fixture does not refuse a misspelled namespace in a plugin that runs after the capability of
  another plugin.

## References

| What | Where |
|---|---|
| D218, the row of this decision | ../architecture/21-decisions.md |
| The two-package weave and `After` | ../architecture/06b-authoring.md |
| RFC-0022, the named key handles and the bind that refuses a name that the registry does not contain | ../rfc/0022-the-shape-catalog.md |
| RFC-0030, `After` of `plugin.CapabilityProvider` | ../rfc/0030-the-contracts-of-the-spi-and-the-authoring-api.md |
| RFC-0041, the gates of a rule and the order of a plugin | ../rfc/0041-the-gates-of-a-rule-and-the-order-of-a-plugin.md |
| Issue 71, a weaver with a gate on a key of its owner faults every composition without the owner | <https://github.com/dokimasia/eidos/issues/71> |
