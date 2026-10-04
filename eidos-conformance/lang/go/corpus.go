// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"io/fs"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/symbol"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
)

// The signature root the corpus states, and its one unexported
// declaration, which a signature-only load drops.
const (
	constantsRoot = "f/constants"
	limitName     = "limit"
)

// Corpus returns Go's entry over tree, with its rules: the first real
// language against the shared inventory. The tree places each feature
// under f/<id>, as the corpus convention does. The one refusal is Go's
// own semantics: overloads do not exist. A typed constant group loads
// as the enum the schema names it.
func Corpus(tree fs.FS) conformance.Corpus {
	return conformance.Corpus{
		Frontend: gofrontend.New(nil),
		Sources:  tree,
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
		Signatures: []string{constantsRoot},
		Dropped: []symbol.Identity{
			{Lang: gofrontend.Lang, Package: constantsRoot, Name: limitName, Kind: symbol.KindConstant},
		},
		Schemas: frontendtest.ScriptedSchemas(),
		Keys:    gofrontend.Keys,
		Rules:   gorules.New(),
	}
}
