# Languages

*Builds on: [03](03-projection.md) (the tiers),
[10](10-cross-language.md) (lowering). Feeds:
[13](13-testing-and-conformance.md) (completeness), and every
satellite module.*

## What a language is

A satellite module of a fixed anatomy: language support groups by
language, uniformly, so documentation and muscle memory transfer
between languages:

```
eidos-lang-<lang>/
  lang.go          language ID registration, the CommentSyntax value
                   both kits share, the few shared spellings
  frontend/        source → symbol graph. Owns: toolchain-workspace
                   resolution for declarative formats (go.work /
                   Cargo.toml / pom.xml / pnpm / uv — script-defined
                   builds are config-supplied; see Hermeticity),
                   carrier recognition + native-sugar lowering,
                   scoped and signature-only loading, unit
                   fingerprints, the neutral gen.module facts,
                   <lang>.* metadata stamping
  rules/           the read half: Tier-1 projections (Callable,
                   TypeShape, Resolve, naming) + the Tier-2
                   optionals the language satisfies
  lowering/        the write half: canonical-type spelling,
                   optionality, error model, naming joins — the
                   hub's spoke
  backend/         templates + finalise (fmt/imports) + render state
  sdk/             Tier 3: language-only questions, importable from
                   binding files alone (conformance ships the lint)
  testing/         the kernel's toolchain-adapter skeleton
                   implemented (parse / typecheck / test /
                   satisfies) + the fixture builder — thin adapters
                   over kernel skeletons, not hand-kept twins
  testdata/features/
                   one fixture per feature row; driven by the
                   kernel's completeness rung from a root-level
                   conformance test. The published support matrix is
                   a CI-generated artifact, not code
```

**Read-only languages ship `frontend/` + `rules/` and stop.**
`eidos-lang-protobuf` is not an unfinished satellite; it is a complete one
of the read-only shape, and the anatomy says so structurally. It
additionally ships the well-known-type canonical mappings
(Timestamp, Duration, …) as policy defaults.

## The language landscape

First-class: **Go**, **TypeScript**; read-only: **protobuf**.
Anticipated by the vocabulary now (each row names the language that
forced the model or tier addition —
[02-symbol-model.md](02-symbol-model.md),
[03-projection.md](03-projection.md)):

| Construct | Forced by | Lands |
|---|---|---|
| Variance on type params | Kotlin, C#, Java wildcards | Tier 1 symbol |
| Extends/Implements vs Embeds | JVM & Python vs Go | Tier 1 symbol |
| Overloads | Java, Kotlin, C#, TS | model law: member slices |
| Sum vs Union | Rust enums, sealed classes, `oneof` vs TS unions | Tier 1, two shapes |
| Normalized Visibility | Java, Kotlin `internal`, Rust `pub(crate)` | Tier 1 symbol + raw in meta |
| Instance/Type member level | JVM statics, companions | Tier 1 symbol |
| Properties | Kotlin/C#/Swift, `@property`, JavaBeans | Tier 2 projection |
| Constructors | JVM/TS/Python ctors vs Go/Rust conventions | Tier 2 |
| Checked throws | Java, Swift typed throws | Tier 2 |
| Annotations, structured | Java/Kotlin, Rust attrs, decorators | Tier 2, read statically |
| Async + async streams | TS, Rust, Python, Kotlin | Tier 1 Callable |
| Default interface bodies | Java 8+, Kotlin, Rust | symbol `HasDefault` |
| Nested modules | Rust `mod`, TS namespaces | hierarchical `Package.Path` |
| Nullability-unknown | Java unannotated refs | metadata + lowering policy |
| Ownership/borrowing | Rust | Tier 2 `OwnershipRules` |
| Erasure | Java/Kotlin generics | Tier 2 `GenericsRules` fact |

Out of scope, recorded rather than half-done: header-based C/C++ —
no declaration model worth projecting without a real compiler
frontend.

## Completeness is a property, not a promise

Three laws make "supports language X" checkable
([03-projection.md](03-projection.md) has the ladder):

