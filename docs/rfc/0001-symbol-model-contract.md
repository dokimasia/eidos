---
rfc: 0001
title: The symbol schema and its contract
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-08-30
updated: 2026-08-30
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0002, ADR-0003
---

# RFC-0001: The symbol schema and its contract

## Summary

This RFC pins the concrete contract that the symbol model
specification gives in outline: the hand-written `symbol/` vocabulary
(the `Kind` type, the walk interfaces, `Identity`, the auxiliary
enums, the `position` leaf package), the complete per-kind field
inventory as the `symbol/schema` source, and the tag vocabulary that
drives generation. Everything in `node/` and `emit/` generates from
this schema. RFC-0002 proposes the generator itself.

## Motivation

The architecture documents deliberately stop at the vocabulary
level: the symbol model specification names its kinds, lists the
elements the target languages force (variance, member level, default
method bodies, nominal supertypes against embedding), and shows one
worked example. Nobody has written down the full field set per kind,
the exact walk interfaces, or the `Identity` struct, and nothing else
can be built until someone has.

Everything else in the kernel consumes these declarations: the store
and its dispatch, the frontends, every generator and backend. Getting
a field wrong once frontends exist costs a regeneration plus a sweep
over every consumer; getting it wrong here costs a review comment.
The model belongs in the kernel and nowhere else: the kernel is the
slow-moving set, and the model is its slowest-moving part.

## Detailed design

### The position leaf

```go
// Package position holds source positions.
package position

// Pos locates a declaration in the workspace. File is
// workspace-relative and slash-separated on every platform; Line
// and Col are 1-based. The zero Pos means "no source position",
// which is what synthesized emit values carry.
type Pos struct {
    File string
    Line int
    Col  int
}

func (p Pos) IsZero() bool
func (p Pos) String() string // "svc/store.go:41:2"
```

### The symbol vocabulary (hand-written)

```go
// Package symbol is the declaration vocabulary shared by the node
// and emit models.
package symbol

// Kind discriminates the declaration kinds. The type is hand-written;
// the constants, String and ParseKind live in kind.gen.go, generated
// from the schema so that adding a kind is a schema edit and nothing
// else.
type Kind uint8

// ParseKind is String's inverse for every kind, which is what lets a
// kind cross a boundary carrying names rather than values: an
// encoded declaration, a diagnostic, a command-line argument.
func ParseKind(name string) (Kind, bool)

// Visibility is the normalized five-way visibility of declarations.
// The zero value is Unknown: emptiness never claims anything, so a
// frontend that has not returned has not silently returned Public.
// The raw source spelling stays in language metadata.
type Visibility uint8

const (
    VisibilityUnknown Visibility = iota
    VisibilityPublic
    VisibilityPackage
    VisibilityProtected
    VisibilityPrivate
    VisibilityInternal
)

// Level says whether a member belongs to instances or to the type
// itself. The zero value is Instance, which is the answer for every
// Go member and the common case everywhere.
type Level uint8

const (
    LevelInstance Level = iota
    LevelType
)

// Variance is the variance of a type parameter. The zero value is
// Invariant, which is what Go and Rust always carry.
type Variance uint8

const (
    VarianceInvariant Variance = iota
    VarianceIn
    VarianceOut
)

// Mutability says whether a binding may be reassigned after
// initialization. The zero value is Unknown, because a language
// that does not distinguish the two has not returned immutable:
// Kotlin val against var, TypeScript readonly, Java final.
type Mutability uint8

const (
    MutabilityUnknown Mutability = iota
    MutabilityMutable
    MutabilityImmutable
)

// Variadic says how a parameter accepts a variable number of
// arguments. Positional and keyword forms stay separate because
// Python, Ruby and PHP have both, and a delegating call has to
// reproduce the right one.
type Variadic uint8

const (
    VariadicNone Variadic = iota
    VariadicPositional
    VariadicKeyword
)
```

