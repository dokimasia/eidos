// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// SourceRules is the read-side contract of one language: the decisions
// inside the kernel's walks, what a spelling names in a scope, the
// values of a type, and the naming join. A language the composition
// registers rules for implements it.
//
// # Concurrency
//
// An implementation is safe for concurrent use, because the run calls
// it from parallel plans and from parallel validations. It keeps no
// state of its own, and every method that reads takes the [View] it
// reads through.
//
// # Allocation contract
//
// The kernel sets no ceiling on an implementation. A projection that
// calls a method allocates what the method allocates, and states so.
type SourceRules interface {
	// Lang returns the language the rules apply to.
	Lang() symbol.Lang

	// Members returns the policy of the language's member walk.
	Members() MemberPolicy

	// ParamRole classifies one parameter of a callable.
	ParamRole(p *node.Param, v View) ParamRole

	// ReturnRoles classifies a callable's returns, one role per
	// return in order, and names the error model the signature
	// states.
	ReturnRoles(rs []*node.Return, v View) ([]ReturnRole, ErrorModel)

	// Builtin projects a named reference the resolution step left
	// without a target: a builtin, a well-known type, an external.
	// It returns [symbol.FormOpaque] for a spelling it cannot
	// classify. Where it returns a structural form without children
	// for a reference with type arguments, the kernel's fold takes the
	// folded arguments as the form's children.
	Builtin(ref *node.TypeRef, v View) TypeShape

	// Resolve returns the declaration a human's spelling names in a
	// scope, at a declared resolution kind, or an error naming what
	// it looked for.
	Resolve(scope Scope, name string, kind directive.ResolutionKind, v View) (symbol.Symbol, error)

	// SamplesOf returns two distinct values of a type, or two
	// refusals with the reason. The kernel reads the values an
	// author stated for the type before it asks. A composite's walk
	// reads the authored values of each element through
	// [View.Authored] before it derives the element, and pairs a
	// stated half with a derived one through [Complete].
	SamplesOf(ref *node.TypeRef, hint string, v View) (sample, alternate Sample)

	// ZeroValue returns the type's zero value, and reports false
	// where the language has no spelling for it.
	ZeroValue(ref *node.TypeRef, v View) (emit.Value, bool)

	// LiteralFor turns text that already went through the
	// language's quoting into a value of a type, and reports false
	// where the text cannot be one.
	LiteralFor(f *node.File, ref *node.TypeRef, text string, v View) (emit.Value, bool)

	// TypeName joins a generator's word onto an author's base name
	// the way the language spells a derived type name.
	TypeName(word, base string) string
}

// Scope is where a spelling is read: the subject the spelling is
// written on, and the file whose imports qualify it.
type Scope struct {
	Subject symbol.Identity
	File    *node.File
}

// ParamRole classifies a callable's parameter.
type ParamRole uint8

const (
	// ParamInput is an ordinary input, the default.
	ParamInput ParamRole = iota
	// ParamContext is a context-like parameter: a cancellation, a
	// deadline, a trace, which a generated call passes through and
	// does not sample.
	ParamContext
)

// String returns the role's spelling, and the decimal number of an
// undeclared role. It allocates nothing for a declared role.
func (r ParamRole) String() string {
	switch r {
	case ParamInput:
		return "input"
	case ParamContext:
		return "context"
	default:
		return strconv.Itoa(int(r))
	}
}

// ReturnRole classifies a callable's return.
type ReturnRole uint8

const (
	// ReturnValue is an ordinary result, the default.
	ReturnValue ReturnRole = iota
	// ReturnOkBool is a presence flag beside a value.
	ReturnOkBool
	// ReturnStream is a stream of values.
	ReturnStream
	// ReturnError is the failure, where the error model is
	// LastReturn or ResultType.
	ReturnError
)

// String returns the role's spelling, and the decimal number of an
// undeclared role. It allocates nothing for a declared role.
func (r ReturnRole) String() string {
	switch r {
	case ReturnValue:
		return "value"
	case ReturnOkBool:
		return "ok-bool"
	case ReturnStream:
		return "stream"
	case ReturnError:
		return "error"
	default:
		return strconv.Itoa(int(r))
	}
}

// ErrorModel names how a callable reports failure.
type ErrorModel uint8