1. The symbol model admits; languages leave parts empty.
2. Every construct lands on one rung of the degradation ladder.
3. The **completeness rung** drives `testdata/features/` — one small
   source file per landscape row beside its expectation:

   ```yaml
   # testdata/features/sum-types/expect.yaml
   rung: 1                      # 1 full | 2 partial+meta | 3 opaque+meta | 4 refused
   meta: [rust.lifetimeParams]  # rungs 2–3: the keys carrying the remainder
   ```

   The rung verifies each fixture lands exactly as declared — a
   feature landing *better* than declared fails too, because an
   undeclared capability is an untested one. The published
   per-language support matrix is generated from this run; "what
   does eidos-lang-java support" is a build artifact.

## The kits

Frontends and backends are the graph's producer and finisher — not
subscribers, so not rules ([06b-authoring.md](06b-authoring.md)); they get
**kits**: the author supplies the handful of things that are
genuinely the language's, the kit owns the ritual every satellite
would otherwise reimplement divergently.

**One `CommentSyntax` value per satellite**, declared in `lang.go`
and shared by both kits — the frontend strips with it, the backend
writes the generated-file header with it:

```go
type CommentSyntax struct {
    Line   []string  // "//", "///", "//!", "#", …
    Blocks []Block   // {Open, Close, Gutter}: {"/**", "*/", "*"}
}
```

That single value covers the landscape: Go (`//`, `/* */`), the
C-family doc blocks with `*` gutters (a `+gen:` carrier inside
`/** … */` resolves through gutter stripping, one-leading-space
tolerance included), Rust's three line forms, Python's `#` — and
Python's docstrings, which are strings rather than comments, enter
through the second door: `Doc(raw)` hands the kit parser-produced
comment text for stripping, `DocLines([]string)` hands it text the
author already holds clean. Native attribute sugar remains its own
declared carrier ([05-directives.md](05-directives.md)).

**The frontend kit** — the author writes source → declarations;
the kit owns everything around it:

```go
eidos.NewFrontend(lang, syntax).
    Match("*.go").                      // include-shaped selection
    Classify(eidos.TestFiles(isTest)).  // stamps go.testFile; excludes NOTHING
    Units(byDirectory).                 // the language defines the unit
    Parse(parseUnit).
    Build()

// Unit — everything a frontend may touch, and the only things:
u.Files() []UnitFile          // the unit's members
u.Read(name) ([]byte, error)  // THE door: jailed + fingerprint-tracked
u.Artifacts() []Artifact      // declared dep artifacts (JARs, .d.ts)
u.Depth() Depth               // Full | Signatures — the kit tells you
u.Graph() *GraphBuilder       // back-pointers, identities, ordering
u.Doc(raw) / u.DocLines(…)    // the comment pipeline
u.Errorf / u.Warnf            // scoped diagnostics
```

Three laws ride the kit:

- **Classification over exclusion.** The frontend parses everything
  parseable and stamps classifications (`go.testFile=true`);
  whether test files participate is the consumer's call through
  Sources scopes and `Where` gating, never a satellite's hardcoded
  opinion. dokimi both generates and reads test files. One
  exclusion is the kernel's, never the satellite's: files this
  workspace itself generated — proven by manifest entry or
  provenance trailer — do not load at all, per the
  outputs-are-never-inputs law
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)); the
  kit enforces it before `Parse` sees a unit. Output of *foreign*
  generators is ordinary input: parsed, stamped with the kit's
  generated-marker classification, gated by consumers like any
  other fact.
- **There is no ambient filesystem.** `u.Read` is the only way
  bytes enter a frontend — jailed to the unit's files, declared
  shared inputs (`go.mod`-class files, folded into every dependent
  fingerprint), and declared dependency artifacts. Every read feeds
  the unit fingerprint automatically, which turns hermeticity (D29)
  and cache correctness from laws into mechanisms: a frontend
  cannot read what the cache doesn't know about.
- **The unit is the language's** (Go: the package directory, so
  package docs spanning files are one unit; Java: the package plus
  its JARs; TS: file-shaped). Cross-*unit* source reads stay
  refused — resolution is Link's job, after Load, on the whole
  graph ([02-symbol-model.md](02-symbol-model.md)). Signature-only
  loading is not a second code path: the same `Parse` runs with
  `Depth() == Signatures`.