The specification lists four walk interfaces: `Symbol`, `Membered`,
`Typed` and `Documented`. This RFC folds `Documented` into `Symbol`,
because every kind can return `Docs()` and an undocumented kind
returns nil, so neutral code holds one interface instead of two. Go
forbids a field and a method sharing a name, so the interface methods
are named apart from the generated struct fields: `Fields` is the
field, `FieldList()` the method, following `go/ast`.

```go
// Symbol is the least any declaration returns.
type Symbol interface {
    Kind() Kind
    Position() position.Pos // zero for synthesized emit values
    Docs() []string         // nil when the kind carries none
}

// Membered is any kind that carries members: Struct, Interface,
// Enum and Sum satisfy it. The slices hold the side's concrete
// kinds, adapted to []Symbol so neutral code needs no side import.
type Membered interface {
    Symbol
    FieldList() []Symbol
    MethodList() []Symbol
    EmbedList() []Symbol
}

// Typed is any kind whose meaning includes a type reference:
// Field, Param, Return, Variable, Constant and Alias satisfy it.
// The result's Kind is KindTypeRef. It may be nil when the source
// declares no type (an inferred variable, an untyped constant).
type Typed interface {
    Symbol
    TypeRef() Symbol
}
```

The symbol model specification defines a canonical identity as
package path, kind, name and a signature discriminator. ADR-0002 adds
the source language, because a proto package and a Go package can
share a directory in a mixed monorepo, and every worked example in
the architecture documents already spells the language prefix. Two
identities are equal when every field is. Overloads differ only in
`Disc`, and a language that cannot overload writes the empty string:

```go
// Lang is the source-language name as identities, metadata
// namespaces and the sealed state store it. It is a registered
// name: workspace Build validates every spelling against the
// language registry.
type Lang string

// Identity names a declaration across runs, machines and reparses.
// It is the join everything long-lived keys on: read sets, exports,
// manifests, drift and explain.
type Identity struct {
    Lang    Lang // "golang", "protobuf"
    Package string // slash path: "svc/store"
    Owner   string // enclosing type name; "" for top-level symbols
    Name    string
    Kind    Kind
    Disc    string // signature discriminator; "" where overloads cannot exist
}

func (id Identity) IsZero() bool
func (id Identity) String() string
func Parse(s string) (Identity, error)
```

`Lang` is a defined type and the four other string fields are not, on
purpose. A language identity is a registered name: workspace Build
validates every spelling against the language registry, and the
defined type lets the compiler refuse an arbitrary string in the
slot. The projection layer's direction-typed `rules.Source` and
`rules.Target` can share the spelling type without giving up their
distinctness. `Package`, `Owner`, `Name` and `Disc` are open data
that no registry validates, so typing them would be ceremony.

The string grammar, per kind family:

| Kind | Form | Example |
|---|---|---|
| Package | `lang:package` | `golang:svc/store` |
| File | `lang:package/file` | `golang:svc/store/store.go` |
| top-level type, variable, constant | `lang:package.Name` | `golang:svc/store.Store` |
| function | `lang:package.Name(disc)` | `golang:svc/store.Open(string)` |
| member field, enum or sum variant | `lang:package.Owner#Name` | `golang:svc/store.Store#timeout` |
| method | `lang:package.Owner#Name(disc)` | `golang:svc/store.Store#Get(ctx,string)` |

Callable identities always carry the parenthesized discriminator,
even when empty (`#Close()`), so a field and a nullary method with
one name, which Java allows, spell differently. `Parse` accepts
exactly these forms. Child kinds (Param, Return, TypeParam, TypeRef,
Embed, Constraint) take identities derived from their parent; the
exact spelling is an open question below, and nothing in this
proposal needs it.

### The schema

`symbol/schema` is the single hand-written definition of the kinds.
It contains plain structs, one per kind, plus one marker:

