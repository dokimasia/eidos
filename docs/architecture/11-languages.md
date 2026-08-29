# Languages

*Builds on: [03](03-projection.md) (the tiers),
[10](10-cross-language.md) (lowering). Feeds:
[13](13-testing-and-conformance.md) (completeness), and every
satellite module.*

## What a language is

A satellite module with a fixed anatomy. Language support groups by
language, uniformly, so that documentation and muscle memory carry
from one language to the next:

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

**A read-only language ships `frontend/` and `rules/` and stops.**
`eidos-lang-protobuf` is a complete satellite of the read-only
shape, and the anatomy says so structurally. It also ships the
well-known-type canonical mappings, meaning Timestamp, Duration and
the rest, as policy defaults.

## The language landscape

Go and TypeScript are first-class. protobuf is read-only.

The vocabulary already anticipates the rest, and each row below
names the language that forced the model or tier addition
([02-symbol-model.md](02-symbol-model.md),
[03-projection.md](03-projection.md)):

| Construct | Forced by | Where it lands |
|---|---|---|
| Variance on type params | Kotlin, C#, Java wildcards | Tier 1 symbol |
| Extends and Implements against Embeds | JVM and Python against Go | Tier 1 symbol |
| Overloads | Java, Kotlin, C#, TS | a model law: member slices |
| Sum against Union | Rust enums, sealed classes and `oneof` against TS unions | Tier 1, two shapes |
| Normalized Visibility | Java, Kotlin `internal`, Rust `pub(crate)` | Tier 1 symbol, raw spelling in metadata |
| Instance and Type member level | JVM statics, companions | Tier 1 symbol |
| Properties | Kotlin, C#, Swift, `@property`, JavaBeans | Tier 2 projection |
| Constructors | JVM, TS and Python constructors against Go and Rust conventions | Tier 2 |
| Checked throws | Java, Swift typed throws | Tier 2 |
| Structured annotations | Java and Kotlin, Rust attributes, decorators | Tier 2, read statically |
| Async and async streams | TS, Rust, Python, Kotlin | Tier 1 Callable |
| Default interface bodies | Java 8 and later, Kotlin, Rust | symbol `HasDefault` |
| Nested modules | Rust `mod`, TS namespaces | hierarchical `Package.Path` |
| Nullability-unknown | Java unannotated refs | metadata plus lowering policy |
| Ownership and borrowing | Rust | Tier 2 `OwnershipRules` |
| Erasure | Java and Kotlin generics | Tier 2 `GenericsRules` fact |

Out of scope and recorded rather than half-built: header-based C and
C++, because there is no declaration model worth projecting without
a real compiler frontend.

## Completeness is checkable

Three rules make "supports language X" something you can check
([03-projection.md](03-projection.md) has the ladder).

First, the symbol model admits what any language needs, and other
languages leave those parts empty. Second, every construct sits on
one rung of the degradation ladder. Third, the **completeness rung**
drives `testdata/features/`, holding one small source file per
landscape row beside its expectation:

```yaml
# testdata/features/sum-types/expect.yaml
rung: 1                      # 1 full | 2 partial+meta | 3 opaque+meta | 4 refused
meta: [rust.lifetimeParams]  # rungs 2–3: the keys carrying the remainder
```

The rung verifies that each fixture lands exactly where it declared.
A feature that lands *better* than declared fails too, because a
capability nobody declared is a capability nobody tested. The
published per-language support matrix is generated from that run, so
"what does eidos-lang-java support" is a build artifact.

## The kits

Frontends and backends produce and finish the graph rather than
subscribing to it, so they are not rules
([06b-authoring.md](06b-authoring.md)). They get **kits**: the
author supplies the handful of things that genuinely belong to the
language, and the kit owns the ritual every satellite would
otherwise reimplement differently.

**Each satellite declares one `CommentSyntax` value** in `lang.go`,
shared by both kits. The frontend strips comments with it, and the
backend writes the generated-file header with it:

```go
type CommentSyntax struct {
    Line   []string  // "//", "///", "//!", "#", …
    Blocks []Block   // {Open, Close, Gutter}: {"/**", "*/", "*"}
}
```

That one value covers the landscape: Go's `//` and `/* */`, the
C-family doc blocks with `*` gutters, where a `+gen:` carrier inside
`/** … */` resolves through gutter stripping with one leading space
tolerated, Rust's three line forms, and Python's `#`.

Python's docstrings are strings rather than comments, so they enter
through a second door. `Doc(raw)` hands the kit comment text the
parser produced, for stripping. `DocLines([]string)` hands it text
the author already holds clean. Native attribute sugar stays its own
declared carrier ([05-directives.md](05-directives.md)).