**The backend kit** — the author supplies what the language
genuinely varies in; assembly is the kit's:

```go
eidos.NewBackend(target, syntax).
    FileTemplate(skeleton).      // {{header}}{{package}}{{imports}}{{decls}};
                                 // kit default provided
    KindTemplates(kinds).        // how this language spells each kind
    Funcs(langFuncmap).          // funcmap-once, per 07
    Imports(renderImports).      // grouping/sorting are language facts
    Finalise(format.Source).
    Build()
```

Kit-owned, uniformly for every satellite: grouping by target, the
generated-file header per
[17-output-and-determinism.md](17-output-and-determinism.md)
rendered through the shared syntax, plugin-template and override
merging per [07-rendering.md](07-rendering.md), slot splicing,
`TemplateRef` resolution, import collection through the spelling
helpers, the `CodeRender` continue-on-failure flow, write routing —
and **per-file render parallelism**: files are independent,
ImportSets are per-file, sinks are atomic, so the kit parallelizes
the render loop the way it owns the header, and a thousand
`Finalise` calls stop being a serial tail. The header law, the format-error law, and the merge order
become kernel behavior, tested once — not divergently reimplemented
per language.

Both kits lower to the SPI roles; the SPI stays public for the
exotic cases, and the conformance suite treats a kit-built
frontend or backend exactly like a hand-built one.

## Hermeticity

**A frontend never executes project code or build tools.** It reads
what is declaratively parseable — `go.work`, `Cargo.toml`,
`pom.xml`, `pnpm-workspace.yaml`, uv config — and where module
identity is only computable by execution (a Gradle script), that
identity is supplied in workspace config instead. A generator that
runs the build to read it is a build step, not a reader, and it
drags the build's whole supply chain into every generation run.

The same law covers **dependencies distributed without source**.
Signature-only loading
([09-incrementality.md](09-incrementality.md)) reads whatever
declaration artifact the ecosystem actually distributes: source
where it exists (Go modules, Rust crates, TS `.d.ts`), binary
declaration formats where it doesn't — JVM class files inside JARs,
.NET assembly metadata. Those are declarative artifacts: parsed,
never executed, squarely inside this law. A satellite whose
ecosystem distributes binaries ships that reader in `frontend/`;
without it, eidos-lang-java could parse every source file in the
workspace and still resolve none of its dependencies.

## Parsers

The law: **a frontend's parser is a pinned library, never a
machine-supplied toolchain.** A version-pinned parser dependency is
hermetic and deterministic whatever its origin — stdlib, a Go
module, a tree-sitter grammar — because the same workspace resolves
the same parser everywhere. A toolchain found on PATH answers
differently per machine and per version, which D29 refuses and
warm≡cold cannot absorb. Within the law, **tree-sitter is the
platform**: one grammar model, one binding layer, well developed
and maintained, uniform across satellites. A satellite deviates
only where a first-class pure-Go parser already exists and is
proven — Go's stdlib parser, protobuf's protocompile — and the kit
keeps even that deviation invisible downstream:

