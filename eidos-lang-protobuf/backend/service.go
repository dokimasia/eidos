// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"slices"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The syntax of a service and of an rpc.
const (
	serviceKeyword = "service "
	rpcKeyword     = "rpc "
	rpcReturns     = ") returns ("
	paramsOpen     = "("
	paramsClose    = ")"
)

// service returns the block of the service i in the context p: its options
// and one rpc for each method. An rpc takes its one request and returns its
// one response, each under the mark stream where it streams. The options of
// an rpc are the statements of a block of the rpc, whose opening brace the
// trailing comment of the rpc follows. An rpc without an option ends with a
// semicolon, which its trailing comment follows.
//
// Error modes: an rpc without exactly one parameter and one result, and the
// refusals of the writer.
func (v *Vocabulary) service(p parent, i *emit.Interface) (*Block, error) {
	if _, err := p.scope.Within(i.Annotations); err != nil {
		return nil, err
	}
	b := &Block{Doc: i.Doc, Head: serviceKeyword + i.Name, Items: make([]Item, 0, 1+i.Methods.Len())}
	if text := optionStatements(i.Annotations, nil, memberIndent); text != "" {
		b.Items = append(b.Items, Item{Text: text})
	}
	for _, m := range i.Methods.Items() {
		text, err := v.rpc(m)
		if err != nil {
			return nil, err
		}
		b.Items = append(b.Items, Item{Text: text})
	}
	b.Open = opening(i.Comment, len(b.Items) > 0)
	return b, nil
}

// bodied reports whether the annotations of an rpc state the body marker,
// which writes an rpc without options with an empty body.
func bodied(as symbol.Annotations) bool {
	return slices.ContainsFunc(as, func(a symbol.Annotation) bool { return a.Name == string(protobuf.BodyMarker) })
}

// rpc writes the method m as an rpc of its service, by the rules of
// [Vocabulary.service].
func (v *Vocabulary) rpc(m *emit.Method) (string, error) {
	params, returns := m.Params.Items(), m.Returns.Items()
	if len(params) != 1 || len(returns) != 1 {
		return "", fmt.Errorf(refusalPrefix+"an rpc takes one request and returns one response, "+
			"and %s takes %d and returns %d", m.Name, len(params), len(returns))
	}
	request, err := v.writer.Spell(params[0].Type)
	if err != nil {
		return "", err
	}
	response, err := v.writer.Spell(returns[0].Type)
	if err != nil {
		return "", err
	}
	head := comments(m.Doc, memberIndent) + memberIndent + rpcKeyword + m.Name + paramsOpen + request + rpcReturns +
		response + paramsClose
	options := optionStatements(m.Annotations, nil, nestedIndent)
	if options == "" {
		if bodied(m.Annotations) {
			return head + space + blockOpen + blockClose + trailing(m.Comment) + "\n", nil
		}
		return head + memberEnd + trailing(m.Comment) + "\n", nil
	}
	return head + space + opening(m.Comment, true) + options + memberIndent + blockClose + "\n", nil
}
