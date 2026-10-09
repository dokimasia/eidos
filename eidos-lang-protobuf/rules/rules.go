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
// The zero value is ready and is the only value. Every method reads
// through the [rules.View] it is handed and caches nothing.
//
// It satisfies [rules.EnumRules] and no other optional capability:
// protobuf states no generics, no promotion, no struct tags and no
// user-defined equality.
//
// # Concurrency
//
// Rules has no state, so one value serves every goroutine and every
// composition.
//
// # Allocation contract
//
// Each method states what it allocates. A parameter's role, the member
// policy and a scalar's shape allocate nothing.
type Rules struct{}

// New returns the protobuf rules as the [rules.SourceRules] a
// composition registers. The value is [Rules], so a caller that needs
// the enum capability asserts [rules.EnumRules] on it. It allocates
// nothing.
func New() rules.SourceRules { return Rules{} }

// Lang returns [protobuf.Lang], the language a composition keys
// these rules under and the language of every subject they project.
// It allocates nothing.
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
// The walk never applies the shadowing rule. The policy states it as
// override so the policy is total and not zero-valued. Members
// allocates nothing.
func (Rules) Members() rules.MemberPolicy {
	return rules.MemberPolicy{Shadowing: rules.ShadowOverride}
}

// ParamRole reports [rules.ParamInput] for every parameter.
//
// An rpc takes one request message and a schema states no context
// parameter, so no parameter has another role. The view is unread,
// and ParamRole allocates nothing.
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
// # Allocation contract
//
// ReturnRoles allocates the list of roles, one entry per return, which
// the caller keeps. A callable without returns allocates nothing.
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
//
// # Allocation contract
//
// TypeName allocates the PascalCase of each part that is not already
// PascalCase, and the joined name: two allocations for a lower-case
// word and a Pascal base, and three for a snake-cased base.
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
//     From an rpc, a value field resolves among the fields of its
//     response message and then of its request message, and a member
//     on a handle among the fields of its response message.
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
// does a spelling nothing declares. Validation reports such an error
// under its own code.
//
// # Allocation contract
//
// A field and a parameter resolve without allocating. A type and a
// callable resolve through candidates that allocate as
// [protobuf.Candidates] states, and the subject's message chain
// allocates its joined name where the message is nested. Every error
// allocates itself.
func (Rules) Resolve(
	scope rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("protobuf: nothing to resolve")
	}
	switch kind {
	case directive.ResolveValueField:
		return member(scope, name, v, throughValue)
	case directive.ResolveMemberOnHandle:
		return member(scope, name, v, throughHandle)
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
	msg := enclosingMessage(subject, v)
	switch {
	case msg == nil:
		return subject.Package, ""
	case msg.ID.Owner == "":
		return msg.ID.Package, msg.ID.Name
	default:
		return msg.ID.Package, msg.ID.Owner + protobuf.NameSep + msg.ID.Name
	}
}

// enclosingMessage returns the innermost message that contains a
// subject, the subject itself where it is a message, and nil where the
// view does not contain the subject or no message contains it.
//
// A member finds its message through its hosts: a field through its
// message or its oneof member, a oneof member through its oneof.
// A oneof, an enum and a service name their message by their owner
// chain, which only messages nest.
func enclosingMessage(subject symbol.Identity, v rules.View) *node.Struct {
	id := subject
	for range hostDepth {
		sym, _ := v.Lookup(id)
		switch d := sym.(type) {
		case *node.Struct:
			return d
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
				return nil
			}
			owner, name := ownerOf(id.Owner)
			id = symbol.Identity{
				Lang: id.Lang, Package: id.Package, Owner: owner, Name: name, Kind: symbol.KindStruct,
			}
		default:
			return nil
		}
	}
	return nil
}

// through names the messages whose fields a reference from an rpc
// resolves among.
type through uint8

const (
	// throughValue is the rpc's value: its response message, then its
	// request message.
	throughValue through = 1
	// throughHandle is the handle the rpc returns: its response message.
	throughHandle through = 2
)

// member resolves a name among the fields of the message of a subject, the
// members of its oneofs included. For an rpc, [rpcField] resolves the name
// among the messages of its value or its handle, which via selects. For
// any other subject, the message is the message that the subject belongs
// to.
func member(scope rules.Scope, name string, v rules.View, via through) (symbol.Symbol, error) {
	sym, _ := v.Lookup(scope.Subject)
	if rpc, isRPC := sym.(*node.Method); isRPC {
		return rpcField(rpc, name, v, via)
	}
	msg := enclosingMessage(scope.Subject, v)
	if msg == nil {
		if _, known := v.Lookup(scope.Subject); !known {
			return nil, fmt.Errorf("protobuf: the view does not contain %s", scope.Subject)
		}
		return nil, fmt.Errorf("protobuf: %s belongs to no message", scope.Subject)
	}
	if f := fieldNamed(msg, name); f != nil {
		return f, nil
	}
	return nil, fmt.Errorf("protobuf: no field of %s is named %s", msg.ID, name)
}

// rpcField resolves a name among the fields of an rpc's messages: its
// response, then, through the value, its request. A stream counts as
// the message of its elements, and a message that the view does not
// contain is left out.
func rpcField(rpc *node.Method, name string, v rules.View, via through) (symbol.Symbol, error) {
	refs := make([]*node.TypeRef, 0, 2)
	for _, r := range rpc.Returns {
		refs = append(refs, r.Type)
	}
	if via == throughValue {
		for _, p := range rpc.Params {
			refs = append(refs, p.Type)
		}
	}
	for _, ref := range refs {
		for ref != nil && ref.Form == symbol.FormStream && len(ref.Elems) == 1 {
			ref = ref.Elems[0]
		}
		var target symbol.Identity
		if ref != nil {
			target = ref.Target
		}
		sym, _ := v.Lookup(target)
		if msg, isMessage := sym.(*node.Struct); isMessage {
			if f := fieldNamed(msg, name); f != nil {
				return f, nil
			}
		}
	}
	return nil, fmt.Errorf("protobuf: no field of the messages of %s is named %s", rpc.ID, name)
}

// fieldNamed returns a message's field of one name, a oneof
// member's field included, and nil where the message declares none.
func fieldNamed(msg *node.Struct, name string) *node.Field {
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
	owner, name, found := strings.CutLast(chain, protobuf.NameSep)
	if !found {
		return "", chain
	}
	return owner, name
}
