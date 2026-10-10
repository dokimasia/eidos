// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust

import (
	"io/fs"
	"path"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	rustfrontend "go.dokimi.dev/eidos/lang/rust/frontend"
)

// The name of the corpus's crate, which every module's package path
// starts with, and the directory the corpus convention places features
// under.
const (
	crateName  = "corpus"
	featureDir = "f"
)

// Corpus returns Rust's entry over tree. Rust spells every feature
// except overloading: a struct is the record type, an inherent impl
// adds its methods, a trait is the interface, #[test] classifies a test
// function, and an attribute of the brand is a marker. The crate is one
// unit, so a signature root would load every feature shallow, and the
// entry states none.
func Corpus(tree fs.FS) conformance.Corpus {
	return conformance.Corpus{
		Frontend: rustfrontend.New(nil),
		Sources:  tree,
		Coverage: conformance.Coverage{
			"struct_fields":       conformance.Loads,
			"struct_methods":      conformance.Loads,
			"constants":           conformance.Loads,
			"cross_package_ref":   conformance.Loads,
			"composite_refs":      conformance.Loads,
			"builtin_ref":         conformance.Loads,
			"directive_carrier":   conformance.Loads,
			"test_classification": conformance.Loads,
			"interfaces":          conformance.Loads,
			"enum_values":         conformance.Loads,
			"directive_sugar":     conformance.Loads,

			// Rust has no overloading: two functions of one name in one
			// scope do not compile.
			"method_overloads": conformance.Refuses,
		},
		Schemas:   frontendtest.ScriptedSchemas(),
		PackageOf: packageOf,
	}
}

// packageOf derives the package a feature's declarations load under:
// the crate's name followed by the module path, which the corpus
// spells as f::<id> and f::<id>::<sub>.
func packageOf(featureID, sub string) string {
	return path.Join(crateName, featureDir, featureID, sub)
}
