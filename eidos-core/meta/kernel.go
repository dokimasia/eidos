// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import "go.dokimi.dev/eidos/core/symbol"

// KernelNamespace is the namespace the kernel's own keys register
// under, and KernelOwner is the registrant that claims it. The
// workspace registers the kernel's keys before any plugin's
// registration runs, and a key registers only into a namespace its
// own registrant claimed, so no plugin registers a key under the
// kernel's namespace.
const (
	KernelNamespace = "gen"
	KernelOwner     = "eidos-core"
)

// The kernel's keys, which every frontend spells the same way. Module
// identity is a neutral fact because scope matching and layout read
// it and know no language. The raw toolchain spelling remains in the
// frontend's own namespace.
const (
	// ModuleKey is a package's toolchain-module identity: a Go
	// module path, a Maven artifact, a crate name. A package
	// outside every module has none, which is what a bare directory
	// tree loads as.
	ModuleKey KeyName = "gen.module"

	// ModuleRootKey is the workspace-relative directory the
	// package's module is declared in, "." for the tree's root.
	ModuleRootKey KeyName = "gen.moduleRoot"

	// SampleKey and AlternateKey are an author's two stated values
	// of a declaration's type, as text in the source language: what
	// the sample directive stamps and the value projection reads
	// before deriving anything.
	SampleKey    KeyName = "gen.sample"
	AlternateKey KeyName = "gen.alternate"

	// WitnessKey is an author's concrete type for one type
	// parameter, as the identity the witness directive resolved,
	// with an empty package for a builtin.
	WitnessKey KeyName = "gen.witness"
)

// KernelKeys are the typed handles [Kernel] returns: what a reader
// of the kernel's own facts keeps. The zero value names nothing, so
// a reader handed one reads nothing, never the wrong key.
type KernelKeys struct {
	Module     Key[string]
	ModuleRoot Key[string]
	Sample     Key[string]
	Alternate  Key[string]
	Witness    Key[symbol.Identity]
}

// IsZero reports whether the handles name nothing. It allocates
// nothing.
func (k KernelKeys) IsZero() bool { return k.Module.IsZero() }

// sampled lists the kinds an authored value may be stamped on: every
// declaration that has one type.
func sampled() []symbol.Kind {
	return []symbol.Kind{
		symbol.KindField, symbol.KindParam, symbol.KindReturn,
		symbol.KindVariable, symbol.KindConstant, symbol.KindAlias,
		symbol.KindStruct, symbol.KindEnum, symbol.KindSum,
	}
}

// Kernel claims the kernel namespace as [KernelOwner] and registers
// the kernel's keys through that registrant's handle, whichever
// handle it is given. A second call on one registry repeats the
// registration and returns the same handles. It refuses, with the
// registry's own error, a kernel namespace that another registrant
// claimed.
//
// # Allocation contract
//
// Kernel allocates what the registry keeps of its five keys, 14
// allocations in an empty registry:
//   - the first entries of the namespace map and of the name map;
//   - the four kind lists;
//   - the spec and type lists, each growing to five entries in four
//     allocations.
func Kernel(r *Registry) (KernelKeys, error) {
	var k KernelKeys
	r = r.For(KernelOwner)
	if err := r.ClaimNamespace(KernelNamespace); err != nil {
		return k, err
	}
	packages := []symbol.Kind{symbol.KindPackage}
	module, err := Register[string](r, KeySpec{
		Name: ModuleKey, Kinds: packages,
		Doc: "records a package's toolchain-module identity, the way every frontend spells it",
	})
	if err != nil {
		return k, err
	}
	root, err := Register[string](r, KeySpec{
		Name: ModuleRootKey, Kinds: packages,
		Doc: "records the workspace-relative directory a package's module is declared in",
	})
	if err != nil {
		return k, err
	}
	sample, err := Register[string](r, KeySpec{
		Name: SampleKey, Kinds: sampled(),
		Doc: "records an author's stated value of a declaration's type, as source text",
	})
	if err != nil {
		return k, err
	}
	alternate, err := Register[string](r, KeySpec{
		Name: AlternateKey, Kinds: sampled(),
		Doc: "records an author's second, distinct value of a declaration's type, as source text",
	})
	if err != nil {
		return k, err
	}
	witness, err := Register[symbol.Identity](r, KeySpec{
		Name: WitnessKey, Kinds: []symbol.Kind{symbol.KindTypeParam},
		Doc: "records an author's concrete type for one type parameter, as the identity it resolved to",
	})
	if err != nil {
		return k, err
	}
	return KernelKeys{
		Module: module, ModuleRoot: root,
		Sample: sample, Alternate: alternate, Witness: witness,
	}, nil
}
