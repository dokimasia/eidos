// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/spell"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// New returns the Rust rendering backend: the module's declared
// pieces composed through the kernel's kit, implementing
// [plugin.Backend] and [plugin.Renderer] both. Rendered files
// pass through Finalise unchanged: the module ships no Rust
// printer, and hermeticity refuses a machine-supplied one, so the
// templates' own spelling is what reaches the stamp.
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
		Finalise(passthrough).
		Build()
}

// passthrough is the identity formatter of a module shipping no
// printer: the rendered bytes stand as the templates spelt them.
func passthrough(src []byte) ([]byte, error) {
	return src, nil
}
