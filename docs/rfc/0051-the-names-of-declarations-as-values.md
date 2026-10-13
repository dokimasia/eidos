---
rfc: 0051
title: The names of declarations as values
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0051: The names of declarations as values

## Summary

`LiteralFor` reads one literal token, so a declared default such as `time.Second` or `MaxRetries`
is no value, and the reference builder reports it as a bad default. An authored sample of an
interface type lifts to raw text, so the Go backend does not import `store` for the sample
`store.NewMemory()`. The value tree has no kind for the name of a declaration, and `LiteralFor`
receives a file without the subject that resolution reads. We propose a value kind for the name of
a declaration and `LiteralFor` in the scope of a subject. The Go `LiteralFor` reads the name of a
constant or a variable and a call of a function without arguments. An authored sample lifts
through `LiteralFor` where its type has no literal of its own.

## Motivation

- The reference builder reports `BadDefault` for `default:"time.Second"` on a `time.Duration` field.
  A text field takes such a name as the text `time.Second`.
- An author gives an interface field the sample `+sample value=store.NewMemory()`. The kernel lifts
  the text to raw Go text, which the Go backend writes without the import of `store`.
- `emit.Value` has a call of a function and no reference to a constant or a variable.
- Every implementation of `LiteralFor` ignores the file that it receives, and the one caller passes
  nil. A name resolves in the scope of a subject, which `rules.Scope` contains.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/emit` | The kind `ValueName` and the constructor `emit.Name` |
| `eidos-lang/scaffold` | The walk spells a name through `Target.Callee` |
| `eidos-core/rules` | `LiteralFor` takes a `Scope`, `View.ScopeOf` returns the scope of a declaration, and `View.Authored` lifts a text of another form through `LiteralFor` |
| `eidos-core/workspace` | The validation finds the scope of a subject through `View.ScopeOf` |
| `eidos-lang-go/rules` | `LiteralFor` reads a name and a call |
| `eidos-lang-typescript/rules`, `eidos-lang-protobuf/rules` | `LiteralFor` takes a `Scope` |
| `eidos-reference/builder` | The builder reads a default in the scope of its struct |

### Invariants

1. Every target that spells a name value spells it with the import that the file needs.
2. `LiteralFor` returns a name or a call only for a declaration that resolves in the scope, or for a
   package that the scope imports and the load does not place.

### The value

```go
// ValueName is the name of a declaration, such as a constant or a
// variable: time.Second. A target spells the name with the import of its
// package, as it spells the callee of a call.
const ValueName ValueKind = ...

// Name returns the value of the name of the declaration id.
func Name(id symbol.Identity) Value
```

`Value.Callee` holds the identity of a name as it holds the identity of a callee. The shared walk of
the scaffold spells a name through `Target.Callee`. It refuses a name without an identity or without
a name, as it refuses such a callee.

### LiteralFor in a scope

```go
LiteralFor(s Scope, ref *node.TypeRef, text string, v View) (emit.Value, bool)
```

`View.ScopeOf` returns the scope of a declaration: its identity and the file whose position the
declaration has. It records a package edge, because it reads the files of the package. The
validation of the workspace finds the scope of a subject through it.

### The Go rules

`LiteralFor` reads three forms of text in this order:

1. A literal.
2. The name of a constant or a variable, bare or qualified, which resolves in the scope through
   `ResolvePackageVar`. It returns `emit.Name` of the declaration.
3. A call of a function without arguments, `New()` or `store.New()`, which resolves in the scope
   through `ResolveCallableInScope`. It returns `emit.Call` of the function.

A qualified name or call whose qualifier the file imports from a package that the load does not
place resolves to a stand-in identity under the import path, as a type of such a package does.
The kind of the stand-in is a constant for a name and a function for a call. A name that resolves
to no declaration and no stand-in is no literal.

The TypeScript and the protobuf rules take the scope and read literals alone.

### Authored samples

`View.Authored` lifts a stated text by the shape of its type where the shape is a text, a truth
value or a number. A text for any other shape lifts through the `LiteralFor` of the rules that the
caller passes, in the scope of the declaration that states it, where that declaration has the
language of the rules. Where `LiteralFor` refuses the text, or the declaration has another
language, the text lifts to raw text in the language of the declaration.

### The builder

The builder reads the default of a field through `LiteralFor` in the scope of its struct, so
`default:"time.Second"` seeds the field with `time.Second` and the import of `time`.

### Verdicts out

| Row | Capability | Reason | Reverses the verdict |
|---|---|---|---|
| none | A call with arguments as a value | An argument is a value of its own, and a default or a sample that needs a computation belongs in a function that the author writes and names | A generator that needs arguments that it computes itself |

### Failures

| Condition | Report |
|---|---|
| A name or a call that resolves to nothing in the scope | `LiteralFor` reports false, and the builder reports `BadDefault` |
| A name without an identity in a value tree | The walk refuses it, as it refuses a callee without one |

### Migration

1. Add the value kind, and spell it in the shared walk.
2. Change `LiteralFor` to take a `Scope` in the contract and in every language, and add
   `View.ScopeOf`.
3. Read names and calls in the Go rules, and lift authored text through `LiteralFor`.
4. Pass the scope from the builder.

## Alternatives

### The file alone

`LiteralFor` could keep its file and resolve a bare name in the package of the file. The file does
not record its package, and the resolution of the language reads the subject for an unexported
name and for a name that a type declares.

### Raw text with imports

A raw value could record the imports that its text needs. The text is source of one language, and a
generator cannot list the imports of a text that the author wrote without resolving it, which the
rules do.

## Drawbacks

- `LiteralFor` changes its signature, which every implementation of `SourceRules` follows.
- An authored sample of a form without a literal reads the files of its package to find its scope.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan rows W5.G13, W5.G14, W9.44 and W9.45.
- [RFC-0014](0014-the-projection-vocabulary.md), the projection vocabulary and the values.
