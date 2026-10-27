<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

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
  rules/           the read half: the decisions the kernel's
                   walks take (member policy, roles, builtins,
                   Resolve, values) + the Tier-2 optionals the
                   language satisfies
  spell/           the write half: canonical-type spelling,
                   optionality, error model, naming joins, the
                   package a routed file declares — the hub's spoke
  backend/         templates + finalise (fmt/imports) + render state
  sdk/             Tier 3: language-only questions, importable from
                   binding files alone (conformance ships the lint)
  testing/         the kernel's toolchain-adapter skeleton
                   implemented (parse / typecheck / test /
                   satisfies): the binaries, project files and
                   probe texts of the language over core/toolchain's
                   Run, Scratch, Sources and Probe + the spelling
                   adapter of nodetest's fixture builder, pinned to
                   the frontend by a parity test, not hand-kept twins
  testdata/features/
                   one fixture per feature row; driven by the
                   kernel's completeness check from a root-level
                   conformance test. The published support matrix is
                   a CI-generated artifact, not code
```

**A read-only language ships `frontend/` and `rules/` and stops.**
`eidos-lang-protobuf` reads and writes every construct of proto2,
proto3 and the editions 2023, 2024 and 2026, so its module has every
package of the anatomy but `sdk/`. Its `testing/` package parses
generated schemas through the parser of its frontend and compiles them
with the experimental compiler of protocompile, both in process. Its
policy defaults map the well-known types onto the canonical forms:

- Timestamp, Duration and Empty map onto the kernel's registry.
- Every wrapper projects as its scalar under the optional form, which
  is the presence a wrapper exists to give.
- FieldMask projects as the list of paths it contains.
- Any and Value project as the top type, Dynamic, because their
  content is decided at run time. ListValue projects as a list of the
  top type, and Struct as a map from text onto the top type.
- NullValue projects as opaque.

A well-known type never resolves against the graph, spelled with or
without its leading dot. Its mapping therefore does not depend on
whether the workspace loads its declaration.

What protobuf states and the projection has no form for is stamped
as declared residue: a field's wire number, a required label and a
json_name, a message's reserved and extension ranges, a file's
imports with public, weak and option marked, its syntax or edition,
its package, and the options every declaration level states. Edition
features are stamped apart from the other options, because editions
replaced the syntax keyword with them.

The frontend loads proto2, proto3 and the editions 2023, 2024 and
2026, and it reports `UnknownEdition` for any other syntax or edition.
The frontend resolves four features from the version's defaults
through the file, each message, a oneof and the declaration's own
options:

- `field_presence` projects a singular field with explicit presence as
  the optional form, as proto2's and proto3's optional label does.
- An enum whose `enum_type` is CLOSED has the marker `protobuf.closed`,
  and the rules derive no value of it outside its declared set.
- A field whose message is encoded delimited has the marker
  `protobuf.delimited`.
- A message or an enum that another file cannot reference, through the
  keyword `local` or the edition's `default_symbol_visibility`, has
  the marker `protobuf.local`.

A message field has presence in every version. Its presence depends on
the declaration that the field's type resolves to, so the rules report
it through `PresenceRules`. Every rpc projects with the context of its
call before its request, through `ContextRules`, and with the error
model `raised`, because every binding of gRPC passes a context to a
handler and fails a call with a status. Under the choices `grpc-go` and
`connect-go` of the policy `golang.grpc`, the Go spoke spells a reference
to a protobuf declaration as protoc-gen-go names it. Under the choice
`connect-es` of `typescript.grpc`, the TypeScript spoke spells it as
protobuf-es names it. Both spokes mark such a reference bound, because
the code generator of the binding writes its referent. A group loads as
a nested message of the group's name and a field of that name in lower
case. The field has the marker `protobuf.delimited`. An extend block
loads as its fields, each with the extended message as its `Receives`.
Each field is a declaration of the scope of the block, which is the
file or a message. An extension is not a field of the message that
declares it.

The backend writes each construct in the syntax of the version of its
file. A generator states a construct through one of five channels: a
field of the model, the tag entry `number` of a field, an annotation for
an option, a marker for a keyword or a statement that is not an option,
and the header of its file. `protobuf.Stated` and `protobuf.Header`
restate the stamps of a protobuf source in these channels, so a
generator over protobuf sources writes a schema as its source states
it.

The frontend attributes comments the way protoc's source info
attributes them, so a schema documented for protoc documents the
same declarations here. Name resolution follows protoc's probe
order in shadowing tiers, with one divergence: protoc fails a
compound name at the innermost scope that declares its first
segment, and the tiers probe past that scope. A schema protoc
accepts resolves identically. The rules read a literal in protoc's
grammar and state an enum value as its number converted to the enum
type, which every target spells.

`eidos-lang-graphql` reads and writes every construct of GraphQL in
the September 2025 edition, so its module has every package of the
anatomy but `sdk/`. Its internal package `document` splits a file
into its two languages. A unit is one schema with its documents. The
partition reads the configurations of graphql-config and of gqlgen in
the directory of each GraphQL file and in every directory above it,
and a file joins the unit of the first schema that claims it. A file
outside every schema joins the unit of the files of its directory.
The executable definitions of a unit load under the package of the
schema followed by `/documents`, so the result types of operations do
not collide with the types of the schema.

A document can mix the type system and executable definitions. The
frontend lexes each file once and parses each language from a copy of
the file without the definitions of the other language, so every
position in a copy is the position in the file. An extension merges
into its type where the schema declares the type, and loads with the
stamp `graphql.extension` where it does not.

The frontend validates the documents of a unit against its schema
under the rules of a client, and lowers each operation and each
fragment into a function. The result of an operation is the struct of
its selection, `<Operation><Kind>`. A selection that selects fields of
particular types of an interface or a union is a sum. A custom scalar
whose `@specifiedBy` URL is in the satellite's table of the scalars of
scalars.graphql.org projects to the shape of the table through
`ShapeRules`. Every other custom scalar projects as a reference.

The backend writes a keyword that the model has no field for from a
marker, an annotation whose name starts with `graphql:`, and the schema
statement of a file from its header. An operation and a fragment write
their selection sets from the verbatim body of a function.
`graphql.Stated` and `graphql.Header` restate the stamps of a GraphQL
source in these channels, so a generator over GraphQL sources writes a
document as its source states it. The policies of the backend choose
between a built-in type and a scalar of the table for the shapes that
GraphQL has no built-in type for. The `testing/` package parses and
validates generated documents through gqlparser, in process.

`eidos-lang-python` reads and writes every declaration construct of
Python 3 in modules and stub files, and every type form of the typing
specification, so its module has every package of the anatomy but
`sdk/`. A unit is one module. Its files are a `.py` file, a `.pyi` file,
or both files of one name in one directory, and the stub states the types
of both.
The package of a module is its dotted name with slashes, below the
source root that the table of the build backend of its `pyproject.toml`
states. The frontend loads the standard library from the stubs of
typeshed, which the satellite embeds at one commit, and the installed
distributions from the site-packages directory of the virtual
environment that `VIRTUAL_ENV` names.

The kind of a class follows from its direct bases: a Protocol and a
TypedDict are interfaces, a class of enum is an enum, and every other
class is a struct. A string annotation is the type that it contains.
The frontend evaluates `TYPE_CHECKING` as true and compares the checks
of `sys.version_info` and `sys.platform` with its options, so it loads
the branches that a type checker reads.

The backend writes modules and stub files for Python 3.11 to 3.15
under the policy `python.version`, in the layout of ruff's formatter.
A construct that the model has no field for is a marker, an annotation
whose name starts with `python:`, and the head of a module is the
header of its file. `python.Stated` and `python.Header` restate the
stamps of a Python source in these channels, so a generator over Python
sources writes a module as its source states it. The `testing/` package
parses generated files with the `ast` module of CPython, type-checks
them with pyright in its strict mode and runs their tests with
unittest.

The TypeScript frontend loads the library of each project from the
declaration files of TypeScript 7.0.2, which the satellite embeds. The
options `noLib`, `lib` and `target` of the tsconfig of a project select
the library, and a file without a tsconfig has the default library of
ES2025. The declarations of a library load into the package
`typescript-lib://<key>`. The key is the root files of the library,
joined with `+`. A global name of a source file resolves in the global
package of the workspace and then in its library. A lib directive at the
head of a file adds its library to the library of that file alone.

