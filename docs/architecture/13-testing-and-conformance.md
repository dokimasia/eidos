# Testing and conformance

*Builds on: every mechanism before it. Feeds:
[15](15-compatibility.md) — the suite is the instrument
compatibility claims are executed with.*

The compatibility instrument of a multi-module public framework.
Everything here ships in the kernel's `conformance/` package and is
run by satellites and consumers alike; claims about compatibility,
determinism, and language support are all *executed*, never asserted.

## The rung ladder

Eight rungs, each proving something the others cannot:

| Rung | Proves |
|---|---|
| plugintest | one plugin's declarations, determinism, panic discipline (every fixture asserted not to panic — the other half of D68), and diagnostic discipline — rendering nothing. Includes template lint: every declared template parses against the merged funcmap per language, and every body-claiming template places the slot marker |
| backendtest | one backend over a hand-built emit graph |
| pipelinetest | plugins + a real backend → rendered files, driving one plan |
| workspacetest | a full workspace: multiple plans, exports, Close — collision detection, sweep, cross-plan checks, audit mode |
| frontendtest | a real frontend with the plugin chain behind it |
| acceptancetest | the consumer's binary end-to-end; the only rung that builds generated output |
| completeness | every `testdata/features/` row lands on its declared degradation rung ([11-languages.md](11-languages.md)) |
| warm≡cold | the same workspace cold and warm produces byte-identical manifests ([09-incrementality.md](09-incrementality.md)) |

Determinism discipline: `-count=2` minimum so map-order defects
cannot hide behind a single pass, `-race` where plugins hold state.

## Every rung, pinned

Each rung is a kernel-owned suite over a caller-supplied fixture:
harnesses and plugins supply fixtures, never assertion logic — the
assertion set and its failure wording are written once.

**plugintest** — `RunPluginSuite(t, f PluginFixture)`. Per plugin:
declaration stability (Name, Version, Provides, Outputs,
EmitVersions answer identically across calls); determinism (two
runs over one fixture store produce byte-equal emit, `-count=2`);
annotator idempotence; no structural writes (node count
unchanged); positioned diagnostics only; emitted tags are declared
tags; attribution (every emit value carries its plugin); options
schema stability, unknown keys and missing-required rejected;
template lint (every declared template parses against each
language's merged funcmap, body templates place the slot marker,
no reserved-name collisions); and the lowering guarantee — a
facade-authored plugin and its hand-rolled SPI twin produce
byte-equal output, 06b's promise held here.

**backendtest** — `RunBackendSuite(t, f BackendFixture)`. Over a
hand-built emit graph: byte-stable render (twice, equal); every
emit kind renders; slot contents render through the kind
machinery; header and trailer present and well-formed; the
format-failure flow continues per [07-rendering.md](07-rendering.md).

**pipelinetest** — `RunPipelineSuite(t, f PipelineFixture)`.
Plugins plus a real backend driving one plan: end-to-end bytes,
layout routing, the manifest slice, diagnostic discipline.

**frontendtest** — `RunFrontendSuite(t, f FrontendFixture)`. A
real frontend with the chain behind it: deterministic graphs, a
populated store, positioned diagnostics, classification stamps
present, owned outputs excluded, and fingerprint-keyed caching —
a probe cache observes that keys fold the unit fingerprint and
the frontend version.

**acceptancetest** — drives the consumer's binary as a process:
exit codes per [16-diagnostics.md](16-diagnostics.md), config
discovery, process-level idempotence, and compiling generated
output — the only rung that builds what was generated.

**completeness** — drives `testdata/features/` per
[11-languages.md](11-languages.md): every row lands exactly on
its declared rung; landing *better* than declared fails too.

Beside plugintest ships the **Tier-3 import lint**: a static pass
over a plugin module refusing imports of any satellite `sdk/`
package from neutral files — binding files are declared and
exempt, which is Tier 3 staying legal and visible
([03-projection.md](03-projection.md)).

## The two rungs that test the frame