```go
// Symbol marks a field that holds any declaration kind. The
// generator maps it to symbol.Symbol on both sides.
type Symbol interface{}
```

The tag vocabulary is closed: `eidos:"side[,walk][,slot=name]"`
with side one of `both`, `node`, `emit`. An unknown token fails
generation with the schema position; RFC-0002 carries the full
validation list. Three fields follow conventions rather than tags:

- `ID symbol.Identity` is node-side on every kind. It is zero until a
  frontend assigns it, and zero claims nothing. Assignment belongs to
  the frontends and the Link step, outside this proposal.
- `Origin symbol.Identity` is emit-side on every declaration-shaped
  kind: the node symbol an emit value derives from. Origin points one
  way, and no node field refers to emit, which is the symbol model's
  one-way origin rule.
- `Host symbol.Identity` is node-side on every owned kind: the
  identity of the declaration that holds it, set when a frontend
  creates the child. It is an identity rather than a pointer for the
  reason `TypeRef.Target` is, and reaching the owner therefore goes
  through a tracked read (ADR-0006).

The full inventory follows. Kinds group by family, one file each
under `symbol/schema`, and the documentation lives with the
declarations:

```go
// The marker that types heterogeneous fields.

type Symbol any

// Containers and module boundaries.

type Package struct {
    ID    symbol.Identity `eidos:"node"`
    Pos   position.Pos    `eidos:"node"`
    Doc   []string        `eidos:"both"`
    Path  []string        `eidos:"both"` // ["svc","store"]
    Name  string          `eidos:"both"` // may differ from the last segment
    Files []*File         `eidos:"node,walk"`
}

type File struct {
    ID      symbol.Identity `eidos:"node"`
    Pos     position.Pos    `eidos:"node"`
    Doc     []string        `eidos:"both"`
    Path    string          `eidos:"both"`      // workspace-relative, slash-separated
    Imports []*Import       `eidos:"node,walk"` // the file's import scope, as written
    Exports []*Export       `eidos:"node,walk"` // re-exports; a declaration's own export is its Visibility
    Decls   []Symbol        `eidos:"node,walk"`
}

type Import struct {
    ID       symbol.Identity `eidos:"node"`
    Pos      position.Pos    `eidos:"node"`
    Path     string          `eidos:"node"`
    Alias    string          `eidos:"node"`      // module-level alias; "" when unaliased
    Names    []*Binding      `eidos:"node,walk"` // per-symbol bindings
    Default  string          `eidos:"node"`      // local name for the module's default export
    Wildcard bool            `eidos:"node"`      // every exported name enters scope
}

type Export struct {
    ID       symbol.Identity `eidos:"node"`
    Pos      position.Pos    `eidos:"node"`
    Doc      []string        `eidos:"node"`
    Path     string          `eidos:"node"`      // source module; "" when re-exporting local names
    Names    []*Binding      `eidos:"node,walk"` // per-symbol bindings
    Default  string          `eidos:"node"`      // name published as the module's default export
    Wildcard bool            `eidos:"node"`      // every name of Path is republished
}

type Binding struct {
    ID    symbol.Identity `eidos:"node"`
    Pos   position.Pos    `eidos:"node"`
    Name  string          `eidos:"node"`
    Alias string          `eidos:"node"` // "" when unrenamed
    Host  symbol.Identity `eidos:"node"`
}

// Structural type declarations.

type Struct struct {
    ID         symbol.Identity   `eidos:"node"`
    Origin     symbol.Identity   `eidos:"emit"`
    Pos        position.Pos      `eidos:"node"`
    Doc        []string          `eidos:"both"`
    Name       string            `eidos:"both"`
    Visibility symbol.Visibility `eidos:"both"`
    Abstract   bool              `eidos:"both"` // no value of it can be made directly
    Final      bool              `eidos:"both"` // subclassing is forbidden
    TypeParams []*TypeParam      `eidos:"both,walk"`
    Fields     []*Field          `eidos:"both,walk,slot=fields"`
    Methods    []*Method         `eidos:"both,walk,slot=methods"`
    Types      []Symbol          `eidos:"both,walk,slot=types"` // nested declarations
    Embeds     []*Embed          `eidos:"both,walk"`            // compositional promotion
    Extends    []*TypeRef        `eidos:"both,walk"`            // nominal supertypes
    Implements []*TypeRef        `eidos:"both,walk"`
}

type Interface struct {
    ID         symbol.Identity   `eidos:"node"`
    Origin     symbol.Identity   `eidos:"emit"`
    Pos        position.Pos      `eidos:"node"`
    Doc        []string          `eidos:"both"`
    Name       string            `eidos:"both"`
    Visibility symbol.Visibility `eidos:"both"`
    TypeParams []*TypeParam      `eidos:"both,walk"`
    Fields     []*Field          `eidos:"both,walk,slot=fields"` // properties, not just methods
    Methods    []*Method         `eidos:"both,walk,slot=methods"`
    Types      []Symbol          `eidos:"both,walk,slot=types"` // nested declarations and associated types
    Embeds     []*Embed          `eidos:"both,walk"`
    Extends    []*TypeRef        `eidos:"both,walk"`
}

type Alias struct {
    ID         symbol.Identity   `eidos:"node"`
    Origin     symbol.Identity   `eidos:"emit"`
    Pos        position.Pos      `eidos:"node"`
    Doc        []string          `eidos:"both"`
    Name       string            `eidos:"both"`
    Visibility symbol.Visibility `eidos:"both"`
    TypeParams []*TypeParam      `eidos:"both,walk"`
    Target     *TypeRef          `eidos:"both,walk"` // nil for an associated type
}

// Enumerated type declarations.

type Enum struct {
    ID         symbol.Identity   `eidos:"node"`
    Origin     symbol.Identity   `eidos:"emit"`
    Pos        position.Pos      `eidos:"node"`
    Doc        []string          `eidos:"both"`
    Name       string            `eidos:"both"`
    Visibility symbol.Visibility `eidos:"both"`
    Variants   []*EnumVariant    `eidos:"both,walk,slot=variants"`
    Fields     []*Field          `eidos:"both,walk,slot=fields"`  // Java enums carry instance state
    Methods    []*Method         `eidos:"both,walk,slot=methods"` // and behaviour
}

type EnumVariant struct {
    ID     symbol.Identity `eidos:"node"`
    Origin symbol.Identity `eidos:"emit"`
    Pos    position.Pos    `eidos:"node"`
    Doc    []string        `eidos:"both"`
    Name   string          `eidos:"both"`
    Value  string          `eidos:"both"` // source spelling, unevaluated
    Host   symbol.Identity `eidos:"node"`
}

type Sum struct {
    ID         symbol.Identity   `eidos:"node"`
    Origin     symbol.Identity   `eidos:"emit"`
    Pos        position.Pos      `eidos:"node"`
    Doc        []string          `eidos:"both"`
    Name       string            `eidos:"both"`
    Visibility symbol.Visibility `eidos:"both"`
    TypeParams []*TypeParam      `eidos:"both,walk"` // Rust data enums are generic
    Variants   []*SumVariant     `eidos:"both,walk,slot=variants"`
    Methods    []*Method         `eidos:"both,walk,slot=methods"`
}

type SumVariant struct {
    ID     symbol.Identity `eidos:"node"`
    Origin symbol.Identity `eidos:"emit"`
    Pos    position.Pos    `eidos:"node"`
    Doc    []string        `eidos:"both"`
    Name   string          `eidos:"both"`
    Fields []*Field        `eidos:"both,walk,slot=fields"` // the payload; unnamed when positional
    Host   symbol.Identity `eidos:"node"`
}

// Callables and parameters.

type Function struct {
    ID         symbol.Identity   `eidos:"node"`
    Origin     symbol.Identity   `eidos:"emit"`
    Pos        position.Pos      `eidos:"node"`
    Doc        []string          `eidos:"both"`
    Name       string            `eidos:"both"`
    Visibility symbol.Visibility `eidos:"both"`
    TypeParams []*TypeParam      `eidos:"both,walk"`
    Params     []*Param          `eidos:"both,walk"`
    Returns    []*Return         `eidos:"both,walk"`
}

type Method struct {
    ID         symbol.Identity   `eidos:"node"`
    Origin     symbol.Identity   `eidos:"emit"`
    Pos        position.Pos      `eidos:"node"`
    Doc        []string          `eidos:"both"`
    Name       string            `eidos:"both"`
    Visibility symbol.Visibility `eidos:"both"`
    Level      symbol.Level      `eidos:"both"`
    Abstract   bool              `eidos:"both"`      // no body; a subtype must supply one
    Final      bool              `eidos:"both"`      // overriding is forbidden
    Override   bool              `eidos:"both"`      // replaces a supertype's member
    HasDefault bool              `eidos:"both"`      // an interface method with a body
    Receiver   *Param            `eidos:"both,walk"` // nil where the receiver is implicit
    Receives   *TypeRef          `eidos:"both,walk"` // set when declared outside the type it attaches to
    TypeParams []*TypeParam      `eidos:"both,walk"`
    Params     []*Param          `eidos:"both,walk"`
    Returns    []*Return         `eidos:"both,walk"`
    Host       symbol.Identity   `eidos:"node"`
}

type Param struct {
    ID       symbol.Identity `eidos:"node"`
    Pos      position.Pos    `eidos:"node"`
    Name     string          `eidos:"both"` // "" when unnamed
    Label    string          `eidos:"both"` // caller-facing name; Swift and Objective-C
    Type     *TypeRef        `eidos:"both,walk"`
    Default  string          `eidos:"both"` // source spelling, unevaluated; "" when none
    Variadic symbol.Variadic `eidos:"both"` // positional or keyword
}

type Return struct {
    ID   symbol.Identity `eidos:"node"`
    Pos  position.Pos    `eidos:"node"`
    Name string          `eidos:"both"` // Go named results; "" elsewhere
    Type *TypeRef        `eidos:"both,walk"`
}

// Members and bindings.

type Field struct {
    ID         symbol.Identity   `eidos:"node"`
    Origin     symbol.Identity   `eidos:"emit"`
    Pos        position.Pos      `eidos:"node"`
    Doc        []string          `eidos:"both"`
    Name       string            `eidos:"both"` // "" when positional
    Visibility symbol.Visibility `eidos:"both"`
    Level      symbol.Level      `eidos:"both"`
    Mutability symbol.Mutability `eidos:"both"`
    Type       *TypeRef          `eidos:"both,walk"`
    Host       symbol.Identity   `eidos:"node"`
}

type Variable struct {
    ID         symbol.Identity   `eidos:"node"`
    Origin     symbol.Identity   `eidos:"emit"`
    Pos        position.Pos      `eidos:"node"`
    Doc        []string          `eidos:"both"`
    Name       string            `eidos:"both"`
    Visibility symbol.Visibility `eidos:"both"`
    Mutability symbol.Mutability `eidos:"both"`
    Type       *TypeRef          `eidos:"both,walk"` // nil when the source states none
}

type Constant struct {
    ID         symbol.Identity   `eidos:"node"`
    Origin     symbol.Identity   `eidos:"emit"`
    Pos        position.Pos      `eidos:"node"`
    Doc        []string          `eidos:"both"`
    Name       string            `eidos:"both"`
    Visibility symbol.Visibility `eidos:"both"`
    Type       *TypeRef          `eidos:"both,walk"` // nil when untyped
    Value      string            `eidos:"both"`      // source spelling, unevaluated
}

// Type machinery.

type TypeRef struct {
    ID       symbol.Identity `eidos:"node"`
    Pos      position.Pos    `eidos:"node"`
    Spelling string          `eidos:"both"` // source text, verbatim
    Target   symbol.Identity `eidos:"both"` // zero until resolution, and for builtins and externals
    Args     []*TypeRef      `eidos:"both,walk"`
}

type TypeParam struct {
    ID           symbol.Identity `eidos:"node"`
    Pos          position.Pos    `eidos:"node"`
    Name         string          `eidos:"both"`
    Variance     symbol.Variance `eidos:"both"`
    Bounds       []*TypeRef      `eidos:"both,walk"`
    Default      *TypeRef        `eidos:"both,walk"` // default type argument; nil when none
    Const        bool            `eidos:"both"`      // the argument is a value, not a type
    Type         *TypeRef        `eidos:"both,walk"` // the value's type, when Const
    DefaultValue string          `eidos:"both"`      // default value spelling, when Const
}

type Constraint struct {
    ID    symbol.Identity `eidos:"node"`
    Pos   position.Pos    `eidos:"node"`
    Terms []*TypeRef      `eidos:"both,walk"` // the projectable terms
}

type Embed struct {
    ID   symbol.Identity `eidos:"node"`
    Pos  position.Pos    `eidos:"node"`
    Ref  *TypeRef        `eidos:"both,walk"`
    Host symbol.Identity `eidos:"node"`
}
```

