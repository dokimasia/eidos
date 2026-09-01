// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/spell"
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// New returns the Rust rendering backend: the module's declared
// pieces composed through the kernel's kit, implementing
// [plugin.Backend] and [plugin.Renderer] both. Rendered files
// finalise through the shared normalizer: the module ships no
// printer and hermeticity refuses a machine-supplied one, so the
// templates' spelling stands, minus trailing whitespace and
// blank-line runs.
func New() plugin.Backend {
	return sdk.NewBackend(rust.Name, rust.Target, rust.Syntax()).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		Coverage(Coverage()).
		Funcs(Funcs()).
		Naming(spell.Filename).
		Respell(spell.Name).
		Lower(Lower).
		Cluster(Cluster).
		Groups(Groups()).
		Scaffold(Scaffold).
		Imports(Imports).
		Finalise(textfmt.Normalize).
		Build()
}
