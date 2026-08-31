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
        (the language contract)       · Members · Values · naming
Tier 2  declared by satisfying,       EnumRules · AnnotationRules ·
        found by asserting            GenericsRules · …
Tier 3  language sdk, importable      the Go-only, Java-only
        from binding files alone      questions
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

Five projections and the naming joins. Implementing Tier 1 is what
makes something a language ([11-languages.md](11-languages.md)).
These spellings are stable enough to pin:

```go
type Callable struct {
    Receiver *ParamView            // nil for free functions
    Params   []ParamView           // Role: Context | Input; Variadic on the last
    Returns  []ReturnView          // Role: Value | OkBool | Stream (Async flag)
    Errors   ErrorModel            // None | LastReturn | ResultType | Thrown | Raised
    Async    bool
}

type SourceRules interface {
    CallableOf(sym symbol.Symbol) (Callable, bool)
    TypeOf(ref *symbol.TypeRef, r Resolver) TypeShape
    Resolve(scope Scope, name string, kind ResolutionKind) (symbol.Symbol, error)
    MembersOf(sym symbol.Symbol, r Resolver) (MemberSet, bool)
    SamplesOf(ref *symbol.TypeRef, hint string, r Resolver) (sample, alternate Sample)
    ZeroLiteral(ref *symbol.TypeRef, r Resolver) (string, bool)
    LiteralFor(f *symbol.File, ref *symbol.TypeRef, text string, r Resolver) (string, bool)
    TypeName(word, base string) string
}
```

Call `OK()` on a `Sample` or any refusable answer before you use it.
When a language cannot reason about a type, it refuses. It never
hands back a zero value you would then compare against.

### Callable

The normalized view of any function or method, and the only thing a
shape detector may read ([12-shape-catalog.md](12-shape-catalog.md)).

It carries an optional receiver and parameters with roles:
context-like, input, variadic. Returns carry roles too: values,
an ok-bool, or a stream. A stream carries an async flag, which
covers Kotlin Flow, Rust Stream, TypeScript AsyncIterator and Python
async generators.

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
The set is closed, so adding a shape is a kernel change:

```
Scalar{class: Int|Uint|Float, bits}   Bool   Text   Bytes
List{elem, fixedLen?}                 Map{key, value}
Tuple{elems}                          Optional{inner}
Union{members}     — untagged (TS unions, Python |)
Sum{ref}           — tagged variants; the Sum kind's shape
Func{callable}     Stream{elem, async}
Reference{symbol, typeArgs}           Opaque
```

Two exclusions are deliberate and recorded. There is **no Set**,
because every set type in these ecosystems is a library type, so it
projects as `Reference`. Only a language with a set literal would
force the shape, and no language in scope has one. And a
fixed-length array is a `List` carrying `fixedLen`, not a shape of
its own.

**Optional** is explicit, so Go pointers, TypeScript `undefined`,
Kotlin `T?` and Python `None` all project without any one language's
spelling leaking into the vocabulary.

**Union and Sum stay separate.** Conflating them is how a lowering
ends up guessing, and proto `oneof` alongside TypeScript unions
forces both shapes on day one.

**Well-known types are not shapes.** They are blessed `Reference`
identities in a small kernel registry, holding `timestamp` and
`duration` and nothing else until a second consumer needs an entry.
A frontend maps its spelling in, so `time.Time` becomes `timestamp`.
A lowering spells it out as `Date`. A semantic type nobody mapped
stays an ordinary Reference. The registry is public API, and growing
it only adds.

Java's unannotated references are nullability-unknown. That does not
become a third Optional state. The frontend stamps
`java.nullability=unknown` and a workspace lowering policy says what
to treat it as. The three-way distinction stays out of the shape
vocabulary.

### Resolve

Scope resolution as a neutral question: resolve a name against a
scope at a declared resolution kind, which is one of
callable-in-scope, package-var, value-field, host-param or
member-on-handle.

Directive params bind through it ([05-directives.md](05-directives.md)),
classification resolves against it
([12-shape-catalog.md](12-shape-catalog.md)), and the kernel's Link
step consults it when it resolves type spellings to canonical
identities after Load ([02-symbol-model.md](02-symbol-model.md)).

### Members

The effective member set of any type-like symbol: its own members
plus whatever arrives through Embeds, Extends and Implements,
resolved through Link identities.

Every generator that builds a double asks this first, wanting the
full contract of an interface, and nothing outside the language
contract can return it. Promotion, override, merge and MRO are
language rules. Go promotes, Java overrides, TypeScript interfaces
merge, and Python linearises. A generator that re-derives any of
them gets it wrong for the next language.

A `MemberSet` records, per member, which supertype or embed it
arrived through. For any contributor that yielded nothing, it
records why: unresolved, not a membered type, cyclic, or generic.
Those reasons carry weight. A mock generator has to be able to say
"this double misses the embedded half, and here is why" instead of
quietly shipping less than it appears to. Reads go through the
tracked Reader, so resolving members is an incrementality edge like
any other read ([09-incrementality.md](09-incrementality.md)).

### Values

Value synthesis, which every check-emitting generator depends on.

