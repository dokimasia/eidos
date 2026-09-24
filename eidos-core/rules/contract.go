// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"errors"
	"fmt"
	"strconv"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// SourceRules is what every language returns. Implementing it is
// what makes a language a language on the read side.
//
// A value is safe for concurrent use: the run calls it from
// parallel plans and from parallel validations, so it holds no
// state of its own, and every method that reads takes the [View]
// it reads through.
type SourceRules interface {
	// Lang names the language the rules return for.
	Lang() symbol.Lang

	// Members states how this language's member walk proceeds.
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
	// classify.
	Builtin(ref *node.TypeRef, v View) TypeShape

	// Resolve says what a human's spelling names in a scope, at a
	// declared resolution kind. It returns the declaration or an
	// error naming what it looked for.
	Resolve(scope Scope, name string, kind directive.ResolutionKind, v View) (symbol.Symbol, error)

	// SamplesOf returns two distinct values of a type, or two
	// refusals carrying the reason. Authored values are read by
	// the kernel before this is asked.
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

// Scope is where a spelling is read: the subject that carried it,
// and the file whose imports qualify it.
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
	// deadline, a trace, which a generated call threads through
	// rather than samples.
	ParamContext
)

// String returns the role's spelling.
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
	// ReturnStream is a stream of values rather than one.
	ReturnStream
	// ReturnError carries the failure where the error model is
	// LastReturn or ResultType.
	ReturnError
)

// String returns the role's spelling.
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

// ErrorModel says how a callable reports failure.
type ErrorModel uint8

const (
	// ErrorsNone states no failure channel.
	ErrorsNone ErrorModel = iota
	// ErrorsLastReturn carries the failure as the last return: Go.
	ErrorsLastReturn
	// ErrorsResultType carries the failure inside a result type:
	// Rust.
	ErrorsResultType
	// ErrorsThrown throws a declared exception: Java's checked
	// throws, Swift's typed throws.
	ErrorsThrown
	// ErrorsRaised raises without declaring: Python, TypeScript.
	ErrorsRaised
)

// String returns the model's spelling.
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

// Contribution names one member list the walk draws on.
type Contribution uint8

const (
	// ContributesEmbeds follows the compositional promotion list.
	ContributesEmbeds Contribution = iota + 1
	// ContributesExtends follows the nominal supertypes.
	ContributesExtends
	// ContributesImplements follows the implemented interfaces.
	ContributesImplements
)

// String returns the contribution's spelling.
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

// Shadowing is the language's rule for one name reached twice.
type Shadowing uint8

const (
	// ShadowPromote takes the shallowest arrival; two at one depth
	// cancel both: Go.
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

// String returns the rule's spelling.
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
	// draws on, in walk order: Embeds for Go, Extends then
	// Implements for the JVM languages, Extends for TypeScript. A
	// policy listing none contributes nothing beyond the declared
	// members.
	Contributes []Contribution
	// Shadowing says how two members of one name settle. The zero
	// value settles nothing and keeps the declared members alone.
	Shadowing Shadowing
	// Depth bounds the walk; 0 takes [DefaultDepth].
	Depth int
	// EmbedsAreFields records each embed of a struct as a member:
	// the embedded field, named by its identity's name, at the depth
	// of the type that declares it. Go selects an embedded field by
	// that name before any member it promotes. An interface's embeds
	// record nothing, because an embedded interface declares no
	// field.
	EmbedsAreFields bool
}

// Registry holds one [SourceRules] per language.
//
// A Registry is not safe for concurrent use while it registers,
// which the composition does single-threaded; it is read-only
// afterwards and safe to read from every plan.
type Registry struct {
	byLang map[symbol.Lang]SourceRules
}

// NewRegistry returns a registry holding nothing.
func NewRegistry() *Registry {
	return &Registry{byLang: map[symbol.Lang]SourceRules{}}
}

// Register records one language's rules. It refuses a nil value, a
// value declaring the zero language, and a second value for one
// language.
func (r *Registry) Register(rules SourceRules) error {
	if rules == nil {
		return errors.New("rules: a nil value returns for no language")
	}
	lang := rules.Lang()
	if lang == "" {
		return errors.New("rules: a value declaring the zero language returns for none")
	}
	if _, taken := r.byLang[lang]; taken {
		return fmt.Errorf("rules: %s registers its rules twice", lang)
	}
	r.byLang[lang] = rules
	return nil
}

// For returns the rules registered for a language, and the
// [Absent] value with false where none registered.
func (r *Registry) For(lang symbol.Lang) (SourceRules, bool) {
	if held, registered := r.byLang[lang]; registered {
		return held, true
	}
	return Absent(lang), false
}

// Languages returns every registered language, in registration
// order of nothing in particular: sorted, so a listing is stable.
func (r *Registry) Languages() []symbol.Lang {
	out := make([]symbol.Lang, 0, len(r.byLang))
	for lang := range r.byLang {
		out = append(out, lang)
	}
	sortLangs(out)
	return out
}

// Absent returns the rules of a language the composition
// registered none for: a member policy contributing nothing, every
// parameter an input and every return a value under no error
// model, Opaque for every builtin, a failing Resolve, and values
// that refuse with [RefusedNoRules]. The refusal is a value, so a
// generator's OK gate holds without a nil check.
func Absent(lang symbol.Lang) SourceRules { return absent{lang: lang} }

// IsAbsent reports whether a value is the one [Absent] returns.
func IsAbsent(r SourceRules) bool {
	_, is := r.(absent)
	return is
}

// absent is the refusing implementation.
type absent struct {
	lang symbol.Lang
}

func (a absent) Lang() symbol.Lang   { return a.lang }
func (absent) Members() MemberPolicy { return MemberPolicy{} }
func (absent) ParamRole(*node.Param, View) ParamRole {
	return ParamInput
}

func (absent) ReturnRoles(rs []*node.Return, _ View) ([]ReturnRole, ErrorModel) {
	return make([]ReturnRole, len(rs)), ErrorsNone
}

func (absent) Builtin(ref *node.TypeRef, _ View) TypeShape {
	return Opaque(ref)
}

func (a absent) Resolve(_ Scope, name string, kind directive.ResolutionKind, _ View) (symbol.Symbol, error) {
	return nil, fmt.Errorf("rules: no rules registered for %s resolve %q at %v", a.lang, name, kind)
}

func (absent) SamplesOf(*node.TypeRef, string, View) (Sample, Sample) {
	return Refused(RefusedNoRules), Refused(RefusedNoRules)
}

func (absent) ZeroValue(*node.TypeRef, View) (emit.Value, bool) {
	return emit.Value{}, false
}

func (absent) LiteralFor(*node.File, *node.TypeRef, string, View) (emit.Value, bool) {
	return emit.Value{}, false
}

func (absent) TypeName(word, base string) string { return base + word }
