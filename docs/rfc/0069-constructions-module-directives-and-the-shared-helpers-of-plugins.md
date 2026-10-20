---
rfc: 0069
title: Constructions, module directives and the shared helpers of plugins
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-19
updated: 2026-10-19
discussion: dokimasia/eidos#7 to dokimasia/eidos#19
supersedes: none
superseded-by: none
produces-adr: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0069: Constructions, module directives and the shared helpers of plugins

## Summary

Thirteen issues report what a plugin author cannot write through the SDK and the satellites:

- Two defects. The TypeScript backend respells names through the initialisms of Go (#19), and the
  scaffold refuses a value whose type the substitution of a type parameter restated (#12).
- Helpers that eidos-reference keeps in internal packages or in local templates (#7, #15, #16, #17
  and #18).
- Contracts that the model or the vocabulary lacks (#8, #9, #10, #11, #13 and #14).

We propose these changes:

- A construction value, which TypeScript and Java write as `new C(args)`. The TypeScript rules
  derive the values of `Set` and `Map` through it.
- A rule on a package, and the carriers of the header of a TypeScript module on its package.
- The template helpers `spellvalue` and `typeargs` in every backend.
- `sdk.TemplatesOf`, `sdk.SubjectMatch`, `rules.Mentioned`, `naming.Params` and
  `golang.RenamePredeclared`.
- The optional flag of a parameter view, and the one value of the empty struct of Go.
- A case conversion without initialisms in the satellites of TypeScript, Java, Rust, protobuf and
  GraphQL.

The table of verdicts lists each departure from the proposal of an issue.

## Motivation

| Issue | The problem |
|---|---|
| #19 | `spell.Name` of TypeScript converts through `naming.Default`, whose initialisms are Go's. A method `getById` of an interface becomes `getByID` in the class that a generator mirrors from it, and tsc 7.0.2 refuses the class with TS2420. The satellites of Java, Rust, protobuf and GraphQL convert through the same set. A mirrored Java class misses `getById` too, and a Rust struct `Uuid` becomes `UUID`. A protobuf message `HttpRule` becomes `HTTPRule`, and a GraphQL field `userId` becomes `userID`, which changes the key of a response |
| #12 | `rules.Substitute` clears the spelling of a structural reference that contains a type parameter, so that a speller restates it from its children. The scaffold refused such a type before a target saw it. A sample of `[]T` at the witness `int` failed with "the type of a value has no spelling" |
| #10 | The Go rules derive no value of an inline struct, so they refuse `map[string]struct{}`, the set of Go |
| #11 | The TypeScript rules refuse `Set`, `Map` and `ReadonlyMap`, because the value tree has no construction. A builder generator writes no check of the setter of such a field |
| #8 | The TypeScript helper `spell` imports a declaration for the type checker alone. A template that writes `new {{spell .Data.Value}}()` renders an `import type`, and tsc refuses the file with TS1361 |
| #9 | `rules.ParamView` drops whether a call can leave a parameter out. A consumer of `ConstructRules.Constructors` cannot tell a constructor that needs an argument from one that does not |
| #13 | The SDK declares no rule on a package. A directive in the doc of a Go package clause reaches no handler |
| #14 | The TypeScript frontend refuses every carrier above the first statement of a module under `TYPESCRIPT-0003` |
| #7, #15, #16, #17, #18 | eidos-reference keeps five mechanisms that every plugin of its kind needs: the template tree of a target in `internal/tree` with eight importers, the names of the parameters of a double in `completed`, the type parameters that a part of a declaration mentions in stubgen, an interface of the methods of a match in `double.Match`, and three local templates that write type arguments. A plugin of another module cannot import them, so it declares each one again |

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/emit` | The value kind `ValueConstruction` and the constructor `Construction` |
| `eidos-core/rules` | `ParamView.Optional` and `Mentioned` |
| `eidos-core`, root | `Reporter`, `SubjectMatch`, `OnPackage`, `PackageMatch`, `TemplatesOf` and `TemplatesDir` |
| `eidos-core/backend/render` | The shared helper names `spellvalue` and `typeargs` |
| `eidos-sdk` | The facade of each addition, which the facade generator writes |
| `eidos-lang/scaffold` | `Target.Construction`. A structural type without a spelling passes to the target |
| `eidos-lang/spellref` | `TypeArgs` |
| `eidos-lang/naming` | `Params` |
| `eidos-lang/treesitter` | `Comments.Leading` ends a run at a comment that another subject took |
| `eidos-lang-go` | `RenamePredeclared`, the value of the empty struct, the two helpers and a refused construction |
| `eidos-lang-typescript` | The case conversion, the values of the collections, the header of a module, the two helpers and the construction |
| `eidos-lang-java` | The case conversion, the two helpers and the construction |
| `eidos-lang-rust` | The case conversion, the two helpers and a refused construction |
| `eidos-lang-protobuf` | The case conversion of the respell and of the lowering |
| `eidos-lang-graphql` | The case conversion of the respell and of the lowering |
| `eidos-conformance` | The TypeScript client of the GraphQL catalog converts names as the TypeScript backend does |
| `eidos-reference` | The adoption of each helper. `internal/tree` is removed |

### Invariants

1. A name that the source writes in the style of the target keeps its spelling in the output.
2. A comment belongs to one subject at most. The run above a declaration never contains a comment
   that another subject took.
3. A value tree contains a construction only where the rules of a language derive one. A target
   without a construction form refuses it under `UnspeltValue`.
4. A helper of the shared vocabulary has one name in every backend. Each backend imports what the
   helper writes.
5. A value that a target cannot spell reports under `UnspeltValue` in every target, also where the
   target does not restate its type.

### Constructions

A construction builds a value of a type from argument values:

```go
// ValueConstruction constructs a value of Type from Args.
ValueConstruction

// Construction returns a construction of t from the values args.
func Construction(t *TypeRef, args ...Value) Value
```

The shared walk passes the type of a construction to the target without a spelling, and the
spellings of the arguments beside it. A language that imports a declaration for its type checker
alone imports the class of a construction for its value, so the target spells the type itself:

```go
// Construction spells a construction of a type from the spellings of its
// arguments, and records the import of the type. A language without a
// construction form refuses it.
Construction(ref *emit.TypeRef, args []string) (string, error)
```

| Target | Spelling | Example |
|---|---|---|
| TypeScript | the class through `SpellValue`, its arguments in parentheses | `new Set<string>(['a'])` |
| Java | a class instance creation | `new HashSet<Long>(List.of(1L))` |
| Go | refused, because Go builds a value through a composite literal or a call | |
| Rust | refused, because Rust builds a value through a struct literal or a call | |

### Restated types

The walk refuses a value without a type and a named type without a spelling. It passes a
structural type without a spelling to the target. The spellers of Go and TypeScript restate such a
type from its children, so a sample of `[]T` at the witness `int` spells `[]int{1}`. The spelling
of a Java or a Rust composite does not follow from its structure, so their `Type` refuses a
structural type without a spelling. The refusal is a value error, which the render reports under
`UnspeltValue`.

### The values of the collections

| Type | Sample and alternate | Zero |
|---|---|---|
| `Set<T>`, `ReadonlySet<T>` | `new Set<T>([s])` and `new Set<T>([a])` from the pair of `T` | `new Set<T>()` |
| `Map<K, V>`, `ReadonlyMap<K, V>` | `new Map<K, V>([[k, v]])` and `new Map<K, V>([[k2, v]])`, with the two keys from the pair of `K` | `new Map<K, V>()` |
| `struct{}` in Go | `struct{}{}`, and the alternate refused with `RefusedNoLiteral` | |
| `map[K]struct{}` in Go | `map[K]struct{}{k: struct{}{}}` and the same map with `k2`, from the pair of `K` | |

A readonly collection has no constructor of its own. Its construction calls the constructor of
the mutable class, whose value is a value of the readonly type. The type arguments of the
reference are part of the construction, so the value has the type of the reference without a
contextual type. The arrays of the arguments have structural types without a spelling, which the
TypeScript speller restates. `ReadonlySet` joins the global types of the satellite. The shape
table classifies `Set` as opaque, because the model has no form of a set. A slice of empty structs
refuses its pair, because the empty struct has one value and the two slices would need two.

### Parameters and type parameters

`ParamView.Optional` reports whether a call can leave the parameter out. The parameter has a
default, the optional mark of its language, or collects the rest of the arguments.

```go
// Mentioned returns the type parameters of tps that the references refs
// mention, in the order of tps.
func Mentioned(tps []*emit.TypeParam, refs ...*emit.TypeRef) []*emit.TypeParam
```

A named reference mentions a parameter through its spelling or a type argument, and a structural
reference through its children. The frontends list the types of the members of an inline body as
its children. So the name of a member does not count as a mention, and the walk does not split a
spelling.
`Mentioned` also returns each type parameter that the bounds, the default or the value type of a
returned parameter mention. A declaration of the result then declares every parameter that its
own parameters use, such as the `K` of `V extends K`. The call allocates the returned list alone.

### The authoring surface

```go
// Reporter reports findings at the subject of a match.
type Reporter interface {
    Errorf(c diag.Code, format string, a ...any)
    Warnf(c diag.Code, format string, a ...any)
    Infof(c diag.Code, format string, a ...any)
    ErrorfAt(c diag.Code, at position.Pos, format string, a ...any)
    WarnfAt(c diag.Code, at position.Pos, format string, a ...any)
    InfofAt(c diag.Code, at position.Pos, format string, a ...any)
}

// SubjectMatch is the part that every match with a subject has. Every
// match of a declaration satisfies it, and so do EmitMatch and
// PackageMatch.
type SubjectMatch interface {
    Matcher
    Reporter
    Lang() symbol.Lang
    Rules() rules.Bound
    RulesFor(lang symbol.Lang) rules.Bound
    Reader() *store.Reader
    Directive() *directive.Directive
    Kernel() meta.KernelKeys
    Export(plan string) (plugin.ExportDoc, bool)
}

// OnPackage runs the handler once for each package of the scope, and once
// for each gating instance of a directive on a package under a Directive
// wrapper.
func OnPackage[E Effect](h func(*PackageMatch, E) error) Rule

// TemplatesOf declares the template tree of the target t: the directory
// templates/<t> of files.
func TemplatesOf(files fs.FS, t plugin.Target) TargetOption
```

The model generator asserts `SubjectMatch` on each match type that it writes. `PackageMatch` has
the field `Package` with the files of the package and their declarations. A handler walks the
declarations without a tracked read, so each invocation of `OnPackage` records the package edge,
and an edit of any declaration of the package runs the handler again.

`TemplatesOf` checks its directory when the plugin is built. A typed build panics where the target
is no name of a single directory, where the files have no directory `templates/<t>`, and on nil
files. Each of these defects comes from the source of the plugin, so the first build in any test
reports it.

### The shared vocabulary

| Target | `spellvalue` | `typeargs` of the parameters `K, V` | `typeargs` of two witnesses |
|---|---|---|---|
| Go | what `spell` writes, with its import | `[K, V]` | `[int, Row]` |
| TypeScript | the reference with a value import of its declaration, and its type arguments with type-only imports | `<K, V>` | `<number, Row>` |
| Java | what `spell` writes, with its import | `<K, V>` | `<Long, Row>` |
| Rust | the reference with its type arguments behind the turbofish of an expression's path, as `Page::<u32>` | `<K, V>` | `<u32, Row>` |

`typeargs` takes a list of type parameters or a list of type references. It writes nothing for
an empty list, and spells each reference as `spell` does. A Rust template writes the separator
`::` before the list in an expression's path, as in `check::<u32>`. `spellref.TypeArgs` is the one
implementation, and each backend passes its brackets and its speller. The TypeScript base of a
class spells through `SpellValue` too.

### Names

```go
// Params names each parameter of m that has no name: a followed by its
// position, unique among the names of m. It returns m.
func Params(m *emit.Method) *emit.Method

// RenamePredeclared renames each parameter and each result of m whose name
// the universe scope of Go declares, such as append. It returns m.
func RenamePredeclared(m *emit.Method) *emit.Method
```

Both take the names of the receiver, the type parameters, the parameters and the results first,
and choose a new name through `naming.Unique`. Each allocates nothing for a method that it leaves
unchanged.

RFC-0011 gives each satellite the initialism set of its own backend. The satellites of TypeScript,
Java, Rust, protobuf and GraphQL convert through `naming.New()`, which recognises no initialism and
keeps an acronym run that the source writes, as the style guide of each language states. Go keeps
`naming.Default()`.

| Source name | Go | TypeScript | Java | Rust | protobuf | GraphQL |
|---|---|---|---|---|---|---|
| `httpRow`, a type | `HTTPRow` | `HttpRow` | `HttpRow` | `HttpRow` | `HttpRow` | `HttpRow` |
| `getById`, a method | `GetByID` | `getById` | `getById` | `get_by_id` | `GetById` | `getById` |
| `toJSON`, a method | `ToJSON` | `toJSON` | `toJSON` | `to_json` | `ToJSON` | `toJSON` |
| `user_id`, a parameter | `userID` | `userId` | `userId` | `user_id` | `user_id` | `userId` |

The lowering of a sum in TypeScript and Java joins the name of the sum and the name of the variant
with a word separator, and the convention of the target spells the join. The lowerings of protobuf
and GraphQL name each declaration that they add, such as `<Service><Rpc>Request` and
`<Type><Method>Payload`, through the conversion of their respell, so the respell keeps each name.
The filename of a Java type converts through the same conversion as the name of the type.

A GraphQL type `id` spells `Id`, which is no built-in scalar, so the respell accepts it. The respell
refuses `string`, which spells the built-in `String`. This departs from the example of RFC-0068, in
which `id` spells `ID` and the respell refuses it.

### The header of a TypeScript module

The header of a module is every comment above its first statement, and below its hashbang line
where it has one. The run directly above a first declaration or export belongs to that statement,
so the header leaves it out. The header keeps the run above a first import, because no rule takes
an import as its subject.

The header has these effects:

- Its carriers attach to the package of the module. The carriers of the doc of a Go package clause
  attach to the package in the same way.
- It does not document the package. The first comment of a module is often a license header.
- A script has no header. Its declarations are in the global package of every script.

`Comments.Leading` of `eidos-lang/treesitter` ends a run at a comment that a run or `Take` took.
The run above the first import then does not read the header again. An `export default class {}`
declares the class, so the class reads the comments above the statement, and the export record
does not.

### Allocations

| Call | Allocations |
|---|---|
| `emit.Construction` with one argument | 1 |
| `rules.Mentioned` | 1 |
| `OnPackage` | 2 |
| `TemplatesOf` over an `fstest.MapFS` | 6 |
| `scaffold.Value` of a construction with arguments | 1, beside what the target allocates |
| `naming.Params` and `golang.RenamePredeclared` | 0 for a method that they leave unchanged, and 2 for one name |
| `spellref.TypeArgs` of two entries | 1 |
| `DeriveSamples` of `Set<string>` in TypeScript | 10 |
| `ZeroValue` of `Map<string, number>` in TypeScript | 4 |
| `Funcs` of Go, TypeScript, Java and Rust | 11, 13, 11 and 18, up from 10, 11, 10 and 16 |
| The build of the Go, TypeScript, Java and Rust backends | 1,609, 1,794, 1,487 and 1,871, up from 1,607, 1,790, 1,485 and 1,863 |
| The lowering of a TypeScript sum of two variants | 18, down from 20 |
| The lowering of a Java sum of two variants | 16, down from 20 |
| The settle of the scaled corpora of TypeScript and Java | 1,075,969 and 1,986,667, down from 1,125,969 and 2,186,667 |

The Go and Java backends bind `spell` and `spellvalue` to one method value, so the two helpers cost
one allocation. A build binds the vocabulary of its backend twice. That accounts for the growth of
the builds of Go, TypeScript and Java, and for four of the eight allocations of Rust. A profile
attributes the other four to two maps of the kit, which grow once more at the 29 helpers of Rust.
They are the set of names that `Builder.Funcs` records and the map into which the build copies the
vocabulary.

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| `TemplatesOf` checks that the directory exists and that the target names one directory (#7) | in, a departure: the issue checks the path alone | A missing directory fails at the first build instead of at the first render | A plugin that declares a target before its templates exist |
| `spellvalue` in every backend (#8) | in, a departure: the issue asks for TypeScript and Go | A template has one name for a value position in every target. Rust needs the turbofish there | none |
| `ParamView.Optional` (#9) | in | | none |
| The one value of the empty struct of Go (#10) | in | | none |
| The construction and the values of the TypeScript collections (#11) | in | | none |
| A zero value with the type arguments, such as `new Set<string>()` (#11) | in, a departure: the issue proposes `new Set()` | `new Set()` without a contextual type is a `Set<unknown>` | none |
| `ReadonlySet` (#11) | in, a departure: the issue names `Set`, `Map` and `ReadonlyMap` | The readonly set constructs as the readonly map does | none |
| A structural type without a spelling passes to the target (#12) | in | | none |
| Java and Rust refuse such a type with a value error (#12) | in | The finding keeps its code `UnspeltValue` | A Java or Rust speller that restates a composite |
| `PackageMatch.Package` is a field, and each invocation records the package edge (#13) | in, a departure: the issue proposes a method whose read records the edge | A match exposes its subject as a field, and a handler of a package reads its declarations in every invocation | A handler that leaves the declarations unread |
| The header is every comment above the first statement (#14) | in, a departure: the issue names the first comment | A carrier below a license header and a blank line addresses the module too | none |
| `Comments.Leading` ends at a taken comment (#14) | in | Each comment belongs to one subject | none |
| `Params` in eidos-lang (#15) | in, a departure: the issue places `NameParams` beside `Mirror` in the SDK | The kernel does not import `naming`, and RFC-0048 makes `naming.Unique` the one rule for a free name | none |
| `RenamePredeclared` in the Go satellite (#15) | in | The universe scope is a fact of Go | none |
| `Mentioned` reads the children of an inline body (#16) | in, a departure: the issue reads the spelling of an inline body | The frontends list the types of the members as children | A frontend that lowers an inline body without children |
| `Mentioned` follows the bounds, the defaults and the value types (#16) | in, a departure | A record that declares `V extends K` without `K` does not compile | none |
| `Reporter` beside `SubjectMatch` (#17) | in, a departure: the issue proposes one interface | The lint baseline caps an interface at 10 entries, and an embedded interface counts as one | A higher cap of the baseline |
| `Kernel` and `Export` on `SubjectMatch` (#17) | in, a departure | Every match with a subject has both | none |
| `typeargs` takes `any` (#18) | in | One helper writes parameters and references. text/template boxes the argument, one allocation | none |
| `typeargs` of Rust writes the type form (#18) | in | The template writes `::` where the path is an expression | none |
| `naming.New()` in TypeScript (#19) | in | | none |
| `naming.New()` in Java, Rust, protobuf and GraphQL (#19) | in, a departure: the issue names TypeScript | The same conversion breaks a mirrored Java class. The Rust API guidelines write `Uuid`, the protobuf style guide writes `GetDnsRequest`, and the guide of graphql.org writes `userId` | none |
| A GraphQL type `id` spells `Id` (#19) | in, a departure from RFC-0068 | `Id` is no built-in scalar, and a source that writes `ID` keeps `ID`, which the respell refuses | none |

### Failures

| Condition | Report |
|---|---|
| A construction in Go or Rust | `UnspeltValue` with the language and the type |
| A structural type without a spelling in Java or Rust | `UnspeltValue` |
| A target that is no name of a single directory, or a missing `templates/<t>`, in `TemplatesOf` | a panic of the typed build that names the plugin and the target |
| A list of another type in `typeargs` | a template refusal that names the type of the list |
| A carrier of the header of a script | `UnaddressedCarrier` |

### Migration

1. Add the construction to the value tree and to the scaffold, and implement it in the four
   targets. Pass a structural type without a spelling to the target.
2. Add the rules of the collections of TypeScript and of the empty struct of Go.
3. Add `ParamView.Optional`, `Mentioned`, `Reporter`, `SubjectMatch`, `OnPackage` and
   `TemplatesOf`, and regenerate the models and the facade.
4. Add `spellvalue` and `typeargs` to the four backends.
5. Add `naming.Params` and `golang.RenamePredeclared`. Move the case conversion of TypeScript, Java,
   Rust, protobuf and GraphQL to `naming.New()`, with the lowerings of protobuf and GraphQL and the
   TypeScript client of the GraphQL catalog.
6. Attach the header of a TypeScript module to its package. End a run of `Comments.Leading` at a
   taken comment.
7. Move eidos-reference to the helpers, update the goldens of its tree and remove `internal/tree`.

## Alternatives

### NameParams in the SDK

`Params` beside `Mirror` would need the kernel to import `naming` or to repeat `naming.Unique`.
The kernel does not import eidos-lang. A second rule for a free name would let two generators give
one parameter two different names.

### One SubjectMatch of thirteen entries

The interface of the issue lists the twelve methods and `Matcher`. The lint baseline refuses an
interface of more than ten entries. `Reporter` is the reporting half, and a function that only
reports takes it alone.

### Raw TypeScript text for a collection

The rules derive a `Date` as raw text, because its arguments are constants. The elements of a set
and the entries of a map are values of other types, and raw text cannot contain a value of the
model. The imports of those values would be lost.

### A set form in the shape vocabulary

Every set type of the languages in scope is a library type, and no language in scope has a set
literal. The construction derives the values of a set without a form of its own.

### The first comment alone as the header

The rule of the issue attaches the first comment of a module alone. A license header, a blank
line and a carrier below it then refuse the carrier, although the author addresses the module.

### A filter of taken comments in the TypeScript frontend

The frontend could drop taken comments from the run of its first import. The double read of an
anonymous default class would remain, and every other frontend of the walk would keep the same
hazard.

### The initialisms of Go in the other satellites

The conversion of Go names `getById` as `getByID`, so a class that a generator mirrors from a
Java interface does not implement the interface. The style guide of Java writes `XmlHttpRequest`,
and the Rust API guidelines write `Uuid`. The protobuf style guide treats an abbreviation as one
word and writes `GetDnsRequest` rather than `GetDNSRequest`. A GraphQL field is a key of the JSON
of a response, so a field `userId` that the conversion writes as `userID` breaks every client of
the schema.

## Drawbacks

- A generated TypeScript, Java, Rust, protobuf or GraphQL name that the conversion raised to an
  initialism changes its spelling: `httpRow` becomes `HttpRow` and no longer `HTTPRow`. Each such
  file changes in the next run.
- A comment directly above the first import of a module documents the module header and no
  longer the import.
- A plugin helper named `spellvalue` or `typeargs` now shadows a shared name, and its plugin
  declares an override.
- The projection passes over the fixtures of the TypeScript and Go rules allocate 520 and 212
  times, up from 462 and 203, because the collections of the fixtures derive values.

## Open questions

None.

## Unresolved and future work

None.

## References

- The issues: [#7](https://github.com/dokimasia/eidos/issues/7),
  [#8](https://github.com/dokimasia/eidos/issues/8),
  [#9](https://github.com/dokimasia/eidos/issues/9),
  [#10](https://github.com/dokimasia/eidos/issues/10),
  [#11](https://github.com/dokimasia/eidos/issues/11),
  [#12](https://github.com/dokimasia/eidos/issues/12),
  [#13](https://github.com/dokimasia/eidos/issues/13),
  [#14](https://github.com/dokimasia/eidos/issues/14),
  [#15](https://github.com/dokimasia/eidos/issues/15),
  [#16](https://github.com/dokimasia/eidos/issues/16),
  [#17](https://github.com/dokimasia/eidos/issues/17),
  [#18](https://github.com/dokimasia/eidos/issues/18) and
  [#19](https://github.com/dokimasia/eidos/issues/19).
- [RFC-0011](0011-neutral-emit-and-the-target-lowering-seams.md), the initialism set of each
  backend.
- [RFC-0023](0023-the-cross-language-hub.md), the samples of TypeScript. This design derives the
  values of a `Map`, which the table of `SamplesOf` refuses.
- [RFC-0048](0048-unique-names.md), the rule of a free name.
- [RFC-0054](0054-typescript-declarations-that-the-model-loses.md), the zero of a `Map`, which
  this design derives.
- [RFC-0067](0067-the-protobuf-backend.md), the names of the protobuf backend.
- [RFC-0068](0068-the-graphql-satellite.md), the names of the GraphQL backend and the refusal of a
  built-in scalar.
- [21-decisions.md](../architecture/21-decisions.md), D128 on restated references and D147 on the
  children of an inline body.
- Google TypeScript Style Guide, camel case:
  <https://google.github.io/styleguide/tsguide.html#identifiers>
- Google Java Style Guide, camel case: <https://google.github.io/styleguide/javaguide.html#s5.3-camel-case>
- Rust API Guidelines, C-CASE: <https://rust-lang.github.io/api-guidelines/naming.html>
- Protocol Buffers style guide: <https://protobuf.dev/programming-guides/style/>
- GraphQL, naming conventions and design standards: <https://graphql.org/learn/naming-design/>
- Go Code Review Comments, initialisms: <https://go.dev/wiki/CodeReviewComments#initialisms>
- interfacebloat: <https://github.com/sashamelentyev/interfacebloat>