`SamplesOf` returns with **two distinct values** of a type. Two
rather than one, because a check that compares against a single
value passes whenever the subject already held that value, and you
cannot always know what it held. The hint names the declaration the
value belongs to, so a value that turns up in a failure message says
where it came from.

`ZeroLiteral` spells the type's zero, which is what a declared
default gets compared against. The spellings differ per language,
`nil` against `None`, so a kernel table would be returning for one
language.

`LiteralFor` renders text as a literal of a type and reports false
when it cannot. That serves values read from carriers that already
consumed the language's quoting, where bare text is the right
literal for some types and not for others.

All three refuse instead of guessing. A type the language cannot
reason about returns not-OK, and never a zero value you would
compare against.

**Authored values beat derived ones.** The kernel registers the
`gen.sample`, `gen.alternate` and `gen.witness` keys and owns the
two directives that stamp them ([05-directives.md](05-directives.md)).
`SamplesOf` and the Tier-2 `Witnesses` read those stamps before they
derive anything, so every consumer gets the authored answer without
knowing any annotator exists. This is a contract rather than a
convention, because a preference each generator has to remember is a
preference two of them will forget.

### Naming

Case conventions and identifier joins: how a generator's word joins
an author's name in this language. The generator owns the word,
"Builder" or "Factory". The language owns the join.

## Tier 2: declared by satisfying, found by asserting

A language declares an optional capability by implementing the
interface. A consumer asserts for it, and when the assertion fails,
reports once and generates nothing. That beats building a projection
on a default nobody chose. If these were required instead, a
language without the concept would have to return by refusing.

The contract shapes, pinned to the same standard as Tier 1:

```go
type EnumRules interface {
    EnumOf(e *symbol.Enum, constants []*symbol.Constant) EnumInfo
}
type ErrorValueRules interface {
    SentinelName(base string) string        // paired inverses, so the
    IsSentinelName(ident string) bool       // two can never drift
}
type TagRules interface {
    Tag(f *symbol.Field, key string) (string, bool)
}
type AnnotationRules interface {
    Annotations(sym symbol.Symbol) []Annotation   // {Name, Args}, read statically
}
type GenericsRules interface {
    Witnesses(params []*symbol.TypeParam) []Ref   // all-or-nothing; nil = no witness set
    Substitute(ref *symbol.TypeRef, params []*symbol.TypeParam) *symbol.TypeRef
    Reified() bool
}
type PropertyRules interface {
    Properties(s *symbol.Struct) []Property       // computed view, never model mutation
}
type ConstructRules interface {
    Constructors(s *symbol.Struct) []Callable
}
type ThrowsRules interface {
    Throws(c Callable) []Ref
}
type OwnershipRules interface {
    Ownership(p ParamView) Ownership              // ByValue | Borrow | BorrowMut
}
type PromotionRules interface {
    Settable(s *symbol.Struct) []Member           // declaration order
}
type EqualityRules interface {
    Comparable(ref *symbol.TypeRef, r Resolve) (ok bool, problems []Ref)
}
```

| Interface | Question | Exercised by |
|---|---|---|
| `EnumRules` | project an enumeration: form, zero, foreign values | Go const groups, Java enum classes, TS enums, proto |
| `ErrorValueRules` | sentinel conventions, name against predicate, paired so they cannot drift | Go |
| `TagRules` | stringly per-field tags | Go struct tags |
| `AnnotationRules` | structured annotations, attributes and decorators, read statically and never executed | Java and Kotlin annotations, Rust attributes, Python and TS decorators |
| `GenericsRules` | witnesses, constraint reasoning, whether generics are reified or erased | every generic language; erasure matters to a Java backend |
| `PropertyRules` | the computed properties view, pairing getters with setters. A projection, never a change to the model | Kotlin, C#, Swift, Python `@property`, JavaBeans |
| `ConstructRules` | constructors and how to spell instantiation | JVM, TS and Python real constructors; Go and Rust conventions |
| `ThrowsRules` | declared throws lists | Java checked exceptions, Swift typed throws |
| `OwnershipRules` | by-value, borrow or mutable borrow on parameters | Rust only |
| `PromotionRules` | settable members, including promotion and embedding | Go embeds |
| `EqualityRules` | whether a type works where the language demands equality (a map key, a switch, deduplication), and which member refs break it | Go comparability, where slices, maps and funcs break it; Java equals and hashCode; Python hashability |

## Tier 3: the language sdk

Per-language questions, importable only from a plugin's
language-binding files. Go cannot enforce a cross-module import
rule, so the conformance suite ships the lint that does
([13-testing-and-conformance.md](13-testing-and-conformance.md)).
Tier 3 is where language-only vocabulary lives legally: present,
bounded, and unable to reach neutral code without somebody noticing.

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
witness for one parameter is worth nothing without the rest; plus
substitution, and whether the language reifies or erases.

Witnesses are usually authored rather than derived. A language can
derive one only where the constraint's type set is knowable without
loading the package that declares it, which in Go means `any` and
`comparable` and nothing else. So `Witnesses` reads the
`gen.witness` stamps first, and derivation is the narrow fallback.

**The unprojectable goes to metadata**: `rust.lifetimeParams`,
`go.constraintTypeSet`, `ts.conditionalType`, `kotlin.reified`.
Anyone may read them, and in practice that language's bindings do,
which is Tier 3 doing its job.
