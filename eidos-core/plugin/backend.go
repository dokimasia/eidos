// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/rules"
)

// nameKeySuffix follows a target's spelling in the key a name
// override is stamped under.
const nameKeySuffix = ".name"

// NameParam is the param of a target's directive that overrides the
// name of one declaration in the target, as in typescript
// name=fetchSession. No policy takes it as its name.
const NameParam directive.ParamKey = "name"

// Target names a rendering target. It is a registered name: the
// composition declares the targets it recognises, and a plan whose
// backend names anything else is a composition fault. The zero
// Target names nothing.
type Target string

// NameKey returns the key a declaration's name override in the
// target is stamped under: the target's spelling followed by .name,
// so the golang target reads golang.name. The settle reads it on a
// declaration's origin, see [Settle]. NameKey allocates the joined
// spelling, one allocation.
func (t Target) NameKey() meta.KeyName {
	return meta.KeyName(string(t) + nameKeySuffix)
}

// Backend renders the files of one target. A plan has exactly one
// backend, whose target is a registered name, and calls every method of
// the interface, so a type that implements Backend passes the checks that
// workspace Build makes on the role. The kit in package backend builds a
// Backend from declared parts, and an exotic backend implements it
// directly.
type Backend interface {
	Plugin
	// Target is the target that the backend renders.
	Target() Target
	Renderer
	FileSpeller
	SyntaxProvider
	TypeSpeller
	Packager
	PolicyProvider
	Lowerer
	Respeller
}

// TypeSpeller is the role of a backend that spells the canonical shape
// of a type of another language in its target. It is the spoke of the
// cross-language hub. The workspace passes it to each generator of a
// plan that renders through the backend.
//
// # Concurrency
//
// The plans of a run call one TypeSpeller concurrently, so an
// implementation is safe for concurrent use.
type TypeSpeller interface {
	// SpellType returns the target's reference for s under the policy p.
	// For a shape without a spelling in the target, it returns an error
	// with the reason. The error has no position, because the caller
	// positions the refusal at the declaration with the type.
	SpellType(s rules.TypeShape, p Policy) (*emit.TypeRef, error)
}

// SupertypeSpeller is the role of a backend whose target spells a
// supertype of another language otherwise than a value, as Rust spells the
// supertrait Node bare and a value of the trait as Box<dyn Node>. The
// emitter asserts it on the TypeSpeller of the plan, and spells a supertype
// through SpellType where the backend does not implement it.
//
// # Concurrency
//
// An implementation is safe for concurrent use, as a TypeSpeller is.
type SupertypeSpeller interface {
	// SpellSupertype returns the target's reference for s where a
	// declaration extends, implements or embeds it, or where it bounds a
	// type parameter. s is a reference or a sum. For a shape without a
	// spelling in the target, it returns an error with the reason, as
	// SpellType does.
	SpellSupertype(s rules.TypeShape, p Policy) (*emit.TypeRef, error)
}
