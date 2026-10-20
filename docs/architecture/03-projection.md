<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# The projection vocabulary (rules/)

*Builds on: [02](02-symbol-model.md). Feeds:
[05](05-directives.md) (params bind through Resolve),
[10](10-cross-language.md) (TypeShape is the hub),
[11](11-languages.md) (the tiers are the satellite contract),
[12](12-shape-catalog.md) (detectors read Callable).*

This is the kernel's read side. Without a rule about where a
question belongs, it ends up wherever the first person to ask it was
standing: on the mandatory interface, so every language has to
answer it; in a per-language package, so neutral code quietly binds
itself to one language; or in a consumer's own helper, so three
consumers end up disagreeing about one declaration. The projection
layer exists so that placement follows a rule rather than a
judgement call.

## The placement rule

> If two or more neutral consumers ask a question, it goes in Tier 1
> or Tier 2. If only a language binding asks it, it goes in Tier 3
> and stays there.

```
Tier 1  every language returns        Callable · TypeShape · Resolve
        (the language contract)       · Members · Values
Tier 2  declared by satisfying,       EnumRules · AnnotationRules ·
        found by asserting            GenericsRules · …
Tier 3  the root package of each      the Go-only, Java-only
        satellite                     questions
        ─────────── below the line: metadata (checks 2–3) ───────────
```

## Typed language identity

`rules.Source` and `rules.Target` are distinct types over an
interned registry. A generator asks about the language a declaration
was written in. A backend serves the language a plan renders
into. Both identities share one namespace, so a frontend stamps the
same name a lowering recognises, but confusing the two directions
does not compile.

Each satellite's `lang.go` exports its identity as typed values, and
Go code references those: plugins, plans and tests never use the
string. The string exists only where a human types it, such as
`target: golang` in config or a language name in a diagnostic, and
it resolves against the registry at workspace Build
([README](README.md)).

## Tier 1: every language returns

Tier 1 has five projections. Implementing Tier 1 is what makes
something a language ([11-languages.md](11-languages.md)).
The kernel owns the walks every language would otherwise repeat, and
the language returns the decisions inside them:

| Projection | The kernel owns | The language returns |
|---|---|---|
| Callable | mapping receiver, parameters, returns, variadic and async from the model | the role of each parameter, the role of each return, the error model |
| TypeShape | folding a reference's form and children; classifying a resolved target | the meaning of a builtin spelling, and the well-known mapping |
| MemberSet | the walk: cycle guard, depth budget, argument binding, provenance, gaps | which member lists contribute, in which order, and how names shadow |
| Resolve | the resolution kinds, the validation binding and the walk of each kind | the probe order of a scope, the members a subject can use, the predeclared types, and the type of a value |
| Values | the value tree, its refusal vocabulary, the authored-value precedence | deriving two values, the zero, and a value from text |

A name that a generator derives is not a projection. The generator
joins its word to the name of a declaration with `Emitter.JoinName`.
The joined name is in the neutral convention of every emitted name.
The backend writes it in the case of the target when the plan
settles.

The contract a language returns, pinned:

```go
type SourceRules interface {
    Lang() symbol.Lang
    Members() MemberPolicy                                          // which lists contribute, how names shadow, the depth
    ParamRole(p *node.Param, v View) ParamRole                      // Context | Input
    ReturnRoles(rs []*node.Return, v View) ([]ReturnRole, ErrorModel) // Value | OkBool | Stream | Error; None | LastReturn | ResultType | Thrown | Raised
    Builtin(ref *node.TypeRef, v View) TypeShape                    // a spelling the resolution step left without a target
    Resolve(scope Scope, name string, kind directive.ResolutionKind, v View) (symbol.Symbol, error)
    DeriveSamples(ref *node.TypeRef, hint string, v View) (sample, alternate Sample)
    ZeroValue(ref *node.TypeRef, v View) (emit.Value, bool)
    LiteralFor(s Scope, ref *node.TypeRef, text string, v View) (emit.Value, bool) // a literal, or a name in the scope
}
```

