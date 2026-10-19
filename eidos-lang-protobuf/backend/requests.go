// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"slices"
	"strconv"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the messages and the fields that the lowering of an rpc
// declares.
const (
	// requestSuffix and responseSuffix close the name of the request and
	// of the response of an rpc, after the names of the service and of
	// the rpc in Pascal case.
	requestSuffix  = "Request"
	responseSuffix = "Response"
	// requestParam is the name of the one parameter of a lowered rpc, as
	// the protobuf frontend names it.
	requestParam = "request"
	// paramField and resultField name the field of a parameter and of a
	// result without a name. In a callable with more than one, the
	// position follows after positionSep.
	paramField  = "param"
	resultField = "result"
	positionSep = "_"
	// nullValue is the one enum of the well-known types.
	nullValue = protobuf.WellKnownPackage + protobuf.NameSep + "NullValue"
)

// lowerService lowers the service i: each of its rpcs takes one request
// and returns one response. An rpc whose one parameter is a message, or a
// stream of messages, keeps the parameter, and [request] lowers the
// parameters of any other rpc. The results follow the same rule through
// [response]. lowerService returns the service and the messages that the
// lowering declares beside it, which take the origin of the service, and
// nil where every rpc takes and returns one message.
func lowerService(i *emit.Interface) ([]symbol.Symbol, error) {
	if err := checkAnnotations(symbol.KindInterface, i.Name, i.Annotations); err != nil {
		return nil, err
	}
	if i.Fields.Len() > 0 {
		return nil, fmt.Errorf(refusalPrefix+"a service declares rpcs alone, and %s declares fields", i.Name)
	}
	methods := i.Methods.Items()
	var out []symbol.Symbol
	for j, m := range methods {
		if err := checkAnnotations(symbol.KindMethod, m.Name, m.Annotations); err != nil {
			return nil, err
		}
		if slices.ContainsFunc(methods[:j], func(prior *emit.Method) bool { return prior.Name == m.Name }) {
			return nil, fmt.Errorf(refusalPrefix+"a service declares each rpc once, and %s declares %s twice",
				i.Name, m.Name)
		}
		params := m.Params.Items()
		for _, p := range params {
			if err := checkAnnotations(symbol.KindParam, p.Name, p.Annotations); err != nil {
				return nil, err
			}
		}
		if len(params) != 1 || !wholeMessage(params[0].Type) {
			req, err := request(i, m)
			if err != nil {
				return nil, err
			}
			out = append(out, req)
		}
		if returns := m.Returns.Items(); len(returns) != 1 || !wholeMessage(returns[0].Type) {
			resp, err := response(i, m)
			if err != nil {
				return nil, err
			}
			out = append(out, resp)
		}
	}
	if out == nil {
		return nil, nil
	}
	return slices.Insert(out, 0, symbol.Symbol(i)), nil
}

// request lowers the parameters of the rpc m of the service i into one
// request, and returns the message that it declares. An rpc whose one
// parameter is a stream of another type than a message streams the message
// <Service><Rpc>Request, whose field value is the element. Any other rpc
// takes the message <Service><Rpc>Request, with one field for each
// parameter in order, numbered from 1:
//
//   - the field has the name of its parameter, or param where the rpc has
//     one parameter without a name, or param_<position> where it has more;
//   - a variadic parameter is a list, and a keyword one a map from string;
//   - an optional parameter is an optional;
//   - the field has the documentation, the comment, the default and the
//     annotations of its parameter.
//
// An rpc without a parameter takes the empty message <Service><Rpc>Request.
//
// Error modes: a stream beside another parameter, because a stream of gRPC
// is the whole request, and a parameter with a label, because a field has
// one name.
func request(i *emit.Interface, m *emit.Method) (*emit.Struct, error) {
	params := m.Params.Items()
	msg := &emit.Struct{Origin: i.Origin, Name: caser.Pascal(i.Name) + caser.Pascal(m.Name) + requestSuffix}
	if len(params) == 1 && params[0].Type != nil && params[0].Type.Form == symbol.FormStream {
		t := params[0].Type
		streamedValue(msg, t)
		params[0].Type = streamOf(msg.Name, t.Async)
		return msg, nil
	}
	for k, p := range params {
		switch {
		case p.Type != nil && p.Type.Form == symbol.FormStream:
			return nil, fmt.Errorf(refusalPrefix+"a stream is the whole request of an rpc, "+
				"and %s streams %s beside another parameter", m.Name, p.Name)
		case p.Label != "":
			return nil, fmt.Errorf(refusalPrefix+"a field of a request has one name, "+
				"and the parameter %s of %s states the label %s", p.Name, m.Name, p.Label)
		}
		t := p.Type
		if t != nil {
			switch p.Variadic {
			case symbol.VariadicPositional:
				t = &emit.TypeRef{
					Spelling: repeatedLabel + t.Spelling,
					Form:     symbol.FormList,
					Elems:    []*emit.TypeRef{t},
				}
			case symbol.VariadicKeyword:
				key := &emit.TypeRef{Spelling: protobuf.TypeString}
				t = &emit.TypeRef{
					Spelling: mapOpen + key.Spelling + mapSep + t.Spelling + mapClose,
					Form:     symbol.FormMap, Elems: []*emit.TypeRef{key, t},
				}
			case symbol.VariadicNone:
			}
			if p.Optional {
				t = &emit.TypeRef{
					Spelling: optionalLabel + t.Spelling,
					Form:     symbol.FormOptional,
					Elems:    []*emit.TypeRef{t},
				}
			}
		}
		msg.Fields.Append(&emit.Field{
			Doc: p.Doc, Comment: p.Comment, Name: positional(p.Name, paramField, k, len(params)),
			Type: t, Value: p.Default, Annotations: p.Annotations,
			Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: strconv.Itoa(k + 1)}),
		})
	}
	if err := lowerMessage(msg); err != nil {
		return nil, err
	}
	m.Params.Replace(&emit.Param{Name: requestParam, Type: &emit.TypeRef{Spelling: msg.Name}})
	return msg, nil
}

