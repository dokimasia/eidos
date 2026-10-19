// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/spell"
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/backend"
)

// New returns the GraphQL rendering backend, which writes the schema
// definition language and executable documents: the pieces of the package
// composed through the kernel's kit into a [backend.Backend]. Its spoke is
// [spell.Type], which spells a type of another language in GraphQL under
// the policies of the satellite. Files finalise through the shared
// normalizer, because the templates write the layout of graphql-js.
func New() backend.Backend {
	return backend.New(graphql.Name, graphql.Target, graphql.Syntax()).
		Version(graphql.BackendVersion).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		RefusedKinds(RefusedKinds()).
		Coverage(Coverage()).
		Funcs(Funcs).
		Naming(spell.Filename).
		Packages(spell.Package).
		Respell(spell.Name).
		Types(spell.Type).
		Policies(graphql.Policies()...).
		Lower(Lower).
		Cluster(Cluster).
		Groups(Groups()).
		Scaffold(Scaffold).
		Imports(Imports).
		Finalise(textfmt.Normalize).
		Build()
}
