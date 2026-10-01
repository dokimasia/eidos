---
milestone: 0004
title: Go source loads into the symbol graph
status: In progress
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
of `//+<brand>:` carriers, `golang.*` and `gen.module` facts, and Link
resolving cross-package spellings. The projections cover Tier 1 for
Go. The TypeScript, Rust and Java frontends load their languages
into the same graph through the tree-sitter platform in eidos-lang.

## Done when

- [x] `RunFrontendSuite` passes for eidos-lang-go: the same fixture
      parses to an identical graph twice, diagnostics are positioned,
      classification stamps are present, and a probe cache observes
      that fingerprint keys fold in the unit fingerprint and the
      frontend version.
- [x] Link works: a two-package fixture resolves `TypeRef` spellings
      to canonical identities, builtins and external types keep
      spelling only, and `Reader.Lookup` joins across the packages
      afterwards.
- [x] Reparsing an unchanged file yields the same identities. This
      seeds milestone 0007's diff-by-identity.
- [x] `//+<brand>:` carriers strip and parse to canonical directives. An
      unclaimed directive is reported, and the workspace opt-out
      silences it.
- [x] The kernel-owned `sample` and `witness` directives stamp their
      `gen.*` keys through the default annotators, and `SamplesOf`
      and `Witnesses` read from those stamps before deriving
      anything (D64).
- [x] `go.mod` is read declaratively, `gen.module` and
      `gen.moduleRoot` are stamped, and a static check asserts the
      frontend never imports `os/exec`. `go.work` is not read, because
      a workspace spans modules by its own configuration.
- [ ] Dependency modules load signature-only from the module cache:
      `go.mod` fixes each module's version, the cache directory
      verifies against its `go.sum` hash, the dependency unit keys on
      both, a `vendor/` tree with a consistent `modules.txt` is the
      second source, and a module the machine lacks fails the load.
- [x] Signature-only loading works: an out-of-scope dependency package
      loads through the same `Parse` with `Depth() == Signatures`,
      bodies and private members absent.
- [x] Test files parse and receive `golang.testFile`. The kit excludes
      nothing except workspace-owned outputs, which it refuses before
      `Parse` when a fixture provides the trailer or manifest proof.
- [x] Tier 1 is covered for Go: `CallableOf`, `TypeOf` into canonical
      shapes, `Resolve`, `MembersOf` across embeds with per-member
      provenance and refusal reasons, the three Values returns, and
      `TypeName`. Tier 2 where Go satisfies it: enums with a small
      iota evaluator, sentinel names, struct tags, promotion,
      comparability, generics with authored witnesses, and the
      whole-graph facts the old frontend stamped through its type
      checker: Stringer satisfaction and iter.Seq returns.
- [ ] The conformance corpus is the completeness check: Go,
      TypeScript, Rust and Java each have a graded entry, and each
      entry passes with every inventory feature on its declared level.
- [ ] eidos-lang contains the tree-sitter bindings and the pinned
      TypeScript, Rust and Java grammars, and is their only importer.
- [ ] `RunFrontendSuite` passes for eidos-lang-typescript: its
      frontend parses through eidos-lang, and an overload set spells
      one discriminator per signature.
- [ ] `RunFrontendSuite` passes for eidos-lang-rust: its frontend
      parses through eidos-lang.
- [ ] `RunFrontendSuite` passes for eidos-lang-java: its frontend
      parses through eidos-lang, an overload set spells one
      discriminator per signature, dependencies load signature-only
      from JARs through the class-file reader, and annotations stamp
      as `java.annotation.*` facts.
- [x] The kernel's toolchain-adapter skeleton exists and
      `eidos-lang-go/testing` implements it: the shared assertion set
      (`AssertParses`, `AssertTypeChecks`, `AssertTestsPass`,
      `AssertSatisfies`) runs over a `Generated` fixture through the
      Go toolchain. A toolchain-dependent assertion skips locally with
      a recorded reason and is required in CI.

## Why now

