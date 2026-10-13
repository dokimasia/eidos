---
rfc: 0065
title: The eidos binary
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-13
updated: 2026-10-13
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0050, ADR-0051, ADR-0052
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0065: The eidos binary

## Summary

eidos does not publish a binary. A team that writes declarative plugins composes a binary in Go
before its first run. We propose the binary `eidos`, the main package `cmd/eidos` of the root module
`go.dokimi.dev/eidos`. It composes the frontends of Go, TypeScript, Java, Rust and protobuf, and one
extensible plan for each of the targets of Go, TypeScript, Java and Rust. It has no generator of its
own. A workspace adds declarative plugins to the plans in `.eidos.yaml`.

Build accepts an extensible plan without a generator, and such a plan does not write a file. A run
calls the store locator of a frontend only when the tree has an input file of the frontend. The root
module has no replace directive, so `go install go.dokimi.dev/eidos/cmd/eidos@latest` builds the
binary from published modules with a C compiler for tree-sitter. Each release of the root module
also publishes archives for darwin, Linux and Windows and a Homebrew cask. A job of the release
workflow builds them with cgo, as the release of techne does.

## Motivation

- A declarative plugin runs in a binary that composes frontends, backends and an extensible plan.
  The fixture of the declarative host in `eidos-conformance` is such a composition in 60 lines of
  Go. A team without a binary of its own writes, tests and releases that composition before it can
  run one plugin.
- The rule that eidos does not publish a binary assumed that the binaries of consumers are the
  products. A binary with typed plugins remains the product of its consumer. The binary `eidos` has
  no plugin. Every file that it writes comes from a plugin of its user.
- The current code blocks a binary that composes every language:
  - Build refuses a plan without a generator. A binary with an extensible plan for each target fails
    in every workspace that extends fewer than four plans.
  - The command kernels call the store locator of every frontend before each run. The Java locator
    returns an error without `JAVA_HOME`. The Go locator returns an error without `GOROOT` or a `go`
    binary on `PATH`. A binary that composes the Java frontend fails for every user without a JDK,
    also over a tree without a Java file.
  - `go install` refuses a module whose `go.mod` has a replace directive. The `go.mod` of
    `eidos-reference` has seven. `go install go.dokimi.dev/eidos/reference/cmd/eidos-reference@v0.1.0`
    fails with "The go.mod file for the module providing named packages contains one or more replace
    directives."

## Detailed design

### Components

| Component | Change |
|---|---|
| Root module `go.dokimi.dev/eidos` | Gains the main package `cmd/eidos`. Its `go.mod` requires `cli`, `core` and the satellites of Go, TypeScript, Java, Rust and protobuf. It has no replace directive. `go.work` uses it as `.` |
| `cmd/eidos` | `main.go` composes the binary. `main_test.go` runs it through go-cli's `clitest` over `testdata/tree` and `testdata/bare` |
| `.goreleaser.yml` and `.ergon/local/.github/workflows/release.yml` | The job `binaries` of `release.yml` builds the archives and the cask of each release of the root module with cgo |
| `eidos-core/workspace` | The plan step accepts an extensible plan without a generator. `Input` gains `Getenv`. `Workspace.Stores` remains for a caller that needs every store before a run |
| `eidos-core/frontend/load` | `Config` gains `Getenv`. The load calls the locator of each frontend with an input file |
| `eidos-cli` | The run of a workspace passes `Getenv` in place of the stores. `watch` renders the error of a locator in its pass and runs the next pass |
| `eidos-conformance/declarative` | The docblock of `Compose` drops the refusal of a plan without a generator |
| Documents | The README, the architecture README, the distribution, workspace and CLI documents and the decision log |

### Invariants

1. The binary composes neither a generator nor an annotator. Each file that it writes comes from a
   declarative plugin that the config file adds to a plan.
2. The binary has one plan for each of its four targets, under the name of the target. Each plan
   reads the packages of its language below the workspace root. The config file can replace the
   sources of a plan.
3. An extensible plan without a generator does not write a file. A run of such a plan removes the
   files that the plan wrote in its last generation.
4. A run calls the locator of a frontend only when the tree has an input file of the frontend. A run
   over a tree with an input file of every frontend locates the stores of every frontend.
5. The `go.mod` of the root module has no replace directive. `go install` of a released version
   builds the binary from published modules.

### The binary

`cmd/eidos/doc.go` contains the comment of the command. `cmd/eidos/main.go` contains the composition
and `main`:

