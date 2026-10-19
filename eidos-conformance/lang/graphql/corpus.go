// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql

import (
	"io/fs"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	graphqlfrontend "go.dokimi.dev/eidos/lang/graphql/frontend"
	graphqlrules "go.dokimi.dev/eidos/lang/graphql/rules"
)

// signatureRoot is the root that the frontend suite loads signature-only.
// A schema has no bodies, so the shallow load keeps every declaration.
const signatureRoot = "f/cross_package_ref/dep"

// Corpus returns the entry of GraphQL over tree, with its rules. An object
// type is the record, a field with arguments is the method, and a carrier
// is a # comment. A feature with a sibling package has a configuration of
// graphql-config whose schema claims the files of the sibling, so the unit
// of the feature reads the sibling as a shared package. The package of a
// unit is its directory, as the corpus convention places it. The entry
// refuses the four features that a schema cannot state: an overload, a
// standalone constant, a test file and a marker of the brand.
func Corpus(tree fs.FS) conformance.Corpus {
	return conformance.Corpus{
		Frontend: graphqlfrontend.New(),
		Sources:  tree,
		Coverage: conformance.Coverage{
			"struct_fields":     conformance.Projects,
			"struct_methods":    conformance.Projects,
			"cross_package_ref": conformance.Projects,
			"composite_refs":    conformance.Projects,
			"builtin_ref":       conformance.Projects,
			"directive_carrier": conformance.Projects,
			"interfaces":        conformance.Projects,
			"enum_values":       conformance.Projects,

			// A type declares the name of a field once.
			"method_overloads": conformance.Refuses,
			// GraphQL has no constants.
			"constants": conformance.Refuses,
			// A schema has no test files.
			"test_classification": conformance.Refuses,
			// A directive of GraphQL is a directive of the schema, which the
			// schema declares. A directive of the brand is a carrier alone.
			"directive_sugar": conformance.Refuses,
		},
		Signatures: []string{signatureRoot},
		Schemas:    frontendtest.ScriptedSchemas(),
		Rules:      graphqlrules.New(),
	}
}
