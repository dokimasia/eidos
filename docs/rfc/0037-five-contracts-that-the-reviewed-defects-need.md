---
rfc: 0037
title: Five contracts that the reviewed defects need
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: 0029
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0037: Five contracts that the reviewed defects need

## Summary

A review of the code found five defects that a fix inside one package cannot close, because each
needs a contract that a plugin or a satellite reads:

- A template's `use` records an import outside the name assignment of the file, so a spelled
  reference can bind the same name to another package.
- `rules.Substitute` keeps the spelling of a composite whose children it replaces, so a backend
  writes the type parameter where the argument belongs.
- A binding that a rules implementation makes for itself resolves no rules of another language.
- The fold classifies a resolved reference to Go's `time.Time` as a reference, while an unresolved
  one folds to the well-known timestamp.
- A `meta drop` of a summary key of the shape catalog hides a classification from `shape.Of` and not
  from the predicates of the catalog.

We propose one contract for each defect.

## Motivation

### A template's import and a spelled reference

The `use` builtin records `use "a/foo"` through `ImportSet.Add` and `use "a/foo" "foo"` through
`ImportSet.AddNamed`. Neither binds a name. A spelled reference to the package `b/foo` then binds
the free name `foo`, so the file imports two packages under one name, and Go refuses the file. The
registrygen plugin writes a named `use` for every package of its records. The TypeScript template of
the stubgen plugin imports a stub by its name the same way, so a spelled reference to another
declaration of that name gets the same name.

### The spelling of a substituted composite

`rules.Substitute` copies each reference on the path to a replaced parameter and keeps the rest of
the reference. A list `[]T` whose child it replaces with `string` keeps the spelling `[]T`. The Go
speller restates a composite only where a child spells otherwise than written. `string` spells as
written, so the backend writes `[]T`. The builder plugin leaves out every field whose type refers to
a type parameter for this reason.

### Bindings inside the rules

A rules implementation that walks members calls `rules.NewBound(r, v, nil)`, as the TypeScript and
Go capabilities and the kernel's `View.Authored` do. A nil resolver binds the absent rules for every
other language. A contributor declared in another language then walks under no rules, while the
binding of a match resolves the rules of its language.

### A resolved time.Time

The Go rules classify `time.Time` and `time.Duration` through `Builtin`, which the fold calls for a
reference without a target. A run that loads the standard library through the goroot store gives the
reference a target. The fold then classifies the declaration as a reference to `golang:time.Time`
inside the scope of the view, and as Opaque outside it. A probe measured both. The cross-language
hub states that `time.Time` projects as the well-known timestamp.

### A dropped summary key

