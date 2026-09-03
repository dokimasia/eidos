---
rfc: 0008
title: The emit body and the scaffolding vocabulary
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Review
created: 2026-08-30
updated: 2026-08-30
discussion: none
supersedes: none
superseded-by: none
produces-adr: tbd
---

# RFC-0008: The emit body and the scaffolding vocabulary

## Summary

This RFC grows the emit side of the symbol model with the one thing
rendering needs and the model does not carry: what goes inside a
callable. `Function` and `Method` gain an emit-only `Body`. A body
is a slot sequence with content in the middle: `prologue` and
`epilogue` slots exist on every body by construction, the owner may
declare named slots between them, and the content is exactly one of
four forms: nothing, scaffolding statements, a template reference,
or verbatim text. The scaffolding vocabulary is deliberately small:
a name, a call, a return, an assignment, and a failure guard, which
is enough to delegate and too little to write logic in. `Body`,
`Stmt`, `Expr` and `TemplateRef` are hand-written emit types beside
`Slot`, carried by a generated field, so the schema stays the one
place that says which kinds have bodies.

## Motivation

The emit model carries signatures and never content: a generated
`Function` today has parameters, returns and documentation, and no
way to say what it does. Rendering needs that answer, and it needs
it in a shape three other mechanisms can hold on to:

- **Composition needs standard slots.** A cross-cutting plugin
  appends an audit call into another plugin's handler body without
  either knowing the other exists. That only works if the extension
  points exist whether the owner anticipated them or not, which is
  what `prologue` and `epilogue` on every body give.
- **The lint check needs a closed shape.** A body whose content is
  one of four declared forms can be checked: a template reference
  resolves or it does not, a scaffolding statement is inside the
  vocabulary or refused at compile time, and verbatim text is
  visibly the form the machinery cannot see into. Nothing can check
  free-shaped content at all.
- **Determinism needs values, not functions.** A generator that
  names a template instead of executing one stays never seeing
  languages, and the same emit graph renders through a Go tree in
  one plan and a TypeScript tree in another. That requires the
  reference to be a plain value on the model.

This RFC draws its boundary at interfaces and stops there. The
vocabulary covers a handler calling the user's function and a stub
recording its arguments. It covers no loops, no arithmetic and no
comparisons, because anything past delegation belongs in a
hand-written file that the generated one calls.

## Detailed design

### The schema declares which kinds have bodies

`Body` joins the schema's marker vocabulary the way `Symbol` did: a
named type the generator maps rather than a structure it walks. The
two callable kinds carry it as an emit-only field, the same side
mechanism `Origin` already uses:

```go
package schema

// Body marks a field that holds a callable's emit-side content.
// The generator maps it to the emit model's Body value; the node
// model never carries one, because parsed bodies are out of scope
// and the signature is all the node side carries.
type Body any
```

```go
// In callable.go, on Function and on Method:
Body Body `eidos:"emit"`
```

Three things follow from that field. Each callable's emit struct
gains a `Body` under the usual omit-when-zero JSON tag. The walk
does not descend into it, because a body holds statements rather
than declarations. And the schema stays the one place that returns
which kinds have bodies.

The callables' shared docblocks change in the same edit. Today they
state that the model never carries a body. That stays true of the
node twin and stops being true of the emit twin, and one schema
comment generates into both. Giving another kind a body is one
schema field plus a regeneration.

### The body: two standard slots, named slots, one content form

`Body` is a hand-written emit type beside `Slot`, a plain value
with codecs and no behaviour beyond its accessors:

