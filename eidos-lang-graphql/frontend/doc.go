// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend loads GraphQL schemas and executable documents into the
// node graph.
//
// [New] builds the frontend through the kit, the value that a composition
// registers. The frontend parses through gqlparser, the parser of gqlgen,
// and lowers the trees into the node model.
//
// # Units and packages
//
// A unit is one schema, and its package is the schema. The partition reads
// the configurations of graphql-config and of gqlgen in the directory of
// each claimed file and in every directory above it. A configuration
// defines a schema for its keys schema and documents, and one for each
// project of its key projects. A file joins the unit of the first schema
// that claims it, and a file that no schema claims joins the unit of the
// files of its directory. Every other schema that claims a file reads the
// package of its unit as a shared package. The package of a schema is the
// directory of its configuration, and the directory joined with the name of
// a project for a project. The executable definitions of a unit load under
// the package of the schema followed by /documents.
//
// A name resolves in the package of its schema and then in the shared
// packages. A name that the schema declares by an extension alone resolves
// in the shared packages first, so a reference binds the type that the
// extension extends.
//
// # Projection
//
//   - A scalar is a defined alias without a target, and a union a defined
//     alias whose target is the union of its members.
//   - An object type is a struct with its interfaces in Implements, and an
//     input object is a struct with the input stamp.
//   - An interface is an interface with its interfaces in Extends.
//   - An enum is an enum with a variant for each value.
//   - A field without arguments is a field, and a field with arguments is
//     a method with a parameter for each argument and one result.
//   - A directive definition is an interface with a field for each
//     argument, with its locations and its repeatability as stamps.
//   - An applied directive is an annotation, and a description is the
//     documentation of its declaration.
//   - An extension merges into its type where the unit declares the type,
//     and loads as a declaration with the extension stamp where it does
//     not.
//   - An operation is a function with a parameter for each variable and the
//     declaration of its selection as its result. A fragment is a function
//     too. A selection set is a struct, or a sum where a fragment selects
//     fields of particular types of an interface or a union.
//
// A nullable type loads as the optional form of its type, and a non-null
// type as the type itself. A list loads as the list form. A field of a
// selection under @include or @skip, or in a fragment with either directive
// or with @defer, loads as the optional form, because a response can lack
// it.
//
// # Comments
//
// A comment of GraphQL is neither documentation nor a directive. The
// carriers of the comments directly above a declaration and of the comment
// after it on its line attach to the declaration, and that comment is its
// trailing comment. A carrier in any other comment reports under
// [UnaddressedCarrier].
//
// # Refusals
//
//   - [UnparsedFile]: a syntax error at its position. The file still
//     contributes every definition outside the definitions with an error.
//   - [InvalidDocument]: an error of the validator of gqlparser in the
//     schema of a unit or in a document. The validation of the documents
//     leaves out the rules that graphql-tools leaves out for a client: a
//     fragment that no operation spreads, a variable that no field reads,
//     and a directive that the schema does not define.
//   - [UnreadConfig]: a configuration in code, one that does not decode,
//     and a configuration of gqlgen with an entry that gqlgen refuses.
//   - [BadCarrier] and [UnaddressedCarrier]: a directive carrier that the
//     grammar refused, or a carrier in a comment of no declaration.
//
// # Dependency position
//
// lang/graphql/frontend imports the sdk facade, lang/graphql,
// lang/graphql/internal/document for the split and the parse of a file, the
// Go standard library, the ast, gqlerror, lexer and validator packages of
// gqlparser, doublestar for the globs of graphql-config, and the decoders of
// YAML and TOML.
package frontend
