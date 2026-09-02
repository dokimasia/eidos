// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package lowering

import (
	"fmt"

	"go.dokimi.dev/eidos/sdk/emit"
)

// CopyTypeParams restates a parameter list without sharing nodes,
// so the settle's walks visit each output's list once: fresh
// parameters, fresh bound and default references.
func CopyTypeParams(ps []*emit.TypeParam) []*emit.TypeParam {
	if len(ps) == 0 {
		return nil
	}
	out := make([]*emit.TypeParam, 0, len(ps))
	for _, p := range ps {
		c := *p
		c.Bounds = CopyTypeRefs(p.Bounds)
		c.Default = CopyTypeRef(p.Default)
		c.Type = CopyTypeRef(p.Type)
		out = append(out, &c)
	}
	return out
}

// CopyTypeRefs restates references without sharing nodes.
func CopyTypeRefs(ts []*emit.TypeRef) []*emit.TypeRef {
	if len(ts) == 0 {
		return nil
	}
	out := make([]*emit.TypeRef, 0, len(ts))
	for _, t := range ts {
		out = append(out, CopyTypeRef(t))
	}
	return out
}

// CopyTypeRef restates one reference tree without sharing nodes.
func CopyTypeRef(t *emit.TypeRef) *emit.TypeRef {
	if t == nil {
		return nil
	}
	c := *t
	c.Args = CopyTypeRefs(t.Args)
	return &c
}

// UniqueMethods reports the first method name declared twice on a
// host, spelled under the language's own error prefix: what a
// backend without overloads checks before its fold spells members.
func UniqueMethods(lang, host string, methods []*emit.Method) error {
	seen := make(map[string]bool, len(methods))
	for _, m := range methods {
		if seen[m.Name] {
			return fmt.Errorf(
				"%s: method %s declared twice on %s, and %s overloads nothing",
				lang, m.Name, host, lang,
			)
		}
		seen[m.Name] = true
	}
	return nil
}
