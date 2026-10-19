// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package graphql contains the identity of the GraphQL satellite and the
// vocabulary that its packages share.
//
// [Lang] is the source language of every declaration that the frontend
// loads. [Name] is the plugin identity of the findings and the stamps of the
// satellite, and [CodePrefix] opens every diagnostic code that it registers.
// The frontend claims the files that end in [Extension], [SchemaExtension]
// or [ShortExtension]. It loads the executable definitions of a schema into
// the package of the schema joined with [DocumentsSegment]. Every unit key
// folds [Version], and [Syntax] returns the comment forms. A plan selects
// the backend through [Target], and the composition fingerprint folds
// [BackendVersion].
//
// # Packages
//
// The module covers GraphQL as a source and as a target:
//
//   - frontend loads schemas and executable documents into the node graph.
//   - rules projects the declarations of GraphQL for the walks of the
//     kernel.
//   - spell spells filenames, declared names, packages and the types of
//     other languages.
//   - backend renders emit declarations as GraphQL documents.
//   - testing parses and validates written documents in process.
//
// # Stamps
//
// The frontend stamps what GraphQL states and the model has no field for,
// under the keys that [Keys] registers:
//
//   - [SchemaKey] marks a file with a schema statement, and [RootsKey] lists
//     the root operation types of its statements.
//   - [OperationKey] is the kind of operation of a root type and of the
//     function of an operation.
//   - [InputKey] marks an input object, and [ExtensionKey] marks an
//     extension of a type that its schema does not declare.
//   - [DirectiveKey] lists the locations of a directive definition, and
//     [RepeatableKey] marks a repeatable one.
//   - [AnonymousKey], [FragmentKey] and [SelectionKey] state the parts of an
//     executable definition. [CoordinateKey] and [SpreadsKey] state the parts
//     of its selection.
//
// An entry of [RootsKey] joins an operation and a type with [NameSep], as
// the argument of the annotation of an applied directive joins the name and
// the value of the argument.
//
// # Restating a source
//
// A generator over GraphQL sources restates the stamps of a declaration
// through [Stated], which returns the markers and the verbatim body of a
// [Statement]. [Header] returns the header of the file of a declaration,
// which states its schema statement. Both read the stamps through a [Match]
// of the generator.
//
// # Channels
//
// A generator states each construct of GraphQL through one channel of the
// emit model: a field of the model that other languages share, an
// annotation for an applied directive, or a [Marker] for a keyword or a
// statement that is not a directive. [Policies] returns the lowering
// policies of the target. Each policy chooses the type of a shape that
// GraphQL has no built-in scalar for.
//
// # Scalars
//
// [IsBuiltin] reports the five built-in scalars, which no declaration
// declares. The table of the satellite lists seven custom scalars of
// scalars.graphql.org. [ScalarShape] returns the shape of a custom scalar
// whose specifiedBy URL the table contains, and [Scalar] returns the emit
// declaration of a scalar of the table.
//
// # Dependency position
//
// lang/graphql imports the sdk facade and the Go stdlib.
package graphql
