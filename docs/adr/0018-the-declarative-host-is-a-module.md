---
adr: 0018
title: The declarative host is a module of its own
status: Accepted
date: 2026-10-10
supersedes: none
superseded-by: none
rfc: RFC-0025
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0018: The declarative host is a module of its own

## Status

Accepted

## Context

The declarative host decodes a YAML manifest and builds a generator from a plugin directory. Two
constraints decide where its code can be:

- The runtime packages of the kernel import the standard library alone. The kernel is part of
  every binary built with eidos, so a dependency of the kernel enters the supply-chain audit of
  every consumer.
- The config package of the CLI module loads the plugin directories that a config file lists. A
  test or a program that builds a workspace directly loads a directory with `Load` as well, as
  the conformance fixture does for the plugin suite.

The accepted design put the loader into the CLI module, because that module already depends on
the kernel and on the YAML library. It rejected a module of its own as one more release and one
more version range for one package.

## Decision

We will keep the declarative host in the module `go.dokimi.dev/eidos/declarative`, beside the
kernel and the CLI module, because the kernel's runtime does not take a third-party dependency
and a composition loads a plugin directory without the command kernels.

## Alternatives Considered

### The loader in the CLI module

The package `eidos-cli/declarative` contains the loader, and the CLI module publishes the
manifest's JSON Schema beside `config.schema.json`. The CLI module already depends on the kernel
and on the YAML library, and a user adds a plugin in the config file that the CLI module reads.
It lost because a test or a program that loads one plugin directory would then depend on the
command kernels and on everything they import.

### The loader in the kernel, with the YAML library

The kernel's package `plugin/declarative` decodes `plugin.yaml` through `go.yaml.in/yaml/v3`,
and every consumer receives the host with the kernel. It lost because the YAML library would
enter every binary built with eidos, including each binary that does not load a declarative
plugin.

## Consequences

**Positive:**

- The runtime packages of the kernel keep importing the standard library alone.
- A composition or a test loads a plugin directory with `declarative.Load`, and passes the
  generator to a plan through `workspace.PlanConfig.Generators`.

**Negative:**

- The repository releases and versions one more module, and the module declares its own range
  of kernel versions.
- Each module of the repository that requires the CLI module also requires the declarative
  module, and replaces it with its directory, as the conformance module does.
- The module registers its codes under its own prefix, `DECLARATIVE`. `IncompleteMembers` is
  `DECLARATIVE-0001` and not a kernel code, so a consumer that selects findings by the kernel
  prefix does not see it.
- The JSON Schema of the manifest is `eidos-declarative/plugin.schema.json`. The modeline of a
  manifest refers to that file of the release that built the binary.

**Neutral:**

- The CLI module depends on the declarative module to load the directories of
  `plans.<name>.plugins`.

## References

| What | Where |
|---|---|
| The module topology and the dependencies of the kernel | ../architecture/01-repos-and-kernel.md |
