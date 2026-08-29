# Contributing to eidos

eidos is not released yet. The [specification](docs/architecture/README.md) is
complete and the modules are stubs, so most work right now is implementing a
mechanism the specification already describes. If you want to change one of
those mechanisms, change the specification first.

## Setup

You need Go 1.27.0 or later. Every `go.mod` and `go.work` pins it, and CI
reads the version from there.

```sh
make bootstrap    # installs gofumpt, gci, golangci-lint, govulncheck, go-license
make install      # downloads and verifies dependencies
pre-commit install --hook-type pre-commit --hook-type commit-msg
```

## Before you open a PR

```sh
make check
```

That runs `ergon check` — module verification, lint, and tests. CI runs the
same command, and so does the pre-commit hook, so a commit that would fail CI
fails on your machine first.

While you work, run the narrower targets:

```sh
make fmt          # SPDX headers, gofumpt, gci, markdownlint — run this before make check
make lint-go      # golangci-lint only
make test         # go test, per module
make build        # compile every module
make help         # every target
```

ergon runs golangci-lint once per module, from each module directory. All of
them read the single `.golangci.yml` at the repository root.

Do not add `gofmt` or `goimports` to that config. gofumpt already does what
gofmt does, and goimports regroups imports that gci then regroups back, which
makes `make fmt` produce a different result every time you run it. gci owns
import grouping, with `prefix(go.dokimi.dev/eidos)`.

## How the repository is laid out

Each component is its own Go module, tagged and released on its own. The
[README](README.md) lists them.

`go.work` holds the module list. ergon reads it from there, which is why
`.ergon.yaml` leaves `modules:` empty. The root module
`go.dokimi.dev/eidos` is not in `go.work`: it holds no packages, it exists to
own the import-path prefix and pin the toolchain for CI, and `go vet ./...`
fails on a module with no Go files.

To add a module, follow [Add a module](docs/how-to/add-a-module.md).

## Commits

Write Conventional Commits. The commit-msg hook runs `ergon check commit-msg`
and rejects anything outside these two lists.

**Types:** `feat`, `fix`, `docs`, `refactor`, `test`, `ci`, `chore`, `perf`,
`build`, `deps`, `revert`

**Scopes:** `core`, `lang`, `go`, `java`, `kotlin`, `php`, `protobuf`,
`rust`, `typescript`, `shape`, `reference` — the module names, without the
`eidos-lang-` and `eidos-plugin-` prefixes. Leave the scope off for a change
that touches the whole repository.

Keep the subject under 80 bytes and body lines under 100. Say what changed
and why; the diff shows how.

## Tests and coverage

Put test files beside what they test, and fixtures in `testdata/` beside the
test that reads them. `make test` runs each package twice, so a test that
only passes in one order fails here rather than in CI.

`.ergon.yaml` sets a coverage threshold per module: 85% for the kernel and
`eidos-lang`, 75% for satellites. The coverage stage is currently switched
off in `checks.disabled`, because a module with no statements reports 0.0%
instead of "nothing to measure" and fails the gate. Delete that entry once
the first module has real code in it.

Plugins, satellites and backends are tested by the conformance suite the
kernel ships — eight rungs, from `plugintest` through `warm≡cold`. Write
fixtures for those rungs rather than your own assertions;
[13-testing-and-conformance.md](docs/architecture/13-testing-and-conformance.md)
says what each rung proves.

## Documentation

Write a Go docblock for the person who is about to call the package. Say what
it is and what it does, in the present tense. Do not mention repository files
— godoc renders where those paths mean nothing. Do not write status,
roadmap or history into a docblock. State the rules as facts.

The [specification](docs/architecture/README.md) is closed: every component,
contract and policy appears in exactly one document. Change a mechanism by
editing the one document that owns it.

The [decision log](docs/architecture/21-decisions.md) lists every settled
decision. A decision moves into an [ADR](docs/adr/README.md) when its revisit
trigger fires, or when the argument stops fitting in one table row.

## Security

Do not open an issue for a vulnerability. Read [SECURITY](SECURITY.md).
