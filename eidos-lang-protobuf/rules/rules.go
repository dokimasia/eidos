// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/naming"
	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// hostDepth bounds the walk from a subject up to its message. The
// deepest walk is a oneof member's field: the field, its variant, its
// oneof, then the message.
const hostDepth = 4

// The kinds a type spelling names, and the kinds a callable spelling
// names.
var (
	typeKinds     = []symbol.Kind{symbol.KindStruct, symbol.KindEnum, symbol.KindSum}
	callableKinds = []symbol.Kind{symbol.KindInterface, symbol.KindMethod}
)

// Rules is protobuf's implementation of the source-rules contract.
//
// The zero value is ready and is the only value: it has no state, so
// one value is safe for concurrent use and may be shared across
// compositions. Every method reads through the [rules.View] it is
// handed and never caches, so a value belongs to the invocation that
// asked for it.
//
// It satisfies [rules.EnumRules] and no other optional capability:
// protobuf states no generics, no promotion, no struct tags and no
// user-defined equality.
type Rules struct{}

// New returns the protobuf rules, the value a composition
// registers with [workspace.Builder.Rules].
//
// The returned value is [Rules]; a caller needing the enum
// capability asserts [rules.EnumRules] on it.
func New() rules.SourceRules { return Rules{} }

// Lang returns [protobuf.Lang], the language a composition keys
// these rules under and the language of every subject they project.
func (Rules) Lang() symbol.Lang { return protobuf.Lang }

// Members reports that nothing contributes to a type's member set.
//
// protobuf states no embedding, no supertypes and no implemented
// interfaces, so a message's effective members are exactly its
// declared fields and the kernel's walk descends into nothing. An
// extension is the one construct that would add a member from
// elsewhere, and the frontend refuses it at the load, so the walk
// never meets one.
//
// The shadowing rule is therefore never applied. It is stated as
// override so the policy is total and not zero-valued.
func (Rules) Members() rules.MemberPolicy {
	return rules.MemberPolicy{Shadowing: rules.ShadowOverride}
}

// ParamRole reports [rules.ParamInput] for every parameter.
//
// An rpc takes one request message and a schema states no context
// parameter, so no parameter has another role. The view is unread.
func (Rules) ParamRole(*node.Param, rules.View) rules.ParamRole { return rules.ParamInput }

// ReturnRoles reports one role per return and the error model.
//
// A return whose type is [symbol.FormStream] is
// [rules.ReturnStream]; every other return is [rules.ReturnValue].
// The model is always [rules.ErrorsNone]: a schema states no failure
// in a signature, and a gRPC status is sent beside the response, not
// inside it, so a generator binding a call reads its transport's
// convention and not the schema's.
//
// The returned slice is one entry per return, allocated per call,
// and is the caller's to keep.
func (Rules) ReturnRoles(rs []*node.Return, _ rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	roles := make([]rules.ReturnRole, len(rs))
	for i, r := range rs {
		if r != nil && r.Type != nil && r.Type.Form == symbol.FormStream {
			roles[i] = rules.ReturnStream
		}
	}
	return roles, rules.ErrorsNone
}

// TypeName joins a generator's word onto a schema's name in
// PascalCase, which is how protobuf spells a message and what every
// generator over a schema produces.
//
// Both parts are recased, so a snake-cased schema name joins the
// same way a Pascal one does: TypeName("check", "row_key") is
// CheckRowKey.
func (Rules) TypeName(word, base string) string {
	return naming.Pascal(word) + naming.Pascal(base)
}

// Resolve returns the declaration a directive's spelling names from
// a subject, or an error naming the spelling and where it searched.
//
// The resolution kinds this language performs:
//
//   - [directive.ResolveValueField] and
//     [directive.ResolveMemberOnHandle] resolve among the fields of
//     the message the subject belongs to, the members of its oneofs
//     included, because a oneof member is a field of the message.
//   - [directive.ResolveHostParam] resolves on the subject's own rpc.
//   - [directive.ResolveTypeInScope] resolves a message, an enum or
//     a oneof; [directive.ResolveCallableInScope] resolves a service
//     or an rpc.
//
// A type or a callable resolves through the candidates the
// frontend's resolution names, [protobuf.Candidates]: from inside
// the subject's innermost message, then each enclosing message, the
// subject's namespace, each namespace above it and the root, each
// scope's first match taken. A leading dot states the whole path and
// probes nothing outward. Any other kind returns an error, and so
// does a spelling nothing declares; validation reports it under its
// own code.
func (Rules) Resolve(
	scope rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("protobuf: nothing to resolve")
	}
	switch kind {
	case directive.ResolveValueField, directive.ResolveMemberOnHandle:
		return member(scope, name, v)
	case directive.ResolveHostParam:
		return hostParam(scope, name, v)
	case directive.ResolveTypeInScope:
		return inScope(scope, name, v, typeKinds)
	case directive.ResolveCallableInScope:
		return inScope(scope, name, v, callableKinds)
	default:
		return nil, fmt.Errorf("protobuf: %s is not a resolution a schema performs", kind)
	}
}

// inScope resolves a spelling outward from the subject through the
// frontend's candidates, taking the first declaration of one of the
// kinds the view contains.
func inScope(scope rules.Scope, name string, v rules.View, kinds []symbol.Kind) (symbol.Symbol, error) {
	pkg, chain := scopeOf(scope.Subject, v)
	for _, tier := range protobuf.Candidates(pkg, chain, name) {
		for _, c := range tier {
			if sym, held := lookup(v, c, kinds); held {
				return sym, nil
			}
		}
	}
	from := pkg
	if chain != "" {
		from = pkg + protobuf.NameSep + chain
	}
	return nil, fmt.Errorf("protobuf: nothing named %s resolves from %s outward", name, from)
}

