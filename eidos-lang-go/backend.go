// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang

import (
	"go/format"

	"go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/plugin"
)

// Backend returns the Go rendering backend: the module's declared
// pieces composed through the kernel's kit, implementing
// [plugin.Backend] and [plugin.Renderer] both. Rendered files
// finalise through go/format, so a file that does not parse is
// withheld and reported rather than written, and the bytes that
// remain are the bytes gofmt leaves.
func Backend() plugin.Backend {
	return eidos.NewBackend(Name, Target, Syntax()).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		Funcs(Funcs()).
		Naming(Naming).
		Scaffold(Scaffold).
		Imports(Imports).
		Finalise(format.Source).
		Build()
}