A handler reaches the kernel's walks through the bound `Rules()` on
its match: `CallableOf`, `TypeOf`, `FieldTypeOf`, `MembersOf`,
`Satisfies`, `Implementers`, `SamplesOf`, `ZeroValue`, `LiteralFor`,
`Witnesses`, `Tag`, `SentinelName`, `SentinelBase` and `ErrorContract`,
over the invocation's tracked view.
Every read that a projection makes records on that invocation. A value
is safe for concurrent use, because it has no state of its own.

The workspace builds each view with `rules.NewView`. The fact store and
the read set of a view are unexported. A rules implementation reads a
fact only through `rules.Fact`, which records the read. An
implementation reads a key of another registrant through a handle from
`meta.Named`, and `rules.Fact` resolves the handle by the name of the
key.

Call `OK()` on a `Sample` or any refusable answer before you use it.
When a language cannot reason about a type, it refuses with a
reason. It never hands back a zero value you would then compare
against.

### Callable

The normalized view of any function or method, and the only thing a
shape detector may read ([12-shape-catalog.md](12-shape-catalog.md)).

It carries an optional receiver and parameters with roles:
context-like, input, variadic. Returns carry roles too: values,
an ok-bool, or a stream. A stream carries an async flag, which
covers Kotlin Flow, Rust Stream, TypeScript AsyncIterator and Python
async generators.

A parameter of the view reports whether a call can leave it out. A
call can leave out a parameter with a default or with the optional
mark of its language, and a parameter that collects the rest of the
arguments. A constructor whose parameters are all optional constructs
without an argument.

**ErrorModel** is `None | LastReturn | ResultType | Thrown |
Raised`. That turns "does this return an error" into one question
across Go's last return, Rust's `Result`, TypeScript's `throw`,
Python's `raise` and Java's checked `throws`. Whether a `throws` is
checked is a Tier-2 question.

**Async** is `Sync | Async`, covering TypeScript functions returning
Promise, Rust futures, Python `async def` and Kotlin `suspend`. Go
always returns Sync, because its concurrency lives in the caller and
never shows in a signature. That is the honest projection.

### TypeShape

The canonical type vocabulary, and the hub that cross-language
conversion turns on ([10-cross-language.md](10-cross-language.md)).
One closed enum, `symbol.TypeForm`, names the structure of a type
reference and of a shape alike. The frontend sets the structural
half from syntax on the reference, beside its verbatim spelling and
in fixed child order, and the kernel's fold adds the leaves:

```
structural   Named   Optional   List   Array   Map   Func   Tuple
             Union   Intersection   Stream   Borrow   Wildcard   Inline
leaves       Scalar{class: Int|Uint|Float, bits}   Bool   Text   Bytes
             Reference{symbol, typeArgs}   Sum{ref}   Opaque   Dynamic
```

Adding a form is a kernel change.

One exclusion is deliberate and recorded. There is **no Set**,
because every set type in these ecosystems is a library type, so it
projects as `Reference`. Only a language with a set literal would
force the shape, and no language in scope has one. A fixed-length
array is a form of its own, `Array`, because its length is part of
the type.

**Optional** is explicit, so Go pointers, TypeScript `undefined`,
Kotlin `T?` and Python `None` all project without any one language's
spelling leaking into the vocabulary.

A field can have presence that its type does not contain: a
TypeScript property with a question mark, or a proto3 message field.
`FieldTypeOf` folds a field's type inside an optional where
`node.Field.Optional` is set or the language's `PresenceRules` reports
presence. It returns an optional or Dynamic unchanged, because both
contain the absent value.

**Dynamic** is the top type, a value whose type is decided at run time:
Go's `any`, TypeScript's `unknown` and `any`, Java's `Object`, and
protobuf's `Value` and `Any`. It has no children.

**Stream** has a flag for elements that arrive asynchronously, such as
the elements of a protobuf stream or of a TypeScript `AsyncIterable`.
A frontend sets the flag on the reference. The fold copies it into the
shape.

**Union and Sum stay separate.** Conflating them is how a lowering
ends up guessing, and proto `oneof` alongside TypeScript unions
forces both shapes on day one. A Go union constraint, such as
`~int | time.Duration`, is a Union of its terms, and its spelling keeps
the approximation mark of each term.

