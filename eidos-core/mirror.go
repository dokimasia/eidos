// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
)

// Mirror returns an emit method mirroring a node method's
// signature, type spellings verbatim, origin set. The signature is
// the name, the visibility, the level, the type parameters, the
// parameters with their labels, defaults, optional and variadic
// forms, the results, the async flag and the announced failure
// types. The receiver is a pointer to host, and Receives names
// host, so the settle scopes the method under its host as it
// scopes a parsed method. The receiver's name differs from every
// parameter, result and type parameter name: a method declaring
// Put(s Session) must not bind its receiver to s, a
// duplicate-identifier compile error a formatter cannot catch.
// Import inference deliberately does not happen here: imports are
// collected at render as a side effect of spelling types.
func Mirror(host string, m *node.Method) *emit.Method {
	taken := make(map[string]bool, len(m.TypeParams)+len(m.Params)+len(m.Returns))
	var typeParams []*emit.TypeParam
	for _, tp := range m.TypeParams {
		taken[tp.Name] = true
		typeParams = append(typeParams, mirrorTypeParam(tp))
	}
	params := make([]*emit.Param, 0, len(m.Params))
	for _, p := range m.Params {
		taken[p.Name] = true
		params = append(params, &emit.Param{
			Name:     p.Name,
			Label:    p.Label,
			Type:     rules.EmitRef(p.Type),
			Default:  p.Default,
			Optional: p.Optional,
			Variadic: p.Variadic,
		})
	}
	returns := make([]*emit.Return, 0, len(m.Returns))
	for _, r := range m.Returns {
		taken[r.Name] = true
		returns = append(returns, &emit.Return{
			Name: r.Name,
			Type: rules.EmitRef(r.Type),
		})
	}
	var throws []*emit.TypeRef
	for _, t := range m.Throws {
		throws = append(throws, rules.EmitRef(t))
	}
	return &emit.Method{
		Origin:     m.ID,
		Name:       m.Name,
		Visibility: m.Visibility,
		Level:      m.Level,
		Async:      m.Async,
		Receiver: &emit.Param{
			Name: recvName(host, taken),
			Type: &emit.TypeRef{Spelling: "*" + host},
		},
		Receives:   &emit.TypeRef{Spelling: host},
		TypeParams: typeParams,
		Params:     params,
		Returns:    returns,
		Throws:     throws,
	}
}

// mirrorTypeParam restates a node type parameter in the emit model,
// bounds, default and value type included.
func mirrorTypeParam(tp *node.TypeParam) *emit.TypeParam {
	out := &emit.TypeParam{
		Name:         tp.Name,
		Variance:     tp.Variance,
		Default:      rules.EmitRef(tp.Default),
		Const:        tp.Const,
		Type:         rules.EmitRef(tp.Type),
		DefaultValue: tp.DefaultValue,
	}
	for _, b := range tp.Bounds {
		out.Bounds = append(out.Bounds, rules.EmitRef(b))
	}
	return out
}

// recvName picks the receiver identifier that no taken name uses:
// the host's first letter, its first two letters, recv, then recv0,
// recv1 and upward. Letters are runes, so a host spelled outside
// ASCII yields a whole character.
func recvName(host string, taken map[string]bool) string {
	lower := []rune(strings.ToLower(host))
	if len(lower) == 0 {
		lower = []rune("recv")
	}
	short := string(lower[:1])
	longer := string(lower[:min(2, len(lower))])
	for _, candidate := range []string{short, longer, "recv"} {
		if !taken[candidate] {
			return candidate
		}
	}
	for i := 0; ; i++ {
		if candidate := "recv" + strconv.Itoa(i); !taken[candidate] {
			return candidate
		}
	}
}