```go
// brand is the brand of the binary. A carrier is +eidos:<directive>, the
// config file is .eidos.yaml, and the state directory is .eidos/.
const brand output.Brand = "eidos"

// help is the help text of the app.
const help = "Generates code with the declarative plugins of a workspace."

// treeSources is the pattern of every package below the workspace root. A
// package that a store provides is in no directory of the tree, so the
// pattern does not admit it.
const treeSources = "./..."

// main runs the app of the binary and exits with the status of the run.
func main() {
    cli.Main(cli.New(help, compose), compose)
}

// compose returns the composition of the binary: the frontends of Go,
// TypeScript, Java, Rust and protobuf, the rules of Go, TypeScript and
// protobuf, and one extensible plan without generators for each target.
func compose() *workspace.Builder {
    return workspace.New().
        Brand(brand).
        Frontends(gofrontend.New(nil), tsfrontend.New(), javafrontend.New(nil), rustfrontend.New(nil),
            protofrontend.New()).
        Rules(gorules.New(), tsrules.New(), protorules.New()).
        Targets(golang.Target, typescript.Target, java.Target, rust.Target).
        Plans(
            workspace.Plan{
                Name:       string(golang.Target),
                Sources:    workspace.Sources{Lang: golang.Lang, Packages: []string{treeSources}},
                Backend:    gobackend.New(),
                Extensible: true,
            },
            workspace.Plan{
                Name:       string(typescript.Target),
                Sources:    workspace.Sources{Lang: typescript.Lang, Packages: []string{treeSources}},
                Backend:    tsbackend.New(),
                Extensible: true,
            },
            workspace.Plan{
                Name:       string(java.Target),
                Sources:    workspace.Sources{Lang: java.Lang, Packages: []string{treeSources}},
                Backend:    javabackend.New(),
                Extensible: true,
            },
            workspace.Plan{
                Name:       string(rust.Target),
                Sources:    workspace.Sources{Lang: rust.Lang, Packages: []string{treeSources}},
                Backend:    rustbackend.New(),
                Extensible: true,
            },
        ).
        Parallel(runtime.GOMAXPROCS(0))
}
```

The brand `eidos` names the config file `.eidos.yaml` and the state directory `.eidos/`. go-cli
names the variable of `--strict` `EIDOS_STRICT` and the variable of `run --plan` `EIDOS_RUN_PLAN`. A
source file sets the directive of a plugin as `+eidos:audit`. A workspace that generates Go and a
TypeScript mirror of the package `ops` writes this config file:

```yaml
version: 1
plans:
    golang:
        plugins: [plugins/audit]
    typescript:
        sources: {lang: golang, packages: [./ops]}
        plugins: [plugins/audit]
options:
    audit: {redact: false}
```

The plans `java` and `rust` of that workspace have no generator. They do not write a file. `plan`
lists them with their backends alone.

### The composition

| Part | In the binary |
|---|---|
| Frontends | Go, TypeScript, Java, Rust and protobuf |
| Rules | Go, TypeScript and protobuf. Java and Rust have no rules package. A run binds the absent rules for their subjects and reports a warning |
| Targets and backends | Go, TypeScript, Java and Rust. The protobuf satellite has no backend. A plan reads proto sources when the config file sets its sources to `{lang: protobuf}` |
| Plans | `golang`, `typescript`, `java` and `rust`, each extensible and without a generator |
| Annotators | none |
| Dispatch | `runtime.GOMAXPROCS(0)` workers. The key `workers` of the config file replaces the count |

### An extensible plan without a generator

The plan step reports `workspace: plan %q lists no generator` only for a plan that the composition
does not mark extensible. The docblock of `Plan.Extensible` states the rule:

```go
// When Extensible is true, Build appends the generators of the plan's
// [PlanConfig.Generators] to Generators, so the users of a binary can add
// plugins to the plan through the config. An extensible plan can have no
// generator after the config applies. Its run then does not render a file.
// It stages the removal of each file of its last generation. Build returns
// an error for a plan without a generator that is not extensible.
Extensible bool
```

The run of such a plan does not run a generator. The settle and the layout see an empty store. The
render does not write a file. The staging removes the files of the last generation of the plan. A
plan whose generators do not emit a declaration runs the same way.

### The stores of the frontends with input files

The load locates the stores. A run passes the environment to the load in place of the stores of
every frontend:

```go
type Input struct {
    // Getenv reads the environment for the store locators of the
    // frontends. The load of the run calls the locator of a frontend only
    // when the tree has an input file of the frontend. It adds the stores
    // that the locator returns to Stores. A nil Getenv turns the location
    // of stores off.
    Getenv func(key string) string
    // The other fields of Input do not change.
}
```

