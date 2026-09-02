---
rfc: 0014
title: The projection vocabulary and the rules seam
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Draft
created: 2026-09-02
updated: 2026-09-02
discussion: none
supersedes: none
superseded-by: none
produces-adr: tbd
---

# RFC-0014: The projection vocabulary and the rules seam

## Summary

A generator that doubles an interface, builds a value or writes a
check asks the same five questions of every declaration, and each
question has a different answer per language: what a callable's
signature means, what shape a type has, what a name in a directive
refers to, which members a type effectively holds, and what a value
of a type looks like. This RFC gives those questions one home. The
kernel package `core/rules` declares the neutral vocabulary and the
`SourceRules` contract every language returns; a satellite's
`rules/` package returns it for one language; the workspace holds
the returned set per language; and every match hands a handler a
bound `Rules()` that reads through the invocation's tracked reader.
Beside the contract sit the two kernel directives that let an
author state a value or a witness the projection reads before it
derives anything, and a conformance suite that holds any
implementation to the same properties.

One model change is proposed with it: a type reference gains a
structured form beside its verbatim spelling, because a projection
that has to re-parse a spelling after resolution cannot recover the
identities the resolution step assigned inside it.

## Motivation

Three pressures decide where these questions live.

- **Two consumers, one declaration.** A builder generator and a
  double generator both ask what a field's type is and what a value
  of it looks like. Each deciding privately is how two plugins come
  to disagree about one declaration, and a plugin that decides in
  its own helper binds neutral code to the one language its author
  happened to test. The placement rule is fixed: a question two
  neutral consumers ask goes in the contract, a question one
  language binding asks stays in that language's own package.
- **The read side stops at declarations.** The frontend parses,
  the resolution step assigns identities and targets, and the
  graph seals. Nothing in the kernel says what a Go `(T, error)`
  return means, which members a struct holds through its embeds,
  or that a `*T` field is optional. Those are language rules, and
  the kernel knows no language. The shape catalog's detectors, the
  cross-language lowering and every check-emitting generator wait
  on one contract that returns them.
- **Silence is the defect.** A projection that hands back a zero
  value where it could not reason lets a generator write a check
  that passes against code doing nothing. Every return in this
  vocabulary refuses visibly, with a reason a consumer can report,
  and the conformance suite holds implementations to it.

The four backends settled the write side by building the kit
before the second consumer arrived. The read side's projections
take the same route: the contract and its suite first, the Go
implementation against them, and the next languages against the
same suite.

## Detailed design

### The package and its position

`core/rules` holds the vocabulary and the contract. It imports
`core/node`, `core/symbol`, `core/store` and `core/meta`, and
nothing imports it back except the authoring root, the workspace
and the satellites' `rules/` packages. The generated facade
re-exports it, so a plugin imports `sdk/rules`.

A language's own implementation lives in the satellite, beside
its frontend: `eidos-lang-go/rules` returns the contract for Go and
nothing else. The kernel never imports a satellite; the satellite
imports the facade.

### Source and target identity

Two spellings of language identity already exist and stay
separate: `symbol.Lang` names the language a declaration was
written in, and `plugin.Target` names the language a plan renders
into. A `SourceRules` value declares the `symbol.Lang` it returns
for. The two identities share one namespace of spellings, so a
frontend's `Lang()` and a rules value's `Lang()` agree by
construction, and a plan's target never reaches a source rules
lookup because the types differ.

### The contract

