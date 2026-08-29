# The projection vocabulary (rules/)

*Builds on: [02](02-symbol-model.md). Feeds:
[05](05-directives.md) (params bind through Resolve),
[10](10-cross-language.md) (TypeShape is the hub),
[11](11-languages.md) (the tiers are the satellite contract),
[12](12-shape-catalog.md) (detectors read Callable).*

The kernel's read-side center. Vocabularies accrete: without a
placement law, a question lands wherever its first asker stood — on
the mandatory interface (so every language must answer it), in a
per-language package (so neutral code quietly binds to one language),
or in a consumer's own helper (so three consumers disagree about one
declaration). The projection layer exists so that placement is law,
not judgment.

## The placement law

> A question asked by two or more neutral consumers is Tier 1 or
> Tier 2. A question asked only inside a language binding is Tier 3
> and stays there.

```
Tier 1  every language answers        Callable · TypeShape · Resolve
        (the language contract)       · Members · Values · naming
Tier 2  declared by satisfying,       EnumRules · AnnotationRules ·
        found by asserting            GenericsRules · …
Tier 3  language sdk, importable      the Go-only, Java-only
        from binding files alone      questions
        ─────────── below the line: metadata (rungs 2–3) ───────────
```

## Typed language identity

`rules.Source` and `rules.Target` are distinct types over an interned
registry. A generator asks about the language a declaration was
*written in*; a backend answers for the language a plan *renders*.
Both identities share one namespace — a frontend stamps the same name
a lowering answers to — but confusing the two directions does not
compile.

Boundary strings, interior constants ([README](README.md)): each
satellite's `lang.go` exports its identity as typed values, and Go
code — plugins, plans, tests — references those, never the string.
The string spelling exists only where humans type it (`target:
golang` in config, a language name in a diagnostic) and resolves
against the registry at workspace Build.

## Tier 1 — every language answers

Five projections and the naming joins. Implementing Tier 1 is what
it means to be a language ([11-languages.md](11-languages.md)). The
contract shape, written out — spellings this terse are stable enough
to pin:

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

Everything a `Sample` or a refusable answer carries asks `OK()`
before use — a type the language cannot reason about yields a
refusal, never a zero value to compare against.

### Callable

The normalized view of any function or method — and the only thing a
shape detector may read ([12-shape-catalog.md](12-shape-catalog.md)).

- Receiver (optional), params with roles: context-like, input,
  variadic.
- Returns with roles: value(s), ok-bool, stream (streams carry an
  async flag: Kotlin Flow, Rust Stream, TS AsyncIterator, Python
  async generators).
- **ErrorModel**: `None | LastReturn | ResultType | Thrown | Raised`.
  "Answers an error" becomes one question across Go's last return,
  Rust's `Result`, TS's `throw`, Python's `raise`, Java's checked
  `throws` (checked-ness itself is Tier 2).
- **Async**: `Sync | Async`. TS Promise-returning, Rust Future,
  Python `async def`, Kotlin `suspend`. Go answers Sync always — its
  concurrency is caller-side and invisible in signatures, and that
  is the honest projection.

### TypeShape

The canonical type vocabulary — also the hub of cross-language
conversion ([10-cross-language.md](10-cross-language.md)). The
enumeration is closed; adding a shape is a kernel change:

```
Scalar{class: Int|Uint|Float, bits}   Bool   Text   Bytes
List{elem, fixedLen?}                 Map{key, value}
Tuple{elems}                          Optional{inner}
Union{members}     — untagged (TS unions, Python |)
Sum{ref}           — tagged variants; the Sum kind's shape
Func{callable}     Stream{elem, async}
Reference{symbol, typeArgs}           Opaque
```

Deliberate exclusions, recorded: **no Set** (ecosystem set types are
library types — they project as `Reference`; only a language with a
set *literal form* would force the shape, and none in scope has
one); fixed-length arrays are `List` with `fixedLen`, not a shape of
their own. **Optional** is explicit so Go pointers, TS `undefined`,
Kotlin `T?`, and Python `None` project without any language's
spelling leaking. **Union and Sum are distinct**: conflating them is
how a lowering ends up guessing, and proto `oneof` plus TS unions
force both on day one.

**Well-knowns** are not shapes — they are blessed `Reference`
identities in a small kernel registry (`timestamp`, `duration`, and
nothing else until a second consumer demands an entry): a frontend
maps its spelling in (`time.Time` → `timestamp`), a lowering spells
it out (`Date`), and an unmapped semantic type stays an ordinary
Reference. The registry is public API; growing it is additive.

Java's unannotated references are nullability-*unknown*: that is not
a third Optional state but a `java.nullability=unknown` metadata fact
plus a workspace lowering policy (`treat-unknown-as`). The tri-state
stays out of the shape vocabulary.

### Resolve

Scope resolution as a neutral question: resolve a name against a
scope at a declared resolution kind — callable-in-scope,
package-var, value-field, host-param, member-on-handle. This is
what directive params bind through
([05-directives.md](05-directives.md)), what classification
resolves against ([12-shape-catalog.md](12-shape-catalog.md)), and
what the kernel's Link step consults when it resolves type
spellings to canonical identities after Load
([02-symbol-model.md](02-symbol-model.md)).

### Members