The schema deliberately carries no metadata accessor, no directive
storage and no emit body model. Those seams belong to the metadata,
directive and rendering designs, and their absence forecloses
nothing: adding a field is a schema edit plus a regeneration, which
is the model rule this whole arrangement exists to keep.

### The generated surface

Per side, from the schema above; RFC-0002 owns how:

```go
// node/kinds.gen.go, emit/kinds.gen.go: one struct per kind with
// that side's fields, plus interface satisfaction:
func (x *Struct) Kind() symbol.Kind
func (x *Struct) Position() position.Pos // emit side returns the zero Pos
func (x *Struct) Docs() []string
func (x *Struct) FieldList() []symbol.Symbol  // Membered, adapted
func (x *Field) TypeRef() symbol.Symbol       // Typed

// walk.gen.go, per side: depth-first over walk-tagged fields, in
// schema order. Returning false prunes the subtree.
func Walk(s symbol.Symbol, visit func(symbol.Symbol) bool)
func All(s symbol.Symbol) iter.Seq[symbol.Symbol] // the range-over-func wrapper

// json.gen.go, per side: kind-discriminated round-trip.
// {"kind":"Struct","name":"Store",...}; identities encode as
// objects, and an owner identity encodes like any other field.
func EncodeJSON(s symbol.Symbol) ([]byte, error)
func DecodeJSON(data []byte) (symbol.Symbol, error)

// emit/slots.gen.go: slot-tagged fields become typed slot storage
// with accessors; appending the wrong kind fails at compile time.
func (x *Struct) FieldsSlot() *Slot[*Field]
func (x *Struct) MethodsSlot() *Slot[*Method]
```

