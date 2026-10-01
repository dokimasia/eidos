// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"os"
	"path"
	"testing"

	"go.dokimi.dev/eidos/conformance"
	rustfrontend "go.dokimi.dev/eidos/lang/rust/frontend"
	"go.dokimi.dev/eidos/sdk/frontendtest"
)

// rustCrate is the name of the Rust corpus's crate, which every module's
// package path starts with.
const rustCrate = "corpus"

// rustPackage derives the package a feature's declarations load under: the
// crate's name followed by the module path, which the corpus spells as
// f::<id> and f::<id>::<sub>.
func rustPackage(featureID, sub string) string {
	return path.Join(rustCrate, featureDir, featureID, sub)
}

// Rust spells every feature except overloading: a struct is the record
// type, an inherent impl adds its methods, a trait is the interface, and
// #[test] classifies a test function. The crate is one unit, so a
// signature root would load every feature shallow, and the corpus states
// none.
func TestRust(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Corpus{
		Frontend: rustfrontend.New(nil),
		Sources:  os.DirFS("testdata/rust"),
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

			// Rust has no overloading: two functions of one name in one
			// scope do not compile.
			"method_overloads": conformance.Refuses,
		},
		Schemas:   frontendtest.ScriptedSchemas(),
		Keys:      rustfrontend.Keys,
		PackageOf: rustPackage,
	})
}
