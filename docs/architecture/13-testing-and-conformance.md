# Testing and conformance

*Builds on: every mechanism before it. Feeds:
[15](15-compatibility.md), since this suite is the instrument every
compatibility claim is executed with.*

This is how a framework split across modules keeps its promises.
Everything here ships in the kernel's `conformance/` package, and
satellites and consumers run it too. Claims about compatibility,
determinism and language support are executed rather than asserted.

## The eight checks

Eight checks, each proving something the others cannot:

| Check | Proves |
|---|---|
| plugintest | one plugin's declarations, its determinism, that no fixture panics (the other half of D68), and its diagnostic discipline, while rendering nothing. It includes template lint: every declared template parses against the merged funcmap per language, and every body-claiming template places the slot marker |
| backendtest | one backend over a hand-built emit graph |
| pipelinetest | plugins plus a real backend producing rendered files, driving one plan |
| workspacetest | a full workspace: several plans, exports, dependencies and Close, covering collision detection, isolation, sweep, cross-plan checks and audit mode |
| frontendtest | a real frontend with the plugin chain behind it |
| acceptancetest | the consumer's binary end to end. The only check that compiles generated output |
| completeness | every corpus feature sits on the degradation level its language declared ([11-languages.md](11-languages.md)) |
| warm≡cold | the same workspace, run cold and warm, produces byte-identical files and manifests and the same findings ([09-incrementality.md](09-incrementality.md)) |

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
it checks these properties:

- Declaration stability. The name, the gate records, the outputs and
  the schemas the plugin registers come back identical across builds.
- Determinism. Two runs over one fixture store produce byte-equal emit
  under `-count=2`.
- Parallel dispatch. A run on eight workers produces the emit, the fact
  values and the findings of a run on one worker. Under the race
  detector, the run on eight workers also exposes state a handler
  writes outside its effects.
- Selective dispatch. A selection that lists every match the full
  call ran reproduces the full call's emit, facts and findings, and
  the journal lists every match once.
- Annotator idempotence.
- No structural write. The node count is unchanged.
- Positioned diagnostics. Every diagnostic has a position.
- Declared tags. Every emitted tag is a declared tag.
- Attribution. Every unit names the plugin that emitted it.
- Options schema stability. Unknown keys and missing required keys are
  rejected.
- Template lint. Every declared template parses against each
  language's merged funcmap, body templates place the slot marker, and
  no name collides with a reserved one.
- The lowering guarantee. A facade-authored plugin and its hand-rolled
  SPI twin produce byte-equal output, the contributors each unit
  records included. This is where 06b's promise is checked.

**backendtest**, through `RunBackendSuite(t, setup Setup)`. Over a
hand-built emit fixture it checks: that the fixture is populated,
because an empty store passes everything vacuously; byte-stable
render, two isolated runs compared as files and as a finding set;
that every emit kind in the fixture renders or reports the refusal
its backend declares; that every body arrives whole, with slot
contents spliced through the kind machinery;
and that a file's failure reports positioned and attributed while
the render continues per [07-rendering.md](07-rendering.md), the
refused file withheld. The header and trailer checks are the output
contract's and join the suite with it.

**pipelinetest**, through `RunPipelineSuite(t, f Fixture)`, where the
fixture is a source tree, the stores its load reads, a composition
over the directory a check runs in, and every file the run generates.
Plugins plus a real backend driving one plan, each check over
directories of its own: a clean run with every finding positioned,
end-to-end bytes at their routed paths with no other file under the
brand's frame, the manifest slice with each file's digest, a second
run, cold, that leaves every byte and every mtime unchanged, and a run
in a second directory that writes the same bytes and records the same
files. The second run is cold because a warm run over an unchanged
tree executes nothing, so it would not check that the phases produce
the same bytes again.

**frontendtest**, through `RunFrontendSuite(t, f FrontendFixture)`.
A real frontend with the chain behind it: deterministic graphs, a
populated store, positioned diagnostics, classification stamps
present, owned outputs excluded, and fingerprint-keyed caching,
where a probe cache observes that the keys fold in the digests of the
unit's inputs and the frontend version.

**acceptancetest** drives the consumer's binary as a process: exit
codes per [16-diagnostics.md](16-diagnostics.md), config discovery,
idempotence at the process level, and compiling the generated
output. It is the only check that builds what was generated.

**completeness** drives the conformance corpus per
[11-languages.md](11-languages.md): every feature sits exactly on
the level its language declared, evaluated through the projections,
and a feature that does better than declared fails too.

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
sources and checks the frame's promises one at a time, each check
over a directory of its own. The fixture supplies a working
composition without its plans, the plans and one edit. The suite
composes every failure it checks: a copy of a plan, a seeded failure,
a probe plan, a dependent plan that reads one export, a contract key,
a cycle, two checks and a recording check.

1. **generated**: the plans generate the wanted files, byte for
   byte, each recorded under the plan whose commit wrote it.
2. **collision**: a copy of the first plan under another name routes
   its files to the same paths. That is an Error at Close naming both
   plans, never last-writer-wins, and nothing commits.
3. **isolation**: a failure seeded into the last plan keeps that
   plan's files and entries, and every other plan commits.
4. **exports**: a probe plan that depends on every plan reads each
   one's export, and each export lists exactly the files, plugins
   and origins that the plan's record lists.
5. **cycles**: two plans that depend on each other are a Build error
   naming both.
6. **sweep**: removing the last plan from the composition deletes
   exactly that plan's files, except one edited since its stamp,
   which remains on disk under a KeptOutput warning
   ([08-workspace-and-plans.md](08-workspace-and-plans.md)).
