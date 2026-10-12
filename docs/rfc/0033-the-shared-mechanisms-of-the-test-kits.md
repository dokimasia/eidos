---
rfc: 0033
title: The shared mechanisms of the test kits
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: 0025
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0033: The shared mechanisms of the test kits

## Summary

The kernel has eight test kits, and the kits repeat mechanisms:

- Five kits define a positioned finding, each in its own way.
- Two kits age a run directory and compare it after a second run, each
  with an instant and a walk of its own.
- The pipeline kit and the workspace kit state one on-disk fixture in two
  types.
- Each fixture reads its expected files by hand, and no kernel suite can
  update them.
- Tests assemble the input of the toolchain kit from a run by hand, and
  Go output compiles in three ways.
- Four places file the classification stamps of a load.
- A plugin author who tests over real source rebuilds the load, the key
  registration and the directive validation, which the reference
  ensemble wrote for itself.

We propose one mechanism for each concern, in the kit of the concern, and
a migration of every caller:

- A finding has one predicate, `diag.Diag.Positioned`.
- The workspace kit has one rerun check, one fixture type, a reader and
  a matcher of golden files, and a bridge from a run to the toolchain kit.
- The kernel has one claim for a load stamp, `meta.LoadClaim`.
- The plugin kit loads a fixture from a source tree.
- The command-line module applies a brand's config file to a
  composition, so a test configures a workspace as the binary does.
- The backend kit and the rules kit return the findings that they now
  discard.

The kits keep their packages. Each kit tests one role, and its concerns
are already in files of their own.

## Motivation

### One rule for a positioned finding

Every finding has a position. A finding without one is a defect in
whatever reported it, and the plugin kit is where the kernel catches it.
Each kit checks the rule with a predicate of its own:

| Kit | The check |
|---|---|
| plugintest | the position is not zero |
| frontendtest | the position has a file, and the finding has an origin |
| backendtest | the position is not zero, and the origin is the suite's |
| pipelinetest | the position has a file |
| coretest | the position is not zero, and the finding has a message |

A position with a line and no file passes three of the five checks, and
no reader can open it. A finding about a declaration has its file, line
and column. A finding about a whole file, such as a file that a family
claims and another family shares, has its file alone. The rule that every
check can state is the file.

### One rerun check

A second run over an unchanged tree rewrites no file. The pipeline kit,
the workspace kit and the acceptance kit each check a form of it:

- pipelinetest ages every file to the start of 2001, runs again cold, and
  compares bytes and modification times outside the sealed state.
- acceptancetest ages every file to the Unix epoch, runs the binary again
  cold, and compares modification times outside the whole state
  directory.
- workspacetest ages every file an hour into the past before a warm run,
  for a different reason: a record of a file that changed within the
  resolution of the clock proves nothing.

The first two state one contract with two instants, two walks and two
rules for the state directory. The acceptance kit leaves the manifest out
of the comparison, and the pipeline kit compares it.

### One fixture type

The pipeline kit and the workspace kit both run a fixture tree through a
composition over a directory of the check. Both fixtures have a tree, the
stores of the load and the expected files. The pipeline fixture composes a
built workspace, and the workspace fixture composes a builder without
plans and states the plans apart. A composition written for one kit does
not serve the other, and the two compositions of the Go conformance
fixtures differ only in that return type.

### Expected files with an update path

The kernel suites compare a run with an in-memory map of expected files.
Each caller fills the map by reading files by hand: the Go workspace and
pipeline fixtures, the hub fixture, the protobuf service fixture and the
TypeScript pipeline fixture read their goldens one path at a time. No
suite can rewrite a golden after an intended change. The reference test
added its own path: it matches each file of the manifest through
`golden.MatchAt` with `-update`, and leaves the map empty under `-update`.

### From a run to the toolchain kit

The toolchain kit checks generated output through a language's compiler,
and takes the output as `toolchain.Generated`. Six cases in four test
files build that value from a run by hand: the TypeScript pipeline case,
the hub case, two protobuf service cases and two declarative composition
cases. Each reads files back from the run's directory, and the
declarative cases walk the whole directory and filter it by suffix. Go
output compiles in three ways: through the toolchain kit, through the Go
adapter as the compile of the reference binary's acceptance suite, and
through a raw `go build` in the Go acceptance fixture. A raw `go build` compiles no `_test.go` file, and the Go adapter
compiles every test file.

