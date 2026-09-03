// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade

// The two modules the facade spans, and where they sit in the
// repository.
//
// The generator runs against a checkout holding both directories
// side by side, which is what lets a kernel test regenerate the
// facade and hold the committed tree to it.
const (
	// KernelModule is the module every re-export points back into.
	KernelModule = "go.dokimi.dev/eidos/core"

	// FacadeModule is the module the generator emits: the one
	// namespace plugin code imports.
	FacadeModule = "go.dokimi.dev/eidos/sdk"

	// KernelDir is the kernel module's directory, relative to the
	// repository root.
	KernelDir = "eidos-core"

	// FacadeDir is the facade module's directory, relative to the
	// repository root.
	FacadeDir = "eidos-sdk"

	// FileName is the one generated file each facade package
	// holds: the package's whole re-exported surface, its package
	// documentation included.
	FileName = "facade.gen.go"
)

// counterpartAlias is the import name every facade file binds its
// kernel counterpart under. A fixed alias keeps the generated
// aliases regular — every right-hand side reads core.Name — and
// cannot collide with a curated package name, because no kernel
// package is called core.
const counterpartAlias = "core"

// Surface is one curated entry: a kernel package admitted to the
// facade, and the package name its facade twin declares.
type Surface struct {
	// Rel is the kernel package's directory relative to the kernel
	// module root, slash-separated; empty names the module root.
	Rel string

	// Name is the facade package's name. It matches the kernel
	// package's except at the root, where the kernel's authoring
	// package is called eidos and the facade's is called sdk.
	Name string
}

// Surfaces is the curated list: the kernel packages whose exported
// symbols the facade re-exports, and nothing else. A kernel export
// outside this list is invisible to plugin code until the list
// admits its package, which is what makes the supported surface a
// reviewed artifact rather than an accident of kernel layout.
var Surfaces = []Surface{
	{Rel: "", Name: "sdk"},
	{Rel: "frontend", Name: "frontend"},
	{Rel: "backend", Name: "backend"},
	{Rel: "plugin", Name: "plugin"},
	{Rel: "emit", Name: "emit"},
	{Rel: "node", Name: "node"},
	{Rel: "symbol", Name: "symbol"},
	{Rel: "position", Name: "position"},
	{Rel: "diag", Name: "diag"},
	{Rel: "meta", Name: "meta"},
	{Rel: "directive", Name: "directive"},
	{Rel: "store", Name: "store"},
	{Rel: "backend/render", Name: "render"},
	{Rel: "output", Name: "output"},
	{Rel: "backend/backendtest", Name: "backendtest"},
	{Rel: "frontend/frontendtest", Name: "frontendtest"},
	{Rel: "plugin/plugintest", Name: "plugintest"},
	{Rel: "rules", Name: "rules"},
	{Rel: "rules/rulestest", Name: "rulestest"},
	{Rel: "authored", Name: "authored"},
}

// OwnedDirs are the repository-relative directories the generator
// owns: where the mirror guard hunts for strays.
var OwnedDirs = []string{FacadeDir}

// curated maps a kernel package's module-relative directory to its
// facade package name, for resolving qualified references in
// re-exported signatures.
func curated() map[string]string {
	m := make(map[string]string, len(Surfaces))
	for _, s := range Surfaces {
		m[s.Rel] = s.Name
	}
	return m
}
