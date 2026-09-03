// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive

import (
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Value is one validated param value.
//
// Kind selects the populated field: [TypeString] fills Str,
// [TypeInt] fills Int, [TypeBool] fills Bool, [TypeList] fills
// List with each element typed as the spec's element type, and
// [TypeReference] fills Ref. Every other field holds its zero
// value, so a consumer switches on Kind and reads one field
// without an assertion.
//
// Value is a plain value: copy it freely. A List is the one field
// sharing storage; validation builds each instance fresh, so no
// two instances alias.
type Value struct {
	Kind ParamType
	Str  string
	Int  int64
	Bool bool
	List []Value
	// Ref is a reference's spelling: validated for the metadata
	// kind, and bound through the resolver for the source kinds.
	Ref string
	// Target is the identity a source reference resolved to. It is
	// zero for a metadata reference, and for a source reference
	// validated without a resolver, which carries the spelling
	// alone.
	Target symbol.Identity
}

// Directive is one validated instance: what a handler receives
// through its match, with every value already typed and every
// check already passed.
//
// A Directive that exists is valid — [Validate] refuses the rest —
// so a handler reads params without re-checking presence beyond
// what the schema declares optional.
type Directive struct {
	// Name is the schema's canonical spelling, whatever the author
	// wrote: prefixed for a plugin's, bare for the kernel's.
	Name Name
	// Args holds the positional values, typed per the schema's
	// positional specs, in source order.
	Args []Value
	// Params holds the keyed values, typed per the schema, the
	// reserved routing keys included.
	Params map[ParamKey]Value
	// Role is the validated role, empty where none was written.
	Role string
	// Pos is the carrier line.
	Pos position.Pos
	// Instance is the source order among a repeatable directive's
	// instances on one subject, starting at zero.
	Instance int
}

// Param returns a keyed value and whether the instance carries it,
// the reserved routing keys included. A required param always
// returns true — validation refused the instance otherwise — so
// the boolean matters only for optional and reserved keys.
func (d *Directive) Param(k ParamKey) (Value, bool) {
	v, held := d.Params[k]
	return v, held
}
