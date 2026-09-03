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
// identity is a neutral fact because scope matching and layout read
// it and know no language. The raw toolchain spelling stays in the
// frontend's own namespace.
const (
	// ModuleKey carries a package's toolchain-module identity: a Go
	// module path, a Maven artifact, a crate name. Absent on a
	// package outside every module, which is what a bare
	// directory tree loads as.
	ModuleKey KeyName = "gen.module"

	// ModuleRootKey carries the workspace-relative directory the
	// package's module is declared in, "." for the tree's root.
	ModuleRootKey KeyName = "gen.moduleRoot"

	// SampleKey and AlternateKey carry an author's two stated
	// values of a declaration's type, as text in the source
	// language: what the sample directive stamps and the value
	// projection reads before deriving anything.
	SampleKey    KeyName = "gen.sample"
	AlternateKey KeyName = "gen.alternate"

	// WitnessKey carries an author's concrete type for one type
	// parameter, as the identity the witness directive resolved,
	// with an empty package for a builtin.
	WitnessKey KeyName = "gen.witness"
)

// KernelKeys are the typed handles [Kernel] returns: what a reader
// of the kernel's own facts holds. The zero value names nothing,
// so a reader handed one reads nothing rather than the wrong key.
type KernelKeys struct {
	Module     Key[string]
	ModuleRoot Key[string]
	Sample     Key[string]
	Alternate  Key[string]
	Witness    Key[symbol.Identity]
}

// IsZero reports whether the handles name nothing.
func (k KernelKeys) IsZero() bool { return k.Module.IsZero() }

// sampled lists the kinds an authored value may sit on: every
// declaration that carries one type.
func sampled() []symbol.Kind {
	return []symbol.Kind{
		symbol.KindField, symbol.KindParam, symbol.KindReturn,
		symbol.KindVariable, symbol.KindConstant, symbol.KindAlias,
		symbol.KindStruct, symbol.KindEnum, symbol.KindSum,
	}
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
	sample, err := Register[string](r, KeySpec{
		Name: SampleKey, Kinds: sampled(),
		Doc: "carries an author's stated value of a declaration's type, as source text",
	})
	if err != nil {
		return k, err
	}
	alternate, err := Register[string](r, KeySpec{
		Name: AlternateKey, Kinds: sampled(),
		Doc: "carries an author's second, distinct value of a declaration's type, as source text",
	})
	if err != nil {
		return k, err
	}
	witness, err := Register[symbol.Identity](r, KeySpec{
		Name: WitnessKey, Kinds: []symbol.Kind{symbol.KindTypeParam},
		Doc: "carries an author's concrete type for one type parameter, as the identity it resolved to",
	})
	if err != nil {
		return k, err
	}
	return KernelKeys{
		Module: module, ModuleRoot: root,
		Sample: sample, Alternate: alternate, Witness: witness,
	}, nil
}
