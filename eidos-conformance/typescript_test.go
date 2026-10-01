// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"os"
	"path"
	"testing"

	"go.dokimi.dev/eidos/conformance"
	tsfrontend "go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The modules the TypeScript corpus spells a feature in: the root module
// a.ts, a sibling package's module <sub>/d.ts, and the test file a.test.ts.
const (
	tsRootModule    = "a"
	tsSiblingModule = "d"
	tsTestModule    = "a.test"
)

// tsPackage derives the package a feature's declarations load under. A
// TypeScript module's package is its file's path without the extension,
// so a feature's root is its a.ts module and a sibling package its
// <sub>/d.ts module. A test file is a module of its own, so the root of
// test_classification is its a.test.ts module.
func tsPackage(featureID, sub string) string {
	switch {
	case sub != "":
		return path.Join(featureDir, featureID, sub, tsSiblingModule)
	case featureID == testClassification:
		return path.Join(featureDir, featureID, tsTestModule)
	default:
		return path.Join(featureDir, featureID, tsRootModule)
	}
}

// TypeScript spells every feature: a class is the record type, a method
// overloads through its signatures, and a module's const is the constant.
// The two modules whose methods the inventory discriminates alias int to
// number, so a discriminator spells int, as the inventory's does.
func TestTypeScript(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Corpus{
		Frontend: tsfrontend.New(),
		Sources:  os.DirFS("testdata/typescript"),
		Coverage: conformance.Coverage{
			"struct_fields":       conformance.Loads,
			"struct_methods":      conformance.Loads,
			"method_overloads":    conformance.Loads,
			"constants":           conformance.Loads,
			"cross_package_ref":   conformance.Loads,
			"composite_refs":      conformance.Loads,
			"builtin_ref":         conformance.Loads,
			"directive_carrier":   conformance.Loads,
			"test_classification": conformance.Loads,
			"interfaces":          conformance.Loads,
			"enum_values":         conformance.Loads,
		},
		Signatures: []string{constantsRoot},
		Dropped: []symbol.Identity{{
			Lang: tsfrontend.Lang, Package: path.Join(constantsRoot, tsRootModule),
			Name: limitName, Kind: symbol.KindConstant,
		}},
		Schemas:   frontendtest.ScriptedSchemas(),
		Keys:      tsfrontend.Keys,
		PackageOf: tsPackage,
	})
}