**Intersection** has one child per member, and a value of it has
every member's type: TypeScript's `A & B`, and a Rust `dyn` or `impl`
type over its trait bounds. The spelling keeps the `&`, the `dyn` or
the `impl`.

**Wildcard** has the bound as its one child and records the variance.
An unbounded wildcard, Java's `?`, has no child. It is not the top
type, because Java's `?` is a type only inside a type argument.

An **Inline** reference records the members of an inline struct,
interface or object body as its fields and methods, without identities
([02-symbol-model.md](02-symbol-model.md)). Its children are the types
that the body states, in the order of the body. A backend binds the
imports of the children and keeps the spelling of the body.

**Well-known types are not shapes.** They are blessed `Reference`
identities in a small kernel registry, which has five entries:
`timestamp`, `duration`, `empty`, a value without data, `error`, the
value of a failure, and `context`, the cancellation and the deadline of
a call. A frontend maps its spelling in, so `time.Time` becomes
`timestamp`, protobuf's `Empty` becomes `empty`, and Go's `error` and
TypeScript's `Error` become `error`. A reference whose target is an
identity of the registry folds to it, as the context of an rpc does. A
lowering spells it out, as `Date`, as `struct{}`, as `Exception` or as
`context.Context`. A semantic type nobody mapped remains an ordinary
Reference. The registry is public API, and growing it only adds.

Java's unannotated references are nullability-unknown. That does not
become a third Optional state. The frontend stamps
`java.nullability=unknown` and a workspace lowering policy says what
to treat it as. The three-way distinction stays out of the shape
vocabulary.

### Resolve

Scope resolution as a neutral question: resolve a name against a
scope at a declared resolution kind, which is one of
callable-in-scope, package-var, type-in-scope, value-field,
host-param or member-on-handle.

Directive params bind through it ([05-directives.md](05-directives.md)),
classification resolves against it
([12-shape-catalog.md](12-shape-catalog.md)), and the kernel's Link
step consults it when it resolves type spellings to canonical
identities after Load ([02-symbol-model.md](02-symbol-model.md)).

The kernel performs the walk of each kind in `rules.Resolve`. A
language passes the walk a `rules.Resolver`, which returns the four
decisions of the language:

- `InScope` returns the first declaration that a name refers to in
  the scope of the subject, in the probe order of the language.
- `Usable` reports whether code of the subject can use a member.
- `Placeholder` returns the declaration of a type name that the view
  does not declare, such as a predeclared type.
- `Value` returns the type of a value of a type, such as the target
  of a pointer.

The Go and TypeScript rules resolve through this walk. The protobuf
rules keep a walk of their own, because a schema resolves oneof
fields, services and rpcs and has no package variables.

### Members

The effective member set of any type-like symbol: its own members
plus whatever arrives through Embeds, Extends and Implements,
resolved through Link identities.

Every generator that builds a double asks this first, wanting the
full contract of an interface, and nothing outside the language
contract can return it. Promotion, override, merge and MRO are
language rules. Go promotes, Java overrides, TypeScript interfaces
merge, and Python linearises its bases in the C3 order of its method
resolution. A generator that re-derives any of them gets it wrong for
the next language.

A `MemberSet` records, per member, which supertype or embed it
arrived through. For any contributor that yielded nothing, it
records why: unresolved, not a membered type, cyclic, or generic.
Those reasons carry weight. A mock generator has to be able to say
"this double misses the embedded half, and here is why" instead of
quietly shipping less than it appears to. Reads go through the
tracked Reader, so resolving members is an incrementality edge like
any other read ([09-incrementality.md](09-incrementality.md)).

`Satisfies` compares the member set of a type with the members of an
interface, at the type arguments of both references. It returns the
members that the type misses and the members that it has with another
signature, so a double reports what it cannot write instead of
failing to compile. `Implementers` returns the declarations of the
view whose member sets satisfy an interface. A language whose
satisfaction reads more than the member sets implements
`SatisfactionRules`. Go reports each method with a pointer receiver,
which a value of the type does not have, and each interface with
type-set elements, which no comparison of members proves.

