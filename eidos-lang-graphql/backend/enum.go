// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import "go.dokimi.dev/eidos/sdk/emit"

// enumKeyword opens the definition of an enum.
const enumKeyword = "enum "

// Enum writes the enum e after its description: its directives, and its
// values in a body, each with its description, its directives and its
// trailing comment. The marker graphql:extend writes extend before the
// keyword, and an extension without a value has no body. GraphQL serializes
// a value as its name, so a value writes no value of its own.
func (v *Vocabulary) Enum(e *emit.Enum) string {
	b := &v.b
	b.Reset()
	writeDescription(b, e.Doc, "")
	if markersOf(e.Annotations)&extendBit != 0 {
		b.WriteString(extendKeyword)
	}
	b.WriteString(enumKeyword)
	b.WriteString(e.Name)
	writeDirectives(b, e.Annotations)
	variants := e.Variants.Items()
	if len(variants) == 0 {
		writeComment(b, e.Comment)
		b.WriteString(newline)
		return b.String()
	}
	b.WriteString(bodyOpen)
	writeComment(b, e.Comment)
	b.WriteString(newline)
	for _, value := range variants {
		writeDescription(b, value.Doc, memberIndent)
		b.WriteString(memberIndent)
		b.WriteString(value.Name)
		writeDirectives(b, value.Annotations)
		writeComment(b, value.Comment)
		b.WriteString(newline)
	}
	b.WriteString(bodyClose)
	b.WriteString(newline)
	return b.String()
}
