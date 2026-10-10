// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package java

import (
	"io/fs"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/symbol"
	javafrontend "go.dokimi.dev/eidos/lang/java/frontend"
)

// The corpus's signature root, and the record type whose two fields
// have package access, which a signature-only load drops.
const (
	fieldsRoot  = "f/struct_fields"
	pointName   = "Point"
	firstField  = "f0"
	secondField = "f1"
)

// Corpus returns Java's entry over tree. Java spells every feature
// except a constant outside a type: a class is the record type, a
// method overloads by its parameter types, a file that Surefire's
// default includes match is a test, and an annotation of the brand is
// a marker. A Java package's path is its package clause with slashes
// for dots, so the default convention places every feature.
func Corpus(tree fs.FS) conformance.Corpus {
	return conformance.Corpus{
		Frontend: javafrontend.New(nil),
		Sources:  tree,
		Coverage: conformance.Coverage{
			"struct_fields":       conformance.Loads,
			"struct_methods":      conformance.Loads,
			"method_overloads":    conformance.Loads,
			"cross_package_ref":   conformance.Loads,
			"composite_refs":      conformance.Loads,
			"builtin_ref":         conformance.Loads,
			"directive_carrier":   conformance.Loads,
			"test_classification": conformance.Loads,
			"interfaces":          conformance.Loads,
			"enum_values":         conformance.Loads,
			"directive_sugar":     conformance.Loads,

			// Java declares a constant inside a type only, as a static
			// final field.
			"constants": conformance.Refuses,
		},
		Signatures: []string{fieldsRoot},
		Dropped: []symbol.Identity{
			{
				Lang:    javafrontend.Lang,
				Package: fieldsRoot,
				Owner:   pointName,
				Name:    firstField,
				Kind:    symbol.KindField,
			},
			{
				Lang:    javafrontend.Lang,
				Package: fieldsRoot,
				Owner:   pointName,
				Name:    secondField,
				Kind:    symbol.KindField,
			},
		},
		Schemas: frontendtest.ScriptedSchemas(),
	}
}
