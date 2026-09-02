// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import "go.dokimi.dev/eidos/core/symbol"

// KernelNamespace is the namespace the kernel's own keys register
// under, and KernelOwner is who a collision with it names. No
// plugin claims the namespace: the workspace registers it before
// any plugin's registration runs, so an impersonation is a plain
// duplicate by the time it arrives.
const (
	KernelNamespace = "gen"
	KernelOwner     = "eidos-core"
)

// The kernel-owned keys every frontend spells the same way. Module
// identity is a neutral fact because kernel machinery — scope
// matching, layout — reads it and knows no language; the raw
// toolchain spelling stays in the frontend's own namespace.
const (
	// ModuleKey carries a package's toolchain-module identity: a Go
	// module path, a Maven artifact, a crate name. Absent on a
	// package outside every module, which is what a bare
	// directory tree loads as.
	ModuleKey KeyName = "gen.module"

	// ModuleRootKey carries the workspace-relative directory the
	// package's module is declared in, "." for the tree's root.
	ModuleRootKey KeyName = "gen.moduleRoot"
)

// KernelKeys are the typed handles [Kernel] returns: what a reader
// of the kernel's own facts holds.
type KernelKeys struct {
	Module     Key[string]
	ModuleRoot Key[string]
}

// Kernel claims the kernel namespace and registers the kernel-owned
// keys. It refuses, with the registry's own errors, a namespace
// already claimed and a key already registered, which is what a
// composition registering it twice reads.
func Kernel(r *Registry) (KernelKeys, error) {
	var k KernelKeys
	if err := r.ClaimNamespace(KernelNamespace, KernelOwner); err != nil {
		return k, err
	}
	packages := []symbol.Kind{symbol.KindPackage}
	module, err := Register[string](r, KeySpec{
		Name: ModuleKey, Kinds: packages,
		Doc: "carries a package's toolchain-module identity, the way every frontend spells it",
	})
	if err != nil {
		return k, err
	}
	root, err := Register[string](r, KeySpec{
		Name: ModuleRootKey, Kinds: packages,
		Doc: "carries the workspace-relative directory a package's module is declared in",
	})
	if err != nil {
		return k, err
	}
	return KernelKeys{Module: module, ModuleRoot: root}, nil
}
