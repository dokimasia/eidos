// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/spell"
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/backend"
)

// memberIndent is the indentation of a member's statements in an impl
// or a trait, four spaces deeper than the statements that [Scaffold]
// writes, because a member is one level inside its block.
const memberIndent = "    "

// New returns the Rust rendering backend: the module's declared pieces
// composed through the kernel's kit into a [backend.Backend]. Its spoke
// is [spell.Type], which spells a type of another language in Rust, and
// [spell.Supertype] spells a supertype of another language.
// Rendered files finalise through the shared normalizer, which strips
// trailing whitespace and blank-line runs and keeps the templates'
// spelling otherwise, because the module does not include a printer and
// hermeticity refuses a printer that the machine provides.
//
// New allocates the kit's build, chiefly the parse of the file
// template, the seven kind templates and the impl template: 1,871
// allocations.
func New() backend.Backend {
	return backend.New(rust.Name, rust.Target, rust.Syntax()).
		Version(rust.Version).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		RefusedKinds(RefusedKinds()).
		Coverage(Coverage()).
		Funcs(Funcs).
		Naming(spell.Filename).
		Packages(spell.Package).
		Respell(spell.Name).
		Types(spell.Type).
		Supertypes(spell.Supertype).
		Lower(Lower).
		Cluster(Cluster).
		Groups(Groups()).
		Scaffold(Scaffold).
		Value(Value).
		MemberIndent(memberIndent).
		Imports(Imports).
		TemplateImport(TemplateImport).
		Finalise(textfmt.Normalize).
		Build()
}
