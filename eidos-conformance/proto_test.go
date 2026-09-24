// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"os"
	"strings"
	"testing"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
)

// protoPackage derives the namespace a feature's declarations load
// under. A protobuf namespace is the dotted name the file's package
// statement declares, not the file's directory, so the satellite
// joins the parts with dots where the corpus convention uses
// slashes.
func protoPackage(featureID, sub string) string {
	parts := []string{"f", featureID}
	if sub != "" {
		parts = append(parts, sub)
	}
	return strings.Join(parts, ".")
}

// protobuf is read-only and a schema language: it spells records,
// namespaces, services and closed value sets, and states nothing
// for a method on a record, an overload, a standalone constant or
// a test file. Each of those rows is refused, and none is omitted.
func TestProtobuf(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Corpus{
		Frontend: protofrontend.New(),
		Sources:  os.DirFS("testdata/proto"),
		Coverage: conformance.Coverage{
			"struct_fields":     conformance.Projects,
			"cross_package_ref": conformance.Projects,
			"builtin_ref":       conformance.Projects,
			"directive_carrier": conformance.Projects,
			"interfaces":        conformance.Projects,
			"enum_values":       conformance.Projects,

			// A message declares no methods. A service declares the
			// behaviour, and a service is the interfaces row.
			"struct_methods": conformance.Refuses,
			// One service declares one rpc per name, so there is no
			// overload to tell apart.
			"method_overloads": conformance.Refuses,
			// A schema states no standalone constant: its fixed
			// values are an enum's, which the enum_values row covers.
			"constants": conformance.Refuses,
			// A schema spells no function type, which the feature's
			// own expectation names.
			"composite_refs": conformance.Refuses,
			// protobuf has no test-file convention to classify by.
			"test_classification": conformance.Refuses,
		},
		// A schema states no bodies, so a signature-only root loads
		// the same declarations as a full one. The suite runs the
		// check to prove it.
		Signatures: []string{"f/cross_package_ref/dep"},
		Schemas:    frontendtest.ScriptedSchemas(),
		Keys:       protobuf.Keys,
		Rules:      protorules.New(),
		PackageOf:  protoPackage,
	})
}
