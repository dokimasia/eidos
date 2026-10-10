// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shape

import (
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
)

// Any returns the predicate that admits a callable with a shape, a mixin
// or a role in a contract instance. It tests the summary key
// [KeyClassified].
//
// A predicate of the package tests a named handle, which the workspace
// binds when it builds, so a plugin that uses one registers no key of the
// catalog. An annotator with such a predicate requires [Capability], and
// the Build of the workspace returns an error for one that does not.
func Any() sdk.Pred { return sdk.HasKey(meta.Named[bool](KeyClassified)) }

// Is returns the predicate that admits a callable with the shape n. It
// tests the family key of the shape, so a meta drop of the group of the
// shape withdraws the callable. For a name that no shape spec has, the
// predicate has no key, and the Build of the plugin panics.
func Is(n Shape) sdk.Pred {
	s, _ := SpecOf(n)
	return sdk.HasKey(meta.Named[bool](s.Key))
}

// Has returns the predicate that admits a callable with the mixin n. It
// tests the family key of the mixin. For a name that no mixin spec has,
// the predicate has no key, and the Build of the plugin panics.
func Has(n Mixin) sdk.Pred {
	s, _ := SpecOf(n)
	return sdk.HasKey(meta.Named[bool](s.Key))
}

// In returns the predicate that admits a callable with any role in an
// instance of the contract n. It tests the family key of the contract.
// For a name that no contract spec has, the predicate has no key, and the
// Build of the plugin panics.
func In(n Contract) sdk.Pred {
	s, _ := SpecOf(n)
	return sdk.HasKey(meta.Named[string](s.Key))
}

// Plays returns the predicate that admits a callable in the role r of an
// instance of the contract n. It tests the family key of the contract,
// whose value is the role of the callable. For a name that no contract
// spec has, the predicate has no key, and the Build of the plugin panics.
func Plays(n Contract, r Role) sdk.Pred {
	s, _ := SpecOf(n)
	return sdk.KeyEquals(meta.Named[string](s.Key), string(r))
}
