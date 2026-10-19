// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package document splits a GraphQL document into its definitions of the
// type system and its executable definitions, and parses each part through
// gqlparser. The frontend of the satellite loads its files through the
// package, and the toolchain adapter checks generated files through it.
//
// A file of GraphQL can contain both languages, and gqlparser parses a
// schema document and a query document apart. [Lex] lexes a file once,
// [Lexed.Definitions] splits its tokens into its top-level definitions by
// the grammar of GraphQL, and [ParseSchema] and [ParseQuery] parse a copy of
// the file without the definitions of the other language. A copy writes a
// space for each rune that it leaves out and keeps every line terminator,
// so a position in a copy is the position in the file.
//
// # Recovery
//
// The lexer and the parser of gqlparser stop at their first error. [Lex]
// continues with a new lexer at the start of the next line, and a parse
// leaves out the definition with the error and parses the copy again. A
// syntax error therefore costs the definition that contains it and no other
// definition.
//
// # Schemas and documents
//
// [Load] loads a schema over the prelude of gqlparser, which declares the
// built-in scalars, the built-in directives and the types of introspection.
// [Rules] returns the rules of the validation of the documents of a client,
// and [LoneAnonymous] is the message of the rule that a client checks in
// each document.
//
// # Dependency position
//
// lang/graphql/internal/document imports the ast, gqlerror, lexer, parser,
// validator, validator/core and validator/rules packages of gqlparser, the
// position facade of the SDK, and the Go stdlib.
package document