The effective member set of any type-like symbol — its own
members plus what arrives through Embeds, Extends, and
Implements, resolved through Link identities. The first question
every doubling generator asks ("the full contract of this
interface"), and unanswerable outside the language contract:
promotion, override, merge, and MRO are language rules (Go
promotes, Java overrides, TS interfaces merge, Python
linearises), and a generator re-deriving any of them re-derives
it wrong for the next language.

A `MemberSet` carries, per member, the supertype or embed it
arrived through — and, per contributor that yielded nothing, the
reason: unresolved, not a membered type, cyclic, or generic. The
reasons are load-bearing: a mock generator must say "this double
misses the embedded half, and here is why" instead of silently
shipping less than it appears to. Reads flow through the tracked
Reader, so member resolution is an incrementality edge like any
other read ([09-incrementality.md](09-incrementality.md)).

### Values

Value synthesis — what every check-emitting generator lives on:

- `SamplesOf` answers **two distinct values** of a type. Two rather
  than one, because a check comparing against a single value passes
  whenever the subject already held it, and what it held is not
  always knowable. The hint names the declaration the value belongs
  to, so a value in a failure message says where it came from.
- `ZeroLiteral` spells the type's zero — what a declared default is
  compared against, and the spellings differ per language (`nil`
  against `None`), so a kernel table would be answering for one.
- `LiteralFor` renders text as a literal of a type, reporting false
  when it cannot be one — for values read from carriers that already
  consumed the language's quoting, where bare text is the right
  literal for some types and not others.

All three refuse rather than guess: a type the language cannot
reason about answers not-OK, never a zero value to compare against.

Authored values outrank derivation. The kernel registers the
`gen.sample`, `gen.alternate`, and `gen.witness` keys and owns
the two directives that stamp them
([05-directives.md](05-directives.md)); `SamplesOf` and Tier-2
`Witnesses` consult the stamps before deriving, so every consumer
gets the authored answer without knowing any annotator exists.
The seam is contract, not convention: a preference each generator
has to remember is a preference two of them will forget.

### Naming

Case conventions and identifier joins: how a generator's word joins
an author's name in this language. The generator owns the word
("Builder", "Factory"); the language owns the join.

## Tier 2 — declared by satisfying, found by asserting

A language declares an optional capability by implementing the
interface; a consumer asserts and, refused, reports once and
generates nothing — better than a projection built from a default
nobody chose. Required instead, these would be methods a language
without the concept has to answer by refusing.

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
| `EnumRules` | project an enumeration (form, zero, foreign values) | Go const-groups, Java enum classes, TS enums, proto |
| `ErrorValueRules` | sentinel conventions (name ↔ predicate, paired so they cannot drift) | Go |
| `TagRules` | stringly per-field tags | Go struct tags |
| `AnnotationRules` | structured annotations/attributes/decorators, read statically, never executed | Java/Kotlin annotations, Rust attributes, Python/TS decorators |
| `GenericsRules` | witnesses, constraint reasoning, reified-vs-erased | all generic languages; erasure matters to a Java backend |
| `PropertyRules` | computed properties view (getter/setter pairing, beans) — a projection, never a model mutation | Kotlin, C#, Swift, Python `@property`, JavaBeans |
| `ConstructRules` | constructors and instantiation spelling | JVM/TS/Python real ctors; Go/Rust conventions |
| `ThrowsRules` | declared throws lists | Java checked exceptions, Swift typed throws |
| `OwnershipRules` | by-value / borrow / mut-borrow on params | Rust only |
| `PromotionRules` | settable members incl. promotion/embedding | Go embeds |
| `EqualityRules` | is this type usable where the language demands equality (map key, switch, dedup) — and if not, which member refs break it | Go comparability (slices/maps/funcs poison), Java equals/hashCode presence, Python hashability |

## Tier 3 — the language sdk

Per-language questions, importable only from a plugin's
language-binding files. Go cannot enforce a cross-module import rule,
so the conformance suite ships the lint that does
([13-testing-and-conformance.md](13-testing-and-conformance.md)).
Tier 3 is where language-only vocabulary lives legally — present,
bounded, and unable to leak into neutral code unnoticed.

## The degradation ladder

Every source construct lands on exactly one rung:

1. **Project fully** into Tier-1/2 vocabulary.
2. **Project partially**, the remainder carried as language metadata
   ([04-metadata.md](04-metadata.md)).
3. **Opaque shape + language metadata** — representable, not
   projectable (Rust lifetimes, TS conditional types, Python
   metaclasses).
4. **Refusal at lowering** with a stable diagnostic code — the only
   rung on the write side, and it never guesses.

Nothing is unrepresentable; some things are un-lowerable; every
failure is a positioned diagnostic. The completeness rung
([13-testing-and-conformance.md](13-testing-and-conformance.md))
verifies every feature-matrix row of every language lands on its
declared rung.

## Generics, split across the tiers

- **Structure — Tier 1**: `TypeParams` with variance and
  bounds-as-refs on the symbol model. Every anticipated language
  answers it.
- **Reasoning — Tier 2** (`GenericsRules`): witnesses (one concrete
  type per parameter, all-or-nothing — a witness for one parameter is
  worth nothing without the rest), substitution, and the
  reified-vs-erased fact. Witnesses are usually *authored*, not
  derived: a language can derive one only where the constraint's
  type set is knowable without loading the package that declares
  it (Go: `any` and `comparable`, nothing else), so `Witnesses`
  answers the `gen.witness` stamps first and derivation is the
  narrow fallback.
- **The unprojectable — metadata**: `rust.lifetimeParams`,
  `go.constraintTypeSet`, `ts.conditionalType`, `kotlin.reified` —
  readable by anyone, consumed in practice by that language's
  bindings, which is Tier 3 doing its job.