```go
type Config struct {
    // Getenv reads the environment for the store locators. After the
    // selection, the load calls in composition order the Stores method of
    // each frontend that implements [plugin.StoreLocator] and has an input
    // file. It adds the stores that each locator returns to Stores. An
    // input file is a file that the selection of the frontend claims and
    // that is not an output of the brand. The load does not locate the
    // stores of a frontend without an input file. A nil Getenv turns the
    // location of stores off.
    Getenv func(key string) string
    // The other fields of Config do not change.
}
```

Location follows the selection and precedes the partition, because every door of the load can read
a store. Its errors:

- The error of a locator behind the name of its frontend. The Java locator without `JAVA_HOME`
  returns `load: frontend java: frontend: set JAVA_HOME, the JDK whose lib/ct.sym the Java frontend
  reads`. The run returns the error with its report and does not run again cold. The error does not
  wrap `state.ErrDamaged`.
- An error for a store name that two locators return, or that a locator returns and `Config.Stores`
  contains. The error contains both sources.

The run of a workspace in `run`, `prune`, `watch` and `doctor` sets `Input.Getenv` to the variables of
the run. A locator error in `run`, `prune` and `doctor` exits 1 with an `error` event. `watch` renders
the error in its pass and runs the next pass, because the error depends on the files of the tree.
`Workspace.Stores` remains for a caller that needs every store before a run, such as the suite of
`eidos-reference`.

### Distribution

`go install go.dokimi.dev/eidos/cmd/eidos@latest` installs the binary. The vanity host serves the
root module at `go.dokimi.dev/eidos`. The release flow tags the root module as `vX.Y.Z`. Like every
module of `go.work`, the root module gets the released versions of the other modules in its
requires. `version` prints the path `go.dokimi.dev/eidos` and the version of the build.

The frontends of TypeScript, Java and Rust parse with tree-sitter through cgo. The go command
enables cgo for a native build when it finds a C compiler on `PATH`. Without one, the build fails in
the bindings of the grammars. `CGO_ENABLED=0 go build ./frontend` in `eidos-lang-typescript` fails
with `build constraints exclude all Go files in .../tree-sitter-typescript@v0.23.2/bindings/go`.

ergon builds the archives of the commands under `binaries` of `.ergon.yaml` with cgo off, so
`.ergon.yaml` of eidos keeps `binaries: []`. The release of the binary follows techne, whose grammars
of tree-sitter need cgo too:

- `.ergon/local/.github/workflows/release.yml` adds the job `binaries`, which `ergon init sync`
  merges into `release.yml`. The job runs after the job `publish` has released the root module. It
  runs on a macOS runner with zig 0.16.0 and UPX.
- `.goreleaser.yml` builds `cmd/eidos` with cgo and `-trimpath`. darwin builds with the clang of the
  runner, and Linux and Windows build with `zig cc`: static musl binaries for Linux and mingw
  binaries for Windows, for amd64 and arm64. UPX packs the Linux binaries and the Windows amd64
  binary. Go writes the version from the tag of the root module, so no build passes an `-X` flag.
- GoReleaser attaches the archives with `LICENSE`, `NOTICE` and the README, a source archive, the
  SBOMs of syft and the checksums that cosign signs. actions/attest attests the archives and the
  binaries. The cask `eidos` goes to dokimasia/homebrew-tap through the GitHub App of ergon.

A snapshot build of the target linux/amd64 through this configuration took 35 seconds. UPX packed
the binary from 30,476,040 to 6,689,512 bytes. The packed static binary generated the files of the
fixture.

### Tests

`cmd/eidos/main_test.go` runs the app of the binary in process through `clitest.Transcript`. Its tree
in `testdata/tree` is a copy of the declarative fixture under the brand `eidos`. The config file of
`testdata/bare` does not extend a plan. The variables `GOROOT` and `GOMODCACHE` of the runs point at
the directory `testdata`. The sources do not import a package, so a run does not read a store. The
transcript is a golden file and contains these runs:

- `eidos --help` lists the eight kernel commands.
- `eidos plan` lists the four plans, with the plugin of the config file in two of them.
- `eidos run --stdout` writes the Go files and the TypeScript file of the fixture. A run with
  `--stdout` does not open a ledger, so it does not write below `testdata`.
- `eidos run --stdout` over `testdata/bare` does not write a file and exits 0.

