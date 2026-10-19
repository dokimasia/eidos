// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/emit"
)

// The keywords of the schema statement.
const (
	schemaKeyword       = "schema"
	extendSchemaKeyword = "extend schema"
)

// roots are the header markers of the root operation types in the order of
// the schema statement, with the keyword of each.
var roots = [...]struct {
	marker  graphql.Marker
	bit     markerSet
	keyword graphql.Operation
}{
	{marker: graphql.QueryMarker, bit: queryBit, keyword: graphql.OperationQuery},
	{marker: graphql.MutationMarker, bit: mutationBit, keyword: graphql.OperationMutation},
	{marker: graphql.SubscriptionMarker, bit: subscriptionBit, keyword: graphql.OperationSubscription},
}

// Schema writes the schema statement of the file whose header is h: the
// documentation of h as the description, the directives of h, and one root
// operation type for each root marker in a body. The marker graphql:extend
// writes extend schema, which has no description. A header without a root
// marker, a directive, documentation and the marker writes nothing, because
// a schema whose root types have the default names needs no statement. A
// statement ends with a blank line, which separates it from the
// declarations of the file.
func (v *Vocabulary) Schema(h *emit.File) string {
	set := markersOf(h.Annotations)
	rooted := set&(queryBit|mutationBit|subscriptionBit) != 0
	if !rooted && set&extendBit == 0 && len(h.Doc) == 0 && !directives(h.Annotations) {
		return ""
	}
	b := &v.b
	b.Reset()
	if set&extendBit != 0 {
		b.WriteString(extendSchemaKeyword)
	} else {
		writeDescription(b, h.Doc, "")
		b.WriteString(schemaKeyword)
	}
	writeDirectives(b, h.Annotations)
	if rooted {
		b.WriteString(bodyOpen)
		b.WriteString(newline)
		for _, r := range roots {
			if set&r.bit == 0 {
				continue
			}
			b.WriteString(memberIndent)
			b.WriteString(string(r.keyword))
			b.WriteString(graphql.NameSep)
			b.WriteString(markerArg(h.Annotations, r.marker))
			b.WriteString(newline)
		}
		b.WriteString(bodyClose)
	}
	b.WriteString(newline)
	b.WriteString(newline)
	return b.String()
}
