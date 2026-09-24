// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/spell"
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/backend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// New returns the Java rendering backend: the module's declared
// pieces composed through the kernel's kit, implementing
// [plugin.Backend] and [plugin.Renderer] both. Rendered files
// finalise through the shared normalizer, which strips trailing
// whitespace and blank-line runs and keeps the templates' spelling
// otherwise, because the module ships no printer and hermeticity
// refuses a machine-supplied one.
func New() plugin.Backend {
	return backend.New(java.Name, java.Target, java.Syntax()).
		Version(java.Version).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		Coverage(Coverage()).
		Funcs(Funcs()).
		Naming(spell.Filename).
		Respell(spell.Name).
		Lower(Lower).
		Split(Split).
		Scaffold(Scaffold).
		Imports(Imports).
		Finalise(textfmt.Normalize).
		Build()
}
