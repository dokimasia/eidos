// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Lang is the source language in the identity of every declaration that
// this frontend loads. It restates [graphql.Lang], so a caller with the
// frontend does not need to import the satellite root.
const Lang = graphql.Lang

// The files that the frontend claims: every file that ends in one of the
// extensions of GraphQL.
const (
	graphqlClaim = "**/*" + graphql.Extension
	schemaClaim  = "**/*" + graphql.SchemaExtension
	shortClaim   = "**/*" + graphql.ShortExtension
)

// UnparsedFile reports a syntax error at the token that caused it.
//
// Severity is Error, so a run with one fails. The file still loads every
// definition outside the definitions with an error. The frontend reports
// the first [plugin.MaxSyntaxFindings] errors of a file one finding each,
// and one more finding counts the rest.
var UnparsedFile = diag.MustRegister(graphql.CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a GraphQL file has a syntax error",
})

// InvalidDocument reports an error of the validator of gqlparser: an error
// of the schema of a unit, or an error of a document against the schema.
// The finding is at the position of the error.
//
// Severity is Error. A schema that does not load leaves the documents of
// its unit without types, and a field that the validator does not type
// loads with a reference without a target.
var InvalidDocument = diag.MustRegister(graphql.CodePrefix, diag.CodeSpec{
	Number:  2,
	Meaning: "a GraphQL document does not validate",
})

// UnreadConfig reports a configuration of graphql-config or of gqlgen that
// the frontend does not read, at the file: a configuration in TypeScript
// or in JavaScript, which is code that the frontend does not run, a
// configuration that does not decode, and a configuration of gqlgen with an
// entry that gqlgen refuses.
//
// Severity is Error. The configuration defines no schema, so the files that
// it would claim join the units of their directories.
var UnreadConfig = diag.MustRegister(graphql.CodePrefix, diag.CodeSpec{
	Number:  3,
	Meaning: "a configuration of GraphQL is code or does not decode",
})

// BadCarrier reports a directive carrier that the kernel grammar refused,
// at the line of the carrier.
//
// Severity is Error. The carrier attaches nothing, and the declaration of
// the carrier still loads. The finding states the reason of the grammar
// and the line as written.
var BadCarrier = diag.MustRegister(graphql.CodePrefix, diag.CodeSpec{
	Number:  4,
	Meaning: "a directive carrier is outside the kernel grammar",
})

// UnaddressedCarrier reports a well-formed directive carrier in a comment
// that the frontend attributes to no declaration, at the carrier.
//
// Severity is Error. The carrier attaches nothing, and the finding quotes
// it, so an authored directive does not disappear without a report.
var UnaddressedCarrier = diag.MustRegister(graphql.CodePrefix, diag.CodeSpec{
	Number:  5,
	Meaning: "a directive carrier is on a subject the model cannot address",
})

// New builds the GraphQL frontend through the kit, the value that a
// composition registers among its frontends. The frontend claims every
// file whose name ends in one of the extensions of GraphQL, partitions the
// files into the units of their schemas, and folds [graphql.Version] into
// every unit key. A type declares a field name once, so the language does
// not overload. The frontend registers the GraphQL keys through its role.
//
// The frontend takes no configuration: the configurations of graphql-config
// and of gqlgen in the workspace decide its units. The load calls Parse once
// per unit and concurrently, and each call writes only through its own
// unit.
//
// The frontend does not return an error for a problem of a document. It
// reports the problem under one of its codes, and the load continues. A
// returned error means that the load itself failed, for example because the
// tree does not contain a file of a unit. Such an error ends the load for
// every frontend.
//
// # Allocation contract
//
// New allocates four times: the list of line openers of the syntax, the
// list of the claim, the list of key registrations and the frontend that
// the kit builds.
func New() plugin.Frontend {
	return frontend.New(graphql.Name, Lang, graphql.Syntax()).
		Version(graphql.Version).
		Match(graphqlClaim, schemaClaim, shortClaim).
		Units(partition).
		Parse(parse).
		Resolve(resolve).
		Keys(graphql.Keys).
		Build()
}
