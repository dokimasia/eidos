// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"os"
	"testing"

	"go.dokimi.dev/eidos/conformance"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/sdk/frontendtest"
)

// Go's corpus entry: the first real language against the shared
// inventory. The one refusal is Go's own semantics: overloads do
// not exist. A typed constant group loads as the enum the schema
// names it.
func TestGolang(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Corpus{
		Frontend: gofrontend.New(nil),
		Sources:  os.DirFS("testdata/go"),
		Coverage: conformance.Coverage{
			"struct_fields":       conformance.Loads,
			"struct_methods":      conformance.Loads,
			"method_overloads":    conformance.Refuses,
			"constants":           conformance.Loads,
			"cross_package_ref":   conformance.Loads,
			"builtin_ref":         conformance.Loads,
			"directive_carrier":   conformance.Loads,
			"test_classification": conformance.Loads,
			"interfaces":          conformance.Loads,
			"enum_values":         conformance.Loads,
		},
		Signatures: []string{"f/constants"},
		Schemas:    frontendtest.ScriptedSchemas(),
		Keys:       gofrontend.Keys,
	})
}