### Findings that the backend and rules kits discard

`backendtest.RenderSettled` returns the rendered files and fails on an
Error. It returns no finding, so a satellite test that pins a Warning of
its backend renders through its own copy of the plumbing.
`rulestest.Loaded` fails on an Error of the load and drops the other
findings. The lowering of a backend fixture to a render context is
unexported, so a test that renders a fixture with a context of its own
cannot reuse it.

### The claim of a load stamp

A load records classification stamps, and a run files each one as a fact
under one claim: plugin authority, the stamp's origin as the plugin, the
stamp's index among the stamps of its subject, and the stamp's position.
The workspace builds that claim in `stampSubject`. The frontend kit, the
rules kit and the reference fixture build it again, field by field.
frontendtest and rulestest also build the key registry of a load twice:
the kernel's keys, the frontend's keys under its language, and the
fixture's keys.

### A plugin fixture from source

A plugin author tests a plugin over a fixture. The fixture builder states
declarations as literals. A test over real source needs the load, the
rules of each language, the keys and schemas of the plugins, the
validation of every directive with a resolver, and the classification
stamps. The reference ensemble wrote every step in its internal package
`referencetest`, which imports the kernel's load because the SDK
re-exports no load. A plugin module may not import the kernel, so an
author outside the repository cannot write the same fixture.

### The brand's config file in a composition test

The command line decodes the brand's config file, such as
`.reference.yaml`, and applies it to the composition: the options of the
plugins, the refinement of each plan, the layout and the parse memo. The
reference test decodes the same file with a struct of its own that reads
the options alone, so the workspace suites test a composition that the
binary never runs.

## Detailed design

### Components

| Component | Package | Callers |
|---|---|---|
| `Diag.Positioned` | core/diag | the positioned checks of plugintest, frontendtest, backendtest, pipelinetest and coretest |
| `AssertRerunUnchanged` | core/workspace/workspacetest | pipelinetest.AssertIdempotent, acceptancetest.AssertIdempotent |
| `Fixture`, one shape | core/workspace/workspacetest | RunWorkspaceSuite, RunWarmColdSuite, RunPipelineSuite and their callers |
| `Want`, `AssertGolden` | core/workspace/workspacetest | the reference test and the conformance fixtures |
| `Generated` | core/workspace/workspacetest | the six cases that assemble toolchain input by hand |
| `Rendered`, `Fixture.Context` | core/backend/backendtest | RenderSettled and satellite tests |
| `Fixture.Findings` | core/rules/rulestest | rules tests |
| `LoadClaim` | core/meta | the workspace, frontendtest, rulestest, plugintest |
| `Registry` | core/frontend/frontendtest | frontendtest, rulestest |
| `Source`, `Loaded` | core/plugin/plugintest | referencetest, plugin authors |
| `Configure` | cli | the reference test |

### Invariants

1. One concept has one implementation, and every kit that checks the
   concept calls it.
2. No check that this proposal changes accepts what it refused before.
   Each unified check is at least as strict as every check that it
   replaces.
3. A kit that the SDK re-exports gains no import of a package that a
   plugin may not see. plugintest gains the kernel's load, which the SDK
   does not re-export, as an implementation dependency alone.
4. A run under a check's directory reads and writes nothing outside that
   directory.
5. Every kit runs its loads, its runs and its reads of a record under the
   context of the test, which ends with the test. pipelinetest,
   workspacetest, acceptancetest and the run directory helpers pass
   `context.Background()` in eight places.

### A positioned finding

```go
package diag

// Positioned reports whether the finding has a position that a reader
// can open: a file. A finding about a declaration also has the line and
// the column of the declaration. A finding about a whole file has the
// file alone.
func (d Diag) Positioned() bool
```