```go
package rules

// SourceRules is what every language returns. Implementing it is
// what makes a language a language on the read side.
type SourceRules interface {
    // Lang names the language the rules return for.
    Lang() symbol.Lang

    // CallableOf projects a function or a method. It reports false
    // for any other kind.
    CallableOf(sym symbol.Symbol, r Reader) (Callable, bool)

    // TypeOf projects a type reference into the canonical shape.
    // It is total: a reference the language cannot reason about
    // projects as Opaque, carrying its spelling.
    TypeOf(ref *node.TypeRef, r Reader) TypeShape

    // Resolve says what a human's spelling names in a scope, at a
    // declared resolution kind. It returns the declaration or an
    // error naming what it looked for.
    Resolve(scope Scope, name string, kind ResolutionKind, r Reader) (symbol.Symbol, error)

    // MembersOf returns a type's effective member set: its own
    // members and what arrives through its embeds and supertypes,
    // with per-member provenance and a stated gap for every
    // contributor that yielded nothing. It reports false for a
    // symbol that is not a type.
    MembersOf(sym symbol.Symbol, r Reader) (MemberSet, bool)

    // SamplesOf returns two distinct values of a type, authored
    // values first, or two refusals carrying the reason.
    SamplesOf(ref *node.TypeRef, hint string, r Reader) (sample, alternate Sample)

    // ZeroLiteral spells the type's zero value, and reports false
    // where the language has no spelling for it.
    ZeroLiteral(ref *node.TypeRef, r Reader) (string, bool)

    // LiteralFor renders text as a literal of a type, and reports
    // false where the text cannot be one.
    LiteralFor(f *node.File, ref *node.TypeRef, text string, r Reader) (string, bool)

    // TypeName joins a generator's word onto an author's base name
    // the way the language spells a derived type name.
    TypeName(word, base string) string
}
```

Every method takes the reader it reads through, so a projection
never holds state across invocations and every read it makes is
recorded on the invocation that asked. Two calls with one reader
over one graph return equal values; the conformance suite holds
implementations to that.

### The reader

A projection reads declarations and facts, and both reads record.

```go
// Reader is what a projection reads through.
type Reader struct {
    // Decls is the invocation's tracked declaration reader.
    Decls *store.Reader
    // Facts is the run's arbitrated fact store, and Reads is where
    // a fact read records: the same read set Decls records into.
    Facts *meta.Facts
    Reads meta.Recorder
}

// Fact returns a subject's winning value for a key, recorded.
func Fact[T meta.FactValue](r Reader, id symbol.Identity, k meta.Key[T]) (T, bool)
```

The reader is a value the workspace mints per invocation. A
projection called with a zero reader refuses with `RefusedNoReader`
rather than reading an untracked graph, because an unrecorded read
is an invalidation edge the engine cannot see.

### Reaching the rules

The composition registers one `SourceRules` per language:

```go
workspace.New().
    Rules(gorules.New(), protorules.New()).
    // ...
```

Build refuses two values for one language and a value declaring
the zero language. Nothing at Build says which languages the graph
holds, because the graph arrives at Run; a handler asking for a
language the composition registered nothing for receives
`rules.Absent(lang)`, an implementation whose every method refuses
with `RefusedNoRules`, and the run records one Warning naming the
language and the plugin that asked. The refusal is a value, so a
generator's `OK()` gate holds without a nil check, and the Warning
is what keeps the omission from passing as a language with no
members.

Every match binds the rules to the invocation:

```go
// On every *Match:
Rules() rules.Bound                    // the subject's language
RulesFor(lang symbol.Lang) rules.Bound // another language's

// Bound is SourceRules with the invocation's reader captured.
type Bound struct { /* unexported */ }
func (b Bound) CallableOf(sym symbol.Symbol) (Callable, bool)
func (b Bound) TypeOf(ref *node.TypeRef) TypeShape
func (b Bound) MembersOf(sym symbol.Symbol) (MemberSet, bool)
func (b Bound) SamplesOf(ref *node.TypeRef, hint string) (Sample, Sample)
func (b Bound) ZeroLiteral(ref *node.TypeRef) (string, bool)
func (b Bound) LiteralFor(f *node.File, ref *node.TypeRef, text string) (string, bool)
func (b Bound) TypeName(word, base string) string
func (b Bound) Source() SourceRules   // for the optional-capability assertions
func (b Bound) Reader() Reader
```

A graph match has no subject, so `Rules()` on it returns the
`Absent` value for the zero language and `RulesFor` is the door.

