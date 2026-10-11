---
milestone: 0012
title: A plugin runs from files alone
status: Done
depends-on: 0005
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: 0025
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Milestone 0012: A plugin runs from files alone

## Goal

A directory holding `plugin.yaml` and templates runs as a generator
with no Go in it. Schemas, outputs, capabilities and options come from
the manifest, the manifest hashes into the run fingerprint, and a
consumer binary that compiles in the loader gives its users extension
without a toolchain.

## Done when

- [x] The loader turns N manifests into N ordinary plugins:
      priorities, capability topology, options and conformance apply
      per declarative plugin, and no kernel package outside the loader
      imports its types.
- [x] The acme-audit example from
      [06-plugins.md](../architecture/06-plugins.md) runs as a
      fixture: its directive validates against the manifest's schema,
      its template renders, and its output routes through its declared
      tagged outputs.
- [x] `RunPluginSuite` passes over a declarative plugin with no
      special-casing.
- [x] A unit test asserts that the manifest and the template tree hash
      into the plugin's fingerprint contribution.
- [x] The manifest schema is published as a versioned JSON Schema, and
      an editor completes manifest fields against it.

## Why now

This starts after 0005: the loader adapts manifests onto the plugin
surface, and the templates render through the machinery that 0005 put
under a real run. Past 0005 its position is capacity: no scheduled
milestone consumes it, but 0014 freezes its manifest schema as public
API, so it must arrive before the release.

## Scope

The declarative host of [06-plugins.md](../architecture/06-plugins.md).

## Not in this milestone

- Typed metadata, projections, or logic beyond template conditionals
  in manifests: structural limits of the tier, stated in
  [06-plugins.md](../architecture/06-plugins.md). Needing them means
  writing Go, and that is the design, not a gap.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| Manifest schema mistakes freeze at 0014 | 0014 | Exercise the schema with real example plugins in this milestone, and change it freely until the release |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-10-11 | Ticked the last bullet and set the milestone to Done | The module contains the schema of format 1 as `plugin.schema.json`, and a golden test compares the file with `Schema`. A probe drives yaml-language-server 1.25.0 over stdio in the way that an editor drives it. The server reads the schema through the modeline of the fixture's manifest, and it reports no diagnostic for the manifest. It completes the keys of the manifest, of a rule, of a family and of a directive param, and the values of `emit` and `per`. It reports an unknown key and a format other than 1 |
| 2026-10-10 | The last bullet waits for a check in an editor | python-jsonschema 4.26.0 over ruamel.yaml, which reads YAML 1.2, accepts the manifest of the fixture under `eidos-declarative/plugin.schema.json`. It refuses an unknown key, an unknown emission and a manifest without `format`. No editor has completed a manifest against the schema yet |
| 2026-10-10 | Ticked the first four bullets | `declarative.Load` builds each plugin directory into a generator through the authoring surface, in the module `eidos-declarative`, which no kernel package imports. The conformance fixture opens a Go tree through `cli.Open`, and a cold run writes `svc/ledger_audit.go`, `svc/ledger_audit_test.go`, `ops/transfer_audit.go` and `ops/transfer.audit.ts`. A warm run writes the same files and reports the same findings. `level=trace` reports `BadSpelling` and `sink=missing` reports `UnresolvedReference` at the carrier. `go vet` passes over the Go output, and `go test` runs the generated test. `RunPluginSuite` passes over the plugin of the fixture and over the plugin of the module's tests. The module's tests assert that an edit of one byte of the manifest or of a template, a renamed template and a new template change the version and the fingerprint |
| 2026-10-10 | A family claims its files through a file template, and the fixture's family `test` writes a Go test of each implementation | A declarative plugin has to generate the tests of the code that it generates, and no rule of a manifest describes a test file ([ADR-0019](../adr/0019-a-family-claims-its-files-through-a-template.md)) |
| 2026-10-10 | The loader is the module `eidos-declarative`, not a package of the CLI module | The kernel's runtime imports the standard library alone, and a composition loads a plugin directory without the command kernels ([ADR-0018](../adr/0018-the-declarative-host-is-a-module.md)) |
| 2026-10-10 | Linked RFC-0025 | The RFC describes the loader, the manifest, the template data, the extensible plans and the fixture |
| 2026-08-30 | Added at position 12 | Ecosystem feature with no scheduled consumer. Placed late but before 0014, which makes its schema public API |
