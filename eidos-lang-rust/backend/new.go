// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/spell"
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/backend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// memberIndent is the indentation of a member's statements in an impl
// or a trait, four spaces deeper than the statements that [Scaffold]
// writes, because a member is one level inside its block.
const memberIndent = "    "

// New returns the Rust rendering backend: the module's declared
// pieces composed through the kernel's kit, implementing
// [plugin.Backend] and [plugin.Renderer] both. Its spoke is
// [spell.Type], which spells a type of another language in Rust.
// Rendered files finalise through the shared normalizer, which strips
// trailing whitespace and blank-line runs and keeps the templates'
// spelling otherwise, because the module does not include a printer and
// hermeticity refuses a printer that the machine provides.
//
// New allocates the kit's build, chiefly the parse of the file
// template, the seven kind templates and the impl template: 1,884
// allocations.
func New() plugin.Backend {
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
		Lower(Lower).
		Cluster(Cluster).
		Groups(Groups()).
		Scaffold(Scaffold).
		MemberIndent(memberIndent).
		Imports(Imports).
		Finalise(textfmt.Normalize).
		Build()
}