7. **audit**: a completeness contract that no annotator meets
   reports at the severity it declared.
8. **checks**: a `WorkspaceCheck` that reads a failed plan does not
   run and reports one FailedDependency Info. One that reads a clean
   plan reads the plan's files as its commit records them, and its
   export.
9. **export cutoff**: the fixture's edit changes the first plan's
   output and none of its export rows. The suite adds a dependent plan
   whose one invocation reads the first plan's export. The warm run
   after the edit writes a file of the first plan again, the probe
   reads an unchanged export, and the dependent runs no invocation.
10. **warm checks**: a recording check that reads the first plan
    reads the same records on the warm run after the edit as on a
    cold run over the edited tree.

The sweep of a narrowed run, which deletes nothing outside its scope,
has no check in workspacetest, because the suite runs over the whole
tree.

**warm≡cold** runs the same fixture through five checks, each in a
parallel subtest over directories of its own:

1. **unchanged**: a cold run and a warm run, each after every file's
   modification time moves into the past, then a run with nothing
   changed. That run reads the sealed state, hashes no file, parses no
   unit, decodes no region, runs no invocation and calls no check, and
   every file under the root is unchanged, the manifest's documents
   and the state included. The first warm run is needed because a
   cold run records the files it created with a zero size.
2. **touched**: the two runs of the first check, then one source
   file's modification time set to the present with its bytes
   unchanged. The next run hashes that file once and parses nothing.
3. **edited**: a cold run, the edit and a warm run in one directory,
   and the edit and a cold run in a fresh copy. The two directories
   contain the same files outside the state directory, the two records
   list the same entries, a probe plan that depends on every plan
   reads the same exports, and the two runs report the same findings.
   The warm path may skip work. It may never change output.
4. **damaged**: a cold run, then every segment of the state cut to
   its first byte. The next run reports one `ColdState` Info, runs
   cold, and leaves the files that the first run left.
5. **restored**: a composition with a parse memo whose cap removes no
   entry runs cold, runs after the edit, and runs again after the
   fixture's tree is written back. The last run parses no unit and
   restores from the memo each unit that the run after the edit
   parsed.

Both harnesses are kernel code, and the fixtures and compositions
are the caller's:

```go
func RunWorkspaceSuite(t *testing.T, f workspacetest.Fixture)
func RunWarmColdSuite(t *testing.T, f workspacetest.Fixture)

type Fixture struct {
    Tree    fs.FS                                // the fixture tree
    Stores  map[string]fs.FS                     // the trees dependency units read
    Compose func(root string) *workspace.Builder // the composition without its plans
    Plans   func() []workspace.Plan              // two or more, fresh instances per call
    Want    map[string][]byte                    // every generated file, frame included
    Edit    func(root string) error              // changes one declaration the first plan
                                                 // generates from: its output changes,
                                                 // its export does not
}
```

The kernel's own tests cover what the suites cannot vary through a
fixture: early cutoff, the scoped membership edge, a package that
moves into a plan's scope or out of it, the package edge, lazy region
decoding, the size of an incremental commit, pending work after a
failed plan, the two re-link cases that parse a unit again, the
memo's eviction, its cold mode and its sharing, and damage at every
read of eight warm runs and at the commit's merge. The Go conformance
fixture covers the (symbol, key) grain across its two plans.

## The toolchain-adapter skeleton

Without a shared skeleton, every language grows its own assertion
DSL and they drift apart by hand. So the kernel defines the
skeleton, meaning the `Generated` fixture lifecycle and the
assertion set (`AssertParses`, `AssertTypeChecks`, `AssertTestsPass`,
`AssertSatisfies`, `AssertDoesNotSatisfy` and the rest), over a
small adapter interface that each language's `testing/` implements:

```go
type Adapter interface {
    Lang() symbol.Lang                               // for the assertions' wording
    Available() (bool, string)                       // and the reason where it is not
    Layout(g Generated) (dir string, err error)      // scratch project the toolchain accepts
    Parse(dir string) error                          // syntax only
    TypeCheck(dir string) error
    RunTests(dir string) (TestReport, error)
    Satisfies(dir, typeName, contract string) (bool, error)
}

type Generated struct {
    Files   map[string][]byte // rendered output, keyed by output path
    Sources fs.FS             // the tree the run read, nil where the output stands alone
    Module  string            // the identity the laid-out project declares
}
```

`Generated` is a value and the adapter lays it out; nothing in the
kernel writes a file, because where a scratch project sits and what
it must contain is the language's own question. `Prepare` runs the
layout and hands back the cleanup, so every assertion removes its
scratch tree whether it passed or not.

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
hide behind a missing toolchain. `Require` settles that case and
`RequiredInCI` decides it, off the `CI` variable every runner sets.
A satellite's own assertions call `Require` too, because the gate
belongs to whoever runs a toolchain rather than to the suite. The
assertions themselves never skip: a caller reaching for one has
already decided the toolchain is there.

Two assertions refuse rather than pass on an absence. A fixture
carrying no output fails, because a toolchain run over nothing
passes while proving nothing, and a test run reporting no case at
all fails for the same reason: generated tests that execute nothing
are the failure the assertion exists to catch.

Go's harness is the first: it parses with the standard library, so a
machine with no toolchain still holds generated output to Go's
grammar, and type-checks, tests and vets by running the go tool.
`AssertVets` is its language-specific addition, because a template
that assembles a call site correctly for one type assembles it
wrongly for the next, and that is exactly what vet catches.

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