### Values

Value synthesis, which every check-emitting generator depends on.

`DeriveSamples` returns **two distinct values** of a type. A check
that compares against a single value passes whenever the subject
already had that value. You cannot always know which value the
subject had. The hint is the name of the declaration that the value
belongs to. A failure message that contains the value shows where the
value came from.

`ZeroValue` spells the type's zero, which is what a declared
default gets compared against. The spellings differ per language,
`nil` against `None`, so a kernel table would be returning for one
language.

A value of a class that a constructor builds from other values is a
construction in the value tree ([07-rendering.md](07-rendering.md)).
The TypeScript rules derive a `Set`, a `ReadonlySet`, a `Map` and a
`ReadonlyMap` this way, as `new Set<string>(['tags-a'])`, and the
empty construction is the zero. The construction of a readonly type
calls the constructor of its mutable class, and keeps the type
arguments of the reference. The Go rules derive `struct{}{}` as the
one value of the empty struct, so a map of empty structs, the set of
Go, derives its pair from the pair of its keys.

A function and a make are fresh values. Each is a new value wherever a
body writes it, so a check compares it by its identity. The two halves
of its pair differ although their trees are equal. The Go rules
derive two functions that return the zero value of each result for a
function type, as `func(context.Context) error { return nil }`, and two
makes for a channel, as `make(chan string)`. A part of a composite, such
as an element or a field of a struct, takes no fresh value, because two
composites would then differ in the identity of a new value alone. The
Go rules derive the pair of a string from the hint for `any` and
`interface{}`.

`LiteralFor` reads text as a value of a type in the scope of a
declaration, and reports false when it cannot. That serves values read
from carriers that already consumed the language's quoting, where bare
text is the right literal for some types and not for others. In a
language whose values can be names, the value can also be a declaration
in the scope. The Go rules read a constant or a variable, such as
`time.Second`, and a call of a function without arguments, such as
`store.NewMemory()`. The TypeScript rules read the same two forms, such
as `top` and `rootEntry()`, through the imports of the file. The value
tree refers to such a declaration by its identity, so the backend
imports its package. `View.ScopeOf` returns the scope of a declaration.

All three refuse instead of guessing. A type the language cannot
reason about returns not-OK, and never a zero value you would
compare against.

**Authored values beat derived ones.** The kernel registers the
`gen.sample`, `gen.alternate` and `gen.witness` keys and owns the
two directives that stamp them ([05-directives.md](05-directives.md)).
The bound `SamplesOf` and `Witnesses` read those stamps before they
ask the language to derive anything, so every consumer gets the
authored answer without knowing any annotator exists. This is a
contract rather than a
convention, because a preference each generator has to remember is a
preference two of them will forget.

An authored text lifts through the `LiteralFor` of the language of the
declaration that states it, in the scope of that declaration, for every
type. A name and a call are values of a number type too, so the author
of `type Level int` can state `NewLevel()`. Where `LiteralFor` does not
read the text, the lift depends on the shape of the type:

- A text type takes the text as the content of a string. Rules with
  `TextRules` give the value of the string, because their language
  writes some texts through a call, as Rust writes `String::from("a")`.
- A boolean and a number type refuse with `RefusedAuthored`, because the
  text is not a value of the type.
- Every other type takes raw text in the language of the declaration.
  Only the backend of that language spells raw text.

`SamplesOf` keeps a refused stated half. The generator then receives the
refusal of the authored text instead of a value that the language
derived.

## Tier 2: declared by satisfying, found by asserting

A language declares an optional capability by implementing the
interface. The bound rules wrap six capabilities: `Tag`,
`SentinelName`, `SentinelBase`, `ErrorContract`, `FieldTypeOf` and
`Witnesses`. A handler reads them through methods that report false for
a language without the capability. A consumer of any other capability asserts
for it. When the assertion fails, the consumer reports once and
generates nothing. It does not build a projection on a default that
nobody chose. If these were required instead, a language without the
concept would have to return by refusing.

The contract shapes, pinned to the same standard as Tier 1:

