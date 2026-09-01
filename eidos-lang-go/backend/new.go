// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go/format"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/spell"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// New returns the Go rendering backend: the module's declared
// pieces composed through the kernel's kit, implementing
// [plugin.Backend] and [plugin.Renderer] both. Rendered files
// finalise through go/format, so a file that does not parse is
// withheld and reported rather than written, and the bytes that
// remain are the bytes gofmt leaves.
func New() plugin.Backend {
	return sdk.NewBackend(golang.Name, golang.Target, golang.Syntax()).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		Coverage(Coverage()).
		Funcs(Funcs()).
		Naming(spell.Filename).
		Respell(spell.Name).
		Lower(Lower).
		Scaffold(Scaffold).
		Imports(Imports).
		Finalise(format.Source).
		Build()
}
