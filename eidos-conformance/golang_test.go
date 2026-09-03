// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"os"
	"testing"

	"go.dokimi.dev/eidos/conformance"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
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
			"struct_fields":       conformance.Projects,
			"struct_methods":      conformance.Projects,
			"method_overloads":    conformance.Refuses,
			"constants":           conformance.Projects,
			"cross_package_ref":   conformance.Projects,
			"composite_refs":      conformance.ProjectsPartly,
			"builtin_ref":         conformance.Projects,
			"directive_carrier":   conformance.Projects,
			"test_classification": conformance.Projects,
			"interfaces":          conformance.Projects,
			"enum_values":         conformance.Projects,
		},
		Signatures: []string{"f/constants"},
		Schemas:    frontendtest.ScriptedSchemas(),
		Keys:       gofrontend.Keys,
		Rules:      gorules.New(),
	})
}
