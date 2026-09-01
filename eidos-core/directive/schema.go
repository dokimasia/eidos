// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive

// ParamType types a schema param.
//
// The zero value types nothing: every declared param states its
// type, and registration refuses one that does not.
type ParamType uint8

const (
	// TypeString accepts any spelling.
	TypeString ParamType = iota + 1
	// TypeInt accepts a base-10 integer, an optional leading minus
	// included.
	TypeInt
	// TypeBool accepts exactly "true" and "false". The wider
	// boolean vocabularies are how YAML turned a country code into
	// a boolean, and one spelling stays greppable.
	TypeBool
	// TypeList accepts a bracketed list; ListOf types its elements.
	TypeList
	// TypeReference accepts a name that resolves against what
	// Resolution declares.
	TypeReference
)

// ResolutionKind says what a reference param's value resolves
// against. The source kinds are declared and carried; nothing in
// this package binds them, because the projection machinery that
// resolves a source name lives elsewhere. The metadata kind binds
// at validation, against the metadata key registry.
type ResolutionKind uint8

const (
	// ResolveNone marks a non-reference param.
	ResolveNone ResolutionKind = iota
	// ResolveCallableInScope names a callable the subject can see.
	ResolveCallableInScope
	// ResolvePackageVar names a package-level variable.
	ResolvePackageVar
	// ResolveValueField names a field on the subject's own type.
	ResolveValueField
	// ResolveHostParam names a parameter of the host callable.
	ResolveHostParam
	// ResolveMemberOnHandle names a member reached through a
	// handle the directive's other params establish.
	ResolveMemberOnHandle
	// ResolveMetadataKey resolves against the metadata registry: a
	// key's boundary spelling or a fact group's name. A typo is a
	// validation Error naming the candidates.
	ResolveMetadataKey
)

// ParamKey is a param's spelling. A schema's owner declares its
// keys as constants, so a misspelled key is a compile error in a
// plugin and a validation Error only for what a human typed.
type ParamKey string

// The reserved keys honoured on every directive: both lower to a
// routing override, and no schema may claim either. Validation
// admits them as strings and they arrive in the instance's params
// like any declared key.
const (
	ReservedOut ParamKey = "out"
	ReservedTag ParamKey = "tag"
)

// roleKey is the spelling role scoping reads. No schema declares
// it: listing Roles is what reserves it.
const roleKey ParamKey = "role"

// ParamSpec declares one param a schema accepts: its spelling, its
// type, and the conditions under which an instance must or may
// carry it.
//
// A spec is data, checked whole at registration: an untyped param,
// an empty doc, a duplicate key, an undeclared role and a list of
// lists are all refused there, so validation never meets a
// malformed spec.
type ParamSpec struct {
	Key  ParamKey
	Type ParamType
	// Required refuses an instance that omits the param. With
	// Roles set, the requirement applies under those roles alone.
	Required bool
	// Roles admits the param only when the instance's role is one
	// of these. Empty admits it under every role.
	Roles []string
	// ListOf types a TypeList param's elements: any scalar type or
	// a reference, never a list. Registration refuses a list of
	// lists, and validation types each element as declared.
	ListOf ParamType
	// Resolution is what a TypeReference value resolves against,
	// for the param itself or for every element of a reference
	// list.
	Resolution ResolutionKind
	// Counterexample marks a param whose value names an input no
	// derivation could invent. It is carried, not consumed.
	Counterexample bool
	// Doc states the param's meaning. Registration refuses an
	// empty one.
	Doc string
}

// Schema declares one directive: the closed contract between the
// author who writes an instance in source and the plugin whose
// handler receives it.
//
// A schema is a value with no behaviour. [Registry.Register]
// checks it whole and refuses what the field comments below
// forbid; [Validate] holds every instance to it and types the
// values; nothing reads it after that. Closure is the only mode:
// a key the schema does not declare is refused, so what a handler
// may assume is exactly what the schema says.
type Schema struct {
	// Plugin names the owner. The kernel's own schemas leave it
	// empty, and only they may.
	Plugin string
	// Name is the bare spelling. The prefixed spelling is
	// Plugin + ":" + Name, and both address the schema.
	Name Name
	// Positional declares the positional params in order, so
	// validation maps each bare argument to a name. An argument
	// past the last declared positional is an Error.
	Positional []ParamSpec
	// Params declares the keyed params. Unknown keys are refused:
	// closure is the only mode.
	Params []ParamSpec
	// Roles is the closed set of values the role key accepts. A
	// schema listing none refuses the role key. Declaring roles is
	// what reserves the key: no entry in Params spells "role".
	Roles []string
	// RolesRequired refuses an instance that writes no role, for a
	// schema where the role decides what applies. Without it a
	// bare instance is admitted, and what it means is the schema's
	// documented semantic.
	RolesRequired bool
	// Repeatable admits more than one instance per subject.
	// Single-instance is the default: a second instance is a
	// contradiction, reported naming both positions.
	Repeatable bool
	// Requires and ConflictsWith constrain the subject's full
	// directive list. They resolve to schemas at the registry's
	// seal, and are met or violated by canonical schema, whatever
	// spelling the subject's author wrote. A conflict is
	// symmetric: one declaration refuses the pair.
	Requires      []Name
	ConflictsWith []Name
	// Doc states the directive's meaning. Registration refuses an
	// empty one.
	Doc string
}

// Canonical returns the schema's canonical spelling: prefixed for
// a plugin's, bare for the kernel's.
func (s Schema) Canonical() Name {
	if s.Plugin == "" {
		return s.Name
	}
	return Name(s.Plugin + string(prefixSep) + string(s.Name))
}