The shape catalog stamps a family key for each classification, and the summary keys `shape.shape`,
`shape.mixed`, `shape.member` and `shape.classified`. `shape.Of` reads a summary key to find the
specs that it then reads. The predicates `Is`, `Has`, `In` and `Plays` test the family key alone,
because a predicate tests one key. A `meta drop` of the group of a classification removes its
family key, and every reader honours the drop. A `meta drop` of a summary key removes the index
alone, so `Of` loses the classification while `Is` still admits the callable.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/backend/render` | `ImportSet.Use` binds the name that a template's import binds. `Language.TemplateImport` returns that binding for each language |
| `eidos-core/rules` | `Substitute` clears the spelling of a structural reference that it restates. `View.WithRules` carries the resolver of the rules of another language, and `NewBound` reads it. `WellKnownRules` maps a declaration to a well-known identity, and the fold asks it |
| `eidos-lang/spellref` | `SpellRestated` restates a structural reference without a spelling, and `SpellWith` returns an error for one |
| `eidos-core/meta` | `KeySpec.Derived` marks a key whose value the registrant derives from other keys |
| `eidos-core/directive` | the validation of a meta instance reports `DerivedMetadataKey` for a derived key |
| `eidos-core`, `eidos-core/workspace` | the match and the directive resolver build their views with the resolver of the rules |
| Go satellite | the backend declares `TemplateImport`, and the rules implement `WellKnownRules` for `time.Time` and `time.Duration` |
| TypeScript satellite | the backend declares `TemplateImport`. `typescript.Restate` writes a structural reference from the spellings of its children, and the backend spells through `spellref.SpellRestated` |
| Rust and Java satellites | the backends declare `TemplateImport` |
| `eidos-plugin-shape` | the catalog registers its summary keys as derived |
| `eidos-reference/builder` | the builder checks a field whose type refers to a type parameter at the witnesses |

### Invariants

1. Every name that a file binds is in the name assignment of the file, whether a spelling or a
   template binds it.
2. A structural reference that the kernel restates has no spelling that the restatement contradicts.
3. A binding resolves the rules of every language that the run composes, wherever it is made.
4. A reference to a type of the well-known registry folds to the well-known identity, with or
   without a target.
5. A directive drops a fact that every reader reads, never an index of other facts.

### 1. A template's import binds its name

```go
// Use records the import of path that a template states and binds the
// name local in the name assignment of the file. item is the declaration
// of path that local refers to, and empty where local refers to the
// whole package. A later Bind of path, or a later BindItem of the item,
// returns local where the import set has no earlier binding of it. A
// later binding of another path that asks for local takes the name with
// the lowest free numeric suffix.
//
// Use returns an error where local binds another path, another item or a
// declaration of the file, because the template refers to the import by
// local and a suffix would break the reference. For the file's own
// package, Use does not record an entry and returns nil.
//
// # Allocation contract
//
// Use allocates what Bind allocates for a new binding, and nothing for a
// binding that it made before under local.
func (s *ImportSet) Use(path, local, item string) error
```

`render.Language` gains one field:

```go
// TemplateImport returns the binding of an import that a template states
// through the use builtin. name is the name that the template states,
// and empty where it states none. local is the name that the import
// binds in the file. item is the declaration of path that local refers
// to, and empty where local refers to the whole package. TemplateImport
// reports false for an import that does not bind a name. With a nil
// TemplateImport, the use builtin records every import outside the name
// assignment.
TemplateImport func(path, name string) (local, item string, ok bool)
```

The `use` builtin calls `TemplateImport`. It records an import that does not bind a name through
`Add` or `AddNamed`, and binds every other import through `Use`. An error of `Use` refuses the template, so
the render leaves the declaration out under `RefusedTemplate`.

Each backend declares the binding of its language:

| Backend | `use "p"` | `use "p" "n"` |
|---|---|---|
| Go | the package as `golang.Qualifier(p)` | the package as `n`, and no binding for `_` and `.` |
| TypeScript | no binding, because the import runs the module for its effects | the declaration `n` of `p` as `n` |
| Rust | the module as the last segment of `p` | the item `n` of `p` as `n` |
| Java | the class as the last segment of `p` | the class `n` of `p` as `n` |

### 2. A substituted composite has no spelling

`rules.Substitute` clears the spelling of every structural reference whose children or arguments it
restates. A named reference keeps its spelling, because its spelling is its name and its arguments
spell apart. The spellers of the satellites read an empty spelling of a structural reference this way:

| Speller | A structural reference without a spelling |
|---|---|
| `spellref.SpellRestated`, which the Go and TypeScript backends call | restates the reference from the spellings of its children |
| `spellref.SpellWith`, which the Rust and Java backends call | returns an error, because neither backend has a restatement |

The TypeScript rules substitute the target of a generic alias before they derive a value of it, and
the value writer spells the type of every composite value. The TypeScript backend needs a
restatement for this reason, as the Go backend has one:

```go
// Restate writes a structural reference of TypeScript from the
// spellings of its children. elems has the spelling of each child of t,
// in the order of t.Elems. Restate writes a list as T[], a tuple as
// [A, B] and a map as an object type of one index signature, each with
// readonly where t is immutable. It writes a union, an intersection and
// a function type, whose parameters are p0, p1 and so on. A union, an
// intersection and a function type inside a list, a union or an
// intersection are in parentheses.
//
// Error modes: an optional, an array, a stream, a borrow, a wildcard
// and an inline type return an error. An emitted optional does not
// state which absent value its source wrote, and the other forms have
// no TypeScript spelling that follows from their children.
func Restate(t *emit.TypeRef, elems []string) (string, error)
```

A restatement also serves a composite whose child imports under another name, which the TypeScript
backend refused before.

The builder checks each field at the witnesses of the type parameters. It substitutes the type of
the field and writes the restated reference.

### 3. The view contains the resolver of the rules

```go
// WithRules returns a copy of v whose bindings resolve the rules of
// another language through forLang. NewBound reads it where its own
// forLang is nil, so a binding that a rules implementation makes through
// the view resolves the rules that the match resolves. WithRules does
// not allocate.
func (v View) WithRules(forLang func(symbol.Lang) SourceRules) View
```

`NewBound(source, view, nil)` binds the resolver of `view`, and the absent rules where the view has
none. The match builds its view with `WithRules(m.resolve)`, and the directive resolver of the
workspace builds its view with the rules that the workspace composes.

### 4. Well-known identities

```go
// WellKnownRules is the optional capability of a language that declares
// a type of its standard library as a well-known type, such as Go's
// time.Time. The fold calls the rules of the language of a target before
// it classifies the target.
type WellKnownRules interface {
	// WellKnown returns the well-known identity of the declaration id,
	// and reports false for a declaration that the registry does not
	// bless. It does not allocate.
	WellKnown(id symbol.Identity) (symbol.Identity, bool)
}
```

The fold of a reference with a target calls the rules of the target's language, and folds a blessed
target to `rules.Reference(ref.Spelling, wellKnown)`. The Go rules bless the declarations `Time` and
`Duration` of the package `time` as `WellKnownTimestamp` and `WellKnownDuration`.

### 5. Derived metadata keys

```go
// Derived is true for a key whose value the registrant derives from
// other keys, such as a summary of a set of keys. A meta directive cannot drop
// a derived key, so a drop removes the facts that every reader reads.
Derived bool
```

The validation of a meta instance reports a derived key under a new code:

```go
// DerivedMetadataKey refuses a meta directive that names a key whose
// value its registrant derives from other keys. The instance is
// dropped, and the facts that the key summarises stay.
var DerivedMetadataKey = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 75, Meaning: "a directive drops a metadata key that its registrant derives",
})
```

The shape catalog registers `shape.shape`, `shape.mixed`, `shape.member` and `shape.classified` as
derived. A drop of the group of a classification withdraws it from `Of` and from the predicates
alike.

### Failures

| Case | Finding |
|---|---|
| A template binds a name that another package or a declaration of the file binds | `RefusedTemplate`, an Error, and the declaration is left out |
| A Rust or Java backend spells a substituted composite | the refusal of the speller, which leaves the declaration out |
| The TypeScript backend restates an optional or another form without a restatement | the refusal of the speller, which leaves the declaration out |
| A meta directive drops a derived key | `DerivedMetadataKey`, an Error at the directive |

### Migration

1. Add `ImportSet.Use`, `Language.TemplateImport` and the call in the `use` builtin, and declare
   `TemplateImport` in the four backends.
2. Clear the spelling in `Substitute`, read it in `spellref`, add `typescript.Restate` and spell
   the TypeScript backend through `SpellRestated`, and check every field in the builder.
3. Add `View.WithRules`, read it in `NewBound`, and build the views of a match and of the directive
   resolver with it.
4. Add `WellKnownRules`, ask it in the fold, and implement it in the Go rules.
5. Add `KeySpec.Derived` and `DerivedMetadataKey`, and register the summary keys of the catalog as
   derived.

## Alternatives

### Bind the name of a template's import outside the name assignment and check at the end

The pass could keep the template's imports apart and report a collision when the file ends. The
spellings before that point would already have bound the name, and the report would come after the
declaration rendered. Binding at the `use` keeps one assignment for the file.

### Respell a substituted composite in the kernel

The kernel would need the composite syntax of every language. `rules.Substitute` runs in the rules
of every language and in the plugins, and the syntax is the backend's.

### Pass the resolver to every call of NewBound

Every rules implementation would need a resolver, which its signature does not have. The methods of
the rules receive a view, so the view is where the resolver travels.

### Map a resolved time.Time through Builtin

`Builtin` classifies a reference without a target. Asking it for a resolved reference would map a
TypeScript class named `Date` to the timestamp.

### Test the summary key in each predicate

A predicate tests one key, and its subscription watches that key. A predicate on two keys needs a
second subscription for every gate of every plugin.

## Drawbacks

- `render.Language` gains one hook, `ImportSet` one method and the backend kit one builder method.
  Each of the four backends declares the hook.
- The TypeScript satellite gains one exported function, `typescript.Restate`, and its spec. A
  restated composite can differ from the source in its whitespace, such as `A | B` for `A|B`.
- A Rust or Java backend refuses a substituted composite, where it wrote a stale spelling before.
  The Rust and Java satellites have no rules, so their runs do not substitute a reference.
- `rules.View` grows by one function value, eight bytes.
- `meta.KeySpec` gains one field, and the directive validation gains one code.

## Open questions

None.

## Unresolved and future work

None.

## References

- The cross-language hub: [RFC-0023](0023-the-cross-language-hub.md).
- The shape catalog: [RFC-0022](0022-the-shape-catalog.md).