**The frontend kit.** The author writes source to declarations, and
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

Three rules ride the kit.

**Classify rather than exclude.** The frontend parses everything it
can parse and stamps classifications, such as `go.testFile=true`.
Whether test files take part is the consumer's call, through Sources
scopes and `Where` gating, never a satellite's hardcoded opinion.
dokimi both generates and reads test files.

One exclusion belongs to the kernel rather than any satellite: files
this workspace generated itself, proven by a manifest entry or a
provenance trailer, do not load at all
([08-workspace-and-plans.md](08-workspace-and-plans.md)), and the
kit enforces that before `Parse` sees a unit. Output from a
*foreign* generator is ordinary input: parsed, stamped with the
kit's generated-marker classification, and gated by consumers like
any other fact.

**There is no ambient filesystem.** `u.Read` is the only way bytes
enter a frontend. It is jailed to the unit's files, to declared
shared inputs such as `go.mod`, which fold into every dependent
fingerprint, and to declared dependency artifacts. Every read feeds
the unit fingerprint automatically, which turns hermeticity and
cache correctness from rules into mechanisms: a frontend cannot read
what the cache does not know about.

**The unit belongs to the language.** For Go it is the package
directory, so package docs spanning several files form one unit. For
Java it is the package plus its JARs. For TypeScript it is
file-shaped. Reading across units stays refused, because resolution
is Link's job, after Load, over the whole graph
([02-symbol-model.md](02-symbol-model.md)). Signature-only loading
is not a second code path: the same `Parse` runs with `Depth() ==
Signatures`.

**The backend kit.** The author supplies what the language genuinely
varies in, and the kit assembles:

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

The kit owns the same work for every satellite: grouping by target,
rendering the generated-file header
([17-output-and-determinism.md](17-output-and-determinism.md))
through the shared syntax, merging plugin templates and overrides
([07-rendering.md](07-rendering.md)), splicing slots, resolving
`TemplateRef`s, collecting imports through the spelling helpers, the
`CodeRender` continue-on-failure flow, and write routing.

It also owns **per-file render parallelism**. Files are independent,
ImportSets are per file, and sinks are atomic, so the kit
parallelises the render loop the way it owns the header, and a
thousand `Finalise` calls stop being a serial tail.

The header rule, the format-error rule and the merge order all
become kernel behaviour, tested once, rather than reimplemented
differently in each language.

Both kits lower to the SPI roles. The SPI stays public for the
exotic cases, and the conformance suite treats a kit-built frontend
or backend exactly like a hand-built one.

## Hermeticity

**A frontend never executes project code or build tools.** It reads
what it can parse declaratively: `go.work`, `Cargo.toml`, `pom.xml`,
`pnpm-workspace.yaml`, uv config. Where module identity can only be
computed by executing something, such as a Gradle script, workspace
config supplies that identity instead. A generator that runs the
build to read it has become a build step, and it drags the build's
whole supply chain into every generation run.

The same rule covers **dependencies distributed without source**.
Signature-only loading
([09-incrementality.md](09-incrementality.md)) reads whatever
declaration artifact the ecosystem actually distributes: source
where it exists, as with Go modules, Rust crates and TypeScript
`.d.ts`, and binary declaration formats where it does not, as with
JVM class files inside JARs and .NET assembly metadata. Those are
declarative artifacts, parsed and never executed, squarely inside
this rule. A satellite whose ecosystem distributes binaries ships
that reader in `frontend/`. Without it, eidos-lang-java could parse
every source file in the workspace and still resolve none of its
dependencies.

## Parsers

The rule: **a frontend's parser is a pinned library, never a
toolchain the machine happens to supply.** A version-pinned parser
dependency is hermetic and deterministic whatever its origin, be
that the standard library, a Go module or a tree-sitter grammar,
because the same workspace resolves the same parser everywhere. A
toolchain found on PATH answers differently per machine and per
version, which hermeticity refuses and warm≡cold cannot absorb.

Within that rule, **tree-sitter is the platform**: one grammar
model, and one binding layer in the shared `eidos-lang` module,
which pins every grammar and is the only importer of the bindings.
A satellite deviates only where a first-class pure-Go parser already
exists and is proven, which means Go's standard-library parser and
protobuf's protocompile. Those two take no `eidos-lang` dependency,
and the kit keeps even that deviation invisible downstream:

