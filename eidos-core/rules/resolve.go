// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// The declaration kinds that each resolution of [Resolve] accepts.
var (
	functionKinds = []symbol.Kind{symbol.KindFunction}
	methodKinds   = []symbol.Kind{symbol.KindMethod}
	valueKinds    = []symbol.Kind{symbol.KindConstant, symbol.KindVariable}
	fieldKinds    = []symbol.Kind{symbol.KindField}
	memberKinds   = []symbol.Kind{symbol.KindField, symbol.KindMethod}
	typeKinds     = []symbol.Kind{
		symbol.KindStruct, symbol.KindInterface, symbol.KindAlias, symbol.KindEnum, symbol.KindSum,
	}
)

// Resolver contains the decisions of one language in the resolution of the
// spelling of a directive. [Resolve] walks the rest.
//
// # Concurrency
//
// An implementation is safe for concurrent use, as every [SourceRules]
// is, and reads through the [View] that each method takes.
type Resolver interface {
	// InScope returns the first declaration of one of the kinds that name
	// refers to in the scope of the subject, in the probe order of the
	// language. It returns an error where no declaration matches, and for
	// a name that is not a name of the language.
	InScope(scope Scope, name string, kinds []symbol.Kind, v View) (symbol.Symbol, error)
	// Usable reports whether code of the subject can use a member that a
	// type declares or inherits. own is the own type of the subject where
	// the walk searches the members of that type, and the zero identity
	// where it searches another type or the subject has no own type.
	Usable(m Member, own symbol.Identity, scope Scope) bool
	// Placeholder returns the declaration that a type name resolves to
	// where the view declares none, such as a predeclared type, and false
	// where the name resolves to nothing.
	Placeholder(scope Scope, name string, v View) (symbol.Symbol, bool)
	// Value returns the reference whose target is the type of a value of
	// the type ref, such as the target of a pointer. It returns nil for a
	// reference without such a type.
	Value(ref *node.TypeRef, v View) *node.TypeRef
}

// ResolveError is the refusal of a resolution. Its text starts with the
// identity of the language, as every refusal of the rules of a language
// does.
type ResolveError struct {
	Lang symbol.Lang // the language whose rules refuse
	Msg  string      // the reason, without the language
}

// Error returns the language, a colon and the reason. It allocates the
// text.
func (e *ResolveError) Error() string { return string(e.Lang) + ": " + e.Msg }

// Resolve resolves a spelling from a subject through the decisions of r,
// the rules src of the language and the member walk over the view v. The
// resolution kind selects the declaration that the walk looks for:
//
//   - A callable resolves among the methods of the type of a method
//     subject, which the method calls through its receiver, and then to a
//     function in scope.
//   - A package variable resolves to a constant or a variable in scope.
//   - A type resolves to a declaration in scope, and then to the
//     placeholder of the language.
//   - A value field and a member on a handle resolve among the effective
//     members of the value or the handle of the subject: the subject
//     itself for a type, the type of a field, and for a callable the type
//     of its first value return, and for a value field the type of each
//     input parameter after it.
//   - A host parameter resolves on the signature of the subject.
//
// A member resolves only where [Resolver.Usable] reports that the subject
// can use it.
//
// Error modes: a refusal of the walk is a *[ResolveError] under the
// language of src, for an empty spelling, for a kind that the walk does
// not perform, and for a spelling that resolves to no declaration. Resolve
// returns an error of the resolver unchanged. Nil rules resolve under
// [Absent] for the zero language.
//
// # Allocation contract
//
// Resolve allocates what the resolver allocates. Its binding of the
// member walk has no memo, so a resolution in scope allocates no object of
// its own. A member resolution also allocates the list of the types that
// it searches and what the member walk allocates. A refusal allocates its
// error.
func Resolve(
	r Resolver, src SourceRules, v View, scope Scope, name string, kind directive.ResolutionKind,
) (symbol.Symbol, error) {
	if src == nil {
		src = Absent("")
	}
	b := Bound{source: src, forLang: Absent, view: v}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, refusal(b, "nothing to resolve")
	}
	switch kind {
	case directive.ResolveCallableInScope:
		return callableNamed(r, b, scope, name)
	case directive.ResolvePackageVar:
		return r.InScope(scope, name, valueKinds, b.view)
	case directive.ResolveValueField:
		return memberIn(r, b, scope, name, true, fieldKinds)
	case directive.ResolveHostParam:
		return hostParam(b, scope, name)
	case directive.ResolveMemberOnHandle:
		return memberIn(r, b, scope, name, false, memberKinds)
	case directive.ResolveTypeInScope:
		sym, err := r.InScope(scope, name, typeKinds, b.view)
		if err == nil {
			return sym, nil
		}
		if placeholder, held := r.Placeholder(scope, name, b.view); held {
			return placeholder, nil
		}
		return nil, err
	default:
		return nil, refusal(b, "%s is not a resolution that %s performs", kind, b.Lang())
	}
}