```go
package emit

// Body is a callable's emit-side content: the standard slots every
// body carries, the named slots its owner declared, and the content
// between them. The zero Body is the kind template's default:
// nothing but the standard slots, rendered automatically.
type Body struct {
    // Prologue and Epilogue exist on every body by construction:
    // the extension points a cross-cutting plugin appends to
    // whether the owner anticipated it or not.
    Prologue Slot[Stmt] `json:"prologue,omitzero"`
    Epilogue Slot[Stmt] `json:"epilogue,omitzero"`
    // Slots holds the owner's declared extension points, in
    // declaration order, rendered between the standard pair. The
    // elements are pointers because Declare returns handles into
    // the list, and a following declaration must not move what an
    // earlier caller already holds.
    Slots []*NamedSlot `json:"slots,omitzero"`

    // The content, exactly one form set. All three zero is the
    // default form. Stmts is scaffolding from the vocabulary
    // below; Ref claims the body for a template in the emitting
    // plugin's tree; Verbatim is literal text.
    Stmts    []Stmt       `json:"stmts,omitzero"`
    Ref      *TemplateRef `json:"ref,omitzero"`
    Verbatim string       `json:"verbatim,omitzero"`
}

// NamedSlot is one owner-declared extension point.
type NamedSlot struct {
    Name string     `json:"name"`
    Slot Slot[Stmt] `json:"stmts,omitzero"`
}

// Declare returns the named slot, adding it in declaration order
// on first use; declaring a name twice returns the existing slot.
// Declaring is the owner's act: a contributor looks a slot up.
func (b *Body) Declare(name string) *Slot[Stmt]

// Slot returns a declared slot and false for a name the owner
// never declared, so a contribution into an invented extension
// point fails where it is made.
func (b *Body) Slot(name string) (*Slot[Stmt], bool)

// Form returns which content form the body holds, and an error
// naming the forms where more than one is set: a body built with
// two contents is a defect, and the render and the lint check both
// ask this one question.
func (b *Body) Form() (Form, error)

// IsZero reports whether the body holds nothing at all, which is
// what lets the encoder omit an untouched one.
func (b Body) IsZero() bool

// Form names a body's content form.
type Form uint8

const (
    // FormDefault is nothing: the kind template renders the
    // standard slots and no content of its own.
    FormDefault Form = iota
    // FormStmts is scaffolding from the neutral vocabulary.
    FormStmts
    // FormTemplate claims the body for the emitting plugin's
    // template.
    FormTemplate
    // FormVerbatim is literal text.
    FormVerbatim
)
```

Verbatim exists only here. No declaration-level verbatim value
exists, so raw text between declarations stays impossible by
construction rather than by review. The slot machinery cannot check
this content and the import resolver cannot see it, and confining
it to a body means only a caller who asked for exactly that gets
it.

### The template reference

```go
// TemplateRef claims a body for a template. The name resolves in
// the emitting plugin's template tree for the plan's target, never
// in the backend's or another plugin's, so the same emit graph renders
// through a different tree per plan. Data is the plugin-supplied
// payload the template executes over; the codec carries it as
// generic JSON values, so a decoded reference reads its payload
// dynamically, which is how a template reads it anyway.
type TemplateRef struct {
    Name string `json:"name"`
    Data any    `json:"data,omitzero"`
}
```

### The scaffolding vocabulary

Statements and expressions are closed unions spelled the way the
validated directive value already is: one struct, a kind field, and
the fields that kind populates, so a consumer switches on the kind
and reads without an assertion.

```go
// StmtKind selects a statement's populated fields.
type StmtKind uint8

const (
    // StmtReturn returns Value, or returns bare when Value is
    // zero.
    StmtReturn StmtKind = iota + 1
    // StmtAssign binds Value to Names, one target or several,
    // because a delegate returning a result and a failure binds
    // two; Declare introduces the names.
    StmtAssign
    // StmtExpr evaluates Value for its effect: a delegate call.
    StmtExpr
    // StmtGuard runs Then when the value bound to Name reads as a
    // failure under the target language's convention. A language
    // whose failure propagates natively renders the guard as that
    // propagation, which may be nothing at all.
    StmtGuard
)

// Stmt is one scaffolding statement.
type Stmt struct {
    Kind StmtKind `json:"kind"`
    // Name is the guarded name; Names are the assignment targets.
    Name    string   `json:"name,omitzero"`
    Names   []string `json:"names,omitzero"`
    Value   Expr     `json:"value,omitzero"`
    Declare bool     `json:"declare,omitzero"`
    Then    []Stmt   `json:"then,omitzero"`
}

// ExprKind selects an expression's populated fields.
type ExprKind uint8

const (
    // ExprName is a local spelling: a parameter, a receiver, a
    // package-local callable, a dotted path onto one. Nothing
    // resolves it and nothing imports for it; a name that needs
    // qualification is a type spelling, which is the lowering
    // seam's job, not this one's.
    ExprName ExprKind = iota + 1
    // ExprCall applies Fn to Args.
    ExprCall
    // ExprValue carries a Value: a sample, an alternate or a zero
    // the projection derived, spelled by the target's own scaffold
    // and qualified for the file it is written into. Added by
    // RFC-0014; it is the one expression that resolves anything.
    ExprValue
)

// Expr is one scaffolding expression. The zero Expr names
// nothing, which is what a bare return carries. Fn is the one
// pointer in the vocabulary, because a struct cannot hold itself.
type Expr struct {
    Kind ExprKind `json:"kind"`
    Name string   `json:"name,omitzero"`
    Fn   *Expr    `json:"fn,omitzero"`
    Args []Expr   `json:"args,omitzero"`
    Val  *Value   `json:"val,omitzero"` // ExprValue
}
```

