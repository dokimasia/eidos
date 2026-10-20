// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python

import (
	"io/fs"
	"path"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/symbol"
	pythonlang "go.dokimi.dev/eidos/lang/python"
	pythonfrontend "go.dokimi.dev/eidos/lang/python/frontend"
	pythonrules "go.dokimi.dev/eidos/lang/python/rules"
)

// The directory that the corpus convention places the features under, which
// the derivation of a package starts from, and the feature whose root a
// test module spells.
const (
	featureDir         = "f"
	testClassification = "test_classification"
)

// The signature root that the corpus states, and its one declaration that
// __all__ leaves out, which a signature-only load drops.
const (
	constantsRoot = "f/constants"
	limitName     = "limit"
)

// The modules that the corpus spells a feature in: the root module a.py, the
// module <sub>/d.py of a sibling package, and the test module test_a.py.
const (
	rootModule    = "a"
	siblingModule = "d"
	testModule    = "test_a"
)

// Corpus returns the entry of Python over tree, with its rules. Python spells
// every feature: a class is the record type, a method overloads through the
// decorator overload, a Final assignment is the constant, a Protocol is the
// interface, and a decorator of the brand is a marker. The tree is one
// project of a pyproject.toml at its root, whose source root is the root, so
// the module f/<id>/a.py is the package f/<id>/a.
func Corpus(tree fs.FS) conformance.Corpus {
	return conformance.Corpus{
		Frontend: pythonfrontend.New(nil),
		Sources:  tree,
		Coverage: conformance.Coverage{
			"struct_fields":       conformance.Projects,
			"struct_methods":      conformance.Projects,
			"method_overloads":    conformance.Projects,
			"constants":           conformance.Projects,
			"cross_package_ref":   conformance.Projects,
			"composite_refs":      conformance.Projects,
			"builtin_ref":         conformance.Projects,
			"directive_carrier":   conformance.Projects,
			"test_classification": conformance.Projects,
			"interfaces":          conformance.Projects,
			"enum_values":         conformance.Projects,
			"directive_sugar":     conformance.Projects,
		},
		Signatures: []string{constantsRoot},
		Dropped: []symbol.Identity{{
			Lang: pythonlang.Lang, Package: path.Join(constantsRoot, rootModule),
			Name: limitName, Kind: symbol.KindConstant,
		}},
		Schemas:   frontendtest.ScriptedSchemas(),
		Rules:     pythonrules.New(),
		PackageOf: packageOf,
	}
}

// packageOf derives the package that the declarations of a feature load
// under. The package of a Python module is its dotted name with slashes, so
// the root of a feature is its module a.py and a sibling package is its
// module <sub>/d.py. A test module is a module of its own, so the root of
// test_classification is its module test_a.py.
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
