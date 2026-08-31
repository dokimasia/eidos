---
milestone: 0012
title: A plugin runs from files alone
status: Planned
depends-on: 0005
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: none
---

# Milestone 0012: A plugin runs from files alone

## Goal

A directory holding `plugin.yaml` and templates runs as a generator
with no Go in it. Schemas, outputs, capabilities and options come from
the manifest, the manifest hashes into the run fingerprint, and a
consumer binary that compiles in the loader gives its users extension
without a toolchain.

## Done when

- [ ] The loader turns N manifests into N ordinary plugins:
      priorities, capability topology, options and conformance apply
      per declarative plugin, and no kernel package outside the loader
      imports its types.
- [ ] The acme-audit example from
      [06-plugins.md](../architecture/06-plugins.md) runs as a
      fixture: its directive validates against the manifest's schema,
      its template renders, and its output routes through its declared
      tagged outputs.
- [ ] `RunPluginSuite` passes over a declarative plugin with no
      special-casing.
- [ ] A unit test asserts that the manifest and the template tree hash
      into the plugin's fingerprint contribution.
- [ ] The manifest schema is published as a versioned JSON Schema, and
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
| 2026-08-30 | Added at position 12 | Ecosystem feature with no scheduled consumer. Placed late but before 0014, which makes its schema public API |
