// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/textfmt"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/spell"
	"go.dokimi.dev/eidos/sdk/backend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// memberIndent is the indentation of a class member's statements, two
// spaces deeper than the statements that [Scaffold] writes, because a
// member is one level inside its class.
const memberIndent = "  "

// New returns the TypeScript rendering backend: the module's
// declared pieces composed through the kernel's kit, implementing
// [plugin.Backend] and [plugin.Renderer] both. Its spoke is
// [spell.Type], which spells a type of another language under the
// target's lowering policies, [typescript.Policies]. Rendered files
// finalise through the shared normalizer, which strips trailing
// whitespace and blank-line runs and keeps the templates' spelling
// otherwise, because the module does not include a printer and
// hermeticity refuses a printer that the machine provides.
//
// New allocates the kit's build, chiefly the parse of the file template
// and the seven kind templates: 1,748 allocations.
func New() plugin.Backend {
	return backend.New(typescript.Name, typescript.Target, typescript.Syntax()).
		Version(typescript.Version).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		RefusedKinds(RefusedKinds()).
		Coverage(Coverage()).
		Funcs(Funcs).
		Naming(spell.Filename).
		Packages(spell.Package).
		Respell(spell.Name).
		Types(spell.Type).
		Policies(typescript.Policies()...).
		Lower(Lower).
		Scaffold(Scaffold).
		MemberIndent(memberIndent).
		Imports(Imports).
		Finalise(textfmt.Normalize).
		Build()
}
