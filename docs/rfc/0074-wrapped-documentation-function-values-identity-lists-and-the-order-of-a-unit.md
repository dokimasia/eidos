---
rfc: 0074
title: Wrapped documentation, function values, identity lists and the order of a unit
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Draft
created: 2026-10-20
updated: 2026-10-20
discussion: dokimasia/eidos#21, dokimasia/eidos#24, dokimasia/eidos#29, dokimasia/eidos#33 and dokimasia/eidos#34
supersedes: none
superseded-by: none
produces-adr: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0074: Wrapped documentation, function values, identity lists and the order of a unit

## Summary

The vocabulary, the value tree and the fact store lack these contracts:

- A template cannot wrap documentation that it composes from derived names (#21).
- The Go rules derive no value of a function type, a channel type or the empty interface (#24).
- A fact contains one identity at most (#29).
- A unit lists its declarations in the order of the identities of their origins (#33).
- A template has no helper that writes a value of the value tree (#34).

We propose these changes:

- The shared helper `wrapdocs`, which wraps text at 72 columns for the `docs` and `memberdocs`
  helpers of the six backends.
- The value form of a function, which the targets of Go, TypeScript, Java and Rust write, and the
  value form of `make`, which the Go target writes.
- The samples of a function type, a channel type and the empty interface in the Go rules.
- `[]symbol.Identity` in the fact vocabulary, in the persisted state and in the `values` of the
  `meta` directive.
- The order of the declarations of a unit by the positions of their origins in the source.
- The builtin `value`, which writes a value through the scaffold target of the backend.

The table of verdicts lists each departure from the proposal of an issue.

## Motivation

| Issue | The problem |
|---|---|
| #21 | The `docs` helper of each backend writes every line as it receives it. A template composes a sentence from derived names, such as the name of a builder and of its constructors, so the author of the template cannot wrap it. A sentence about `ConfigurationRegistryBuilder` writes one comment line of 233 columns |
| #24 | The Go rules refuse a sample of `func(context.Context) error`, of `chan string` and of `any`. A generator that checks a setter with two samples checks no field of a struct of hooks or of streams. The value tree has no form for a function literal or for `make` |
| #29 | `meta.FactValue` is `string \| int64 \| bool \| []string \| symbol.Identity`. An annotator whose directive lists two declarations, such as the error values that a double of a method can return, stamps one identity and drops the rest, or stamps the spellings and resolves them again in every reader |
| #33 | The flush sorts the placements of a unit by the identity of their origins. A file of builders lists the builder of `MixedConfig` before the builder of `TagConfig`, where the source declares `TagConfig` first |
| #34 | The helpers of the backends take types, parameters, declarations and documentation. A template that needs a value inside an expression, such as a field of a composite literal or the initializer of a variable, places a body that assigns the value to a local instead |

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/emit` | The value kinds `ValueFunc` and `ValueMake`, the constructors `Func` and `Make`, and `Value.Fresh` |
| `eidos-core/rules/rulestest` | The check of distinct samples passes a pair of fresh values |
| `eidos-core/meta` | `[]symbol.Identity` in `FactValue`, with its copy and its comparison |
| `eidos-core/internal/state` | The value tag of a list of identities, and `Format` 5 |
| `eidos-core/directive` | The `values` of a set of a key of `[]symbol.Identity`, each element resolved as a type in scope |
| `eidos-core/workspace` | A set of such a key stamps the list of the resolved identities |
| `eidos-core`, root | The flush orders the placements of a unit by the positions of their origins |
| `eidos-core/plugin` | The documented order of `Unit.Decls` |
| `eidos-core/backend/render` | The shared helper name `wrapdocs`, the builtin `value` and `Language.Value` |
| `eidos-core/backend` | `Builder.Value`, which sets the writer of values of a kit backend |
| `eidos-sdk` | The facade of each addition, which the facade generator writes |
| `eidos-lang/textfmt` | `Wrap`, `Markup` and `DocsWidth` |
| `eidos-lang/scaffold` | `Target.Func`, `Target.Make` and the walk of the two forms |
| `eidos-lang-go` | `wrapdocs`, the two forms in the target, the samples, and `backend.Value` |
| `eidos-lang-typescript`, `eidos-lang-java`, `eidos-lang-rust` | `wrapdocs`, the function in the target, the refusal of `make`, and `backend.Value` |
| `eidos-lang-protobuf`, `eidos-lang-graphql` | `wrapdocs` |
| `eidos-conformance`, `eidos-reference`, the backends | The golden files of every unit whose order changes |

### Invariants

1. A wrapped line has at most 72 columns of text, except a line of one word.
2. A wrapped line never starts with a word that the documentation syntax of the target reads as
   the start of a block.
3. A value form has one meaning in every target. A target without a syntax for a form refuses it
   under `UnspeltValue`.
4. A fact value is a term of the closed vocabulary. The persisted state encodes every term, and a
   state of another format runs cold.
5. The order of a unit is a function of its placements and of the positions of their origins. It
   never depends on the schedule.
6. `value` and `body` write a value through one target, so a value has one spelling in a file.

### Wrapped documentation

`textfmt.Wrap` splits text into paragraphs at its blank lines and wraps each paragraph greedily:

```go
// DocsWidth is the width in columns at which the backends wrap
// documentation.
const DocsWidth = 72

// Markup selects the words that a documentation syntax reads as the start
// of a block where they open a line. The values combine with |.
type Markup uint8

const (
    MarkupNone     Markup = 0 // no word starts a block
    MarkupTags     Markup = 1 // the block tags of TSDoc and Javadoc
    MarkupMarkdown Markup = 2 // the block openers of CommonMark
)

// Wrap returns the lines of text wrapped at width columns. A blank line of
// text separates two paragraphs, and an empty line separates them in the
// result.
func Wrap(text string, width int, m Markup) []string
```

The rules of the wrap:

- A run of spaces, tabs and line breaks inside a paragraph is one space.
- A line takes each next word that keeps it at the width or below. The width counts runes, and
  the comment marker and the indent are not part of it.
- A word longer than the width is a line of its own. The wrap never breaks a word, so a link such
  as `[ConfigurationRegistryBuilder.Build]` is never split.
- A word that the markup of the target reads as the start of a block never opens a line after the
  first line of its paragraph. The wrap keeps it on the line before it, past the width where it
  must.
- Only the bytes of ASCII white space separate two words, so a no-break space is part of its word.
- A width below one wraps nothing, so each paragraph is one line.

| Markup | The words that open no line |
|---|---|
| `MarkupTags` | a word that starts with `@` |
| `MarkupMarkdown` | `#` to `######`, a word that starts with `>`, `+` and `*` alone, an ordinal of up to nine digits with `.` or `)`, a run of `-` or `=`, a run of three or more `*` or `_`, a word that starts with three backticks or three tildes, and a word that starts with `<` |

Each backend binds `wrapdocs` to `textfmt.Wrap` at `textfmt.DocsWidth` and the markup of its
target. The helper returns lines, so a template passes them to the comment helper of its position:

```text
{{docs (wrapdocs (printf "%sBuilder constructs [%s] values. ..." .Name .Name))}}
{{memberdocs (wrapdocs .Data.Summary)}}
```

| Target | Comment | Markup |
|---|---|---|
| Go | `//` and a space on each line, `//` between paragraphs | none, because a Go doc comment reads an unindented line as text |
| TypeScript | a `/** … */` block with ` * ` on each line | tags and Markdown, because a TSDoc block tag is the first element of its line, and the text of a tag may contain Markdown |
| Java | a `/** … */` block with ` * ` on each line | tags, because a Javadoc block tag appears at the beginning of a line |
| Rust | `///` and a space on each line | Markdown, because rustdoc uses the CommonMark specification |
| protobuf | `//` and a space on each line | none |
| GraphQL | a block string | Markdown, because the specification defines a description in the Markdown of CommonMark |

The sentence of #21 wraps in Go as:

```go
// ConfigurationRegistryBuilder constructs [ConfigurationRegistry] values.
// Start with [NewConfigurationRegistryBuilder] or
// [NewConfigurationRegistryBuilderFrom], set what you care about, and call
// [ConfigurationRegistryBuilder.Build].
```

### Function and channel values

The value tree gains a form for a function and a form for `make`:

```go
// ValueFunc is a function of Type that returns Args: a function literal
// that a check compares by its code pointer.
ValueFunc

// ValueMake is the empty value of Type that make returns: a channel that a
// check compares by its identity.
ValueMake

// Func returns a function of the function type t that returns the values
// results, in the order of the results of t.
func Func(t *TypeRef, results ...Value) Value

// Make returns the empty value of the type t that make returns.
func Make(t *TypeRef) Value
```

The shared walk passes each form to the target:

```go
// Func spells a function of a function type that returns the spellings of
// results, and records the imports of the signature.
Func(ref *emit.TypeRef, results []string) (string, error)

// Make spells the empty value of a type that make returns, and records the
// import of the type.
Make(ref *emit.TypeRef, typ string) (string, error)
```

| Target | `Func` | `Make` |
|---|---|---|
| Go | `func(context.Context) error { return nil }`, and `func(string) {}` for a type without results | `make(chan string)`, and `make(<-chan string)` for a receive-only channel |
| TypeScript | `() => 0`, `() => {}` without results, and `() => [0, 'a']` for two results | refused |
| Java | `(unused0) -> 0L`, and `() -> {}` without parameters or results | refused |
| Rust | `\|_\| 0`, `\|_, _\| {}` without results, and `\|\| (0, 1)` for two results | refused |

The Go target spells the signature of a function through its speller, so the file imports each
type of the signature. The other targets do not write a type, and a function there does not import
anything:

- The TypeScript target writes an arrow function without parameters, because TypeScript assigns a
  function of fewer parameters to a function type. TypeScript reads a brace after the arrow as a
  block, so the target puts an object literal that the function returns in parentheses.
- The Java target names each parameter of a lambda `unused` with its position, because Java
  names an unused parameter `_` only from version 22.
- The Rust target writes a wildcard pattern for each parameter of a closure. A closure without
  captures coerces to a function pointer.

A function of two or more results returns a tuple in TypeScript and Rust, as the spokes write the
function type of such a function. `Target` has ten methods with the two forms, which is the cap of
the lint baseline.

The Go rules derive these pairs:

| Type | Sample and alternate |
|---|---|
| a function type | two functions of the type that return the zero value of each result |
| a channel type | two values of `make` of the type |
| `any`, and `interface{}` without methods | the pair of a string from the hint |

A function literal is a new function value wherever it is written, and each `make` is a new
channel, so the two halves of each pair differ. A function type with a result without a zero value
refuses the pair with `RefusedNoLiteral`.

`Value.Fresh` reports a function and a make, also under conversions. The check of distinct samples
of the rules kit passes a pair of fresh values, because their trees are equal and their values are
not. A part of a composite, such as an element of a slice or a field of a struct, derives no fresh
value, because two composites would then differ in the identity of a new value alone. A composite
with a function inside equals no composite under `reflect.DeepEqual`, not even itself. A struct
whose first field is a function sets its next field, as it does for a field without a literal.

### Lists of identities

`FactValue` gains `[]symbol.Identity`:

```go
type FactValue interface {
    string | int64 | bool | []string | symbol.Identity | []symbol.Identity
}
```

The fact store copies a list of identities at its boundary. It compares two such lists element by
element, as it compares two lists of strings. The persisted state encodes the list behind a value
tag of its own. `Format` rises to 5, and a generation of format 4 runs cold.

The `meta` directive sets a key of `[]symbol.Identity` through `values`, the param that a key of
`[]string` takes. Each element resolves as a type in scope. The `value` of a key of
`symbol.Identity` resolves the same way:

```text
+acme:meta set=acme.fault.errors values=[NotFound, Gone]
```

A plugin directive with a param of a list of references resolves each element through the
resolution of the param. An annotator then stamps the error values of #29 as one fact.

### The order of a unit

The flush orders the placements of a unit by the position of the declaration of each origin in
the graph: the file, then the line, then the column. Placements of one position sort by the
identity of the origin, then by the instance under a repeatable directive, then by the sequence.
An origin without a position, such as a declaration that a plugin creates, sorts after every
origin with a position.

A unit of a package lists the declarations of its files in the order of their paths, and the
declarations of each file in the order of its source. The flush reads each position through the
index of its phase call. A move of a declaration changes its position and its digest, and the
invocation that placed it runs again. So a warm run orders a unit as a cold run does.

The decision log orders an accumulator by the identities of its subjects, so that the order of a
unit never depends on the schedule. The order of positions replaces it and keeps that property.
The provenance of a unit, `Unit.Origins`, remains sorted by identity, so a manifest records the
same sources in the same order.

| Unit | Before | After |
|---|---|---|
| the builders of `Config`, `TagConfig` and `MixedConfig` in one file | `ConfigBuilder`, `MixedConfigBuilder`, `TagConfigBuilder` | `ConfigBuilder`, `TagConfigBuilder`, `MixedConfigBuilder` |
| the records of `Zebra` and `Apple` of #33 | `AppleGen`, `ZebraGen` | `ZebraGen`, `AppleGen` |

### The value builtin

The language of a backend gains a writer of values:

```go
// Value spells one value of the value tree the language's way, and records
// into set the imports of the references and the callees of the value. The
// value builtin calls it. With a nil Value, the builtin refuses every call.
Value func(v emit.Value, set *ImportSet) (string, error)
```

The builtin `value` writes its argument through `Language.Value` with the import set of the file
under render. A kind template, a body template and a file template resolve it. In a language
without a writer, the call fails with an execute-time error that contains the name of the
language. The backends of Go, TypeScript, Java and Rust write a value through the scaffold target
of the file. Their `Scaffold` writes the statements of a body through the same target:

```text
var {{.Decl.Name}} = {{value .Data.Value}}
```

### Allocations

| Call | Allocations |
|---|---|
| `textfmt.Wrap` of one paragraph of four lines | 1, the list of lines, where one space separates each two words, because each line is a part of the text. Each line that joins words that other spaces separate allocates once more |
| `New` of a backend | the function maps that text/template keeps for each template grow with the two new names: 16 more in Java, 24 in protobuf and 12 in GraphQL, and none in Go, TypeScript and Rust |
| `emit.Func` with one result, and `emit.Make` | 1 and 0 |
| `emit.Value.Fresh` | 0 |
| `scaffold.Value` of a function with one result, and of a make | 1 and 0, beside what the target allocates |
| The Go pair of `func(string) error` | 5: the reference to the type with its list of children and its two children, and the list of the zero values that both halves share |
| The Go pair of `chan int`, and of `any` | 3 for the reference to the channel with its child, and 2 for the hinted texts |
| A stamp of a list of identities | a copy of the list, beside what a stamp of `[]string` allocates |
| The flush of a unit | one lookup of a position for each append call, and the sort of the origins, which allocate nothing |
| `backend.Value` of an integer literal | 0 in Go, TypeScript, Java and Rust |
| `backend.Value` of a call of another package whose import the file has | 2 in Go, TypeScript and Java, and 1 in Rust |
| `backend.Value` of a function with one result | 2 in TypeScript, Java and Rust: the list of the spelled results and the text of the function |
| One call of the builtin `value` in a template | what the writer allocates, and 4 for the reflective call of text/template: the arguments, the results and a holder of each of the two results |

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| `wrapdocs` returns lines for `docs` and `memberdocs` (#21) | in, a departure: the issue's helper writes the comment | One wrap composes with the comment helper of each position, members included, and the comment form stays in one helper of each target | A position whose comment form the wrap must know |
| The width 72 in every target (#21) | in, a departure: the issue states Go and TypeScript | One width gives every generated comment one measure. 72 columns of text, a marker and one level of indent keep a line below 80 columns | A target whose style guide sets a limit below 80 columns |
| The width in runes (#21) | in | A column of a wide character counts as one, and the documentation of the generated languages is mostly ASCII | A generator whose documentation is in a script of wide characters |
| A word that opens markup never opens a line (#21) | in, a departure | A wrapped `@param` or `- item` at the start of a line would turn text into a tag or a list | none |
| The function and `make` forms in the value tree (#24) | in | A function literal and a channel need the imports of their types, which raw text cannot record | none |
| A function returns the zero value of each result (#24) | in, a departure: the issue declares a local of each result | `func(context.Context) error { return nil }` is one line that gofmt keeps | A result type without a zero value that a generator needs |
| `make` of the type of the reference (#24) | in, a departure: the issue makes the bidirectional channel | Go makes a channel of every direction, and the value then has the type of the field without a conversion | A check that sends on a receive-only channel |
| TypeScript, Java and Rust write a function (#24) | in | Each language writes a function literal: an arrow function, a lambda and a closure | none |
| The Java target names the parameters of a lambda `unused0`, `unused1` and so on (#24) | in | Java names an unused parameter `_` only from version 22. The Java backend names an unnamed parameter of a method `arg0`, so a lambda in the body of that method takes another prefix | A Java version of 22 or later in every generated project |
| TypeScript, Java and Rust refuse a make (#24) | in | `make` is the built-in of Go. Another language makes a stream through a call, such as `Stream.empty()` in Java, which the value tree writes as a call | A language with a built-in that makes a channel |
| A part of a composite derives no fresh value (#24) | in | Two composites would differ in the identity of a new value alone, and a composite with a function inside equals no composite under `reflect.DeepEqual` | A check that compares composites by the identity of their parts |
| `[]symbol.Identity` in the fact vocabulary (#29) | in | | none |
| `values` of a list of identities resolves each element as a type in scope (#29) | in | It is the resolution of the `value` of a key of an identity | A key whose elements are values rather than types |
| `Format` 5 (#29) | in | A generation of format 4 cannot contain the new tag, and a generation of format 5 must not reach a binary of format 4 | none |
| The order by the positions of the origins (#33) | in | A generated file follows the order of its source, as a file that a person writes does | none |
| An origin without a position sorts last (#33) | in | A declaration that a plugin creates has no place in the source | none |
| The builtin `value` and `Language.Value` (#34) | in | A value has one spelling in a file, whether a body or a template writes it | none |
| `value` refuses in protobuf and GraphQL (#34) | in | Neither language writes bodies, and their backends have no scaffold target | A protobuf or GraphQL template that writes a value |

### Failures

| Condition | Report |
|---|---|
| A `make` in TypeScript, Java or Rust | `UnspeltValue` with the language and the form |
| A function whose type is not a function type | `UnspeltValue` in every target. Go converts the literal of a signature to a defined function type, and the reference of a named functional interface of Java states no parameters |
| A function of a constructor type in TypeScript | `UnspeltValue`, because an arrow function constructs nothing |
| A function of two or more results in Java | `UnspeltValue`, because a lambda returns one value |
| A function type whose result has no zero value | the pair refuses with `RefusedNoLiteral` |
| A list of identities whose element does not resolve | `UnresolvedReference` at the instance |
| The builtin `value` in a language without a writer | `RefusedTemplate`, at the file and the line of the template |
| A state of format 4 | the run goes cold |

### Migration

1. Add `textfmt.Wrap`, the shared name `wrapdocs` and the helper of the six backends.
2. Add the two value forms, the walk and the targets, and the samples of the Go rules.
3. Add the list of identities to the fact vocabulary, the state, the `meta` directive and the
   workspace. Raise `Format`.
4. Order the flush by positions, and update the golden files of every tree whose order changes.
5. Add `Language.Value` and the builtin `value`, and set the writer of the four backends with
   values.
6. Regenerate the facade.

## Alternatives

### A wrap that writes the comment

The helper of the issue writes the comment of its target. Each backend would then need one wrapping
helper for each comment helper, such as one for a member in TypeScript, Java and Rust. The lines of
the wrap pass to the comment helper that the template already calls.

### The display width of a character

`golang.org/x/text/width` would count a wide character as two columns. It adds a dependency to
eidos-lang for text that the generated languages rarely contain.

### A local for each result

A function that declares `var r0 error` and returns it works for every result type, a type
parameter included. Its body takes a statement for each result, and gofmt writes each statement on
a line of its own.

### A per-backend helper for a value

Each backend could register a helper named `value`. A builtin has one name in every target, and the
template lint reads it from the builtins of the pass.

### An option for the order of a unit

A generator could choose between the order of the identities and the order of the source. Two
orders of one unit would make the output of a plugin depend on a setting that its consumers do not
see.

### The spellings of a list of declarations

An annotator could stamp the spellings of the declarations as `[]string`. Each reader would then
resolve each spelling again in the scope of the subject, and the fact would name declarations by
text, which the fact vocabulary refuses.

## Drawbacks

- Every unit whose origins are in another order than their identities renders in a new order. The
  golden files of the conformance, the reference plugins and the backends change once, and so does
  the output of every consumer.
- `Format` 5 sends every sealed state of format 4 cold once.
- `scaffold.Target` gains two methods, so an implementation outside the modules of eidos adds them.
- A word that opens markup can make a wrapped line longer than 72 columns.

## Open questions

None.

## Unresolved and future work

None.

## References

- The issues: [#21](https://github.com/dokimasia/eidos/issues/21),
  [#24](https://github.com/dokimasia/eidos/issues/24),
  [#29](https://github.com/dokimasia/eidos/issues/29),
  [#33](https://github.com/dokimasia/eidos/issues/33) and
  [#34](https://github.com/dokimasia/eidos/issues/34).
- [RFC-0069](0069-constructions-module-directives-and-the-shared-helpers-of-plugins.md), the
  construction value and the shared helpers.
- [04-metadata.md](../architecture/04-metadata.md), the closed vocabulary of facts.
- [07-rendering.md](../architecture/07-rendering.md), the funcmap and the scaffolding vocabulary.
- [21-decisions.md](../architecture/21-decisions.md), D46: the cardinality of outputs and the
  order of an accumulator.
- Go doc comments: <https://go.dev/doc/comment>
- TSDoc, block tags: <https://tsdoc.org/pages/spec/tag_kinds/>
- Javadoc, block tags: <https://docs.oracle.com/en/java/javase/21/docs/specs/javadoc/doc-comment-spec.html>
- CommonMark, the blocks that interrupt a paragraph: <https://spec.commonmark.org/0.31.2/>
- rustdoc, how to write documentation: <https://doc.rust-lang.org/rustdoc/how-to-write-documentation.html>
- The GraphQL specification, descriptions: <https://spec.graphql.org/October2021/#sec-Descriptions>
- The Go specification, making slices, maps and channels: <https://go.dev/ref/spec#Making_slices_maps_and_channels>
- TypeScript, comparing two functions: <https://www.typescriptlang.org/docs/handbook/type-compatibility.html>
- MDN, the object literal of an arrow function: <https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Functions/Arrow_functions>
- JEP 456, unnamed variables and patterns: <https://openjdk.org/jeps/456>
- The Rust reference, the coercion of a closure without captures: <https://doc.rust-lang.org/reference/type-coercions.html>
