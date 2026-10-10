<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Contributing to eidos

eidos is not released yet. Most work implements a mechanism the
[specification](docs/architecture/README.md) describes. If you want to change
one of those mechanisms, change the specification first.

## Setup

You need Go 1.27.2 or later, ergon and pre-commit. `go.work` requires that Go
version, and CI installs it from there.

Install ergon with Homebrew, or with Go from source:

```sh
brew install --cask dokimasia/tap/ergon
go install go.dokimi.dev/ergon/cmd/ergon@latest
```

The targets of the Makefile run their tools through `ergon tool run`. On its
first run, ergon installs each tool at the version that `.ergon.yaml` pins.
Install the git hooks once:

```sh
pre-commit install
```

The hooks run `make lint` and `make test` before each commit, `make check`
before each push, and commitlint on each commit message.

## Before you open a PR

```sh
make check
```

`make check` runs the gate of Go in every module of `go.work`. The gate runs
golangci-lint with its format check, the tests, the tests under the race
detector, and govulncheck. CI runs the same gate on Linux, macOS and Windows.
CI also checks the commit messages, the changesets, the license headers, the
Markdown files and the managed files.

While you work, run the narrower targets:

```sh
make fmt            # the formatters of .golangci.yml
make lint-go        # golangci-lint and its format check
make test           # go test, per module
make help           # every target
ergon license fix   # the SPDX header of every file
```

The Makefile runs golangci-lint once per module, from the directory of the
module. In every module, golangci-lint reads the one `.golangci.yml` at the
repository root.
gci groups the imports, with the modules of eidos in a group of their own
under `prefix(go.dokimi.dev/eidos)`.

## Managed files

ergon writes the Makefile, `.golangci.yml`, the workflows and every other
file whose first line starts with `Managed by ergon init`. Do not edit these
files. Put a setting of eidos into the file of the same path under
`.ergon/local/`, run `ergon init sync`, and commit both files. The Baseline
job of CI runs `ergon init check`, which fails when a managed file differs
from the file that ergon writes.

## Changesets

A pull request that changes a module adds a changeset to `.changeset/`. The
changeset contains each module that the change releases with its bump, and a
summary. The summary becomes the entry in the changelog of each module:

```sh
ergon release add --bump go.dokimi.dev/eidos/core=minor -m "Add the rename of a symbol."
```

A change that releases nothing, such as a change to the tests or the
documentation alone, adds a changeset without modules with
`ergon release add --empty`. The Changeset job of CI runs
`ergon release status`, which fails a pull request that changes a module
without a changeset for it.

## How the repository is laid out

Each component is its own Go module, tagged and released on its own. The
[README](README.md) lists them.

`go.work` holds the module list, and ergon reads it from there. The root
module `go.dokimi.dev/eidos` declares the import-path prefix and has no
packages. It stays out of `go.work`, because `go vet ./...` fails on a module
without Go files.

To add a module, follow [Add a module](docs/how-to/add-a-module.md).

## Commits

Write Conventional Commits. The commit-msg hook runs commitlint with
`.commitlint.yaml`, and rejects anything outside these two lists.

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `ci`, `chore`, `perf`,
`build`, `revert`.

Scopes: `cli`, `conformance`, `core`, `lang`, `go`, `java`, `kotlin`, `php`,
`protobuf`, `rust`, `typescript`, `shape`, `reference`, `sdk`. These are the
module names without their `eidos-`, `eidos-lang-` or `eidos-plugin-` prefix.
Leave the scope off when a change touches the whole repository.

Keep the subject to 72 characters and body lines to 100. Say what changed
and why; the diff shows how.

## Tests

Put test files beside what they test, and fixtures in `testdata/` beside the
test that reads them.

The kernel ships a conformance suite that tests plugins, satellites and
backends. It has eight checks, from `plugintest` through `warm≡cold`. Write
fixtures for those checks rather than your own assertions.
[13-testing-and-conformance.md](docs/architecture/13-testing-and-conformance.md)
says what each check proves.

## Documentation

Write a Go docblock for the person who is about to call the package. Say what
it is and what it does, in the present tense. Do not mention repository
files, because godoc renders where those paths mean nothing. Do not write
status, roadmap or history into a docblock. State the rules as facts.

The [specification](docs/architecture/README.md) is closed: every component,
contract and policy appears in exactly one document. To change a mechanism,
edit the one document that owns it.

The [decision log](docs/architecture/21-decisions.md) lists every settled
decision. Write a full [ADR](docs/adr/README.md) for a decision when the
event you said would make you reconsider it actually happens, or when the
reasoning no longer fits in one table row.

## Vocabulary

Write plain English, in code comments and documents alike. A word
stays when it names a mechanism a reader can look up (seam,
provenance, manifest, drift) or is the ordinary word for the thing;
it goes when it is imagery standing in for a plain verb. Functions
return; booleans report whether. Banned, with their replacements:
rung and ladder (check, step, level, scale), answers as a verb for
returns, fires, lands, rides, travels, seat, floor, bill, ritual,
story, journey, stranger, world, inhabited, first-class, pays,
priced, earns, heals, and blind to.

## Security

Do not open an issue for a vulnerability. Read [SECURITY](SECURITY.md).