// callableNamed resolves a callable from a subject. For a method subject,
// a method of the type of the method comes before a function in scope.
// Every other subject resolves a function in scope alone.
func callableNamed(r Resolver, b Bound, scope Scope, name string) (symbol.Symbol, error) {
	sym, _ := b.view.Lookup(scope.Subject)
	method, isMethod := sym.(*node.Method)
	if !isMethod {
		return r.InScope(scope, name, functionKinds, b.view)
	}
	owner, _ := b.view.Lookup(method.Host)
	if host, isType := owner.(node.Declaration); isType {
		if sibling, found := memberNamed(r, b, host, name, method.Host, scope, methodKinds); found {
			return sibling, nil
		}
	}
	if fn, err := r.InScope(scope, name, functionKinds, b.view); err == nil {
		return fn, nil
	}
	return nil, refusal(b, "%s has no method named %s, and no function named %s is in scope of %s",
		method.Host, name, name, scope.Subject)
}

// memberIn resolves a name among the effective members of the types that
// [holders] returns for the subject. The first type with a usable member
// of the name and one of the kinds decides. value selects the value of a
// callable over its handle.
func memberIn(r Resolver, b Bound, scope Scope, name string, value bool, kinds []symbol.Kind) (symbol.Symbol, error) {
	own, types, err := holders(r, b, scope.Subject, value)
	if err != nil {
		return nil, err
	}
	for _, t := range types {
		if m, found := memberNamed(r, b, t, name, own, scope, kinds); found {
			return m, nil
		}
	}
	searched := make([]string, 0, len(types))
	for _, t := range types {
		searched = append(searched, t.Identity().String())
	}
	return nil, refusal(b, "%s has no %s member named %s that %s can use",
		strings.Join(searched, " or "), kindList(kinds), name, scope.Subject)
}

// holders returns the own type of the subject, and the types among whose
// members a member reference of the subject resolves. For a type, that is
// the subject itself, and for a field, the type of the field. For a
// callable, it is the types of its value, which [valueTypes] returns. A
// function has no own type, and a method has the type of the method.
// holders refuses a subject, or the type of a field, that the view does
// not declare.
func holders(r Resolver, b Bound, subject symbol.Identity, value bool) (symbol.Identity, []node.Declaration, error) {
	sym, _ := b.view.Lookup(subject)
	switch s := sym.(type) {
	case *node.Field:
		owner, _ := b.view.Lookup(s.Host)
		if decl, isType := typeDecl(owner); isType {
			return s.Host, []node.Declaration{decl}, nil
		}
	case *node.Function:
		types, err := valueTypes(r, b, subject, s.Params, s.Returns, value)
		return symbol.Identity{}, types, err
	case *node.Method:
		types, err := valueTypes(r, b, subject, s.Params, s.Returns, value)
		return s.Host, types, err
	default:
		if decl, isType := typeDecl(sym); isType {
			return subject, []node.Declaration{decl}, nil
		}
	}
	return symbol.Identity{}, nil, refusal(b, "%s does not belong to a type that the view declares", subject)
}