This starts after 0001 and 0002: the kit registers through Build and
fills 0002's store. Go is the first language because its parser is the
standard library's, so no tree-sitter layer is needed (D16). The
TypeScript, Rust and Java frontends load in the same milestone,
because a defect in the kit's contract shows through the language the
others do not share. Milestone 0005 joins this graph to the write
side.

## Scope

The frontend-kit half of
[11-languages.md](../architecture/11-languages.md) including
hermeticity, classify-not-exclude and the tree-sitter platform, Go's
`rules/` per [03-projection.md](../architecture/03-projection.md), Link
from [02-symbol-model.md](../architecture/02-symbol-model.md), carriers
from [05-directives.md](../architecture/05-directives.md), eidos-lang
per [01-repos-and-kernel.md](../architecture/01-repos-and-kernel.md),
and frontendtest plus the completeness check from
[13-testing-and-conformance.md](../architecture/13-testing-and-conformance.md).

## Not in this milestone

- Consuming fingerprints for warm runs: they are recorded only.
  Milestone 0007.
- The projection rules of TypeScript, Rust and Java: TypeScript's go
  to milestone 0009, and nobody has scheduled Rust's or Java's yet.
- Wazero-based bindings: adopt when they mature. The binding choice
  is private to eidos-lang, so migrating later touches one module.
- Types the declarations do not state (inferred `var x = f()`): these
  are at level 2, with `golang.inferred` recording the spelling, per the
  degradation scale. That is the declared behaviour, not deferred
  work.

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| `MembersOf` and generics substitution are the deepest rules code, and wrong returns make later generators ship incomplete doubles silently | 0005, 0011 | The MemberSet failure reasons and the completeness check make degradation visible. Rows sits at level 2 or 3 honestly instead of blocking the milestone |
| The tree-sitter Go bindings use cgo, which taxes every consumer build that embeds a tree-sitter satellite | 0013 (the reference binary builds it), consumers | Recorded in [11-languages.md](../architecture/11-languages.md). The binding choice is private to eidos-lang, so a wazero migration later changes one module |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-09-30 | Added Done-when bullets for the tree-sitter platform and the TypeScript, Rust and Java frontends, and moved the wazero note and the cgo risk from milestone 0009 | The widening of 2026-09-01 pulled the platform and the three frontends into this milestone, and Done when and Not in this milestone had not followed |
| 2026-09-30 | The completeness bullet names the conformance corpus | The specification makes the corpus the completeness check, one graded entry per language, in place of a `testdata/features/` tree per satellite |
| 2026-09-30 | The module-file bullet names `go.mod` alone, and is checked | The frontend reads `go.mod` and stamps both module facts, and it does not read `go.work`, because a workspace spans modules by its own configuration |
| 2026-09-30 | Status set to In progress | Nine of the Done-when bullets are checked, and the status read Planned |
| 2026-09-30 | Added the bullet for dependency modules | The frontend excludes `vendor/` and reads nothing from the module cache, so a type from another module keeps its spelling alone, and every embed of a third-party interface reports a gap |
| 2026-09-30 | The Go keys are spelled `golang.*` | The satellite registers its keys under its language identity, `golang` |
| 2026-09-30 | The carrier spelling in the goal and the carrier criterion changed from `//+gen:` to `//+<brand>:` | The carrier mark follows the composition's brand. The Go frontend reads carriers under the brand, and its suite passes with brand-marked fixtures |
| 2026-09-01 | Widened to a horizontal wave: Go, protobuf, TypeScript, Rust and Java frontends build together, per capability, over one shared cross-language feature corpus with read-side coverage declared as data | Four audits of the old frontends against RFC-0013 found every contract defect through a language the others did not share; the backend kit held for the same reason two consumers arrived together. Pulls the tree-sitter platform forward from 0009; Java forces the two surfaces no sibling touches, dependency artifacts and read-side annotations; 0009 and 0010 keep their policy halves |
| 2026-08-30 | Pinned the `sample`/`witness` directives and the toolchain-adapter skeleton into Done when | A coverage audit against the architecture found them held by Scope reference only |
| 2026-08-30 | Added at position 4 | First real language. Go comes first because its parser needs no tree-sitter layer |