The vocabulary is complete for delegation and deliberately
incomplete for everything else: no literals, no comparisons, no
loops, no conditionals beyond the failure guard. A generated file
that wants logic calls a hand-written one.

### What this deliberately does not carry

The rendering of any of it: how a Go backend spells a guard, where
slot contents order, what the `{{slots}}` marker demands of a
body-claiming template, and the lint that enforces it are the
render pass's contracts, stated where that pass is proposed. This
RFC also carries no `Sample` and no literal expression: both are
the rendered form of projection returns, and no projection
machinery exists to return them; adding a literal kind is one more
`ExprKind` constant beside the two that are here.

Slot content ordering at render, capability topology then name, is
likewise recorded where rendering is: a slot stores insertion
order, and canonical order is the reader's rule, the same split the
emit store's units already follow.

## Alternatives considered

### Content as a closed interface union

A `Content` interface with `Stmts`, `TemplateRef` and `Verbatim`
implementations models "exactly one form" in the type system. It
lost because the emit model is plain values with JSON codecs
throughout, and an interface field forces polymorphic encoding with
a type discriminator the flat struct carries anyway.
The kind-field union is the same shape the validated directive
value uses, checked at one accessor instead of by the type system,
and the cost of that check is stated in Drawbacks.

### Body as a schema-declared kind

Declaring `Body`, `Stmt` and `Expr` in the schema would generate
their codecs for free. It lost because the schema generates node
and emit twins from every declaration, and these types are
emit-only whole: the node twin would be a struct nobody may
construct, and suppressing it takes a kind-level side marker the
generator does not have and nothing else needs. The per-field side
mechanism already exists, `Origin` uses it, so the field generates
and the types stay hand-written, the same split `Slot` already
lives on.

### A richer statement vocabulary

Loops, conditionals over comparisons, and literals would let
generated bodies carry real logic. It lost on two counts. Every
statement kind is a rendering obligation on every backend forever,
and nobody can debug generated logic where it was written. The
boundary is delegation: a body that needs more than that belongs in
a hand-written file.

### Declaration-level verbatim

A free-floating text value among typed declarations would make
claiming a whole file unnecessary for one odd construct. It lost
because the slot machinery, the collision logic and the import
resolver would all be never seeing such a value, and the file-claim
path already serves a plugin that wants that much control.

### Named markers without standard slots

Only owner-declared slots, no `prologue` and `epilogue`, keeps the
body smaller. It lost because the owner would then have to
anticipate every extension point, and a cross-cutting plugin is
written by the one author who cannot ask the owner for one.

## Drawbacks

- Four more hand-written types with codecs beside the generated
  model: `Body`, `NamedSlot`, `TemplateRef` and the two unions,
  roughly three source files and their test twins.
- The exactly-one-form rule is checked where `Form` is asked, not
  where the body is built: a two-form body is a defect the render
  or the lint check reports, and nothing at append time refuses it.
- `TemplateRef.Data` is `any`: the codec round-trips it as generic
  JSON values, so a decoded reference does not return the author's
  concrete type. Templates read their payload dynamically either
  way, and `encoding/json` orders map keys, so the bytes stay
  deterministic.
- The failure guard is convention-shaped: what "reads as a failure"
  means belongs to each backend, and a language whose failures
  propagate natively may render the guard as nothing. A plugin that
  needs the guard's consequence to run must read its backend's
  contract for it.
- A multi-name assignment is a rendering obligation: a backend
  whose language cannot destructure the binding refuses the
  statement at render, positioned, rather than inventing temporary
  names the author never wrote.

## Open questions

- Should `Declare` refuse a duplicate name instead of returning the
  existing slot? Idempotence is friendlier to builders that declare
  in a loop; refusal catches two owners fighting over one name.
  The proposal returns the existing slot.
- Do property accessors want bodies? The two callable kinds cover
  every emitter the fixture suites hold; a getter with content is
  one schema field on the member kind when a language demands it.

## Unresolved and future work

- The render pass, the backend kit, the `{{slots}}` marker rule and
  the template-lint check consume these values and are proposed
  separately.
- `Sample` and a literal expression kind wait on the projection
  machinery whose returns they render.

## References

- [07-rendering.md](../architecture/07-rendering.md), the verbs,
  the body forms and the render-dispatch order this vocabulary
  serves
- [02-symbol-model.md](../architecture/02-symbol-model.md), the
  schema, the side tags and the slot declarations
- [11-languages.md](../architecture/11-languages.md), the backend
  kit that executes template references
- [13-testing-and-conformance.md](../architecture/13-testing-and-conformance.md),
  the template-lint check that reads the closed shape
- [RFC-0014: The projection vocabulary](0014-the-projection-vocabulary.md),
  which adds the one expression kind carrying a derived value
