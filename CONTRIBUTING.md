# Contributing to eidos

eidos is pre-release and specification-first: the architecture in
[docs/architecture](docs/architecture/README.md) is complete, and the modules
are stubs. Contributions that implement a specified mechanism are welcome;
contributions that change one need the specification changed first.

## Setup

```sh
make bootstrap    # install gofumpt, gci, golangci-lint, govulncheck, go-license, ...
make install      # download and verify module dependencies
pre-commit install --hook-type pre-commit --hook-type commit-msg
```

## The gate

```sh
make check        # what CI runs: mod verify + lint + test
```

`make check` is also the pre-commit hook, so a commit that would fail CI fails
locally first. Narrow it while iterating:

```sh
make fmt          # SPDX headers, gofumpt + gci, markdownlint — run before check
make lint-go      # golangci-lint only
make test         # go test per module
make build        # compile every module
make help         # every target
```

Lint runs per module, from each module directory, against the one
`.golangci.yml` at the repository root. Formatting is not negotiable: `gci`
groups imports with `prefix(go.dokimi.dev/eidos)`, and `gofmt`/`goimports` are
deliberately disabled because they fight `gofumpt` and `gci`.

## Repository layout

One module per component, each tagged and released independently. `go.work`
is the single source of truth for the module list — ergon discovers modules
from it, and `.ergon.yaml` keeps `modules: []` for that reason.

The root module `go.dokimi.dev/eidos` anchors the import-path prefix and pins
the toolchain for CI. It holds no packages and is deliberately **not** a
workspace member: `go vet ./...` exits 1 on a module with no Go files.

### Adding a module

1. `mkdir eidos-<name>` and write `go.mod` with module path
   `go.dokimi.dev/eidos/<name>` — the directory keeps the `eidos-` prefix,
   the import path drops it.
2. Add a `use` line to `go.work`.
3. Add the coverage layer and the commit scope to `.ergon.yaml`.
4. Add the row to the [README](README.md) module table.
5. Add the row to
   [01-repos-and-kernel.md](docs/architecture/01-repos-and-kernel.md).
6. Write `doc.go` — see the documentation rules below.

Dependencies point one way: consumers → satellites → kernel. A satellite
never imports a satellite; cross-language needs go through the kernel's
canonical-type hub. The kernel carries zero third-party dependencies, as a
hard property.

## Commits

Conventional Commits, validated by `ergon check commit-msg` from the
commit-msg hook. Both sets are closed — an unlisted type or scope is an
error naming the candidates.

**Types:** `feat`, `fix`, `docs`, `refactor`, `test`, `ci`, `chore`, `perf`,
`build`, `deps`, `revert`

**Scopes:** `core`, `lang`, `go`, `java`, `kotlin`, `php`, `protobuf`,
`rust`, `typescript`, `shape`, `reference` — the module names with the
`eidos-lang-` and `eidos-plugin-` prefixes dropped. Scope-less commits are
legal for repository-wide changes.

Subject lines cap at 80 bytes, body and footer lines at 100. Write what
changed and why; the diff already shows how.

## Tests and coverage

Test files mirror their subjects, and fixture sources live in `testdata/`
beside the test that drives them. `make test` runs each package twice
(`count: 2`) so map-order defects cannot hide behind a single pass.

Per-module coverage thresholds are declared in `.ergon.yaml` — 85% for the
kernel and the shared tree-sitter module, 75% for satellites. The coverage
stage is currently in `checks.disabled` because a layer with zero statements
reports 0.0% rather than "nothing to measure", which would fail the gate on
stub modules. **Delete that entry when the first module gains real code.**

Beyond unit tests, the specification defines a conformance ladder of eight
rungs — plugintest through warm≡cold — shipped from the kernel and run by
satellites and consumers alike. A satellite or plugin is held to those rungs,
not to hand-written assertions; see
[13-testing-and-conformance.md](docs/architecture/13-testing-and-conformance.md).

## Documentation

**Go docblocks** are API documentation for a reader at the import site.
Write what a package is and does, in present tense. No repository-file
references (godoc renders where those paths are dead text), no status,
no roadmap, no history. State laws as facts.

**The specification** in `docs/architecture/` is closed: every component,
contract, and policy appears in exactly one document. A change to a mechanism
edits the one document that owns it. Prose is for arguments; tables are for
comparisons across fixed sets; diagrams are for structure.

**Decisions** are indexed in the
[decision log](docs/architecture/21-decisions.md) and graduate to full
[ADRs](docs/adr/README.md) when their revisit trigger fires or their argument
outgrows a table row.

## Security

Do not open a public issue for a vulnerability. See [SECURITY](SECURITY.md).