// lookup returns the declaration a candidate names under the first
// of the kinds the view contains. An rpc's identity is discriminated
// by its parameter types, which a spelling does not state, so an rpc
// is found among its service's methods by name.
func lookup(v rules.View, c symbol.Identity, kinds []symbol.Kind) (symbol.Symbol, bool) {
	for _, kind := range kinds {
		if kind == symbol.KindMethod {
			if m, held := rpcNamed(v, c); held {
				return m, true
			}
			continue
		}
		c.Kind = kind
		if sym, held := v.Lookup(c); held {
			return sym, true
		}
	}
	return nil, false
}

// rpcNamed returns the rpc a candidate names: the method of the
// candidate's name on the service its owner chain names.
func rpcNamed(v rules.View, c symbol.Identity) (symbol.Symbol, bool) {
	if c.Owner == "" {
		return nil, false
	}
	owner, name := ownerOf(c.Owner)
	sym, _ := v.Lookup(symbol.Identity{
		Lang: protobuf.Lang, Package: c.Package, Owner: owner, Name: name, Kind: symbol.KindInterface,
	})
	service, is := sym.(*node.Interface)
	if !is {
		return nil, false
	}
	for _, m := range service.Methods {
		if m != nil && m.Name == c.Name {
			return m, true
		}
	}
	return nil, false
}

// scopeOf returns the package and the message chain a directive on a
// subject resolves in: inside the innermost message that contains
// the subject, the subject itself where it is a message, and the
// package alone for a declaration outside every message.
func scopeOf(subject symbol.Identity, v rules.View) (pkg, chain string) {
	msg, held := enclosingMessage(subject, v)
	if !held {
		return subject.Package, ""
	}
	if msg.Owner == "" {
		return msg.Package, msg.Name
	}
	return msg.Package, msg.Owner + protobuf.NameSep + msg.Name
}

// enclosingMessage returns the innermost message that contains a
// subject, the subject itself where it is a message, and false where
// the view does not contain the subject or no message contains it.
//
// A member finds its message through its hosts: a field through its
// message or its oneof member, a oneof member through its oneof.
// A oneof, an enum and a service name their message by their owner
// chain, which only messages nest.
func enclosingMessage(subject symbol.Identity, v rules.View) (symbol.Identity, bool) {
	id := subject
	for range hostDepth {
		sym, held := v.Lookup(id)
		if !held {
			return symbol.Identity{}, false
		}
		switch d := sym.(type) {
		case *node.Struct:
			return d.ID, true
		case *node.Field:
			id = d.Host
		case *node.SumVariant:
			id = d.Host
		case *node.EnumVariant:
			id = d.Host
		case *node.Method:
			id = d.Host
		case *node.Sum, *node.Enum, *node.Interface:
			// The view keys each declaration by its identity, so id is
			// the declaration's own.
			if id.Owner == "" {
				return symbol.Identity{}, false
			}
			owner, name := ownerOf(id.Owner)
			id = symbol.Identity{
				Lang: id.Lang, Package: id.Package, Owner: owner, Name: name, Kind: symbol.KindStruct,
			}
		default:
			return symbol.Identity{}, false
		}
	}
	return symbol.Identity{}, false
}

// member resolves a name among the fields of the message a subject
// belongs to, the members of its oneofs included.
func member(scope rules.Scope, name string, v rules.View) (symbol.Symbol, error) {
	msgID, held := enclosingMessage(scope.Subject, v)
	if !held {
		if _, known := v.Lookup(scope.Subject); !known {
			return nil, fmt.Errorf("protobuf: the view does not contain %s", scope.Subject)
		}
		return nil, fmt.Errorf("protobuf: %s belongs to no message", scope.Subject)
	}
	sym, _ := v.Lookup(msgID)
	msg, _ := sym.(*node.Struct)
	if f := fieldNamed(msg, name); f != nil {
		return f, nil
	}
	return nil, fmt.Errorf("protobuf: no field of %s is named %s", msgID, name)
}

// fieldNamed returns a message's field of one name, a oneof
// member's field included, and nil where the message declares none.
func fieldNamed(msg *node.Struct, name string) *node.Field {
	if msg == nil {
		return nil
	}
	for _, f := range msg.Fields {
		if f != nil && f.Name == name {
			return f
		}
	}
	for _, t := range msg.Types {
		oneof, is := t.(*node.Sum)
		if !is {
			continue
		}
		for _, variant := range oneof.Variants {
			if variant == nil {
				continue
			}
			for _, f := range variant.Fields {
				if f != nil && f.Name == name {
					return f
				}
			}
		}
	}
	return nil
}

// hostParam resolves a parameter on the subject's own rpc.
func hostParam(scope rules.Scope, name string, v rules.View) (symbol.Symbol, error) {
	sym, held := v.Lookup(scope.Subject)
	if !held {
		return nil, fmt.Errorf("protobuf: the view does not contain %s", scope.Subject)
	}
	m, is := sym.(*node.Method)
	if !is {
		return nil, fmt.Errorf("protobuf: %s is no rpc, so it has no parameter named %s",
			scope.Subject, name)
	}
	for _, p := range m.Params {
		if p != nil && p.Name == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("protobuf: %s declares no parameter named %s", scope.Subject, name)
}

// ownerOf splits a dotted chain into the chain above its last
// segment and that segment.
func ownerOf(chain string) (owner, name string) {
	at := strings.LastIndex(chain, protobuf.NameSep)
	if at < 0 {
		return "", chain
	}
	return chain[:at], chain[at+1:]
}
