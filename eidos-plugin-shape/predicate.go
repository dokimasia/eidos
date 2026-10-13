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
// catalog. An annotator with such a predicate runs after the plugin
// shape, such as one that requires [Capability]. The Build of the
// workspace returns an error for one that runs before it.
func Any() sdk.Pred { return sdk.HasKey(meta.Named[bool](KeyClassified)) }

// Is returns the predicate that admits a callable with the shape n. It
// tests the family key of the shape, so a meta drop of the group of the
// shape withdraws the callable. For a name that no shape spec has, the
// name of a mixin or a contract included, the predicate has no key, and
// the Build of the plugin panics.
func Is(n Shape) sdk.Pred {
	return sdk.HasKey(meta.Named[bool](familyKey(n, FormShape)))
}

// Has returns the predicate that admits a callable with the mixin n. It
// tests the family key of the mixin. For a name that no mixin spec has,
// the predicate has no key, and the Build of the plugin panics.
func Has(n Mixin) sdk.Pred {
	return sdk.HasKey(meta.Named[bool](familyKey(n, FormMixin)))
}

// In returns the predicate that admits a callable with any role in an
// instance of the contract n. It tests the family key of the contract.
// For a name that no contract spec has, the predicate has no key, and the
// Build of the plugin panics.
func In(n Contract) sdk.Pred {
	return sdk.HasKey(meta.Named[string](familyKey(n, FormContract)))
}

// Plays returns the predicate that admits a callable in the role r of an
// instance of the contract n. It tests the family key of the contract,
// whose value is the role of the callable. For a name that no contract
// spec has, the predicate has no key, and the Build of the plugin panics.
func Plays(n Contract, r Role) sdk.Pred {
	return sdk.KeyEquals(meta.Named[string](familyKey(n, FormContract)), string(r))
}

// familyKey returns the family key of the spec of the name n, and the
// empty key where no spec of the form f has the name. A handle of the
// empty key makes the Build of the plugin that gates on it panic. It
// allocates nothing after the first read of the catalog in the process.
func familyKey[N ~string](n N, f Form) meta.KeyName {
	s, found := SpecOf(n)
	if !found || s.Form != f {
		return ""
	}
	return s.Key
}