`Slot[T]` itself is a minimal hand-written container: append and
items, nothing else. Ordering, provenance and collision semantics
belong to the rendering design, which is where they mean something.

On the emit side a `slot=` field replaces the plain slice with slot
storage, so node gets `Fields []*Field` and emit gets the accessor
pair; the walk and JSON code on the emit side read the slot's items.

## Alternatives considered

### A. One merged model instead of two generated ones

The symbol model specification already argues and refuses this: a
merged model loses compile-time phase discipline and multi-plan
safety, and embedding fails under covariant recursion. This RFC only
pins the concrete shape of the decision already made.

### B. Typed member access on the interfaces

`Membered` returning `[]*node.Field` per side. The interface then
cannot live in `symbol/`, because Go interfaces cannot vary a
result's element type per implementation, and neutral code written
once against `symbol/` is the entire point of the walk interfaces
(differs, codecs, audits). The `[]Symbol` adapter costs one slice
allocation per call; code on a hot path walks the concrete structs
instead.

### C. A separate Documented interface

The symbol model specification lists it among the walk interfaces.
Implementing it as a fourth interface means every neutral consumer
type-asserts twice for a question every kind can return. Folding
`Docs()` into `Symbol` with nil for undocumented kinds keeps one
assertion, and loses nothing: nil already means "no docs".