A load contains the declarations of the library that its units use. A
unit uses each global name of its library that one of its references can
resolve to, and the dependency round of the frontend places one unit for
each use: the statements of the name in each file of the library, which a
generated index of the statements of each file slices without a parse.
The rounds repeat while a loaded unit has a use that no round passed. A
one-file tree that references `Map` loads 402 declarations of the library
in 5 rounds, and one that references `HTMLElement` loads 43,231 in 15.

The Go frontend records a use for each declaration outside the package
of a file that a named type of a declaration or an argument of a
directive can resolve to. Its dependency round places the package of
each use. A package that only a function body or a value refers to does
not load. A one-file tree that reads `runtime.GOOS` in a value loads no
package of the standard library. The rules write the value with a
stand-in for `runtime`.

The Go rules resolve a directive argument in the scope of the file of
its subject, and then in the scope of its package. A qualifier that the
imports of the file do not bind takes the import of another file of the
package. A qualifier that two files bind to different paths resolves to
no declaration. A bare name that the package and the dot imports of the
file do not declare resolves in the external test package of the
directory. It then resolves in the external test package of the
directory of the route of the subject, where a plugin writes its tests
under `out`. The path of that package is the path of the package joined
with the route below the directory of the file. The frontend records the
uses of an argument in the same scope, so a signature-only file keeps an
import that a directive of another file names.

