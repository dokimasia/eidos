# eidos

A public code-generation framework: frontends parse source into a
language-neutral symbol graph; plugins annotate it and generate
output entities; backends render those to target languages —
deterministically, incrementally, and across languages in one run.

eidos is a library and nothing else. No binary is shipped, ever;
consumers build binaries on the kernel's command kernels and those
binaries are the products.

**Status: specification.** The architecture is fully specified in
[docs/architecture](docs/architecture/README.md); every module below
is a stub. Start with
[one declaration's journey](docs/architecture/00-one-declarations-journey.md).

## Modules

One repository, one module per component. The kernel moves slowly
and its breaks are expensive; catalogs and language support churn on
their own cadence, released independently through per-module tags
(`eidos-core/v0.1.0`).

| Module | Import path | Contents |
|---|---|---|
| [eidos-core](eidos-core) | `go.dokimi.dev/eidos/core` | the kernel: symbol model, projections, directives, plugins, workspace, engine, conformance, command kernels |
| [eidos-lang-go](eidos-lang-go) | `go.dokimi.dev/eidos/lang-go` | Go language satellite |
| [eidos-lang-typescript](eidos-lang-typescript) | `go.dokimi.dev/eidos/lang-typescript` | TypeScript satellite |
| [eidos-lang-protobuf](eidos-lang-protobuf) | `go.dokimi.dev/eidos/lang-protobuf` | protobuf satellite (read-only by design) |
| [eidos-lang-java](eidos-lang-java) | `go.dokimi.dev/eidos/lang-java` | Java satellite (placeholder) |
| [eidos-lang-kotlin](eidos-lang-kotlin) | `go.dokimi.dev/eidos/lang-kotlin` | Kotlin satellite (placeholder, sequenced late) |
| [eidos-lang-php](eidos-lang-php) | `go.dokimi.dev/eidos/lang-php` | PHP satellite (placeholder) |
| [eidos-lang-rust](eidos-lang-rust) | `go.dokimi.dev/eidos/lang-rust` | Rust satellite (placeholder) |
| [eidos-plugin-shape](eidos-plugin-shape) | `go.dokimi.dev/eidos/plugin-shape` | the classification catalog: spec-first shapes, mixins, contracts |
| [eidos-reference](eidos-reference) | `go.dokimi.dev/eidos/reference` | reference plugin ensemble; the compat canary and perf rig |

Dependencies point one way: consumers → satellites → kernel. A
satellite never imports a satellite; cross-language needs go through
the kernel's canonical-type hub. The kernel knows no language and
carries zero third-party dependencies.

`go.work` is the single source of truth for the module list; ergon
discovers modules from it. The root module (`go.dokimi.dev/eidos`)
anchors the import-path prefix and pins the toolchain for CI; it
contains no packages and is not a workspace member.

## Development

The repository uses [ergon](https://go.thesmos.sh/ergon) for the
build, test, lint, and release lifecycle:

```sh
make bootstrap    # install dev tools
make check        # full pre-merge gate (mod verify + lint + test)
make fmt          # apply SPDX headers, gofumpt + gci, markdownlint
make help         # every target
```

Commits follow Conventional Commits with module-name scopes
(`feat(core): …`, `fix(lang-go): …`); the closed scope set lives in
[.ergon.yaml](.ergon.yaml).

## License

MIT — see [LICENSE](LICENSE).
