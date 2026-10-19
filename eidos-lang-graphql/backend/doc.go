// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package backend renders emit declarations as GraphQL documents: the
// definitions of the type system, the schema statement and the executable
// definitions.
//
// [New] composes the backend through the kernel's backend kit. The pieces
// are the file skeleton [FileTemplate], the kind templates of a type, an
// interface, an enum, an alias and a function, the fact [Coverage], the
// template vocabulary of a [Vocabulary], which [Funcs] binds to each render
// worker, the filename, package and name spelling of
// the spell package, the lowering [Lower], the extend [Cluster] and its
// [Groups], the statement printer [Scaffold], the import renderer [Imports]
// and the shared normalizer.
//
// # Layout
//
// The templates write the layout of graphql-js: two spaces for each member
// of a body, a blank line between two definitions, a description before its
// definition, one line for the arguments of a field unless an argument has a
// description or a comment, and the trailing comment of a definition after
// its opening brace.
//
// # Channels
//
// A generator states each construct of GraphQL through one channel of the
// emit model:
//
//   - a field of the model states a fact that other languages share, such
//     as the optional form for a nullable type and the value of a field of an
//     input object for its default;
//   - an annotation states an applied directive, and an annotation whose name
//     starts with graphql: is a marker that states a keyword or a statement;
//   - the header of a file, an emit.File in a unit, states the schema
//     statement of the file;
//   - the verbatim body of a function states the selection set of an
//     operation or a fragment.
//
// # Dependency position
//
// lang/graphql/backend imports the sdk's backend, emit, render and symbol
// facades, the module root for the language identity, the markers and the
// directives, lang/graphql/spell for the spoke and the spelling of names,
// files and packages, lang/graphql/internal/value for the check of a value,
// the ast and parser packages of gqlparser for the check of a selection set,
// the naming and textfmt helpers of eidos-lang, and the Go stdlib.
package backend