`golang.CheckReference` returns the reason that Go code of a package
cannot refer to a declaration. It refuses these declarations:

- a declaration of the external test package of another directory
- a declaration of an internal package of a tree that does not contain
  the code
- an unexported declaration of another package

The Go backend checks each import with it. The layout calls it through
`spell.Importable` for each reference of a routed declaration, and the
rules decide `Bound.Importable` and `Bound.ImportableValue` through it.

A signature-only parse of the Go frontend keeps an unexported type that
an exported type of its package embeds or names as the target of an
alias, and each unexported type that a kept type reaches the same way.
The exported type promotes the members of such a type. `testing.T`
embeds `testing.common`, so the member walk of a struct that embeds
`*testing.T` lists the methods of `testing.common`. A kept type loads its
exported fields and methods.

The parse also lowers each unexported method of an interface that it
loads. A type implements the interface only where it has the method.
Only a type of the package of the interface can declare such a method.
The parse keeps the method of that name of each type of the package. `testing.common` declares the method `private` of `testing.TB`,
so `testing.T` implements `testing.TB` in a dependency unit. The Go rules
scope an unexported name to its package. A type of another package with a
method `private` does not implement `testing.TB`.

The TypeScript and Python frontends mark an overload signature with
`Overload`, and the callable of its name without the mark is the
implementation. The render writes the declarations of a TypeScript file
and of a Python module in the order that the module evaluates them: the
signatures of a name right before its implementation, and a class after
its bases and its decorators. A Python stub file keeps the order of the
flush and states signatures without an implementation.

The TypeScript backend writes the variance of a type parameter of a
class, of an interface, and of an alias of a function type or an object
type. tsc refuses a variance annotation on every other declaration, so
the lowering leaves out the variance of the type parameters of a
function, a method and every other alias. TypeScript infers the variance
of a generic type from its structure.

The TypeScript frontend spells a type from its tokens with the spaces
that Biome and Prettier write on one line, so a reformatted signature
keeps one spelling. The backend writes an object literal with a space
inside its braces. It writes each string in the quote that the plan
policy `typescript.quote` selects: `double`, the default of both
formatters, or `single`. A generated file whose lines have at most 80
columns is a fixed point of either formatter in the style of the policy.

