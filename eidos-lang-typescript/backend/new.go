// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/plugin"
	typescript "go.dokimi.dev/eidos/lang-typescript"
	"go.dokimi.dev/eidos/lang-typescript/spell"
)

// New returns the TypeScript rendering backend: the module's
// declared pieces composed through the kernel's kit, implementing
// [plugin.Backend] and [plugin.Renderer] both. Rendered files
// pass through Finalise unchanged: the module ships no TypeScript
// printer, and hermeticity refuses a machine-supplied one, so the
// templates' own spelling is what reaches the stamp.
func New() plugin.Backend {
	return eidos.NewBackend(typescript.Name, typescript.Target, typescript.Syntax()).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		Funcs(Funcs()).
		Naming(spell.Filename).
		Scaffold(Scaffold).
		Imports(Imports).
		Finalise(passthrough).
		Build()
}

// passthrough is the identity formatter of a module shipping no
// printer: the rendered bytes stand as the templates spelt them.
func passthrough(src []byte) ([]byte, error) {
	return src, nil
}