### Callable

```go
type Callable struct {
    Receiver *ParamView   // nil for a free function
    Params   []ParamView
    Returns  []ReturnView
    Errors   ErrorModel   // None | LastReturn | ResultType | Thrown | Raised
    Async    bool
}

type ParamView struct {
    Name     string
    Ref      *node.TypeRef
    Type     TypeShape
    Role     ParamRole     // Context | Input
    Variadic bool
}

type ReturnView struct {
    Name string
    Ref  *node.TypeRef
    Type TypeShape
    Role ReturnRole        // Value | OkBool | Stream | Error
}
```

`Errors` says how the callable reports failure, and the `Error`
return role marks which slot carries it where the model is
`LastReturn` or `ResultType`, so a generator binding a call knows
where the failure arrives without re-deriving the language's
convention. `Async` covers a result that arrives asynchronously in
the language's own form; a stream return carries its own async
flag inside its shape. Go returns `Sync` always, because its
concurrency is caller-side and never appears in a signature.

The Go rules return: a first parameter of type `context.Context`
as `Context`; a trailing variadic parameter with `Variadic`; a
last return of type `error` as `LastReturn` with the `Error` role;
a two-value return whose second is `bool` as `OkBool` on the
second; a return of `iter.Seq` or `iter.Seq2` as `Stream`.

### TypeShape

The canonical type vocabulary is closed, so adding a shape is a
kernel change:

```go
type ShapeKind uint8

const (
    ShapeOpaque ShapeKind = iota // representable, not projectable; carries the spelling
    ShapeScalar                  // Class and Bits
    ShapeBool
    ShapeText
    ShapeBytes
    ShapeList                    // Elems[0]; FixedLen when the length is part of the type
    ShapeMap                     // Elems[0] key, Elems[1] value
    ShapeTuple                   // Elems
    ShapeOptional                // Elems[0]
    ShapeUnion                   // Elems; untagged
    ShapeSum                     // Ref names the Sum declaration
    ShapeFunc                    // Callable
    ShapeStream                  // Elems[0]; Async
    ShapeReference               // Ref, Args
)

type TypeShape struct {
    Kind     ShapeKind
    Spelling string          // the source spelling, on every kind
    Class    ScalarClass     // Int | Uint | Float, when Scalar
    Bits     int             // 0 for the platform width
    FixedLen int             // List with a fixed length; 0 otherwise
    Async    bool            // Stream
    Elems    []TypeShape
    Callable *Callable       // Func
    Ref      symbol.Identity // Reference and Sum
    Args     []TypeShape     // Reference type arguments
}
```

There is no Set, because every set in the languages in scope is a
library type and projects as a Reference. A fixed-length array is
a List carrying `FixedLen`. Union and Sum stay separate: a Sum is
a declaration the reference names, and a Union is a shape the
spelling states.

Well-known types are not shapes. A small kernel registry holds
blessed Reference identities, `timestamp` and `duration` and no
others, and a frontend's rules map their spellings in: `time.Time`
projects as the `timestamp` Reference. A semantic type nobody
mapped stays an ordinary Reference or an Opaque spelling.

`TypeOf` is total and never panics. The Go rules project builtins
by name, `*T` as Optional, `[]byte` as Bytes, `[]T` and `[N]T` as
List, `map[K]V` as Map, `chan T` as a synchronous Stream, `func`
as Func with its Callable, `any` and `error` as Opaque, an inline
struct or interface body as Opaque, and every named type as a
Reference carrying its target, or as Opaque with its spelling when
the resolution step assigned none.

### The model change TypeOf needs

A type reference carries its spelling verbatim, its resolved
target, and the arguments of an explicit instantiation. The
resolution step resolves the one named type a decorated spelling
names, `[]api.User` to `User`, and stops at shapes no single
declaration owns: a map's key and value, a function type's
parameters, a channel's element. Their named types stay spellings.

