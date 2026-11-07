---
adr: 0123
title: The rules state the error value that an error value wraps
status: Accepted
date: 2026-11-07
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0123: The rules state the error value that an error value wraps

## Status

Accepted

## Context

A package can declare a family of error values. The family has a parent and children whose initializers wrap the parent, as in `ErrExposure = fmt.Errorf("%w: no exposure", ErrPolicy)`. A caller handles a class of errors by the parent and one cause of the class by the child.

- `fmt.Errorf` returns an error whose method `Unwrap` returns the operand of its verb `%w`, so `errors.Is` matches the child with the parent.
- Where the format opens with `%w`, the message of the child opens with the message of the parent.
- The sentinel plugin of dokimi checks that no error value of a package matches another, and that no message opens with another. Each child of a family fails both checks.

The rules state each language fact of an error that a plugin reads: the base of a sentinel name, and the contract of an error type with its cause and its hooks. No rule states the error value that an initializer wraps. A plugin that finds the parent itself parses the initializer as Go and binds the qualifier `fmt` through the imports of its file.

## Decision

We will add the optional capability `rules.WrapRules` for the error value that the initializer of an error value wraps at the opening of its message, because a plugin reads every language fact of an error from the rules.

```go
// WrapRules is the optional capability of a language whose error value can
// wrap another error value in its initializer, as the Go error value
// fmt.Errorf("%w: no exposure", ErrPolicy) wraps ErrPolicy. [Bound.Wrapped]
// calls it.
type WrapRules interface {
	// Wrapped returns the error value that the initializer of the error
	// value v wraps at the opening of its message, and reports false where
	// the initializer wraps none. The error functions of the language match
	// v with that error value, and the message of v opens with its message.
	// Wrapped resolves the error value in the scope of v through view.
	Wrapped(v *node.Variable, view View) (*node.Variable, bool)
}

// Wrapped returns the error value that the initializer of the error value v
// wraps at the opening of its message, through the rules of the language of
// v. It reports false for a nil v, for an initializer that wraps no error
// value, and where those rules do not implement WrapRules.
func (b Bound) Wrapped(v *node.Variable) (*node.Variable, bool)
```

- The Go rules return the first operand of a call of `fmt.Errorf` whose format is a string literal that opens with `%w`. The call refers to the package `fmt` through an import of the file of the error value, by its qualifier or by a dot import. The operand resolves in the scope of the error value to a variable, of its own package or of an imported one.
- The rules of TypeScript, Python, Rust, Java, protobuf and GraphQL do not implement the capability. A TypeScript error keeps its cause in the field `cause`, and no function of the language matches an error with the errors of its causes.
- The reference sentinel plugin leaves the ancestors of a child out of its checks of distinct sentinels. It checks that each child matches its parent, and that the message of the child starts with the message of its parent.

## Alternatives Considered

### A parse of the initializer in the plugin

The sentinel plugin could parse the initializer of each error value itself. The plugin would then know the syntax of Go and the binding of an import, which the rules of Go know already. The cause of an error type is in the rules for the same reason.

### Each error value that an initializer wraps

The capability could return each error value that an initializer wraps, such as each operand of `errors.Join` and of a second `%w`. `errors.Is` matches the error value with each of them, but its message opens with the message of the first operand of a leading `%w` alone. The checks of a family read the match and the opening of the message from one error value.

## Consequences

**Positive:**

- A plugin finds the parent of each child of a family of error values without the syntax of its language.

**Negative:**

- The rules do not state a wrap through another call, such as `errors.Join`, a second `%w` or a function of the package. A plugin treats such an error value as an error value without a parent.
- The Go rules parse each initializer whose spelling contains the name `Errorf` and the verb `%w`.
- The rules of another language state no parent, so a plugin that checks the error values of that language does not find a family.

## References

| What | Where |
|---|---|
| D241, the row of this decision | ../architecture/21-decisions.md |
| The error contract of a type | [RFC-0042](../rfc/0042-the-error-contract-of-a-type.md) |
| The documentation of fmt.Errorf | <https://pkg.go.dev/fmt#Errorf> |
| The Go blog on errors in Go 1.13 | <https://go.dev/blog/go1.13-errors> |
| Issue 62 of dokimi, the checks refuse an error value that wraps another error value of its package | <https://github.com/dokimasia/dokimi/issues/62> |