const (
	// ErrorsNone states no failure channel.
	ErrorsNone ErrorModel = iota
	// ErrorsLastReturn reports the failure in the last return: Go.
	ErrorsLastReturn
	// ErrorsResultType wraps the failure in a result type: Rust.
	ErrorsResultType
	// ErrorsThrown throws a declared exception: Java's checked
	// throws, Swift's typed throws.
	ErrorsThrown
	// ErrorsRaised raises without declaring: Python, TypeScript.
	ErrorsRaised
)

// String returns the model's spelling, and the decimal number of an
// undeclared model. It allocates nothing for a declared model.
func (m ErrorModel) String() string {
	switch m {
	case ErrorsNone:
		return "none"
	case ErrorsLastReturn:
		return "last-return"
	case ErrorsResultType:
		return "result-type"
	case ErrorsThrown:
		return "thrown"
	case ErrorsRaised:
		return "raised"
	default:
		return strconv.Itoa(int(m))
	}
}

// Contribution names one member list the walk reads.
type Contribution uint8

const (
	// ContributesEmbeds follows the compositional promotion list.
	ContributesEmbeds Contribution = iota + 1
	// ContributesExtends follows the nominal supertypes.
	ContributesExtends
	// ContributesImplements follows the implemented interfaces.
	ContributesImplements
)

// String returns the contribution's spelling, and the decimal number of
// an undeclared contribution. It allocates nothing for a declared
// contribution.
func (c Contribution) String() string {
	switch c {
	case ContributesEmbeds:
		return "embeds"
	case ContributesExtends:
		return "extends"
	case ContributesImplements:
		return "implements"
	default:
		return strconv.Itoa(int(c))
	}
}

// Shadowing is the language's rule for one name that arrives twice.
type Shadowing uint8

const (
	// ShadowPromote takes the shallowest arrival. Two arrivals at one
	// depth cancel each other: Go.
	ShadowPromote Shadowing = iota + 1
	// ShadowOverride settles each signature apart, taking the
	// nearer declaration over the farther and the first at one
	// depth over the rest, so a declared method hides no inherited
	// overload: the JVM languages.
	ShadowOverride
	// ShadowMerge keeps every arrival, as overloads: TypeScript
	// interfaces.
	ShadowMerge
	// ShadowLinearise takes the nearest arrival, the first at one
	// depth over the rest: Python.
	ShadowLinearise
)

// String returns the rule's spelling, and the decimal number of an
// undeclared rule. It allocates nothing for a declared rule.
func (s Shadowing) String() string {
	switch s {
	case ShadowPromote:
		return "promote"
	case ShadowOverride:
		return "override"
	case ShadowMerge:
		return "merge"
	case ShadowLinearise:
		return "linearise"
	default:
		return strconv.Itoa(int(s))
	}
}

// DefaultDepth bounds a member walk that states no budget of its
// own: no compiling source nests contributors this deep, and a
// hand-built graph can.
const DefaultDepth = 8

// MemberPolicy is what a language states about its member walk.
type MemberPolicy struct {
	// Contributes lists the member lists a type's effective set
	// reads, in walk order: Embeds for Go, Extends then Implements
	// for the JVM languages, Extends for TypeScript. A policy
	// listing none contributes nothing beyond the declared members.
	Contributes []Contribution
	// Shadowing states how two members of one name settle. The zero
	// value settles nothing and keeps the declared members alone.
	Shadowing Shadowing
	// Depth bounds the walk. 0 takes [DefaultDepth].
	Depth int
	// EmbedsAreFields records each embed of a struct as a member:
	// the embedded field, named by its identity's name, at the depth
	// of the type that declares it. Go selects an embedded field by
	// that name before any member it promotes. An interface's embeds
	// record nothing, because an embedded interface declares no
	// field.
	EmbedsAreFields bool
}

// Registry maps each language to its [SourceRules].
//
// The zero Registry looks up and lists no language, and
// [Registry.Register] panics on it. Build one with [NewRegistry].
//
// # Concurrency
//
// A Registry is not safe for concurrent use while it registers, which
// the composition does on one goroutine. It is read-only afterwards and
// safe to read from every plan.
//
// # Allocation contract
//
// [NewRegistry] allocates the registry and its map, two allocations.
// The first registration allocates the map's first group, and a later
// one allocates only as the map grows. A lookup of a registered language
// allocates nothing.
type Registry struct {
	byLang map[symbol.Lang]SourceRules // the rules of each registered language
}