A projection cannot recover those identities. The import scope
the resolution step resolved through is the load's, not the sealed
graph's, so `TypeOf(map[string]api.User)` reads a spelling whose
value type it can name and cannot identify. Re-parsing the
spelling gives the form and loses the target.

The proposal is that the frontend states the structure it parsed,
and the reference carries it beside the spelling:

```go
type TypeRef struct {
    ID       symbol.Identity `eidos:"node"`
    Pos      position.Pos    `eidos:"node"`
    Spelling string          `eidos:"both"`      // source text, verbatim
    Target   symbol.Identity `eidos:"both"`      // zero until resolution, and for builtins and externals
    Form     symbol.TypeForm `eidos:"both"`      // Named | Pointer | Slice | Array | Map | Chan | Func | Tuple | Union | Inline
    Elems    []*TypeRef      `eidos:"both,walk"` // the form's children, in the form's fixed order
    Args     []*TypeRef      `eidos:"both,walk"` // the type arguments of an instantiation
}
```

`Form` says what the spelling is, and `Elems` holds the child
references in an order fixed per form: one child for a pointer, a
slice, an array and a channel; the key then the value for a map;
the parameters then the returns for a function type, split by the
count the language records in its own metadata; the members for a
tuple or a union; none for an inline body. `Spelling` stays
verbatim, so a backend spelling a type from the emit graph reads
what it read before, and a frontend that decomposes nothing leaves
`Form` as `Named` with no children, which is the whole surface
today.

The resolution step walks `Elems` the way it walks `Args`, so a
map's value type gains its target. `TypeOf` then reads `Form`,
`Elems` and `Target`, and parses nothing.

The alternative, resolving nested spellings on demand from inside
the projection, is refused below, because the scope it needs is
gone after the load.

### Resolve

Directive reference parameters carry a human's spelling. The
freeze's validation binds each one through `Resolve` before any
handler runs, so "this names a sibling callable" is schema rather
than prose.

```go
type ResolutionKind uint8

const (
    ResolveCallableInScope ResolutionKind = iota + 1
    ResolvePackageVar
    ResolveValueField
    ResolveHostParam
    ResolveMemberOnHandle
)

// Scope is where a spelling is read: the subject that carried it,
// and the file whose imports qualify it.
type Scope struct {
    Subject symbol.Identity
    File    *node.File
}
```

`Validate` gains a resolver the workspace supplies from its rules
set, and a reference that resolves to nothing is a positioned
Error under the directive's own code, naming the spelling and the
kind it was read at. The Go rules resolve a bare name in the
subject's package, a qualified name through the file's recorded
imports, a value field through `MembersOf` on the subject's type,
a host parameter on the subject's own signature, and a member on
a handle through the handle's resolved type.

### MemberSet

```go
type MemberSet struct {
    Members []Member
    Gaps    []Gap
}

// Member is one effective member and where it came from.
type Member struct {
    Symbol  symbol.Symbol     // *node.Field or *node.Method
    Owner   symbol.Identity   // the declaration that declared it
    Through []symbol.Identity // the contributors traversed, outermost first; empty for a declared member
    Depth   int
}

// Gap is one contributor that yielded nothing, and why.
type Gap struct {
    Host        symbol.Identity
    Contributor *node.TypeRef
    Reason      GapReason // Unresolved | NotMembered | Cyclic | Generic
}

func (s MemberSet) Complete() bool
```

Promotion, override, merge and linearisation are language rules
and stay in the language. The Go rules apply Go's own: a declared
member shadows a promoted one, the shallowest promotion wins, two
at one depth cancel and neither appears, an embedded interface
contributes its whole method set, and a walk deeper than eight
embeds reports `Cyclic`, because no compiling source reaches that
depth and a hand-built graph can. A contributor whose reference
carries no target reports `Unresolved`, one whose declaration is
not a type with members reports `NotMembered`, and one carrying
type arguments reports `Generic`, because its members are typed in
its own parameters and copying them across names identifiers not
in scope.