| Language | Parser | Why |
|---|---|---|
| Go | stdlib `go/parser` | the reference grammar as a zero-dependency pure-Go library, with exact positions and comment attachment |
| TypeScript | tree-sitter | proven in the current satellite. The native TS 7 compiler is written in Go but ships internal packages only, so there is nothing importable to pin |
| protobuf | `bufbuild/protocompile` | pure Go, declarative, proven |
| Rust | tree-sitter-rust | no production pure-Go Rust parser exists, and the grammar is org-maintained |
| Python | tree-sitter-python | the same, and docstrings enter through the kit's `Doc` and `DocLines` door |
| Java | tree-sitter-java plus the class-file reader (D31) | a mature grammar, and JARs answer signature-only dependencies |
| Kotlin | tree-sitter-kotlin, **flagged** | the community grammar reports a 61% structural match against the JetBrains PSI reference. Sequence this satellite late and expect more rung-2 and rung-3 rows in its feature matrix until the grammar closes the gap |
| PHP | tree-sitter-php | a pure-Go parser exists, VKCOM's `php-parser` for PHP 8, but nobody has maintained it since 2022. Adopt it only with a maintenance commitment |

One operational note. The official tree-sitter Go bindings use cgo,
which taxes every consumer binary that embeds such a satellite with
a C toolchain and complicates cross-compilation. Wazero-based
bindings, which are pure Go over WASM, exist but are pre-release.
The binding choice is private to `eidos-lang`, so satellites migrate
when those mature and nothing downstream changes.

The recorded revisit trigger, per satellite: a compiler-as-library
frontend needs an importable, public API. A port that ships internal
packages only, as the native TypeScript compiler does today, does
not qualify, and neither does a compiler that exists only as a
machine toolchain, such as javac or a JVM. If one publishes a public
API for a language that has become a heavy source language for
semantic generation, that satellite revisits the choice. Until then
the ladder carries the residue.

### What syntax plus the graph answers, and what it cannot

A tree-sitter frontend never type-checks, but the frontend was never
the layer that answered semantic questions. **The frontend parses,
Link resolves, and the rules project.** A satellite's `rules/`
answers over the Link-resolved graph
([02-symbol-model.md](02-symbol-model.md)), which gives
whole-workspace declaration knowledge without a toolchain, and every
question lands on a declared source of truth:

| Question | Answered by |
|---|---|
| declarations, signatures, docs, carriers | syntax |
| build and config constraints on a file | syntax: parsed and stamped as classification facts, never resolved by the frontend. Sources scopes and `Where` decide |
| enum and constant values, including iota-style arithmetic | syntax plus a small constant evaluator in `rules/` |
| cross-package types, embeds, effective member sets | the linked graph, through `Members` and `Resolve` ([03-projection.md](03-projection.md)) |
| comparability, zero values, samples | `rules/` walking the linked graph |
| dependencies distributed without source | declared artifacts such as JARs and `.d.ts`, parsed and never executed |
| inferred types, such as `var x = f()` with no annotation | rung 2: the declaration projects with the type opaque, and `<lang>.inferred` carries the spelling |
| anything needing execution or macro expansion | rungs 3 and 4 of the ladder |

The residue is honest and small: what a compiler infers that the
declarations do not state. Every such row sits on the degradation
ladder, and the completeness rung verifies where
([13-testing-and-conformance.md](13-testing-and-conformance.md)).
"Syntax-derived" is a declared property per feature row rather than
a blanket caveat.

A compiler shipped as a pinned, importable library, which today
means Go's standard-library parser, can shrink that residue without
touching hermeticity. The rule above governs where the parser comes
from, never how much it knows.

Considered and refused. **Toolchain frontends** that fork `go list`,
invoke Node or require a JVM: each drags its language's toolchain
into every consumer's runs, and its answers move with whatever
version the machine carries, which warm≡cold across machines cannot
absorb. **Hybrid loading**, using the toolchain when present and a
parser library otherwise: one workspace producing different graphs
on different machines is a nondeterminism no rung can license.

## The next satellites

**eidos-lang-java and eidos-lang-kotlin** are demanding but
conventional, and the vocabulary above already holds them. Their
payoff in an enterprise is annotation lifting: `@Entity`,
`@Nullable` and `@Transactional` become structured, queryable,
overridable metadata through `AnnotationRules`, with no special
cases. Lowering has to account for erasure. C# is conceptually free
once Java exists, and Swift adds protocol conformance the model
already holds.

**eidos-lang-rust and eidos-lang-python** are anticipated by
decision (D6): Result and async in Tier 1, ownership in Tier 2,
Optional surviving `None`, and dynamic typing degrading through the
ladder.

**eidos-lang-php** forces no new model element. Native union types
are already in the shape vocabulary, since TypeScript forced them.
Attributes ride `AnnotationRules`. PHP 8.1 enums land on the Enum
kind. Traits ride Embeds with `php.trait` metadata. The parser table
above names its options, and the Kotlin sequencing note applies in
reverse: PHP is cheap and can come early.
