// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"strings"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
)

// Mirror returns an emit method mirroring a node method's
// signature, type spellings verbatim, origin set. The receiver is
// named against the host type's name and the parameter names: a
// method declaring Put(s Session) must not bind its receiver to s,
// a duplicate-identifier compile error a formatter cannot catch.
// Import inference deliberately does not happen here: imports are
// collected at render as a side effect of spelling types.
func Mirror(host string, m *node.Method) *emit.Method {
	taken := make(map[string]bool, len(m.Params))
	params := make([]*emit.Param, 0, len(m.Params))
	for _, p := range m.Params {
		taken[p.Name] = true
		params = append(params, &emit.Param{
			Name:     p.Name,
			Type:     mirrorRef(p.Type),
			Variadic: p.Variadic,
		})
	}
	returns := make([]*emit.Return, 0, len(m.Returns))
	for _, r := range m.Returns {
		returns = append(returns, &emit.Return{
			Name: r.Name,
			Type: mirrorRef(r.Type),
		})
	}
	return &emit.Method{
		Origin: m.ID,
		Name:   m.Name,
		Receiver: &emit.Param{
			Name: recvName(host, taken),
			Type: &emit.TypeRef{Spelling: "*" + host},
		},
		Params:  params,
		Returns: returns,
	}
}

// mirrorRef copies a type reference tree, spellings verbatim and
// resolved targets carried.
func mirrorRef(t *node.TypeRef) *emit.TypeRef {
	if t == nil {
		return nil
	}
	out := &emit.TypeRef{
		Spelling: t.Spelling,
		Target:   t.Target,
	}
	for _, arg := range t.Args {
		out.Args = append(out.Args, mirrorRef(arg))
	}
	return out
}

// recvName picks the receiver identifier against the taken
// parameter names.
func recvName(host string, taken map[string]bool) string {
	lower := strings.ToLower(host)
	if lower == "" {
		lower = "recv"
	}
	short := lower[:1]
	longer := lower[:min(2, len(lower))]
	for _, candidate := range []string{short, longer, "recv"} {
		if !taken[candidate] {
			return candidate
		}
	}
	return "recv0"
}