// response lowers the results of the rpc m of the service i into one
// response by the rules of [request], under the name
// <Service><Rpc>Response. A result without a name is result, or
// result_<position> in an rpc with more than one result, and its field has
// the comment of the result.
//
// Error modes: a stream beside another result, because a stream of gRPC is
// the whole response.
func response(i *emit.Interface, m *emit.Method) (*emit.Struct, error) {
	returns := m.Returns.Items()
	msg := &emit.Struct{Origin: i.Origin, Name: caser.Pascal(i.Name) + caser.Pascal(m.Name) + responseSuffix}
	if len(returns) == 1 && returns[0].Type != nil && returns[0].Type.Form == symbol.FormStream {
		t := returns[0].Type
		streamedValue(msg, t)
		returns[0].Type = streamOf(msg.Name, t.Async)
		return msg, nil
	}
	for k, r := range returns {
		if r.Type != nil && r.Type.Form == symbol.FormStream {
			return nil, fmt.Errorf(refusalPrefix+"a stream is the whole response of an rpc, "+
				"and %s streams beside another result", m.Name)
		}
		msg.Fields.Append(&emit.Field{
			Comment: r.Comment, Name: positional(r.Name, resultField, k, len(returns)), Type: r.Type,
			Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: strconv.Itoa(k + 1)}),
		})
	}
	if err := lowerMessage(msg); err != nil {
		return nil, err
	}
	m.Returns.Replace(&emit.Return{Type: &emit.TypeRef{Spelling: msg.Name}})
	return msg, nil
}

// streamedValue declares in msg the field value of the element of the
// stream t, in the shape that [shape] gives it. The field of a stream
// without exactly one child states no type, and the writer refuses it.
func streamedValue(msg *emit.Struct, t *emit.TypeRef) {
	var elem *emit.TypeRef
	if len(t.Elems) == 1 {
		elem = shape(valueField, t.Elems[0], func(m *emit.Struct) { msg.Types.Append(m) })
	}
	msg.Fields.Append(&emit.Field{
		Name: valueField, Type: elem,
		Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "1"}),
	})
}

// streamOf returns the stream of the message name, whose elements arrive
// asynchronously where async is set.
func streamOf(name string, async bool) *emit.TypeRef {
	elem := &emit.TypeRef{Spelling: name}
	return &emit.TypeRef{
		Spelling: streamMark + name,
		Form:     symbol.FormStream,
		Async:    async,
		Elems:    []*emit.TypeRef{elem},
	}
}

// positional returns the name of the field of the parameter or the result
// at the index k of n: its own name, and otherwise base, after which the
// position follows where n is more than one.
func positional(name, base string, k, n int) string {
	switch {
	case name != "":
		return name
	case n == 1:
		return base
	default:
		return base + positionSep + strconv.Itoa(k+1)
	}
}

// wholeMessage reports whether t is a whole request or response of an rpc:
// a message, or a stream of messages.
func wholeMessage(t *emit.TypeRef) bool {
	if t != nil && t.Form == symbol.FormStream && len(t.Elems) == 1 {
		return messageType(t.Elems[0])
	}
	return messageType(t)
}

// messageType reports whether t names a message: a named reference
// without type arguments to a declaration that is not a scalar of
// protobuf, an enum, google.protobuf.NullValue or a type parameter. A
// reference that does not resolve names a message, because a protobuf
// schema refers to messages far more often than to enums.
func messageType(t *emit.TypeRef) bool {
	switch {
	case t == nil || t.Form != symbol.FormNamed || t.Spelling == "" || len(t.Args) > 0:
		return false
	case protobuf.IsScalar(t.Spelling):
		return false
	}
	if known, wellKnown := protobuf.WellKnown(t.Spelling); wellKnown {
		return known != nullValue
	}
	return t.Target.Kind != symbol.KindEnum && t.Target.Kind != symbol.KindTypeParam
}