### D. Kind as part of the identity string

Spellings like `golang:svc/store.Store~struct#Get(ctx,string)` would
round-trip the whole struct through `Parse`. Every worked example in
the architecture documents spells identities without a kind token,
manifests already use that form, and the kind is recoverable from the
store when a parsed identity is looked up. The grammar above keeps
the documented spellings; the struct keeps `Kind` for equality.

## Drawbacks

- 23 kind structs generate on each side: roughly 3,000 lines of
  committed generated Go across ten `*.gen.go` files. Reviewers see
  them in every schema-touching diff; the mirror guard keeps them
  honest.
- The `[]Symbol` adapters on `Membered` and `Typed` allocate a slice
  per call. Neutral tooling carries it; hot paths use the concrete
  structs and cost nothing.
- `Identity` at six fields is wide for a value the engine uses as a
  map key throughout. The engine's interning replaces it with dense
  run-local IDs; the struct is the boundary form.
- The metadata, directive and emit-body seams are absent by design,
  so each arrives as a schema edit that regenerates both sides. The
  regeneration is the designed cost, and whoever reviews those
  changes reads the diff noise it makes.

## Open questions

- Should `Membered` also cover `Package` and `File` (containers of
  declarations), or stay member-of-a-type as proposed?
- What is the identity spelling for child kinds (Param, TypeParam,
  TypeRef, Embed, Constraint)? Nothing in this proposal needs one;
  the frontend work that assigns identities is the place to settle
  it, next to what explain and read-set records need.
