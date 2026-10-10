// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive

import "strconv"

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
	// TypeBool accepts exactly "true" and "false", so every boolean
	// has one searchable spelling and no other text reads as one.
	TypeBool
	// TypeList accepts a bracketed list. ListOf types its elements.
	TypeList
	// TypeReference accepts a name that resolves against what
	// Resolution declares.
	TypeReference
)

// ResolutionKind names what a reference param's value resolves
// against. This package declares the source kinds and binds none
// of them: the projection machinery that resolves a source name is
// outside it. The metadata kind binds at validation, against the
// metadata key registry, and the diagnostic kind against the codes that
// diag.MustRegister registered.
type ResolutionKind uint8

const (
	// ResolveNone marks a non-reference param.
	ResolveNone ResolutionKind = iota
	// ResolveCallableInScope names a callable the subject can see: for
	// a method, a method of the type it belongs to before a function in
	// scope.
	ResolveCallableInScope
	// ResolvePackageVar names a package-level variable.
	ResolvePackageVar
	// ResolveValueField names a field of the subject's value: the
	// subject's own type for a type, the type a field belongs to, and
	// for a callable the type of its first value return, then the type
	// of each input parameter.
	ResolveValueField
	// ResolveHostParam names a parameter of the host callable.
	ResolveHostParam
	// ResolveMemberOnHandle names a field or a method of the subject's
	// handle: the subject's own type for a type, the type a field
	// belongs to, and for a callable the type of its first value
	// return.
	ResolveMemberOnHandle
	// ResolveMetadataKey resolves against the metadata registry: a
	// key's boundary spelling or a fact group's name. A typo is a
	// validation Error naming the candidates.
	ResolveMetadataKey
	// ResolveTypeInScope names a type the subject's file can see, a
	// bare or qualified type spelling: what a witness names.
	ResolveTypeInScope
	// ResolveDiagnosticCode names a registered diagnostic code in the
	// spelling that diag.ParseCode reads, such as EID-0062: what a diag
	// directive suppresses. A spelling that is no registered code is a
	// validation Error under [UnknownCode].
	ResolveDiagnosticCode
)

// String returns the kind's spelling, for a refusal.
func (k ResolutionKind) String() string {
	switch k {
	case ResolveNone:
		return "no resolution"
	case ResolveCallableInScope:
		return "a callable in scope"
	case ResolvePackageVar:
		return "a package variable"
	case ResolveValueField:
		return "a field of the subject's value"
	case ResolveHostParam:
		return "a parameter of the host callable"
	case ResolveMemberOnHandle:
		return "a member of the subject's handle"
	case ResolveMetadataKey:
		return "a metadata key or group"
	case ResolveTypeInScope:
		return "a type in scope"
	case ResolveDiagnosticCode:
		return "a registered diagnostic code"
	default:
		return "resolution kind " + strconv.Itoa(int(k))
	}
}

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

// Valid reports whether the key is one whole identifier of the grammar.
// An identifier is a letter followed by letters, digits, hyphens and
// underscores. Registration refuses a schema with a key that is not
// valid. Valid does not allocate.
func (k ParamKey) Valid() bool { return isIdentifier(string(k)) }

// ParamSpec declares one param a schema accepts: its spelling, its
// type, and the conditions under which an instance must or may
// write it.
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
	// Choices closes a string param to the spellings it lists.
	// Validation refuses another value under [BadSpelling], and the
	// message lists the choices. Registration refuses choices on a
	// param of another type, an empty choice and a repeated choice.
	// Empty admits every string.
	Choices []string
	// Counterexample marks a param whose value names an input no
	// derivation could invent. The kernel records the mark and
	// reads nothing from it.
	Counterexample bool
	// Doc states the param's meaning. Registration refuses an
	// empty one.
	Doc string
	// Deprecated marks the param deprecated where it is not empty, and
	// states the rewrite, such as "write bound= in place of limit=".
	// Validation types the param as before, and reports each instance
	// that writes it under [DeprecatedDirective].
	Deprecated string
}