// valueTypes returns the declarations of the value of a callable. They are
// the type of its first value return, and for a value the type of each
// input parameter after it, in order. [Resolver.Value] returns the type of
// each reference. A type without a declaration in the view, such as a builtin,
// has no members and is left out. valueTypes refuses a callable whose
// value has no declaration.
func valueTypes(
	r Resolver, b Bound, subject symbol.Identity, params []*node.Param, returns []*node.Return, value bool,
) ([]node.Declaration, error) {
	var out []node.Declaration
	roles, _ := b.source.ReturnRoles(returns, b.view)
	for i, ret := range returns {
		if ret != nil && roles[i] == ReturnValue {
			out = appendValue(r, b, out, ret.Type)
			break
		}
	}
	if value {
		for _, p := range params {
			if p != nil && b.source.ParamRole(p, b.view) == ParamInput {
				out = appendValue(r, b, out, p.Type)
			}
		}
	}
	if len(out) == 0 {
		return nil, refusal(b, "%s has no value of a type that the view declares", subject)
	}
	return out, nil
}

// appendValue appends to out the declaration of the type of a value of
// ref, and returns out unchanged where the view declares no such type.
func appendValue(r Resolver, b Bound, out []node.Declaration, ref *node.TypeRef) []node.Declaration {
	if ref == nil {
		return out
	}
	t := r.Value(ref, b.view)
	if t == nil {
		return out
	}
	sym, _ := b.view.Lookup(t.Target)
	if decl, isType := typeDecl(sym); isType {
		return append(out, decl)
	}
	return out
}

// typeDecl returns the declaration of a type with members, and false for
// every other symbol, nil included.
func typeDecl(sym symbol.Symbol) (node.Declaration, bool) {
	switch d := sym.(type) {
	case *node.Struct:
		return d, true
	case *node.Interface:
		return d, true
	case *node.Enum:
		return d, true
	case *node.Sum:
		return d, true
	default:
		return nil, false
	}
}

// memberNamed returns the effective member of host with a name and one of
// the kinds that the subject can use, and false where host has none. The
// walk passes own to [Resolver.Usable] where host is the own type of the
// subject, and the zero identity otherwise.
func memberNamed(
	r Resolver, b Bound, host node.Declaration, name string, own symbol.Identity, scope Scope,
	kinds []symbol.Kind,
) (symbol.Symbol, bool) {
	if host.Identity() != own {
		own = symbol.Identity{}
	}
	set, _ := b.MembersOf(host)
	for _, m := range set.Members {
		decl, names := m.Symbol.(node.Declaration)
		if !names {
			continue
		}
		if id := decl.Identity(); id.Name == name && slices.Contains(kinds, id.Kind) && r.Usable(m, own, scope) {
			return m.Symbol, true
		}
	}
	return nil, false
}

// hostParam resolves a parameter on the signature of the subject.
func hostParam(b Bound, scope Scope, name string) (symbol.Symbol, error) {
	sym, held := b.view.Lookup(scope.Subject)
	if !held {
		return nil, refusal(b, "%s is outside the view", scope.Subject)
	}
	var params []*node.Param
	switch c := sym.(type) {
	case *node.Function:
		params = c.Params
	case *node.Method:
		params = c.Params
	default:
		return nil, refusal(b, "%s is not a callable, so it has no parameter named %s", scope.Subject, name)
	}
	for _, p := range params {
		if p != nil && p.Name == name {
			return p, nil
		}
	}
	return nil, refusal(b, "%s has no parameter named %s", scope.Subject, name)
}

// kindList joins the names of a set of kinds for a refusal.
func kindList(kinds []symbol.Kind) string {
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, k.String())
	}
	return strings.Join(parts, " or ")
}

// refusal returns a *[ResolveError] under the language of b, with the
// reason that format and args write.
func refusal(b Bound, format string, args ...any) error {
	return &ResolveError{Lang: b.Lang(), Msg: fmt.Sprintf(format, args...)}
}
