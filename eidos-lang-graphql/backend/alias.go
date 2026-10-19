// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import "go.dokimi.dev/eidos/sdk/emit"

// The keywords and the marks of a scalar and of a union.
const (
	scalarKeyword = "scalar "
	unionKeyword  = "union "
	membersAssign = " = "
	memberSep     = " | "
)

// Alias writes the alias a after its description: a scalar where a has no
// target, and a union of the members of its target otherwise, as the
// lowering leaves an alias. The marker graphql:extend writes extend before
// the keyword. A union writes its directives before the equals sign of its
// members.
func (v *Vocabulary) Alias(a *emit.Alias) string {
	b := &v.b
	b.Reset()
	writeDescription(b, a.Doc, "")
	if markersOf(a.Annotations)&extendBit != 0 {
		b.WriteString(extendKeyword)
	}
	if a.Target == nil {
		b.WriteString(scalarKeyword)
		b.WriteString(a.Name)
		writeDirectives(b, a.Annotations)
	} else {
		b.WriteString(unionKeyword)
		b.WriteString(a.Name)
		writeDirectives(b, a.Annotations)
		for k, member := range a.Target.Elems {
			if k == 0 {
				b.WriteString(membersAssign)
			} else {
				b.WriteString(memberSep)
			}
			b.WriteString(member.Spelling)
		}
	}
	writeComment(b, a.Comment)
	b.WriteString(newline)
	return b.String()
}