A generator that must not ship a partial double reads `Gaps` and
reports; one filling a table treats them as a footnote. Severity
is the consumer's policy, which is why the set returns the gaps
rather than reporting them.

### Values

```go
type Sample struct {
    Text    string            // the literal, in the source language
    Needs   []symbol.Identity // declarations the text names, for the import set
    Refusal Refusal
}

func (s Sample) OK() bool

type Refusal uint8

const (
    RefusedNone Refusal = iota // a value was derived
    RefusedNoReader
    RefusedNoRules
    RefusedNoLiteral           // the type admits no distinguishable literal
    RefusedUnresolved          // a named type the graph does not hold
    RefusedDepth               // a self-referential type past the walk's budget
)
```

A sample is text in the source language, because a check over a
declaration is written in that declaration's language and the
emit scaffold carries no literal form on purpose. `Needs` lists
the declarations the text names, so a backend's import set
registers them by identity, the way it qualifies every other
reference by package path.

`SamplesOf` returns two distinct values because a check comparing
against one value passes whenever the subject already held it,
and what it held is not always knowable. The hint names the
declaration the value belongs to, so a value in a failure message
says where it came from. `ZeroLiteral` spells the type's zero,
which is what a declared default is compared against. `LiteralFor`
renders text that already went through the language's quoting,
such as a struct tag's value, as a literal of the type, and
reports false where the text cannot be one.

All three refuse rather than guess, and a refusal carries its
reason, because only `RefusedNoLiteral` is a fact about the type.
The rest describe an input the caller can fix, and the response to
every one is to emit no check, so a run missing a package
otherwise produces a test that asserts less than it appears to.

The Go rules derive: a builtin's pair from a fixed table, with the
text pair carrying the hint; a defined type's pair as a conversion
of its underlying type's; a struct's as a composite literal setting
its first settable exported field; a slice's and an array's as a
one-element literal differing in the element; a map's as a
one-entry literal differing in the key, because a map to an empty
struct is how Go spells a set; a pointer's as the address of the
inner value; and `time.Time` and `time.Duration` from a curated
table, because the standard library is never in the graph. An
interface, a function type, a channel and an inline body refuse
with `RefusedNoLiteral`.

### Authored values

An author knows values a derivation cannot: which string a
validator accepts, which types a generic parameter is meant to run
at. Two kernel directives carry them, and the projections read
their stamps before deriving anything, so every consumer receives
the authored answer without knowing an annotator exists.

```
//+gen:sample value="us-east" alternate="eu-west"
//+gen:witness T=int U=string
```

`sample` takes two string params, `value` and `alternate`, on any
declaration that carries a type: a field, a parameter, a variable,
a constant, an alias, a struct, an enum. `witness` takes one key
per type parameter, so its schema is open: every undeclared key is
typed as a string, and the annotator holding the declaration
refuses a key that names none of the subject's type parameters,
positioned at the carrier. The schema contract gains the one field
that admits it:

```go
type Schema struct {
    // ...
    // Open types every key the Params do not declare; nil closes
    // the schema, which is the default.
    Open *ParamSpec
}
```

The kernel registers three keys beside the module keys:
`gen.sample` and `gen.alternate` as strings on the kinds `sample`
admits, and `gen.witness` as a string on type parameters. The two
handlers are ordinary annotators built on the authoring surface,
in `core/authored`, and a composition lists them like any other
annotator. They are default in the sense that the reference
composition and every corpus fixture register them; the kernel's
workspace never imports the authoring root, so it registers no
plugin on anyone's behalf.

`SamplesOf` reads `gen.sample` and `gen.alternate` on the
declaration that carries the type before deriving, each half
independently, because a derived first value is often fine where
the second has to differ in a way the derivation cannot know.
`Witnesses` reads `gen.witness` per parameter first and derives
only for a bound whose type set is knowable without loading the
declaring package, which in Go means `any` and `comparable`.

### Naming

`TypeName(word, base)` joins a generator's word onto an author's
name the way the language spells a derived type: Go joins in
PascalCase and keeps the base's export by its first rune. The
generator owns the word, the language owns the join, and the
family helpers in the shared language module supply the casing.