The five checks call it, and keep what each checks beside the position:
frontendtest its origin, backendtest the suite's origin, and coretest the
message. The plugin kit's check and the backend kit's check become
stricter, because a position with a line and no file now fails.

### The rerun check

```go
package workspacetest

// AssertRerunUnchanged sets the times of every file under root outside
// the sealed state of brand to one instant in the past, calls rerun, and
// then compares each file outside the sealed state with its state before
// the call. It fails on each file whose bytes or modification time rerun
// changed, and on each file that rerun added. The sealed state is the
// state directory of brand outside the documents of its manifest, which a
// run rewrites as it records what it read. The manifest is compared.
func AssertRerunUnchanged(tb assert.TB, root string, brand output.Brand, rerun func())
```

The instant is the start of 2001, so a rewrite moves the time on a file
system of any resolution. pipelinetest.AssertIdempotent passes a cold run
of the fixture as rerun. acceptancetest.AssertIdempotent passes a cold
run of the binary, and keeps its own check that the binary reports every
file unchanged. The acceptance check gains the manifest in its
comparison. The workspace kit's aging before a warm run is a separate
contract, and this proposal leaves it as it is.

### One fixture type

The pipeline kit takes the workspace kit's fixture, and the fixture's
composition returns its error:

```go
package workspacetest

// Fixture is an on-disk case: a source tree, the stores that its load
// reads, the composition without its plans, the plans, every file the
// plans generate, and an edit of the tree.
type Fixture struct {
    Tree    fs.FS
    Stores  map[string]fs.FS
    // Compose returns a builder over root with every part of the
    // composition except its plans, and the error of a part that does
    // not configure, such as a config file that does not decode.
    Compose func(root string) (*workspace.Builder, error)
    Plans   func() []workspace.Plan
    Want    map[string][]byte
    Edit    func(root string) error
}

package pipelinetest

// RunPipelineSuite checks the fixture's one plan. It reads Tree, Stores,
// Compose, Plans and Want, requires Plans to return one plan, and reads
// no Edit.
func RunPipelineSuite(t *testing.T, f workspacetest.Fixture)

func AssertClean(tb assert.TB, f workspacetest.Fixture, root string)
func AssertGenerated(tb assert.TB, f workspacetest.Fixture, root string)
func AssertRecorded(tb assert.TB, f workspacetest.Fixture, root string)
func AssertIdempotent(tb assert.TB, f workspacetest.Fixture, root string)
func AssertRelocated(tb assert.TB, f workspacetest.Fixture, one, two string)
```

`pipelinetest.Fixture` is deleted. The workspace kit stops a check where
Compose returns an error. The Go and TypeScript conformance packages split
`ComposeStubs` into a composition without plans and a function that
returns the one plan.

### Expected files

```go
package workspacetest

// Want returns every file under dir, a slash-separated directory relative
// to the test's directory, keyed by its slash-separated path relative to
// dir: the files that a fixture's plans generate. Where dir does not
// exist and the run updates goldens, it returns an empty map, because the
// update creates the directory. It stops the test where dir does not
// read.
func Want(tb assert.TB, dir string) map[string][]byte

// AssertGolden runs the fixture's plans in root, an empty directory, and
// matches each file that the record of the run lists against the file at
// its path under dir, through golden.MatchAt, which writes the file where
// the run updates goldens.
func AssertGolden(tb assert.TB, f Fixture, root, dir string)
```

A test fills the Want of its fixture from the directory, and runs
AssertGolden as its golden case. The suites keep comparing against the
map, because an update that rewrites the directory while parallel checks
read it races with them. Under `-update`, the other checks compare with
the goldens of before the update. A second run without `-update` checks
the new goldens. AssertGolden deletes no file. A golden that the run no
longer generates fails the generated check of each suite, and a person
deletes it.

### From a run to the toolchain kit

```go
package workspacetest

// Generated returns the files that the record of the run in root lists,
// with one of the extensions exts, read from root, as the output of a
// toolchain run with sources as its sources. Without exts it returns
// every listed file. It stops the test where root has no record of brand
// or a listed file does not read.
func Generated(tb assert.TB, root string, brand output.Brand, sources fs.FS, exts ...string) toolchain.Generated
```

