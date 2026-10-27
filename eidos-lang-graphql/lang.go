// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql

import (
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Lang is the source language in the [symbol.Identity] of every
// declaration that the frontend loads. A composition registers the
// projection rules of the satellite under it.
const Lang symbol.Lang = "graphql"

// CodePrefix opens every diagnostic code that the satellite registers, such
// as GQL-0001.
const CodePrefix diag.Prefix = "GQL"

// Name is the one identity of the satellite. Its findings report under the
// name, and the kernel attributes the stamps of its frontend to the name. A
// composition claims the graphql namespace of metadata for the name.
const Name plugin.ID = "graphql"

// The extensions of the files that the frontend claims. A schema file of
// gqlgen ends in .graphqls, and every other GraphQL file in .graphql or
// .gql.
const (
	Extension       = ".graphql"
	SchemaExtension = ".graphqls"
	ShortExtension  = ".gql"
)

// ReservedPrefix opens the names that the specification of GraphQL reserves
// for its introspection, such as __typename. A schema declares no name with
// the prefix.
const ReservedPrefix = "__"

// DocumentsSegment is the last segment of the package of the executable
// definitions of a schema. The frontend loads the operations and the
// fragments of a unit into the package of its schema joined with the
// segment, so the result types of the operations do not share a namespace
// with the types of the schema.
const DocumentsSegment = "documents"

// Version is the version of what the frontend writes into the graph, and
// not the release of the module. Every unit key and the composition
// fingerprint fold it. A change of the load changes Version, which
// invalidates every unit that the old value keyed. A warm run then equals a
// cold run.
const Version = "0.1.0"

// Target is the rendering target through which a plan selects the GraphQL
// backend. Its spelling is the spelling of [Lang], so the emitter copies a
// reference of a GraphQL source as written into a plan of the target.
const Target plugin.Target = "graphql"

// BackendVersion is the version of the output of the backend. The
// composition fingerprint folds it, and every change of the rendered output
// changes it.
const BackendVersion = "0.3.0"

// Syntax returns the comment forms of GraphQL. The frontend reads carriers
// through them, and the output contract writes the frame of a written file
// through them. GraphQL has the line comment # and no block comment. A
// comment of GraphQL does not contain a tool directive, so the syntax does
// not declare the directive convention. A description is a string, and the
// frontend reads it as documentation apart from the comments. A call
// returns a fresh value, which the caller may keep. It allocates the list
// of line openers.
func Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{Line: []string{"#"}}
}
