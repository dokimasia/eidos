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

// New returns the TypeScript rendering backend: the module's
// declared pieces composed through the kernel's kit, implementing
// [plugin.Backend] and [plugin.Renderer] both. Rendered files
// finalise through the shared normalizer: the module ships no
// printer and hermeticity refuses a machine-supplied one, so the
// templates' spelling stands, minus trailing whitespace and
// blank-line runs.
func New() plugin.Backend {
	return backend.New(typescript.Name, typescript.Target, typescript.Syntax()).
		Version(typescript.Version).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		Coverage(Coverage()).
		Funcs(Funcs()).
		Naming(spell.Filename).
		Respell(spell.Name).
		Lower(Lower).
		Scaffold(Scaffold).
		Imports(Imports).
		Finalise(textfmt.Normalize).
		Build()
}