```go
type EnumRules interface {
    EnumOf(e *node.Enum, v View) EnumInfo
}
type ErrorValueRules interface {
    SentinelName(base string) string          // the base recognises what the
    SentinelBase(ident string) (string, bool) // name composes, so the two cannot drift
}
type ErrorTypeRules interface {
    ErrorContract(decl symbol.Symbol, v View) (ErrorContract, bool) // message, cause, hooks, pointer
}
type ResultRules interface {
    Result(ref *node.TypeRef, v View) (value, failure *node.TypeRef, ok bool) // the parts of a result type
}
type TextRules interface {
    Text(ref *node.TypeRef, text string, v View) emit.Value // the value of an authored text
}
type ContextRules interface {
    Context(sym symbol.Symbol, v View) (ParamView, bool) // the context that a signature does not state
}
type TagRules interface {
    Tag(f *node.Field, key string) (string, bool)
}
type WellKnownRules interface {
    WellKnown(id symbol.Identity) (symbol.Identity, bool) // the well-known identity of a library type
}
type ShapeRules interface {
    ShapeOf(ref *node.TypeRef, decl symbol.Symbol, v View) (TypeShape, bool) // the shape of the values of a reference
}
type SatisfactionRules interface {
    ValueReceiver(m *node.Method, v View) bool // whether a value of the type of m has m
    TypeSet(i *node.Interface, v View) bool    // whether i constrains a type set beside its methods
}
type GenericsRules interface {
    Derive(p *node.TypeParam, v View) (*node.TypeRef, bool)   // one witness the author left unstated
    Reified() bool
}
type PropertyRules interface {
    Properties(s *node.Struct, v View) []Property       // computed view, never model mutation
}
type ConstructRules interface {
    Constructors(s *node.Struct, v View) []Callable
}
type ThrowsRules interface {
    Throws(c Callable) []*node.TypeRef
}
type OwnershipRules interface {
    Ownership(p ParamView) Ownership                    // ByValue | Borrow | BorrowMut
}
type PromotionRules interface {
    Settable(s *node.Struct, v View) MemberSet          // declaration order, with the walk's gaps
}
type EqualityRules interface {
    Comparable(ref *node.TypeRef, v View) (ok bool, problems []*node.TypeRef)
}
type PresenceRules interface {
    Presence(f *node.Field, v View) bool                // presence that the field's type does not contain
}
```

Annotations need no capability: every declaration kind carries
`Annotations` on the model, read statically by the frontend, so the
field is the projection.

| Interface | Question | Exercised by |
|---|---|---|
| `EnumRules` | project an enumeration: form, zero, foreign values | Go const groups, Java enum classes, TS enums, proto |
| `ErrorValueRules` | sentinel conventions: the name of a base and the base of a name, paired so they cannot drift | Go |
| `ErrorTypeRules` | the error contract of a type: the member of its message, the field of its cause, the hooks that the error functions call, and whether a pointer to it is the error | Go method sets, TypeScript classes that extend Error |
| `ResultRules` | the value and the failure of a result type, which `CallableOf` splits into two views of a return under the result-type error model | Rust `Result<T, E>` |
| `TextRules` | the value of a text that an author states for a text type, which `View.Authored` asks for in place of a string literal | Rust `String::from("a")`, and the character literal of `char` |
| `ContextRules` | the context of a call that a signature does not state, which `CallableOf` puts first among the parameters | the context of every protobuf rpc, which every gRPC binding passes to a handler |
| `TagRules` | stringly per-field tags | Go struct tags |
| `WellKnownRules` | the well-known identity of a type of the standard library, which the fold reads before it classifies a target | Go `time.Time` and `time.Duration` |
| `ShapeRules` | the shape of the values of a reference to a declared type, which the fold reads after `WellKnownRules` and before it classifies a target. The fold takes the folded type arguments of the reference as the children of a structural shape without children, and the first argument alone as the child of a stream, because a generator states the types that it receives and returns after the type that it yields | GraphQL custom scalars with a `@specifiedBy` URL of the table of scalars, the builtins and the generic classes of Python's typeshed, and `Literal` |
| `SatisfactionRules` | the satisfaction of an interface beyond the member sets: a method of a pointer to the type alone, and an interface that constrains a type set | Go pointer receivers and constraint interfaces |
| `GenericsRules` | witnesses, constraint reasoning, whether generics are reified or erased | every generic language; erasure matters to a Java backend |
| `PropertyRules` | the computed properties view, pairing getters with setters. A projection, never a change to the model | Kotlin, C#, Swift, Python `@property`, JavaBeans |
| `ConstructRules` | constructors and how to spell instantiation | JVM, TS and Python real constructors; Go and Rust conventions |
| `ThrowsRules` | declared throws lists | Java checked exceptions, Swift typed throws |
| `OwnershipRules` | by-value, borrow or mutable borrow on parameters | Rust only |
| `PromotionRules` | settable members, including promotion and embedding | Go embeds |
| `EqualityRules` | whether a type works where the language demands equality (a map key, a switch, deduplication), and which member refs break it | Go comparability, where slices, maps and funcs break it; Java equals and hashCode; Python hashability |
| `PresenceRules` | whether a field has presence that its type does not contain, decided by what the type resolves to | protobuf message fields |

