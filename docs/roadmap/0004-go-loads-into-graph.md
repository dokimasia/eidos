---
milestone: 0004
title: Go source loads into the symbol graph
status: Planned
depends-on: 0001, 0002
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: none
---

# Milestone 0004: Go source loads into the symbol graph

## Goal

The Go frontend parses real packages into the frozen graph through the
frontend kit: identities, docs and positions, canonical directives out
of `//+gen:` carriers, `go.*` and `gen.module` facts, and Link
resolving cross-package spellings. The projections cover Tier 1 for
Go.

## Done when

- [ ] `RunFrontendSuite` passes for eidos-lang-go: the same fixture
      parses to an identical graph twice, diagnostics are positioned,
      classification stamps are present, and a probe cache observes
      that fingerprint keys fold in the unit fingerprint and the
      frontend version.
- [ ] Link works: a two-package fixture resolves `TypeRef` spellings
      to canonical identities, builtins and external types keep
      spelling only, and `Reader.Lookup` joins across the packages
      afterwards.
- [ ] Reparsing an unchanged file yields the same identities. This
      seeds milestone 0007's diff-by-identity.
- [ ] `//+gen:` carriers strip and parse to canonical directives. An
      unclaimed directive is reported, and the workspace opt-out
      silences it.
- [ ] The kernel-owned `sample` and `witness` directives stamp their
      `gen.*` keys through the default annotators, and `SamplesOf`
      and `Witnesses` read from those stamps before deriving
      anything (D64).
- [ ] `go.work` and `go.mod` are read declaratively, `gen.module` and
      `gen.moduleRoot` are stamped, and a static check asserts the
      frontend never imports `os/exec`.
- [ ] Signature-only loading works: an out-of-scope dependency package
      loads through the same `Parse` with `Depth() == Signatures`,
      bodies and private members absent.
- [ ] Test files parse and carry `go.testFile`. The kit excludes
      nothing except workspace-owned outputs, which it refuses before
      `Parse` when a fixture provides the trailer or manifest proof.
- [ ] Tier 1 is covered for Go: `CallableOf`, `TypeOf` into canonical
      shapes, `Resolve`, `MembersOf` across embeds with per-member
      provenance and refusal reasons, the three Values returns, and
      `TypeName`. Tier 2 where Go satisfies it: enums with a small
      iota evaluator, sentinel names, struct tags, promotion,
      comparability, and generics with authored witnesses.
- [ ] `testdata/features/` holds one fixture per Go row of the
      landscape table, and the completeness check, built in the kernel
      as part of this milestone, passes with every row on its declared
      check.
- [ ] The kernel's toolchain-adapter skeleton exists and
      `eidos-lang-go/testing` implements it: the shared assertion set
      (`AssertParses`, `AssertTypeChecks`, `AssertTestsPass`,
      `AssertSatisfies`) runs over a `Generated` fixture through the
      Go toolchain. A toolchain-dependent assertion skips locally with
      a recorded reason and is required in CI.

## Why now

This starts after 0001 and 0002: the kit registers through Build and
fills 0002's store. Go is the first language because its parser is the
standard library's, so no tree-sitter layer is needed (D16). Milestone
0005 joins this graph to the write side.

## Scope

The frontend-kit half of
[11-languages.md](../architecture/11-languages.md) including
hermeticity and classify-not-exclude, Go's `rules/` per
[03-projection.md](../architecture/03-projection.md), Link from
[02-symbol-model.md](../architecture/02-symbol-model.md), carriers from
[05-directives.md](../architecture/05-directives.md), and frontendtest
plus the completeness check from
[13-testing-and-conformance.md](../architecture/13-testing-and-conformance.md).

## Not in this milestone

- Consuming fingerprints for warm runs: they are recorded only.
  Milestone 0007.
- eidos-lang and tree-sitter: milestone 0009.
- Types the declarations do not state (inferred `var x = f()`): these
  sits at level 2 with `go.inferred` carrying the spelling, per the
  degradation scale. That is the declared behaviour, not deferred
  work.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| `MembersOf` and generics substitution are the deepest rules code, and wrong returns make later generators ship incomplete doubles silently | 0005, 0011 | The MemberSet failure reasons and the completeness check make degradation visible. Rows sits at level 2 or 3 honestly instead of blocking the milestone |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-08-30 | Pinned the `sample`/`witness` directives and the toolchain-adapter skeleton into Done when | A coverage audit against the architecture found them held by Scope reference only |
| 2026-08-30 | Added at position 4 | First real language. Go comes first because its parser needs no tree-sitter layer |