### The optional capabilities

A language declares an optional capability by satisfying an
interface on its `SourceRules` value, and a consumer finds it by
asserting on `Bound.Source()`. A consumer that asserts and is
refused reports once and generates nothing for that language,
which beats a projection built on a default nobody chose.

```go
type EnumRules interface {
    EnumOf(e *node.Enum, r Reader) EnumInfo
}

type EnumInfo struct {
    Form       EnumForm     // Identifier | Value: where a variant's text comes from
    Variants   []VariantText
    Zero       string       // the variant whose value is the type's zero; "" when none
    Duplicate  string       // the first text two variants share; "" when none
    OutOfRange string       // a literal outside the declared set; "" when none derives
    Foreign    []string     // packages declaring variants outside the type's own, sorted
}

type ErrorValueRules interface {
    SentinelName(base string) string
    IsSentinelName(ident string) bool
}

type TagRules interface {
    Tag(f *node.Field, key string) (string, bool)
}

type GenericsRules interface {
    Witnesses(params []*node.TypeParam, r Reader) []*node.TypeRef // all or nothing; nil means no witness set
    Substitute(ref *node.TypeRef, params []*node.TypeParam, witnesses []*node.TypeRef) *node.TypeRef
    Reified() bool
}

type PropertyRules interface {
    Properties(s *node.Struct, r Reader) []Property
}

type ConstructRules interface {
    Constructors(s *node.Struct, r Reader) []Callable
}

type ThrowsRules interface {
    Throws(c Callable) []*node.TypeRef
}

type OwnershipRules interface {
    Ownership(p ParamView) Ownership // ByValue | Borrow | BorrowMut
}

type PromotionRules interface {
    Settable(s *node.Struct, r Reader) []Member
}

type EqualityRules interface {
    Comparable(ref *node.TypeRef, r Reader) (ok bool, problems []*node.TypeRef)
}
```

`AnnotationRules` is not minted. Every declaration kind carries
`Annotations` on the model, read statically by the frontend, so
the model field is the projection and an interface over it would
return the field.

The Go rules satisfy `EnumRules`, `ErrorValueRules`, `TagRules`,
`GenericsRules`, `PromotionRules` and `EqualityRules`. `EnumOf`
reads the exact values the frontend stamped under
`golang.constValue`, so iota arithmetic is evaluated once, at
parse, and read here. `Comparable` is the rule the Go annotator
already applies to stamp `golang.comparable`; the annotator calls
the rules rather than carrying a second copy, so the stamped fact
and the projected answer cannot drift.

### Reads and re-execution

A projection reads only through the reader it was handed, so every
declaration and fact it touched is an edge on the invocation that
asked, at the same grain as any other read: a lookup records the
identity, an enumeration records set membership, a fact records
the subject and key. A projection holds no cache across
invocations, and a language whose walk is expensive keeps its
cost inside the invocation, where the engine can see it. The
benchmark for the Go member walk at the canonical scale is part
of the satellite's own gate.

### The conformance suite

`core/rules/rulestest` holds any implementation to the properties
the contract promises, over a fixture the caller supplies: a
loaded graph and the rules under test.

- `AssertDeterministic`: every projection returns equal values on
  two calls with one reader.
- `AssertTotal`: `TypeOf` returns a shape for every reference in
  the graph, and `CallableOf` reports false for every non-callable
  kind, without panicking.
- `AssertRefusesWithReason`: a sample that is not OK carries a
  refusal other than `RefusedNone`, and a member set with a gap
  names the contributor and a reason.
- `AssertDistinctSamples`: where both halves are OK, the texts
  differ.
- `AssertWitnessesWhole`: a witness set is nil or one entry per
  parameter.
- `AssertRecorded`: a projection over a subject records at least
  the subject's identity on the reader it was handed.