The six cases that build the value by hand call it. The Go acceptance
fixture compiles through the Go adapter's type check, which compiles every
package and every test file, and its raw `go build` is deleted.

### Findings of the backend and rules kits

```go
package backendtest

// Context lowers the fixture to the context of one render call over
// sink, under the suite's identity, with the store routed into files
// through the backend's own filename speller.
func (f *Fixture) Context(b backend.Backend, sink *diag.Sink) *plugin.RenderContext

// Rendered settles one setup's fixture, renders it once, and returns the
// files and the findings of the settle and the render in report order.
// It stops the test where the settle or the render returns an error.
func Rendered(tb assert.TB, setup Setup) ([]plugin.RenderedFile, []diag.Diag)

package rulestest

type Fixture struct {
    // Findings are the findings of the load, in report order. The load
    // of a fixture reports no Error, so they are Warnings and Infos.
    Findings []diag.Diag
    // The existing fields follow.
}
```

RenderSettled calls Rendered and keeps its contract.

### The claim of a load stamp and the key registry of a load

```go
package meta

// LoadClaim returns the claim under which a run files the classification
// stamp s of subject, the i-th stamp of subject in load order: plugin
// authority, the origin of the stamp as its plugin, the order of its
// index and the position of the stamp.
func LoadClaim(subject symbol.Identity, i int, s RawStamp) Claim

package frontendtest

// Registry returns the key registry that a workspace builds for the
// frontend f and the kernel's key handles: the kernel's keys, then the
// keys that f registers through its role under its language, then keys,
// each under the handle of the composition. It stops the test where a
// registration fails.
func Registry(tb assert.TB, f plugin.Frontend, keys ...func(*meta.Registry) error) (*meta.Registry, meta.KernelKeys)
```

The workspace, frontendtest, rulestest and plugintest file stamps through
LoadClaim. rulestest builds its registry through Registry.

### A plugin fixture from source

```go
package plugintest

// Source is a source tree that [Loaded] loads into a fixture.
type Source struct {
    // Tree is the workspace tree, and Stores are the trees that its
    // dependency units read, by name.
    Tree   fs.FS
    Stores map[string]fs.FS
    // Brand is the brand whose carriers the tree writes.
    Brand output.Brand
    // Frontends load the tree, and Rules are the rules of their
    // languages, which validation and the phase calls bind.
    Frontends []plugin.Frontend
    Rules     []rules.SourceRules
    // Plugins are the plugins whose keys the fixture registers and whose
    // directives it validates.
    Plugins []plugin.Plugin
    // Packages are the paths of the packages that the fixture contains,
    // and nil contains every package of the load.
    Packages []string
}

// Loaded loads the source into a fixture, as a workspace prepares its
// first phase call. It registers the kernel's keys, the keys of each
// frontend under its language and the keys of each plugin under its name,
// and seals the registry, as a workspace seals it before the first phase
// call. It registers the kernel's schemas and the schemas of each plugin.
// It files the load's classification stamps under LoadClaim, and
// validates the directives of every declaration of the packages, binding
// each reference through the rules of the subject's language in the scope
// of its file. It stops the test where the load returns an error, where a
// registration, a stamp or a schema fails, where the load or the
// validation reports an Error, and where the load declares no package
// that Packages names.
func Loaded(tb assert.TB, s Source) *Fixture
```

The registry is sealed, so a test that needs the keys of another plugin
lists the plugin in Plugins instead of calling [Key] afterwards.

referencetest keeps its Spec, its annotators and its hosts, and builds
its fixture through Loaded. Its import of the kernel's load is deleted.

### The brand's config file

```go
package cli

// Configure applies the config file of the brand of b in root, where root
// has one, to b, as [Open] configures a workspace: the options, the
// refinement of each plan, the layout and the parse memo. It returns an
// error for a file that does not read or decode, and for a file that
// lists workspaces.
func Configure(b *workspace.Builder, root string) error
```

The reference fixture's Compose calls Configure on its base composition,
so the workspace suites run the composition that the binary runs over the
tree.

### Failures

