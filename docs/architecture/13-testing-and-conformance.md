# Testing and conformance

*Builds on: every mechanism before it. Feeds:
[15](15-compatibility.md), since this suite is the instrument every
compatibility claim is executed with.*

This is how a framework split across modules keeps its promises.
Everything here ships in the kernel's `conformance/` package, and
satellites and consumers run it too. Claims about compatibility,
determinism and language support are executed rather than asserted.

## The check ladder

Eight checks, each proving something the others cannot:

| Check | Proves |
|---|---|
| plugintest | one plugin's declarations, its determinism, that no fixture panics (the other half of D68), and its diagnostic discipline, while rendering nothing. It includes template lint: every declared template parses against the merged funcmap per language, and every body-claiming template places the slot marker |
| backendtest | one backend over a hand-built emit graph |
| pipelinetest | plugins plus a real backend producing rendered files, driving one plan |
| workspacetest | a full workspace: several plans, exports, and Close, covering collision detection, sweep, cross-plan checks and audit mode |
| frontendtest | a real frontend with the plugin chain behind it |
| acceptancetest | the consumer's binary end to end. The only check that compiles generated output |
| completeness | every `testdata/features/` row sits on the degradation level it declared ([11-languages.md](11-languages.md)) |
| warm≡cold | the same workspace, run cold and warm, produces byte-identical manifests ([09-incrementality.md](09-incrementality.md)) |

Two disciplines apply throughout. Run with `-count=2` at minimum, so
a defect that depends on map order cannot hide behind a single pass.
Run with `-race` wherever plugins hold state.

## Every check, pinned

Each check is a suite the kernel owns, running over a fixture the
caller supplies. Harnesses and plugins supply fixtures and never
assertion logic, so the assertion set and the wording of its
failures are written once.

**plugintest**, through `RunPluginSuite(t, setup Setup)`, where the
setup builds the plugin with its fixture fresh per call. Per plugin
it checks: declaration stability, meaning the name, the gate
records, the outputs and the owned schemas answer identically
across builds;
determinism, meaning two runs over one fixture store produce
byte-equal emit under `-count=2`; annotator idempotence; that no
structural write happened, since the node count is unchanged; that
every diagnostic is positioned; that emitted tags are declared tags;
attribution, meaning every emit value carries its plugin; options
schema stability, with unknown keys and missing required keys
rejected; template lint, meaning every declared template parses
against each language's merged funcmap, body templates place the
slot marker, and no name collides with a reserved one; and the
lowering guarantee, meaning a facade-authored plugin and its
hand-rolled SPI twin produce byte-equal output, which is where 06b's
promise is held.

**backendtest**, through `RunBackendSuite(t, setup Setup)`. Over a
hand-built emit fixture it checks: that the fixture is inhabited,
because an empty store passes everything vacuously; byte-stable
render, two isolated runs compared as files and as a finding set;
that every emit kind the fixture carries renders; that every body
lands whole, with slot contents spliced through the kind machinery;
and that a file's failure reports positioned and attributed while
the render continues per [07-rendering.md](07-rendering.md), the
refused file withheld. The header and trailer checks are the output
contract's and join the suite with it.

**pipelinetest**, through `RunPipelineSuite(t, f PipelineFixture)`.
Plugins plus a real backend driving one plan: end-to-end bytes,
layout routing, the manifest slice, and diagnostic discipline.

**frontendtest**, through `RunFrontendSuite(t, f FrontendFixture)`.
A real frontend with the chain behind it: deterministic graphs, a
populated store, positioned diagnostics, classification stamps
present, owned outputs excluded, and fingerprint-keyed caching,
where a probe cache observes that the keys fold in the unit
fingerprint and the frontend version.

**acceptancetest** drives the consumer's binary as a process: exit
codes per [16-diagnostics.md](16-diagnostics.md), config discovery,
idempotence at the process level, and compiling the generated
output. It is the only check that builds what was generated.

**completeness** drives `testdata/features/` per
[11-languages.md](11-languages.md): every row sits exactly on the
level it declared, and a feature that lands better than declared
fails too.