The Go backend formats each file with gofumpt v0.12.0 and its three
extra rules, and then with golines v0.16.0 at the width that the plan
policy `golang.width` selects: `80`, `100`, `120` or `none`. The
default is `100`, the default width of golines and of golangci-lint. A
generated file is then a fixed point of gofmt, of `gofmt -s`, of
gofumpt and of golines at that width or a wider one. The backend
refuses to format a file while the environment sets
`GOFUMPT_SPLIT_LONG_LINES=on`, because gofumpt then splits long lines.

The rules of `eidos-lang-rust` project a crate without a load of the
standard library. A type of the standard library that a signature names
folds through the table of builtins of the rules: the primitives and the
texts, `Option`, the lists, the maps, `Box`, `Rc`, `Arc` and the cells as
borrows, `Duration`, `SystemTime`, the error types, and the iterators and
streams of their `Item`. A set, `Instant`, a type of another crate and a
`Result` outside the return of a callable fold to Opaque, because the
kernel has no form for a set or for a tagged union of two types.

A return of `Result<T, E>`, of an alias of the crate for it and of a
`Result` of a module, such as `io::Result<T>`, has the result-type error
model. The kernel splits it into the views of its value and its failure
through `ResultRules`. The frontend resolves `Self` to the struct, the
enum or the trait of its impl block or its body, and a receiver `&self` is
a borrow of `Self`. A trait impl adds its trait to the `Implements` of the
struct, the enum or the data enum that it names. The rules read it for the
error contract, the equality and the zero value of the type. The rules
derive the values of Rust's grammar of expressions, such as
`String::from("a")`, `vec![x]` and `HashMap::from([(k, v)])`, and an
authored text of a `String` takes its call through `TextRules`.

## The language landscape

Go and TypeScript are fully supported. protobuf reads and writes every
construct of its versions, GraphQL every construct of the September
2025 edition, and Python every declaration construct of Python 3.

The vocabulary already anticipates the rest, and each row below
names the language that forced the model or tier addition
([02-symbol-model.md](02-symbol-model.md),
[03-projection.md](03-projection.md)):

| Construct | Forced by | Where it arrives |
|---|---|---|
| Variance on type params | Kotlin, C#, Java wildcards | Tier 1 symbol |
| Extends and Implements against Embeds | JVM and Python against Go | Tier 1 symbol |
| The contracts of an enum and a sum | Java and Kotlin enums, Rust trait impls of enums | Tier 1 symbol |
| Overloads | Java, Kotlin, C#, TS, Python | a model rule: member slices, and Tier 1 `Overload` for a signature without a body |
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
| A failure in the type of a return | Rust `Result<T, E>` | Tier 2 `ResultRules` |
| A text that a call writes | Rust `String::from` | Tier 2 `TextRules` |
| Erasure | Java and Kotlin generics | Tier 2 `GenericsRules` fact |

Out of scope and recorded rather than half-built: header-based C and
C++, because there is no declaration model worth projecting without
a real compiler frontend.

## Completeness is checkable

Three rules make "supports language X" something you can check
([03-projection.md](03-projection.md) has the scale).

First, the symbol model admits what any language needs, and other
languages leave those parts empty. Second, every construct sits on
one level of the degradation scale. Third, the **completeness check**
is the conformance corpus: one neutral feature inventory, one tree
per language spelling each feature its own way, and a coverage
verdict per feature per language on the four levels. Each level is a
predicate the check evaluates through the projections: a feature
projects fully when every reference folds to a form other than
Opaque, every callable projects, every member set is complete and
the entry names no remainder keys; partly when a reference folds to
Opaque or a set carries a gap and every named remainder key is
present; as Opaque when its own reference folds so and its keys are
present; and it refuses when the tree carries nothing under it.

The check verifies that each feature arrives exactly where it
declared. A feature that arrives *better* than declared fails too,
because a capability nobody declared is a capability nobody tested.
The published per-language support matrix is generated from that
run, so "what does eidos-lang-java support" is a build artifact.

## The kits

Frontends and backends produce and finish the graph rather than
subscribing to it, so they are not rules
([06b-authoring.md](06b-authoring.md)). They get **kits**: the
author supplies the handful of things that genuinely belong to the
language, and the kit owns the steps every satellite would
otherwise reimplement differently.