`TestMain` runs `main` when the environment sets the variable of the child. One case runs the test
binary again as `eidos version` to cover `main`. The kernel changes have their own cases:

- The plan step accepts an extensible plan without a generator. It refuses the same plan without
  `Extensible`.
- A run of an extensible plan without a generator does not write a file. It removes the files of a
  plan whose config file dropped its last plugin.
- The load calls the locator of a frontend with an input file. It does not call the locator of a
  frontend without one, nor of a frontend whose claimed files are outputs of the brand. It returns
  the error of a locator. It refuses a store name that two sources provide.
- A run returns the error of a locator and does not report `ColdState` for it.
- `watch` renders the error of a locator once over its passes. Its harness counts the passes in the
  locator, so the case of a locked state runs a list whose second member loads.

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| The binary `eidos` | in | The declarative host runs plugins as files. A binary with every language serves every user of the host | none |
| The main package in the root module | in | The root module has no replace directive. ergon and techne keep their commands in the root module the same way | none |
| One extensible plan for each target, under the name of the target | in | A config file refines the plans of a binary and does not assemble a plan. A plan per target lets a workspace add a plugin for each output language | A workspace that needs two plans of one target in the stock binary |
| Plans that the config file declares | out | The config file refines what the binary composed and assembles nothing new | The same workspace |
| The annotators of the satellites and the shape catalog | out | A declarative rule or template does not read a fact | A key of the manifest that makes a rule depend on a fact |
| The reference plugins | out | They are the canary of the ring and examples of typed plugins. A workspace that wants them runs `eidos-reference` | none |
| Kotlin and PHP | out | Their satellites have no code | A frontend or a backend in either satellite |
| Java and Rust under the absent rules | in | Their frontends and backends exist. A declarative rule that lists members reports `DECLARATIVE-0001` where the absent rules cannot list them | A rules package in either satellite |
| Parallel dispatch on `GOMAXPROCS` | in | The output does not depend on the count. The key `workers` replaces it | none |
| An extensible plan without a generator | in | A workspace extends the plans of the targets that it generates and leaves the others empty | none |
| A store locator called only for a frontend with an input file | in | A binary with every frontend otherwise fails without the toolchain of a language that the tree does not have | none |
| `Workspace.Stores` for a caller that needs every store before a run | in | The suite of `eidos-reference` passes the stores of its composition as `Input.Stores` | none |
| `go install` with a C compiler | in | The root module has no replace directive. The go command enables cgo when it finds a compiler | none |
| Archives of the binary for each platform | in | The job `binaries` builds every target with cgo on macOS, as the release of techne does | none |
| A Homebrew cask | in | techne and ergon publish their casks to dokimasia/homebrew-tap | none |
| `binaries` of `.ergon.yaml` | out | ergon builds those commands with cgo off | ergon building a command with cgo |
| A finding for a plan without a generator | out | `plan` lists the generators of each plan. A workspace that generates one language extends one plan | A user who misses the output of a plan without a finding |

### Failures

| Condition | Report |
|---|---|
| A tree with a Java file and no `JAVA_HOME` | The error of the Java locator, status 1 |
| A tree with a Go file, and neither `GOROOT` nor a `go` binary on `PATH` | The error of the Go locator, status 1 |
| A store name that two locators return, or that a locator and `Input.Stores` both provide | An error that contains both, status 1 |
| A locator error in a pass of `watch` | The error of the pass. `watch` runs the next pass |
| A plugin directory that does not load | A config error, status 64 |
| A plugin without the template tree of the target of its plan | The error of Build, status 64 |
| `go install` without a C compiler | The build error of the go command in the bindings of the grammars |
| A release whose job `binaries` fails | The packages and the release of the tag exist without archives. A run of the job again attaches them, because the release keeps its existing notes |

### Migration

1. Let the plan step accept an extensible plan without a generator. State the rule on
   `Plan.Extensible`.
2. Add `Config.Getenv` to the load and `Input.Getenv` to the run.
3. Let the command kernels pass `Getenv`, and let `watch` run the next pass after a locator error.
4. Add `cmd/eidos` and its requires to the root module. Add the root module to `go.work`.
5. Describe the binary in the README, the architecture README, the distribution, workspace and CLI
   documents and the decision log.
6. Add `.goreleaser.yml` and the job `binaries`, and merge the job into `release.yml` with
   `ergon init sync`.

## Alternatives

### The binary in eidos-cli

`eidos-cli` could contain `cmd/eidos`. Its `go.mod` would then require every satellite. Every
consumer of the command kernels would then require the grammars of tree-sitter in its module graph.