Beside plugintest ships the **Tier-3 import lint**, a static pass
over a plugin module that refuses an import of any satellite `sdk/`
package from a neutral file. Binding files are declared and exempt,
which is Tier 3 staying legal and visible
([03-projection.md](03-projection.md)).

## The two checks that test the frame

Six checks test parts. workspacetest and warm≡cold test the frame
itself. They are specified as precisely as the adapter below,
because they are the hardest to retrofit: every workspace mechanism
they exercise had to be built so it could be tested.

**workspacetest** composes a real multi-plan workspace over fixture
sources and checks the frame's promises one at a time:

1. **collision**: routing two plans to one path is an Error at
   Close, naming both plans, and never last-writer-wins;
2. **isolation**: a plan seeded to fail leaves its siblings' staged
   output and manifest slices intact;
3. **exports**: a dependent plan reads the producer's export in
   topological order, and an unchanged export causes no dependent to
   run again;
4. **sweep**: removing a plan from the composition deletes exactly
   that plan's files, except drifted ones, and a narrowed run
   deletes nothing outside its scope
   ([08-workspace-and-plans.md](08-workspace-and-plans.md));
5. **audit**: a completeness contract left unmet by a deliberately
   absent annotator reports at the severity it declared;
6. **checks**: a `WorkspaceCheck` sees identical records cold and
   warm, including carried plans.

**warm≡cold** is three runs and two comparisons:

1. Run cold over the fixture, then snapshot the manifest and every
   byte.
2. Run warm with nothing changed. Assert that nothing re-executed,
   meaning the run stats report no rule ran, and that the manifest
   is byte-identical.
3. Change one declaration and run warm. Separately, run cold over
   the changed tree in a fresh state directory. The two runs'
   manifests and bytes must be identical. The warm path may skip
   work; it may never change output.

Both harnesses are kernel code, and the fixtures and compositions
are the caller's:

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
DSL and they drift apart by hand. So the kernel defines the
skeleton, meaning the `Generated` fixture lifecycle and the
assertion set (`AssertParses`, `AssertTypeChecks`, `AssertTestsPass`,
`AssertSatisfies`, `AssertDoesNotSatisfy` and the rest), over a
small adapter interface that each language's `testing/` implements:

```go
type ToolchainAdapter interface {
    Layout(fixture Fixture) (dir string, err error)  // scratch project the toolchain accepts
    Parse(dir string) error                          // syntax only
    TypeCheck(dir string) error
    RunTests(dir string) (TestReport, error)
    Satisfies(dir, typeName, contract string) (bool, error)
}
```

Language harnesses stay thin adapters. The assertion semantics and
the failure wording are written once, in the kernel, against this
interface.

An adapter may add language-specific assertions on top of the shared
set, such as an `AssertVets`-style check that only means something
under one toolchain. Those export from the satellite's `testing/`
under the same naming scheme. The kernel set is the floor rather
than the ceiling.

An assertion that depends on a toolchain skips locally with a
recorded reason, and is **required in CI**, so a regression cannot
hide behind a missing toolchain.

The fixture builders follow the same pattern: a neutral
symbol-fixture core in `conformance/`, with per-language spelling
adapters in each satellite.

## Fixtures

Test files mirror their subjects, and fixture sources live in
`testdata/` beside the test that drives them.

The feature matrix is `testdata/features/` plus one root conformance
test per satellite. It is fixtures and a check rather than a package.

Golden files are canonicalised: a position outside the fixture's own
sources keeps its file and loses its numbers. Positions in
dependency sources move between toolchain releases, and pinning them
makes goldens fail on an upgrade with a diff nobody can act on.

## Who runs what

- **Satellites** run the kernel suite at their declared minimum and
  maximum kernel versions, on every commit, plus their own
  feature-matrix and parity artifacts per release.
- **The kernel** runs the canary ring before tagging
  ([15-compatibility.md](15-compatibility.md)).
- **Consumers** run plugintest over their own plugins and
  acceptancetest over their binary, plus workspacetest when they
  compose multi-plan workspaces.
