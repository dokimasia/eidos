# eidos

eidos generates code. A frontend parses your source into a symbol graph that
belongs to no language. Plugins annotate that graph and produce output
entities. A backend renders those into a target language. One run regenerates
only what changed, produces the same bytes every time, and can target several
languages at once.

eidos ships no binary. You build the binary. It composes the command kernels,
and it is the thing your users install.

**eidos is not released yet.** The [specification](docs/README.md) is
complete, and the module table says what each module contains. Start with
[one declaration, end to end](docs/architecture/00-one-declaration-end-to-end.md),
which follows one declaration through the whole system.

## Modules

Each component is its own module in this repository, tagged and released on
its own as `eidos-core/v0.1.0`. The kernel changes slowly and breaking it is
expensive. Catalogs and language support change often, so they release on
their own schedule.

| Module | Import path | Contents |
|---|---|---|
| [eidos-core](eidos-core) | `go.dokimi.dev/eidos/core` | the kernel: the symbol, node and emit models, metadata, directives, diagnostics, the store, projection rules, the plugin SPI and authoring surface, the frontend and backend kits, output, the workspace, the toolchain harness and the conformance kits |
| [eidos-sdk](eidos-sdk) | `go.dokimi.dev/eidos/sdk` | the generated facade plugin modules import: one re-export per exported symbol of the kernel's plugin-facing packages |
| [eidos-conformance](eidos-conformance) | `go.dokimi.dev/eidos/conformance` | the cross-language corpus: one feature inventory and one set of expectations that every language's frontend is checked against |
| [eidos-lang](eidos-lang) | `go.dokimi.dev/eidos/lang` | the helper packages the satellites share: lowering, naming, scaffold, spellref and textfmt |
| [eidos-lang-go](eidos-lang-go) | `go.dokimi.dev/eidos/lang/go` | Go satellite: frontend, projection rules, annotator, backend and the toolchain adapter for generated tests |
| [eidos-lang-typescript](eidos-lang-typescript) | `go.dokimi.dev/eidos/lang/typescript` | TypeScript satellite: backend |
| [eidos-lang-protobuf](eidos-lang-protobuf) | `go.dokimi.dev/eidos/lang/protobuf` | protobuf satellite, read-only by design |
| [eidos-lang-java](eidos-lang-java) | `go.dokimi.dev/eidos/lang/java` | Java satellite: backend |
| [eidos-lang-kotlin](eidos-lang-kotlin) | `go.dokimi.dev/eidos/lang/kotlin` | Kotlin satellite: a statement of scope and no code |
| [eidos-lang-php](eidos-lang-php) | `go.dokimi.dev/eidos/lang/php` | PHP satellite: a statement of scope and no code |
| [eidos-lang-rust](eidos-lang-rust) | `go.dokimi.dev/eidos/lang/rust` | Rust satellite: backend |
| [eidos-plugin-shape](eidos-plugin-shape) | `go.dokimi.dev/eidos/plugin-shape` | the classification catalog of shapes, mixins and contracts: a statement of scope and no code |
| [eidos-reference](eidos-reference) | `go.dokimi.dev/eidos/reference` | the reference plugin ensemble, a consumer of the authoring surface: a statement of scope and no code |

Consumers depend on satellites, satellites depend on the kernel, and nothing
depends the other way. One satellite never imports another. Anything
cross-language goes through the kernel's canonical-type hub. The kernel knows
no language and takes no third-party dependencies.

`go.work` holds the module list and ergon reads it from there. The root
module `go.dokimi.dev/eidos` owns the import-path prefix and pins the
toolchain for CI. It holds no packages and stays out of `go.work`.

## Development

[ergon](https://go.thesmos.sh/ergon) drives the build, the tests, the linters
and the releases:

```sh
make bootstrap    # install dev tools
make check        # the full pre-merge gate: mod verify, lint, test
make fmt          # apply SPDX headers, gofumpt, gci, markdownlint
make help         # every target
```

Write Conventional Commits, scoped by module name, such as `feat(core):` or
`fix(go):`. [.ergon.yaml](.ergon.yaml) lists the accepted types and scopes,
and the commit-msg hook rejects anything else.

## Contributing

[CONTRIBUTING](CONTRIBUTING.md) tells you how to set up, what to run before a
PR, and how to write a commit message. Everyone taking part follows the
[Code of Conduct](CODE_OF_CONDUCT.md). Email vulnerabilities to
security@dokimi.dev, as [SECURITY](SECURITY.md) explains.

## License

Apache-2.0. See [LICENSE](LICENSE).
