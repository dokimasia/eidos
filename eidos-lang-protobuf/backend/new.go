// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/spell"
	"go.dokimi.dev/eidos/sdk/backend"
)

// New returns the protobuf rendering backend, which writes proto2, proto3
// and edition files: the module's declared pieces composed through the
// kernel's kit into a [backend.Backend]. Its spoke is [spell.Type], which
// spells a type of another language in protobuf, under the encoding
// policies of the satellite. The templates write the layout of buf format,
// and rendered files finalise through [Check], which refuses a file that
// the parser of protocompile does not read.
func New() backend.Backend {
	return backend.New(protobuf.Name, protobuf.Target, protobuf.Syntax()).
		Version(protobuf.BackendVersion).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		RefusedKinds(RefusedKinds()).
		Coverage(Coverage()).
		Funcs(Funcs).
		Naming(spell.Filename).
		Packages(spell.Package).
		Respell(spell.Name).
		Types(spell.Type).
		Policies(protobuf.Policies()...).
		Lower(Lower).
		Cluster(Cluster).
		Groups(Groups()).
		Scaffold(Scaffold).
		Imports(Imports).
		TemplateImport(TemplateImport).
		Finalise(Check).
		Build()
}