### The binary in eidos-reference

`eidos-reference` already contains a binary. Its plugins are the canary of the ring. Its `go.mod`
has seven replace directives, and `go install` refuses such a module.

### A module of its own

A module such as `go.dokimi.dev/eidos/bin` could contain the command. Its import path would read
`go.dokimi.dev/eidos/bin/cmd/eidos`. The root module already has the path `go.dokimi.dev/eidos`.
ergon and techne keep their commands in the root module.

### Plans that the config file declares

A config file could declare a plan with a name, a target and sources. A workspace could then run two
plans of one target over different sources. The CLI document refuses a config file that assembles a
composition. A plugin whose rules require a directive already runs only for the subjects that set the
directive.

### A generator that emits nothing in each plan

Each plan could list a generator without rules, so Build would accept it. The generator would appear
in `plan` and in the fingerprint of every run. It would generate nothing.

### Plans that the command kernels disable

The command kernels could disable each extensible plan that the config file does not extend. A
disabled plan does not appear in `plan` as a plan that runs. The key `enabled` would then have two
sources. The plan step is the place where the composition and the config file meet.

### Locators that return an empty map without a toolchain

The Java and Go locators could return an empty map where the environment has no toolchain. The load
would then read every import of the standard library as a need without a place and report it as a
warning. A Java user without `JAVA_HOME` would lose the error that contains the name of the variable.

### Eager location with errors for the frontends with input files

The command kernels could locate every store before the run and pass the errors to the run. The run
would report the error of a frontend with an input file. A run over a tree without a Java file would
still read the `ct.sym` of the JDK into memory.

### A binary without the Java and Rust frontends

The binary could leave out the frontends whose locators need a toolchain or whose languages have no
rules. The binary composes every frontend and backend that eidos has. The change of the load serves
every consumer binary that composes the Java frontend as well.

## Drawbacks

- The root module joins `go.work`. ergon's targets then run in 17 modules instead of 16. The tests
  of the root module build the grammars of tree-sitter with cgo.
- The `go.mod` of the root module requires seven modules of the repository: `cli`, `core`, and the
  satellites of Go, TypeScript, Java, Rust and protobuf.
- `go install` needs a C compiler on every platform.
- Each release of the root module runs a job of up to 60 minutes on a macOS runner. The repository
  maintains `.goreleaser.yml` and the job `binaries` beside the baseline of ergon.
- A run over Java or Rust binds the absent rules. A declarative rule that lists the members of a
  subject of those languages reports `DECLARATIVE-0001`.
- A run over a tree with a Go file needs a Go toolchain, also when no plan reads Go.
- The load returns a locator error after the walk of the tree. `watch` no longer exits with the
  error of a locator.
- A caller passes stores to a run through `Input.Stores` with `Workspace.Stores`, or through
  `Input.Getenv`. With both set, the run returns an error for a store name that both provide.
- The brand `eidos` makes `.eidos.yaml` and `.eidos/` the files of the binary in a repository that
  runs it.
- A config file that does not extend a plan runs without output and without a finding.

## Open questions

None.

## Unresolved and future work

- Rules packages for Java and Rust are not proposed here.

## References

- [14](../architecture/14-distribution-and-cli.md), the distribution of eidos and the binary eidos.
- [20](../architecture/20-cli.md), the command kernels and the config file.
- [08](../architecture/08-workspace-and-plans.md), the plans and their refinements.
- [21-decisions.md](../architecture/21-decisions.md), the decision log.
- [RFC-0015](0015-tree-sitter-frontends-and-dependency-stores.md), the tree-sitter frontends and the
  dependency stores.
- [RFC-0021](0021-the-command-kernels.md), the store locators and `Workspace.Stores`.
- [RFC-0064](0064-the-command-line-on-go-cli.md), the command line on go-cli.
- `go help install`, the rule for a module with replace directives.
- `go doc cmd/cgo`, the default of cgo for a native build.
- ergon RFC-0007, binary releases and cgo:
  <https://github.com/dokimasia/ergon/blob/main/docs/rfc/0007-binary-releases.md>
- The release of techne with cgo: <https://github.com/dokimasia/techne/blob/main/.goreleaser.yml> and
  <https://github.com/dokimasia/techne/blob/main/.github/workflows/release.yml>
- go-cli RFC-0002, the package `clitest`:
  <https://github.com/stealth-scale/go-cli/blob/main/docs/rfc/0002-a-clitest-package.md>