**Each satellite declares one `CommentSyntax` value** in `lang.go`,
shared by both kits. The frontend strips comments with it, and the
backend carries it for the output contract, which writes the
generated-file header through it:

```go
type CommentSyntax struct {
    Line   []string        // "//", "///", "//!", "#", …
    Blocks []CommentBlock  // {Open, Close, Gutter}: {"/**", "*/", "*"}
}
```

That one value covers the landscape: Go's `//` and `/* */`, the
C-family doc blocks with `*` gutters, where a `+<brand>:` carrier inside
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
frontend.New(name, lang, syntax).
    Match("**/*.go").                                       // the files that the frontend claims
    Classify(frontend.TestFiles(golang.TestFileKey, isTest)). // stamps the test files; excludes NOTHING
    Units(byDirectory).                                     // the language defines the unit
    Parse(parseUnit).
    Build()

// Unit — everything a frontend may touch, and the only things:
u.Files() []UnitFile          // the unit's members
u.Read(name) ([]byte, error)  // THE door: jailed + fingerprint-tracked
u.Artifacts() []Artifact      // declared dep artifacts (JARs, .d.ts)
u.Depth() Depth               // Full | Signatures — the kit tells you
u.Graph() *GraphBuilder       // back-pointers, identities, ordering
u.Doc(raw) / u.DocLines(…)    // the comment pipeline
u.RefuseCarriers / RefuseSugar // a directive on a subject the model cannot address
u.ReportSyntax(code, errs)    // ten syntax errors one by one, then their count
u.Errorf / u.Warnf            // scoped diagnostics
```

A partition finds the manifest of a unit through `frontend.Nearest`,
which reads each directory once for all the probes of one name. The
builder stamps the module of each package through `StampModule`. A
tree-sitter frontend attributes its comments through the comment walk
of `eidos-lang/treesitter`, with the facts of its language as the
parameters of the walk.

Three rules ride the kit.

**Classify rather than exclude.** The frontend parses everything it
can parse and stamps classifications, such as `golang.testFile=true`.
Whether test files take part is the consumer's call, through Sources
scopes and `Where` gating, never a satellite's hardcoded opinion.
dokimi both generates and reads test files.

One exclusion belongs to the kernel rather than any satellite: files
this workspace generated itself, proven by a manifest entry or a
provenance trailer, do not load at all
([08-workspace-and-plans.md](08-workspace-and-plans.md)), and the
kit enforces that before `Parse` sees a unit. Output from a
*foreign* generator is ordinary input. Its parse stamps it with the
generated marker of its language, and a consumer selects it by the
marker as by any other fact.

**There is no ambient filesystem.** `u.Read` is the only way bytes
enter a frontend. It is jailed to the unit's files, to declared
shared inputs such as `go.mod`, which fold into every dependent
unit's key, and to declared dependency artifacts. The unit's key
folds the digest of every file the door admits, which turns
hermeticity and cache correctness from rules into mechanisms: a
frontend cannot read what the key does not cover.

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
backend.New(name, target, syntax).
    FileTemplate(skeleton).      // {{imports}}{{decls}} is the kit
                                 // default; the header is not the
                                 // skeleton's to spell
    KindTemplates(kinds).        // how this language spells each kind
    RefusedKinds(refused).       // the kinds it cannot spell, and why
    Funcs(helpersFor).           // func(*ImportSet) FuncMap, per 07
    Naming(spellFilename).       // word, tag and file-key stem join per target
    Packages(packageAt).         // the package a file at a routed path declares
    ImportRule(importable).      // the references that a file of a package
                                 // cannot refer to
    Scaffold(spellStmt).         // the neutral statement vocabulary
    Imports(renderImports).      // grouping/sorting are language facts
    Finalise(format.Source).
    Binder(serveRPC, homeOf).    // the handler of an rpc and the package of
                                 // the code of each gRPC binding
    Build()                      // panics on a declaration defect
```