// Variant is one variant of a schema, such as writer in shape writer. The
// first positional argument of an instance selects it. Its params join the
// schema's, and its roles apply in place of the schema's.
//
// A variant is data, checked whole with its schema at registration: a name
// the grammar cannot spell, a name declared twice, an empty doc, and a
// param with the key of a schema param are refused there, and its params
// and roles are checked as a schema's are.
type Variant struct {
	// Name is the spelling of the first positional argument.
	Name string
	// Params declares the keyed params of the variant, beside the
	// schema's Params, which apply to every variant.
	Params []ParamSpec
	// Roles is the closed set of values the role key accepts on an
	// instance of the variant.
	Roles []string
	// RolesRequired refuses an instance of the variant that writes no
	// role.
	RolesRequired bool
	// Doc states the variant's meaning. Registration refuses an empty one.
	Doc string
	// Deprecated marks the variant deprecated where it is not empty, and
	// states the rewrite. Validation types each instance as before, and
	// reports it under [DeprecatedDirective].
	Deprecated string
}

// Schema declares one directive: the closed contract between the
// author who writes an instance in source and the plugin whose
// handler receives it.
//
// A schema is a value with no behaviour. [Registry.Register]
// checks it whole and refuses what the field comments below
// forbid. [Validate] checks every instance against it and types the
// values, and nothing reads it after that. Closure is the default:
// a key the schema does not declare is refused, so what a handler
// may assume is exactly what the schema states, and only a schema
// stating Open admits keys it does not name.
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
	// Params declares the keyed params. Unknown keys are refused
	// unless Open types them.
	Params []ParamSpec
	// Open types every key the Params do not declare. Nil closes the
	// schema, which is the default. The spec states no Key: the
	// instance's own spelling is the key, and the value types as the
	// spec declares. The reserved keys keep their meaning under an
	// open schema: role, out and tag are never read as open keys.
	Open *ParamSpec
	// Roles is the closed set of values the role key accepts. A
	// schema listing none refuses the role key. Declaring roles is
	// what reserves the key: no entry in Params spells "role".
	Roles []string
	// RolesRequired refuses an instance that writes no role, for a
	// schema where the role decides what applies. Without it a
	// bare instance is admitted, and what it means is the schema's
	// documented semantic.
	RolesRequired bool
	// Variants declares the closed set of variants. Where it is not
	// empty, the first positional argument of every instance is the name
	// of a variant, and Positional declares the positional arguments
	// after it. A schema with variants declares no Roles, because each
	// variant declares its own.
	Variants []Variant
	// Repeatable admits more than one instance per subject, and one
	// instance of each variant of a schema with variants.
	// Single-instance is the default: a second instance is a
	// contradiction, reported naming both positions.
	Repeatable bool
	// Negatable admits the negated form, which opts a subject out of
	// the plugin's bare and fact-gated rules. The zero value refuses
	// a negated instance with a positioned Error. Registration
	// refuses a negatable kernel schema.
	Negatable bool
	// Overrides marks an override schema. A valid instance withdraws its
	// subject from the bare and fact-gated rules of the plugin, as a
	// negated instance does, and the rules that a directive gates still
	// run. A plugin declares one where an author's statement replaces
	// what its own rules derive. Registration refuses an override kernel
	// schema.
	Overrides bool
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
	// Deprecated marks the directive deprecated where it is not empty,
	// and states the rewrite, such as "write sizer:bound in place of
	// sizer:limit". Validation types each instance as before, and reports
	// it under [DeprecatedDirective].
	Deprecated string
}

// Canonical returns the schema's canonical spelling: prefixed for
// a plugin's, bare for the kernel's.
func (s Schema) Canonical() Name {
	if s.Plugin == "" {
		return s.Name
	}
	return Name(s.Plugin + string(prefixSep) + string(s.Name))
}