| Case | Behaviour |
|---|---|
| A finding with a line and no file | the plugin kit and the backend kit fail it, where they passed it |
| A second cold run that rewrites the manifest | the acceptance kit fails it, where it passed it |
| A fixture whose Compose returns an error | the check stops with the error |
| A pipeline fixture with two plans | RunPipelineSuite stops, as it does today |
| A golden directory missing outside `-update` | Want stops the test |
| A run without a record | Generated stops the test |
| A source tree with an Error | Loaded stops the test |
| A config file that lists workspaces | Configure returns an error |

### Migration

1. The five positioned checks call `Positioned`.
2. The two idempotence checks call `AssertRerunUnchanged`, and their
   instants and walks are deleted.
3. `pipelinetest.Fixture` is deleted. The Go and TypeScript conformance
   pipelines split `ComposeStubs`. Every workspace fixture's Compose
   returns an error.
4. The conformance fixtures and the reference fixture read their goldens
   through `Want`. The reference test's golden case calls `AssertGolden`.
5. The six hand assemblies call `Generated`. The Go acceptance fixture
   compiles through the Go adapter.
6. `runRender` becomes `Rendered`, and `context` becomes `Context`.
7. The four stamp sites call `LoadClaim`. rulestest calls `Registry`.
8. referencetest builds through `plugintest.Loaded`.
9. The reference fixture calls `cli.Configure`.
10. The eight calls of `context.Background()` in the kits take the
    context of the test.

## Alternatives

- **A package per concern.** backendtest contains a suite, the canonical
  fixture, a benchmark harness, a file router and the output-contract
  check. frontendtest contains a suite and the scripted language, which 65
  files import. A package for each concern would change no contract, and
  would move one role's tests across several imports. Each concern is a
  file of its own already. We keep the packages.
- **The golden directory as a field of the fixture.** The suites would
  read and write the expected files themselves. Under `-update` one
  parallel check would write the directory while the others read it. We
  keep the map, and add a reader and a matcher.
- **`golden.MatchTree` for the expected files.** It compares a whole tree
  and removes stale files, and it requires the goldens under
  `testdata/golden/<name>`. The fixtures keep their goldens beside their
  trees, and moving them deletes files that the repository records. We
  match each file through `golden.MatchAt`.
- **One positioned rule of file, line and column.** The diagnostics
  document states the position of a declaration. The kernel reports some
  findings about a whole file, such as a file that two families share,
  and those have no line. The predicate requires the file.
- **A pipeline suite inside the workspace kit.** One package would hold
  both suites. The design documents name pipelinetest as one of the eight
  checks, and its one-plan contract differs from the workspace frame's.
  We keep the package and share the fixture.
- **The config file decoded in the workspace kit.** The kit would decode
  the brand's file itself. The command-line module defines the file's
  format and its decoder, and a second decoder drifts from it. The
  command-line module exports the application.

## Drawbacks

- Callers change. The fixture shape changes every workspace and pipeline
  fixture: two pipeline fixtures, four workspace fixtures and two
  compositions split into a builder and a plan.
- Two checks become stricter. A plugin or a backend that reports a
  finding with a line and no file now fails its suite, and a binary whose
  second cold run rewrites its manifest fails the acceptance suite.
- plugintest grows. Loaded adds about 120 lines and an import of the
  kernel's load to the kit that the SDK re-exports.
- The command-line module gains one exported function, which ties the
  reference test to the module's config format.

## Open questions

None.

## Unresolved and future work

- This proposal adds no check that a generated declaration has
  documentation, and no golden of a generated surface. Both need the
  toolchain adapters.

## References

- [RFC-0007: The Build steps and the fixture run](0007-build-steps.md)
- [RFC-0012: The SDK facade module](0012-the-sdk-contract-module.md)
- [RFC-0032: The fixture builder](0032-the-fixture-builder.md)
- [Testing and conformance](../architecture/13-testing-and-conformance.md)
- [Diagnostics](../architecture/16-diagnostics.md)
- [go.dokimi.dev/assert golden](https://github.com/dokimasia/assert-go/tree/main/golden)
