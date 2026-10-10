// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"io/fs"
	"path"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/symbol"
	tsfrontend "go.dokimi.dev/eidos/lang/typescript/frontend"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
)

// The directory the corpus convention places features under, which the
// package derivation starts from, and the feature whose root a test
// file spells.
const (
	featureDir         = "f"
	testClassification = "test_classification"
)

// The signature root the corpus states, and its one unexported
// declaration, which a signature-only load drops.
const (
	constantsRoot = "f/constants"
	limitName     = "limit"
)

// The modules the corpus spells a feature in: the root module a.ts, a
// sibling package's module <sub>/d.ts, and the test file a.test.ts.
const (
	rootModule    = "a"
	siblingModule = "d"
	testModule    = "a.test"
)

// Corpus returns TypeScript's entry over tree, with its rules.
// TypeScript spells every feature: a class is the record type, a method
// overloads through its signatures, a module's const is the constant,
// and a decorator of the brand is a marker. The two modules whose
// methods the inventory discriminates alias int to number, so a
// discriminator spells int, as the inventory's does. The function type
// of composite_refs returns the global class Error, which the builtin
// table does not classify, so the feature projects partly.
func Corpus(tree fs.FS) conformance.Corpus {
	return conformance.Corpus{
		Frontend: tsfrontend.New(),
		Sources:  tree,
		Coverage: conformance.Coverage{
			"struct_fields":       conformance.Projects,
			"struct_methods":      conformance.Projects,
			"method_overloads":    conformance.Projects,
			"constants":           conformance.Projects,
			"cross_package_ref":   conformance.Projects,
			"composite_refs":      conformance.ProjectsPartly,
			"builtin_ref":         conformance.Projects,
			"directive_carrier":   conformance.Projects,
			"test_classification": conformance.Projects,
			"interfaces":          conformance.Projects,
			"enum_values":         conformance.Projects,
			"directive_sugar":     conformance.Projects,
		},
		Signatures: []string{constantsRoot},
		Dropped: []symbol.Identity{{
			Lang: tsfrontend.Lang, Package: path.Join(constantsRoot, rootModule),
			Name: limitName, Kind: symbol.KindConstant,
		}},
		Schemas:   frontendtest.ScriptedSchemas(),
		Rules:     tsrules.New(),
		PackageOf: packageOf,
	}
}

// packageOf derives the package a feature's declarations load under. A
// TypeScript module's package is its file's path without the
// extension, so a feature's root is its a.ts module and a sibling
// package its <sub>/d.ts module. A test file is a module of its own, so
// the root of test_classification is its a.test.ts module.
func packageOf(featureID, sub string) string {
	switch {
	case sub != "":
		return path.Join(featureDir, featureID, sub, siblingModule)
	case featureID == testClassification:
		return path.Join(featureDir, featureID, testModule)
	default:
		return path.Join(featureDir, featureID, rootModule)
	}
}