// NewRegistry returns an empty registry. It allocates the registry and
// its map, two allocations.
func NewRegistry() *Registry {
	return &Registry{byLang: map[symbol.Lang]SourceRules{}}
}

// Register records one language's rules.
//
// Error modes: a plain error for nil rules, for rules that declare the
// zero language, and for a second value of one language. Only a defect
// of the composition produces any of them.
//
// # Allocation contract
//
// The first registration allocates the map's first group, one
// allocation. A later one allocates only as the map grows.
func (r *Registry) Register(rules SourceRules) error {
	if rules == nil {
		return errors.New("rules: register nil rules")
	}
	lang := rules.Lang()
	if lang == "" {
		return errors.New("rules: register rules of the zero language")
	}
	if _, taken := r.byLang[lang]; taken {
		return fmt.Errorf("rules: register %s twice", lang)
	}
	r.byLang[lang] = rules
	return nil
}

// For returns the rules registered for a language and true, and the
// [Absent] rules and false where none registered. It allocates nothing
// for a registered language, and the absent rules, one allocation, for
// another.
func (r *Registry) For(lang symbol.Lang) (SourceRules, bool) {
	if held, registered := r.byLang[lang]; registered {
		return held, true
	}
	return Absent(lang), false
}

// Languages returns every registered language, sorted by spelling, so a
// listing is stable. It allocates the list, one allocation.
func (r *Registry) Languages() []symbol.Lang {
	out := make([]symbol.Lang, 0, len(r.byLang))
	for lang := range r.byLang {
		out = append(out, lang)
	}
	slices.Sort(out)
	return out
}

// Absent returns the rules of a language the composition registered
// none for: a member policy contributing nothing, every parameter an
// input and every return a value under no error model, Opaque for every
// builtin, a failing Resolve, and values that refuse with
// [RefusedNoRules]. The refusal is a value, so a generator's OK gate
// works without a nil check. The rules satisfy no optional capability.
//
// # Allocation contract
//
// Absent allocates the value the interface contains, one allocation.
func Absent(lang symbol.Lang) SourceRules { return absent{lang: lang} }

// IsAbsent reports whether rules are the value [Absent] returns. It
// reports false for nil, and allocates nothing.
func IsAbsent(r SourceRules) bool {
	_, is := r.(absent)
	return is
}

// absent is the implementation [Absent] returns.
type absent struct {
	lang symbol.Lang // the language the composition registered no rules for
}

// Lang returns the language the value was minted for.
func (a absent) Lang() symbol.Lang { return a.lang }

// Members returns the zero policy, which contributes nothing.
func (absent) Members() MemberPolicy { return MemberPolicy{} }

// ParamRole classifies every parameter as an input.
func (absent) ParamRole(*node.Param, View) ParamRole {
	return ParamInput
}

// ReturnRoles classifies every return as a value under no error
// model. It allocates the list of roles.
func (absent) ReturnRoles(rs []*node.Return, _ View) ([]ReturnRole, ErrorModel) {
	return make([]ReturnRole, len(rs)), ErrorsNone
}

// Builtin returns Opaque for every spelling.
func (absent) Builtin(ref *node.TypeRef, _ View) TypeShape {
	return Opaque(ref)
}

// Resolve returns an error naming the language without rules and
// the spelling it was asked for.
func (a absent) Resolve(_ Scope, name string, kind directive.ResolutionKind, _ View) (symbol.Symbol, error) {
	return nil, fmt.Errorf("rules: no rules registered for %s resolve %q at %v", a.lang, name, kind)
}

// SamplesOf refuses both halves with [RefusedNoRules].
func (absent) SamplesOf(*node.TypeRef, string, View) (Sample, Sample) {
	return Refused(RefusedNoRules), Refused(RefusedNoRules)
}

// ZeroValue reports false for every type.
func (absent) ZeroValue(*node.TypeRef, View) (emit.Value, bool) {
	return emit.Value{}, false
}

// LiteralFor reports false for every text.
func (absent) LiteralFor(*node.File, *node.TypeRef, string, View) (emit.Value, bool) {
	return emit.Value{}, false
}

// TypeName joins the word onto the base unchanged.
func (absent) TypeName(word, base string) string { return base + word }