A target with a gRPC binding declares `Binder`
([10-cross-language.md](10-cross-language.md)). Its handler writes
the method that serves an rpc in the form of the binding that the
policy selects. The Go binder writes the forms of protoc-gen-go-grpc
and protoc-gen-connect-go, and the TypeScript binder writes the
`ServiceImpl` of Connect-ES. Its home returns the package of the code
that the binding generates for a schema: the Go package of the
directory of the schema, the connect package below it for a service,
and the module `<stem>_pb` beside the schema in TypeScript. A backend
without a binder serves the canonical method of an rpc, and the layout
refuses each bound reference of its plan.

The kit does the same work for every satellite: the filename and
package halves the plan's layout routes files through
([18-routing-and-layout.md](18-routing-and-layout.md)), merging plugin
helpers and declared overrides ([07-rendering.md](07-rendering.md)),
splicing slots, resolving `TemplateRef`s, collecting imports through
the spelling helpers, and the continue-on-failure flow. The render
ends at files as values: the generated-file header, the provenance
trailer and the write are the output contract's
([17-output-and-determinism.md](17-output-and-determinism.md)),
stamped after the formatter ran.

It also owns **per-file render parallelism**. Files are independent
and ImportSets are per file, so the kit parallelises the render
loop, and a thousand `Finalise` calls stop being a serial tail.
Each worker binds the language's helpers to its own import set once,
so a helper that spells a type records the import the spelling
needs in the file under render.

The marker rule, the format-error rule and the merge order all
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
toolchain found on PATH returns differently per machine and per
version, which hermeticity refuses and warm≡cold cannot absorb.

Within that rule, **tree-sitter is the platform**: one grammar
model, and one binding layer in the shared `eidos-lang` module,
which pins every grammar and is the only importer of the bindings.
A satellite deviates only where a fully supported pure-Go parser already
exists and is proven, which means Go's standard-library parser,
protobuf's protocompile and GraphQL's gqlparser. Those three never
import the grammar packages. Every satellite shares the pure helper
packages of `eidos-lang`, such as its case conversion, and the kit
keeps even the parser deviation invisible downstream:

| Language | Parser | Why |
|---|---|---|
| Go | stdlib `go/parser` | the reference grammar as a zero-dependency pure-Go library, with exact positions and comment attachment |
| TypeScript | the fork `dokimasia/tree-sitter-typescript` of tree-sitter-typescript, pinned to a commit | proven in the current satellite. The native TS 7 compiler is written in Go but ships internal packages only, so there is nothing importable to pin. The upstream grammar has not changed since v0.23.2 of 2024-11-11. The fork fixes seven constructs that it reads differently from TypeScript, and it parses every valid file of the example projects of upstream's CI |
| protobuf | the experimental parser of `bufbuild/protocompile`, pinned to a commit | pure Go and declarative. The released parser stops at Edition 2023, and a file of Edition 2026, whose grammar is Edition 2024's, parses as Edition 2024 |
| GraphQL | the fork `dokimasia/gqlparser/v2` of gqlparser, pinned to a commit | the parser of gqlgen, pure Go, with the comments of a document and their positions. Its validator types each selection against its schema. The fork fixes four forms that gqlparser refuses, and the satellite contributes the fixes upstream |
| Rust | tree-sitter-rust | no production pure-Go Rust parser exists, and the grammar is org-maintained |
| Python | the fork `dokimasia/tree-sitter-python` of tree-sitter-python, pinned to a commit | no pure-Go parser reads Python 3. The upstream grammar has merged no change since 2025-09-15, and the fork adds the lazy imports of Python 3.15, the unpacking of PEP 798, the defaults of PEP 696 and the plus sign of a literal pattern. Docstrings enter through the kit's `Doc` and `DocLines` door |
| Java | tree-sitter-java plus the class-file reader (D31) | a mature grammar, and JARs supply signature-only dependencies |
| Kotlin | tree-sitter-kotlin, **flagged** | the community grammar reports a 61% structural match against the JetBrains PSI reference. Sequence this satellite late and expect more level-2 and level-3 rows in its feature matrix until the grammar closes the gap |
| PHP | tree-sitter-php | a pure-Go parser exists, VKCOM's `php-parser` for PHP 8, but nobody has maintained it since 2022. Adopt it only with a maintenance commitment |

