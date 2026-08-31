---
milestone: 0002
title: A typed plugin runs over a hand-built graph
status: Done
depends-on: 0001
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: RFC-0003, RFC-0004, RFC-0005, RFC-0006, RFC-0007
---

# Milestone 0002: A typed plugin runs over a hand-built graph

## Goal

A plugin author writes a plugin as a value, runs it over a hand-built
fixture graph, and proves it byte-deterministic with plugintest. Build
validates the composition and reports every fault at once. No language
exists yet, and none is needed: sources are fixture stores and
backends are test fakes until milestones 0004 and 0005 supply real
ones.

## Done when

- [x] The authoring surface of
      [06b-authoring.md](../architecture/06b-authoring.md) compiles:
      kind-indexed triggers with generated Match types, `OnEmit` and
      `OnGraph`, `Directive` and `Where` gates, the Emitter and
      Stamper effects, and `NewPlugin(...).Build()`.
- [x] Build runs the validation sequence of
      [08-workspace-and-plans.md](../architecture/08-workspace-and-plans.md)
      and collects faults: a fixture composition seeded with five
      distinct faults reports all five in one error.
- [x] Every registry refuses a duplicate with an error naming both
      claimants: metadata keys and their namespaces, directive
      schemas, diagnostic codes, capability labels.
- [x] The store enforces freeze: a structural write after Annotate is
      refused with a stable diagnostic code.
- [x] Metadata writes arbitrate by the four-step rank in
      [04-metadata.md](../architecture/04-metadata.md), `meta drop`
      removes a fact or a group, and every write records provenance
      with its read set. The `explain` command that walks it comes in
      milestone 0008; the record starts here.
- [x] Directive validation runs over fixture directives: closure,
      param types, repeatability, `Requires` and `ConflictsWith`, all
      reported as positioned Errors before any handler runs.
- [x] `RunPluginSuite` passes over the fixture plugins: declaration
      stability, byte-equal emit under `-count=2`, annotator
      idempotence, no structural writes, positioned diagnostics,
      declared tags, attribution, options schemas, and no fixture
      panics (the conformance half of D68).
- [x] The lowering guarantee holds: a facade-authored plugin and its
      hand-rolled SPI twin produce byte-equal emit in plugintest.
- [x] Dispatch is indexed: a rule gated on a directive visits only the
      subjects that carry it, which a fixture checks by counting
      handler calls.
- [x] The kernel `skip` directive works at dispatch: it excludes a
      subject from bare and fact-gated rules, `skip plugin=<name>`
      excludes one plugin, and directive-gated rules are unaffected.

## Why now

The first frontend forces this order, because a frontend is a plugin.
It registers through Build, writes `go.*` facts through the metadata
machinery, and hands every carrier line to the directive registry to
be checked against a schema. Milestone 0004 cannot start until those
seams exist.

Testing does not have to wait for a language: plugintest is specified
to run a plugin over a fixture store while rendering nothing
([13-testing-and-conformance.md](../architecture/13-testing-and-conformance.md)),
so this milestone is checkable on its own.

This starts after 0001 because the triggers, the Match types and the
store's structs generate from the schema. Milestone 0003 also needs
only 0001, so 0002 and 0003 can run in parallel; their order in the
plan is arbitrary. Milestone 0005 runs this dispatch over real source.

## Scope

[06-plugins.md](../architecture/06-plugins.md),
[06b-authoring.md](../architecture/06b-authoring.md),
[04-metadata.md](../architecture/04-metadata.md), the grammar and
validation halves of [05-directives.md](../architecture/05-directives.md),
the Build sequence of
[08-workspace-and-plans.md](../architecture/08-workspace-and-plans.md),
the store and Reader of
[02-symbol-model.md](../architecture/02-symbol-model.md), and
plugintest from
[13-testing-and-conformance.md](../architecture/13-testing-and-conformance.md).

## Not in this milestone

- Any user-visible generation: nothing parses and nothing arrives on
  disk until milestones 0004 and 0005. Build here validates plans
  against fake backends and a fake registered target.
- The template-lint half of plugintest: needs the template machinery,
  goes to milestone 0003.
- Carriers in real source files: directives enter as fixture data
  here. Parsing them out of comments goes to milestone 0004.
- Consuming the recorded read edges: nothing invalidates yet. Goes to
  milestone 0007.
- The declarative host: milestone 0012.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| The Effect type parameter (the handler signature picks the role) may not survive contact with real plugin code | 0005, 0011 | The SPI stays public. Worst case the facade narrows to explicit constructors, which is free to do before anything is tagged |
| The largest single milestone in the plan; scope tends to grow toward "all of the kernel" | 0004, 0005 | plugintest green defines done. Anything no check exercises moves out |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-08-30 | Status Planned to Done | Every exit criterion is met and `make check` passes; RFC-0003 through RFC-0007 are accepted and linked |
| 2026-08-30 | Reworded the bullet from seven steps to the validation sequence | The accepted sequence carries no policy step and no version handshake: no policy registry or second versioned component exists to check against, and each is an addition between the steps that are |
| 2026-08-30 | Pinned the `skip` directive and the no-panic assertion into Done when | A coverage audit against the architecture found them held by Scope reference only, so nothing forced them to exist |
| 2026-08-30 | Retitled from "Typed plugins compose and Build validates"; goal restated over fixture graphs | The old title claimed a composition capability that only exists at 0005. The plugin frame still comes second: a frontend is a plugin, so 0004 needs these seams first |
| 2026-08-30 | Added at position 2 | The middle of the machine comes before the edges: plugins and Build are testable over hand-built graphs, so no language needs to exist first |