Six of the eight rungs test parts; workspacetest and warm≡cold
test the frame itself. They are specified to the same
depth as the adapter below because they are the hardest to
retrofit — every workspace mechanism they exercise must have been
built testable.

**workspacetest** composes a real multi-plan workspace over
fixture sources and asserts the frame's promises one by one:

1. **collision** — two plans routed to one path is an Error at
   Close naming both plans, never last-writer-wins;
2. **isolation** — a plan seeded to fail leaves siblings' staged
   output and manifest slices intact;
3. **exports** — a dependent plan reads the producer's export in
   topo order; an unchanged export wakes no dependent;
4. **sweep** — removing a plan from the composition sweeps
   exactly that plan's files, drifted files excepted; a narrowed
   run sweeps nothing outside its scope
   ([08-workspace-and-plans.md](08-workspace-and-plans.md));
5. **audit** — a completeness contract left unsatisfied by a
   deliberately absent annotator reports at its declared
   severity;
6. **checks** — a `WorkspaceCheck` sees identical records cold
   and warm, carried plans included.

**warm≡cold** is three runs and two diffs:

1. cold run over the fixture; snapshot the manifest and every
   byte;
2. warm run with nothing changed: assert *zero* re-execution
   (the run stats report no rule fired) and a byte-identical
   manifest;
3. mutate one declaration, run warm; separately, run cold over
   the mutated tree in a fresh state directory: the two runs'
   manifests and bytes must be identical. The warm path may skip
   work; it may never change output.

Both harnesses are kernel code; fixtures and compositions are the
caller's:

```go
func RunWorkspaceSuite(t *testing.T, f WorkspaceFixture)
func RunWarmColdSuite(t *testing.T, f WorkspaceFixture)

type WorkspaceFixture struct {
    Sources fs.FS                       // the fixture tree
    Compose func() *workspace.Workspace // plans, plugins, config
    Mutate  func(Tree)                  // warm≡cold's one edit
}
```

## The toolchain-adapter skeleton

Without a shared skeleton, every language grows its own assertion
DSL and they drift by hand. The kernel therefore defines the
skeleton — the `Generated` fixture lifecycle and the assertion set
(`AssertParses` / `AssertTypeChecks` / `AssertTestsPass` /
`AssertSatisfies` / `AssertDoesNotSatisfy` / …) — over a small
adapter interface each language's `testing/` implements:

```go
type ToolchainAdapter interface {
    Layout(fixture Fixture) (dir string, err error)  // scratch project the toolchain accepts
    Parse(dir string) error                          // syntax only
    TypeCheck(dir string) error
    RunTests(dir string) (TestReport, error)
    Satisfies(dir, typeName, contract string) (bool, error)
}
```

Language harnesses are thin adapters; assertion semantics and
failure wording are written once, in the kernel, against this
interface. Adapters may *add* language-specific assertions on top
of the shared set — an `AssertVets`-class check that only means
something under one toolchain — exported from the satellite's
`testing/` under the same naming scheme; the kernel set is the
floor, not the ceiling. Toolchain-dependent assertions
skip with a recorded reason locally and are **required in CI** — a
regression must not hide behind a missing toolchain.

The fixture builders follow the same pattern: a neutral
symbol-fixture core in `conformance/`, per-language spelling
adapters in each satellite.

## Fixtures

- Test files mirror their subjects; fixture sources live in
  `testdata/` beside the test that drives them.
- The feature matrix is `testdata/features/` + one root conformance
  test per satellite — fixtures and a rung, not a package.
- Golden files are canonicalised: positions outside the fixture's
  own sources keep the file and lose the numbers. Positions in
  dependency sources move between toolchain releases; pinning them
  makes goldens fail on upgrades with diffs nobody can act on.

## Who runs what

- **Satellites**: the kernel suite at min and max declared kernel
  versions, every commit; their own feature-matrix and parity
  artifacts per release.
- **The kernel**: the canary ring before tagging
  ([15-compatibility.md](15-compatibility.md)).
- **Consumers**: plugintest over their own plugins, acceptancetest
  over their binary; workspacetest when they compose multi-plan
  workspaces.