| Language | Parser | Why |
|---|---|---|
| Go | stdlib `go/parser` | the reference grammar as a zero-dep pure-Go library; positions and comment attachment are exact |
| TypeScript | tree-sitter | proven in the current satellite; the native TS 7 compiler is written in Go but ships internal packages only — nothing importable to pin |
| protobuf | `bufbuild/protocompile` | pure Go, declarative, proven |
| Rust | tree-sitter-rust | no production pure-Go Rust parser exists; the grammar is org-maintained |
| Python | tree-sitter-python | same; docstrings enter through the kit's `Doc`/`DocLines` door |
| Java | tree-sitter-java + the class-file reader (D31) | mature grammar; JARs answer signature-only dependencies |
| Kotlin | tree-sitter-kotlin — **flagged** | the community grammar self-reports ~61% structural match against the JetBrains PSI reference; sequence the satellite late and expect a fatter rung-2/3 tail in its feature matrix until the grammar closes the gap |
| PHP | tree-sitter-php | a pure-Go parser exists (VKCOM's `php-parser`, PHP 8) but has been unmaintained since 2022 — adopt it only with a maintenance commitment |

One operational note: the official tree-sitter Go bindings are cgo,
which taxes every consumer binary embedding such a satellite with a
C toolchain and complicates cross-compilation. Wazero-based
(pure-Go, WASM) bindings exist but are pre-release; the kit hides
the swap, so satellites migrate when those mature, without any
downstream change.

The recorded revisit trigger, per satellite: a compiler-as-library
frontend needs an *importable, public* API. A port that ships
internal packages only — the native TypeScript compiler today —
does not qualify, and neither does a compiler that exists only as
a machine toolchain (javac, a JVM). If one publishes a public API
for a language that has become a heavy *source* language for
semantic generation, that satellite revisits; until then the
ladder carries the residue.

### What syntax plus the graph answers — and what it cannot

A tree-sitter frontend never type-checks, but the frontend was
never the layer that answered semantic questions: **the frontend
parses; Link resolves; the rules project.** A satellite's `rules/`
answers over the Link-resolved graph
([02-symbol-model.md](02-symbol-model.md)) — whole-workspace
declaration knowledge without a toolchain — and every question
lands on a declared source of truth:

| Question | Answered by |
|---|---|
| declarations, signatures, docs, carriers | syntax |
| build/config constraints on a file | syntax — parsed and stamped as classification facts, never resolved by the frontend; Sources scopes and `Where` decide |
| enum and constant values (iota-class arithmetic) | syntax plus a small constant evaluator in `rules/` |
| cross-package types, embeds, effective member sets | the linked graph — `Members`, `Resolve` ([03-projection.md](03-projection.md)) |
| comparability, zero values, samples | `rules/` walking the linked graph |
| dependencies distributed without source | declared artifacts (JARs, `.d.ts`) — parsed, never executed |
| inferred types (`var x = f()`, no annotation) | rung 2: the declaration projects with the type opaque; `<lang>.inferred` carries the spelling |
| anything requiring execution or macro expansion | rungs 3–4 of the ladder |

The residue is honest and small: what a compiler *infers* that
declarations do not state. Every such row lands on the
degradation ladder, and the completeness rung verifies where
([13-testing-and-conformance.md](13-testing-and-conformance.md)) —
"syntax-derived" is a declared property per feature row, never a
blanket caveat.

A compiler shipped *as a pinned, importable library* — Go's stdlib
parser is the one that exists today — may shrink that residue
without touching hermeticity: the law above is about where the
parser comes from, never about how much it knows.

Considered and refused: **machine-supplied toolchain frontends**
(forking `go list`, invoking Node, requiring a JVM) — each drags
its language's toolchain into every consumer's runs (D29's
supply-chain argument), and its answers move with whatever version
the machine carries, which warm≡cold across machines cannot
absorb; **hybrid loading** (toolchain when present, parser library
otherwise) — one workspace producing different graphs on
different machines is a nondeterminism no rung can license.

## The next satellites

- **eidos-lang-java / eidos-lang-kotlin**: demanding but conventional — the
  vocabulary above already holds them. Their enterprise payoff is
  annotation lifting: `@Entity`, `@Nullable`, `@Transactional` as
  structured, queryable, overridable metadata via `AnnotationRules`,
  no special cases. Erasure-aware lowering. C# is conceptually free
  once Java exists; Swift adds protocol conformance the model
  already holds.
- **eidos-lang-rust / eidos-lang-python**: the vocabulary anticipates them by
  decision (D6): Result/async in Tier 1, ownership in Tier 2,
  Optional surviving `None`, dynamic-typing degrade paths through
  the ladder.
- **eidos-lang-php**: nothing in PHP forces a new model element — native
  union types are already in the shape vocabulary (TS forced them),
  attributes ride `AnnotationRules`, 8.1 enums land on the Enum
  kind, and traits ride Embeds with `php.trait` metadata. The
  parser table above names its options; the Kotlin sequencing note
  applies in reverse — PHP is *cheap* and can come early.