The third column lists the constructs that motivate each capability.
The projection side of the support matrix, which the conformance
module renders, states which languages implement each capability.
The matrix finds a capability by asserting it on the rules of each
language, as a consumer does.

## Tier 3: the root package of each satellite

The root package of each satellite exports the vocabulary of its
language: the identity, the target, the fact keys and the naming
rules, such as `golang.Target`, `golang.SatisfiesErrorKey` and
`golang.AssumedName`. A plugin imports it where it binds the
language, such as the registration of its templates of the target.
A fact that two neutral consumers read is a Tier-2 capability and not
a key of one language. The depguard rules of the repository deny the
satellites to a module that is neutral as a whole, such as the shape
catalog ([13-testing-and-conformance.md](13-testing-and-conformance.md)).

## The degradation scale

Every source construct sits on exactly one check:

1. **Projected fully** into Tier-1 or Tier-2 vocabulary.
2. **Projected partly**, with the remainder carried as language
   metadata ([04-metadata.md](04-metadata.md)).
3. **Opaque shape plus language metadata**: representable, but not
   projectable. Rust lifetimes, TypeScript conditional types and
   Python metaclasses sit here.
4. **Refused at lowering**, with a stable diagnostic code. This is
   the only level on the write side, and it never guesses.

Everything is representable. Some things cannot be lowered. Every
failure arrives as a positioned diagnostic. The completeness check
([13-testing-and-conformance.md](13-testing-and-conformance.md))
verifies that every feature-matrix row of every language sits on the
level it declared.

## Generics, split across the tiers

**Structure is Tier 1.** `TypeParams` carry variance and bounds as
refs on the symbol model, and every anticipated language returns it.

**Reasoning is Tier 2**, through `GenericsRules`: witnesses, meaning
one concrete type per parameter and all of them or none, since a
witness for one parameter is worth nothing without the rest, and
whether the language reifies or erases.

**Substitution is the kernel's.** `rules.Substitute` rewrites every
reference to a type parameter into its argument the same way for every
language, so the member walk and `Witnesses` do not assert a
capability.

**A type parameter folds to Opaque.** The argument of a parameter
decides its shape, so the fold returns a reference to a type parameter
as `Opaque` with the identity of the parameter in `Ref`. A reader that
classifies a shape by its form, such as `DeriveSamples`, reads an
unknown shape. The spoke of a target spells the shape at every depth of
a type ([10-cross-language.md](10-cross-language.md)).

Witnesses are usually authored rather than derived. A language can
derive one only where the constraint's type set is knowable without
loading the package that declares it, which in Go means `any` and
`comparable` and nothing else. So `Witnesses` reads the
`gen.witness` stamps first, and derivation is the narrow fallback.

**The unprojectable goes to metadata**: `rust.lifetimeParams`,
`golang.typeSet`, `typescript.conditionalType`, `kotlin.reified`.
Anyone may read them, and in practice that language's bindings do,
which is Tier 3 doing its job.
