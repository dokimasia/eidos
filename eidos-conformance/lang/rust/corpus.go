// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust

import (
	"io/fs"
	"path"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	rustfrontend "go.dokimi.dev/eidos/lang/rust/frontend"
	rustrules "go.dokimi.dev/eidos/lang/rust/rules"
)

// The name of the corpus's crate, which every module's package path
// starts with, and the directory the corpus convention places features
// under.
const (
	crateName  = "corpus"
	featureDir = "f"
)

// Corpus returns the entry of Rust over tree, with its rules. Rust spells
// every feature except overloading: a struct is the record type, an inherent
// impl adds its methods, a trait is the interface, #[test] classifies a test
// function, and an attribute of the brand is a marker. The crate is one
// unit, so a signature root would load every feature shallow, and the entry
// states none.
func Corpus(tree fs.FS) conformance.Corpus {
	return conformance.Corpus{
		Frontend: rustfrontend.New(nil),
		Sources:  tree,
		Coverage: conformance.Coverage{
			"struct_fields":     conformance.Projects,
			"struct_methods":    conformance.Projects,
			"constants":         conformance.Projects,
			"cross_package_ref": conformance.Projects,
			// The function pointer returns a Result, and a Result outside
			// the return of a callable folds to Opaque, because the kernel
			// has no form of a tagged union of two types.
			"composite_refs":      conformance.ProjectsPartly,
			"builtin_ref":         conformance.Projects,
			"directive_carrier":   conformance.Projects,
			"test_classification": conformance.Projects,
			"interfaces":          conformance.Projects,
			"enum_values":         conformance.Projects,
			"directive_sugar":     conformance.Projects,

			// Rust has no overloading: two functions of one name in one
			// scope do not compile.
			"method_overloads": conformance.Refuses,
		},
		Schemas:   frontendtest.ScriptedSchemas(),
		Rules:     rustrules.New(),
		PackageOf: packageOf,
	}
}

// packageOf derives the package a feature's declarations load under:
// the crate's name followed by the module path, which the corpus
// spells as f::<id> and f::<id>::<sub>.
func packageOf(featureID, sub string) string {
	return path.Join(crateName, featureDir, featureID, sub)
}
