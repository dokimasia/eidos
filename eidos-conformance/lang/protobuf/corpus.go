// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf

import (
	"io/fs"
	"strings"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
)

// The directory the corpus convention places features under, which the
// namespace derivation starts from, the separator a namespace joins its
// parts with, and the corpus's signature root.
const (
	featureDir    = "f"
	namespaceSep  = "."
	signatureRoot = "f/cross_package_ref/dep"
)

// Corpus returns protobuf's entry over tree. protobuf is read-only and a
// schema language: it spells records, namespaces, services and closed
// value sets, and states nothing for a method on a record, an overload,
// a standalone constant or a test file. Each of those rows is refused,
// and none is omitted.
func Corpus(tree fs.FS) conformance.Corpus {
	return conformance.Corpus{
		Frontend: protofrontend.New(),
		Sources:  tree,
		Coverage: conformance.Coverage{
			"struct_fields":     conformance.Projects,
			"cross_package_ref": conformance.Projects,
			"composite_refs":    conformance.Projects,
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
			// protobuf has no test-file convention to classify by.
			"test_classification": conformance.Refuses,
			// A custom option refers to an extension that a schema of the
			// consumer declares, so the brand alone cannot spell a marker.
			"directive_sugar": conformance.Refuses,
		},
		// A schema states no bodies, so a signature-only root loads
		// the same declarations as a full one, and the corpus lists
		// no dropped identity.
		Signatures: []string{signatureRoot},
		Schemas:    frontendtest.ScriptedSchemas(),
		Rules:      protorules.New(),
		PackageOf:  packageOf,
	}
}

// packageOf derives the namespace a feature's declarations load under.
// A protobuf namespace is the dotted name the file's package statement
// declares, not the file's directory, so the entry joins the parts with
// dots where the corpus convention uses slashes.
func packageOf(featureID, sub string) string {
	parts := []string{featureDir, featureID}
	if sub != "" {
		parts = append(parts, sub)
	}
	return strings.Join(parts, namespaceSep)
}
