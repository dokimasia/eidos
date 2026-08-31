// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"go/format"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/plugin"
	golang "go.dokimi.dev/eidos/lang-go"
	"go.dokimi.dev/eidos/lang-go/spell"
)

// New returns the Go rendering backend: the module's declared
// pieces composed through the kernel's kit, implementing
// [plugin.Backend] and [plugin.Renderer] both. Rendered files
// finalise through go/format, so a file that does not parse is
// withheld and reported rather than written, and the bytes that
// remain are the bytes gofmt leaves.
func New() plugin.Backend {
	return eidos.NewBackend(golang.Name, golang.Target, golang.Syntax()).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		Funcs(Funcs()).
		Naming(spell.Filename).
		Scaffold(Scaffold).
		Imports(Imports).
		Finalise(format.Source).
		Build()
}
