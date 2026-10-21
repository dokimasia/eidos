---
adr: 0096
title: A TypeScript file has the spaces and the quotes of Biome and Prettier
status: Accepted
date: 2026-10-21
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0096: A TypeScript file has the spaces and the quotes of Biome and Prettier

## Status

Accepted

## Context

The TypeScript frontend spelled a type as its tokens without the whitespace between them, so a
reformatted signature kept one spelling. The backend writes the spelling of a structural type as the
frontend records it.

- A union `T | undefined` reached a generated file as `T|undefined`, a function type `(t: T) => U` as
  `(t:T)=>U`, and an object type as `{product:Product;limit:number}`.
- The backend wrote an object literal as `{id: 1}`, and a module specifier, a string and a quoted key
  in single quotes.
- Biome and Prettier change each of these forms in their default style. Issue 67 showed that a
  project whose gate formats the generated files fails its next run under EID-0055, because a run does
  not overwrite a file that someone edited.
- Biome 2.5.11 and Prettier 3.9.10 agree on every space and every quote that they change in the 17
  TypeScript goldens of the repository. Both also write a string literal inside a type in the quote of
  their style.
- Both keep a string in the other quote where the string contains more of their quote than of the
  other quote, because the literal then has fewer escapes.

## Decision

We will write a generated TypeScript file with the spaces and the quotes of Biome and Prettier.

- The frontend spells a type from its tokens with the spaces that both formatters write on one line.
  A space surrounds `|`, `&`, `=>` and `=`, the `?` and the `:` of a conditional type, and a keyword
  such as `keyof`, `readonly` or `extends`. A space follows a comma, a colon and the separator of the
  members of an object type. A space is inside the braces of an object type and of an object pattern.
- The members of an object type are separated by semicolons. A leading `|` or `&`, a separator before
  the closing brace and a comma before a closing bracket are left out.
- The tokens of a string and of a template literal type keep the text of the source. The spelling
  does not depend on the whitespace of the source, so a reformatted signature keeps one spelling.
- `treesitter.Node.Tokens` passes each token with the kind of its parent. It also reports whether the
  token is of a named kind and whether it is the first child of its parent.
- The backend writes an object literal with a space inside its braces, as `{ id: 1 }`, and an empty
  object literal as `{}`.
- The plan policy `typescript.quote` selects the quote of each string that the backend writes:
  `double`, which both formatters write by default, or `single`. The backend writes a module
  specifier, a string value, a quoted key and each string literal of a type in that quote. It writes
  the other quote where the string contains more of the selected quote than of the other quote.
- The default of a parameter, the initializer of a field and the argument of a decorator keep the text
  of the source.

## Alternatives Considered

### Biome or Prettier as the last step of the render

Hermeticity refuses a formatter that the machine provides, because each machine would then write other
bytes. Biome is a program of Rust and Prettier a program of JavaScript, so the backend cannot pin
either of them as a library of Go.

### A restatement of every structural type from its children

`Restate` writes the parameters of a function type as `p0` and `p1`, because the model does not record
their names. It refuses an inline object type, a conditional type and a mapped type.

### The spaces of the source

The source can write a type on more than one line, and the generated file would then contain its
indentation. A reformatted source would also change the spelling of each of its types, and every file
that a generator writes from them.

### A wrap of a line at 80 columns

Biome and Prettier wrap a line of more than 80 columns in two forms. Biome writes each member of a
long union alias on its own line, and Prettier writes the union on one indented line, as the GraphQL
catalog of the conformance goldens shows. No wrap is a fixed point of both formatters. The backend
writes each construct on one line, and a formatter in either style still wraps a line of more than 80
columns.

## Consequences

**Positive:**

- A generated file whose lines have at most 80 columns is a fixed point of Biome and of Prettier in
  the style that the policy selects.
- A string literal of a type that a generator writes, such as the discriminant of a lowered sum, takes
  the quote of the policy.

**Negative:**

- The spelling of every TypeScript type with a space changes. A generator that compares a spelling
  with text compares it with the new spelling.
- The version of the frontend and of the backend change, so every warm state of a TypeScript plan
  builds once again.
- A formatter still wraps a generated line of more than 80 columns.

## References

| What | Where |
|---|---|
| D214, the row of this decision | ../architecture/21-decisions.md |
| The TypeScript satellite | ../architecture/11-languages.md |
| The lowering policy of a plan that the render reads | ../architecture/07-rendering.md |
| Issue 67, a run fails after Biome formats the generated files | <https://github.com/dokimasia/eidos/issues/67> |
| The option `javascript.formatter.quoteStyle` of Biome | <https://biomejs.dev/reference/configuration/> |
| The rationale of Prettier for its strings | <https://prettier.io/docs/rationale#strings> |