The suite runs in the Go satellite over the conformance corpus,
and in the corpus module over every language that returns rules.
The corpus's coverage verdicts grow from the two read-side
verdicts to the four levels of the degradation scale: a feature
projects fully, projects partly with its remainder under stated
metadata keys, projects as Opaque with its metadata, or refuses.
The check reads the projections, so a feature that arrives better
than declared fails too, because a capability nobody declared is
a capability nobody tested.

### Refusal codes

Three diagnostic codes join the kernel's: a reference param that
resolves to nothing at validation, an authored witness naming a
type parameter the subject does not declare, and a rules lookup
for a language the composition registered none for. Each is
positioned at the carrier or the subject that caused it.

## Alternatives considered

- **Projections as facts.** Have the frontend or an annotator stamp
  every projection on every symbol at load, and let generators
  read facts. A member set with provenance and gaps is a structure,
  and facts are flat values; more to the point, stamping every
  projection of every declaration is eager work over the whole
  graph on every run, against an engine that runs the readers a
  change reaches and nothing else.
- **Re-parsing spellings inside TypeOf.** Keep the reference
  verbatim and have each language parse its composite grammar in
  the projection. The form comes back and the identities do not:
  the import scope the resolution step resolved through belongs
  to the load, and the sealed graph does not carry it. The model
  change above is the smaller cost.
- **Rules bound to the frontend.** Have `plugin.Frontend` return
  its language's rules. A graph loaded from a recorded key needs
  rules with no frontend present, a read-only language ships rules
  the same way, and a composition that runs over a graph it did
  not load has nowhere to ask. The composition registers rules by
  language, the way it registers everything else.
- **A neutral sample as a scaffolding expression.** Return values
  in the emit vocabulary so every backend spells them. The
  scaffold carries a name and a call and no literal, on purpose,
  and the consumer that exists writes checks in the source
  language. Text in the source language serves it; a cross-language
  value spelling is a separate proposal when a consumer needs one.
- **A spelling-keyed resolver.** Hand projections a resolver that
  returns a declaration for a spelling, as a resolver over
  qualified names does. Identities exist by the time a projection
  runs, and the tracked reader already resolves one; a second
  resolver keyed on text would be a second place for scope rules to
  drift.
- **One `Rules()` returning the unbound contract.** Have the match
  hand back `SourceRules` and let handlers pass a reader per call.
  Every call then repeats the reader argument, and a handler that
  passes a reader from another invocation records its reads on the
  wrong one. Binding at the match keeps the read set and the rules
  on one invocation.

## Drawbacks

- The model change touches every frontend: a language that fills
  `Form` and `Elems` for its composites has to keep them consistent
  with the spelling, and the conformance suite gains a check that
  the two agree.
- `SourceRules` is wide. A language returns eight methods before it
  returns anything, and the refusing defaults make a thin
  implementation honest rather than small.
- Samples in the source language mean a cross-language check
  generator cannot exist on this contract alone.

## Unresolved and future work

- Which kinds `sample` admits beyond the typed declarations listed
  here; a method's return is the open case.
- Whether a Go channel projects as a synchronous Stream or as
  Opaque, decided against the first consumer that reads streams.
- A cross-language value spelling, if a consumer generating checks
  in another language arrives.

## References

- [Architecture: the projection vocabulary](../architecture/03-projection.md)
- [Architecture: the symbol model](../architecture/02-symbol-model.md)
- [Architecture: directives](../architecture/05-directives.md)
- [Architecture: authoring](../architecture/06b-authoring.md)
- [Architecture: cross-language conversion](../architecture/10-cross-language.md)
- [Architecture: the shape catalog](../architecture/12-shape-catalog.md)
- [Architecture: testing and conformance](../architecture/13-testing-and-conformance.md)
- [RFC-0005: The directive grammar, schemas and validation](0005-directive-grammar.md)
- [RFC-0008: The emit body and the scaffolding vocabulary](0008-emit-body-content.md)
- [RFC-0013: The frontend kit and its conformance suite](0013-the-frontend-kit.md)
