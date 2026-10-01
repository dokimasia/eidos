// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"os"
	"testing"

	"go.dokimi.dev/eidos/conformance"
	javafrontend "go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The Java corpus's signature root, and the record type whose two fields
// have package access, which a signature-only load drops.
const (
	javaFieldsRoot = "f/struct_fields"
	pointName      = "Point"
	firstField     = "f0"
	secondField    = "f1"
)

// Java spells every feature except a constant outside a type: a class is
// the record type, a method overloads by its parameter types, and a file
// Surefire's default includes name is a test. A Java package's path is its
// package clause with slashes for dots, so the default convention places
// every feature.
func TestJava(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Corpus{
		Frontend: javafrontend.New(nil),
		Sources:  os.DirFS("testdata/java"),
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

			// Java declares a constant inside a type only, as a static
			// final field.
			"constants": conformance.Refuses,
		},
		Signatures: []string{javaFieldsRoot},
		Dropped: []symbol.Identity{
			{
				Lang:    javafrontend.Lang,
				Package: javaFieldsRoot,
				Owner:   pointName,
				Name:    firstField,
				Kind:    symbol.KindField,
			},
			{
				Lang:    javafrontend.Lang,
				Package: javaFieldsRoot,
				Owner:   pointName,
				Name:    secondField,
				Kind:    symbol.KindField,
			},
		},
		Schemas: frontendtest.ScriptedSchemas(),
		Keys:    javafrontend.Keys,
	})
}
