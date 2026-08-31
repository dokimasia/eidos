// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package java

import (
	"go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/plugin"
)

// Backend returns the Java rendering backend: the module's
// declared pieces composed through the kernel's kit, implementing
// [plugin.Backend] and [plugin.Renderer] both. Rendered files
// pass through Finalise unchanged: the module ships no Java
// printer, and hermeticity refuses a machine-supplied one, so the
// templates' own spelling is what reaches the stamp.
func Backend() plugin.Backend {
	return eidos.NewBackend(Name, Target, Syntax()).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		Funcs(Funcs()).
		Naming(Naming).
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