One operational note. The official tree-sitter Go bindings use cgo,
which taxes every consumer binary that embeds such a satellite with
a C toolchain and complicates cross-compilation. Wazero-based
bindings, which are pure Go over WASM, exist but are pre-release.
The binding choice is private to `eidos-lang`, so satellites migrate
when those mature and nothing downstream changes.

The Go frontend folds a package's methods onto the struct that
declares their receiver, whichever file spelled them, so a type
carries its members the way the model states them and the member
walk sees them; a method on a defined type over a builtin stays a
file-level declaration owned by the receiver's name, because the
alias kind carries no members.

The recorded revisit trigger, per satellite: a compiler-as-library
frontend needs an importable, public API. A port that ships internal
packages only, as the native TypeScript compiler does today, does
not qualify, and neither does a compiler that exists only as a
machine toolchain, such as javac or a JVM. If one publishes a public
API for a language that has become a heavy source language for
semantic generation, that satellite revisits the choice. Until then
the scale carries the residue.

### What syntax plus the graph returns, and what it cannot

A tree-sitter frontend never type-checks, but the frontend was never
the layer that returned semantic questions. **The frontend parses,
Link resolves, and the rules project.** A satellite's `rules/`
returns over the Link-resolved graph
([02-symbol-model.md](02-symbol-model.md)), which gives
whole-workspace declaration knowledge without a toolchain, and every
question arrives on a declared source of truth:

| Question | Answered by |
|---|---|
| declarations, signatures, docs, carriers | syntax |
| build and config constraints on a file | syntax: parsed and stamped as classification facts, never resolved by the frontend. Sources scopes and `Where` decide |
| enum and constant values, including iota-style arithmetic | the layer that has the facts: the Go frontend evaluates through the type checker and stamps the exact value, and the TypeScript and protobuf rules evaluate from the declaration |
| cross-package types, embeds, effective member sets | the linked graph, through `Members` and `Resolve` ([03-projection.md](03-projection.md)) |
| comparability, zero values, samples | `rules/` walking the linked graph |
| dependencies distributed without source | declared artifacts such as JARs and `.d.ts`, parsed and never executed |
| inferred types, such as `var x = f()` with no annotation | level 2: the declaration projects without a type, and its value keeps the spelling of the initializer |
| anything needing execution or macro expansion | levels 3 and 4 of the scale |

The residue is honest and small: what a compiler infers that the
declarations do not state. Every such row sits on the degradation
scale, and the completeness check verifies where
([13-testing-and-conformance.md](13-testing-and-conformance.md)).
"Syntax-derived" is a declared property per feature row rather than
a blanket caveat.

A compiler shipped as a pinned, importable library, which today
means Go's standard-library parser, can shrink that residue without
touching hermeticity. The rule above governs where the parser comes
from, never how much it knows.

Considered and refused. **Toolchain frontends** that fork `go list`,
invoke Node or require a JVM: each drags its language's toolchain
into every consumer's runs, and its returns move with whatever
version the machine carries, which warm≡cold across machines cannot
absorb. **Hybrid loading**, using the toolchain when present and a
parser library otherwise: one workspace producing different graphs
on different machines is a nondeterminism no check can license.

## The next satellites

**eidos-lang-java and eidos-lang-kotlin** are demanding but
conventional, and the vocabulary above already holds them. Their
payoff in an enterprise is annotation lifting: `@Entity`,
`@Nullable` and `@Transactional` become structured, queryable,
overridable metadata through `AnnotationRules`, with no special
cases. Lowering has to account for erasure. C# is conceptually free
once Java exists, and Swift adds protocol conformance the model
already holds.

**eidos-lang-php** forces no new model element. Native union types
are already in the shape vocabulary, since TypeScript forced them.
Attributes ride `AnnotationRules`. PHP 8.1 enums arrive on the Enum
kind. Traits ride Embeds with `php.trait` metadata. The parser table
above names its options, and the Kotlin sequencing note applies in
reverse: PHP is cheap and can come early.