- Does `Enum` need an underlying-type field (`Base *TypeRef`) for
  C#-style `enum : byte` and proto value types, or does that stay in
  language metadata until a satellite forces it?
- `Method.Override` is a keyword in Kotlin, C#, Swift and
  TypeScript and an annotation in Java. Should a Java frontend stamp
  the field anyway from `@Override`, so neutral consumers get one
  answer, or leave it false and let the annotation speak?
- Does `File` need a `Doc` distinct from `Package` docs for every
  language, or only for some (Python module docstrings against Go
  package comments)? The schema proposes both carry `Doc`.

## Unresolved and future work

- The metadata accessor, directive storage and the emit body model
  (`Body`, `Expr`, `Stmt`, `Target`, full `Slot` semantics) are
  follow-up schema edits owned by the metadata, directive and
  rendering designs; nothing here fixes their shape.
- The `schema.Symbol` marker could grow per-kind-set markers (for
  example "type-shaped kinds only") if heterogeneous fields need
  narrowing; nothing in this proposal does.

## References

- [02-symbol-model.md](../architecture/02-symbol-model.md), the
  symbol model specification this contract pins
- [03-projection.md](../architecture/03-projection.md), the
  projection tiers and `rules.Source` / `rules.Target`
- [00-one-declaration-end-to-end.md](../architecture/00-one-declaration-end-to-end.md)
  and
  [17-output-and-determinism.md](../architecture/17-output-and-determinism.md),
  the worked identity spellings
- [21-decisions.md](../architecture/21-decisions.md), the decision
  log (D1, D4, D44, D56)
- [RFC-0002](0002-model-generator.md), the model generator
- [ADR-0002](../adr/0002-identity-carries-source-language.md),
  identity carries the source language
- [ADR-0003](../adr/0003-json-codecs-round-trip.md), JSON codecs
  round-trip
