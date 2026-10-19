// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// enumKeyword opens the head of an enum.
const enumKeyword = "enum "

// enum returns the block of the enum e in the context p: its options, its
// reserved ranges and its values. Every enum of proto2 is closed, every
// enum of proto3 is open, and an edition writes the feature enum_type where
// the resolved openness differs from the marker protobuf:closed. An open
// enum starts with the value 0.
//
// Error modes: an enum of proto2 without the marker protobuf:closed, the
// marker in proto3, a stated enum_type that contradicts the marker, an open
// enum whose first value is not 0, and the refusals of its visibility and
// its ranges.
func enum(p parent, e *emit.Enum) (*Block, error) {
	inner, err := p.scope.Within(e.Annotations)
	if err != nil {
		return nil, err
	}
	keyword, err := visibility(p, symbol.KindEnum, e.Name, e.Annotations)
	if err != nil {
		return nil, err
	}
	version := inner.Version()
	closed := marked(e.Annotations, protobuf.ClosedMarker)
	var added []feature
	switch {
	case version.Name == protobuf.Proto2 && !closed:
		return nil, fmt.Errorf(refusalPrefix+"every enum of proto2 is closed, and %s does not state the marker %s",
			e.Name, protobuf.ClosedMarker)
	case version.Name == protobuf.Proto3 && closed:
		return nil, fmt.Errorf(refusalPrefix+"every enum of proto3 is open, and %s states the marker %s",
			e.Name, protobuf.ClosedMarker)
	case !version.Syntax:
		want := protobuf.EnumOpen
		if closed {
			want = protobuf.EnumClosed
		}
		if inner.Features().EnumType == want {
			break
		}
		if stated := statedFeatures(e.Annotations).EnumType; stated != "" {
			return nil, fmt.Errorf(
				refusalPrefix+"%s states the enum type %s, and its marker needs %s",
				e.Name,
				stated,
				want,
			)
		}
		added = append(added, feature{name: protobuf.FeatureEnumType, value: string(want)})
	}
	variants := e.Variants.Items()
	if !closed && len(variants) > 0 {
		var first int32
		if first, err = enumValue(variants[0]); err != nil {
			return nil, err
		}
		if first != 0 {
			return nil, fmt.Errorf(refusalPrefix+"an open enum starts with the value 0, and %s starts with %s",
				e.Name, variants[0].Value)
		}
	}
	reserved, err := reservedStatements(version, e.Name, e.Annotations, memberIndent)
	if err != nil {
		return nil, err
	}
	b := &Block{Doc: e.Doc, Head: keyword + enumKeyword + e.Name, Items: make([]Item, 0, 1+len(variants))}
	if text := optionStatements(e.Annotations, added, memberIndent) + reserved; text != "" {
		b.Items = append(b.Items, Item{Text: text})
	}
	// Each value writes its documentation, its name, its number as written,
	// its options and its trailing comment.
	for _, value := range variants {
		b.Items = append(b.Items, Item{Text: comments(value.Doc, memberIndent) + memberIndent + value.Name +
			numberAssign + value.Value + compactOptions("", value.Annotations, nil, memberIndent) + memberEnd +
			trailing(value.Comment) + "\n"})
	}
	b.Open = opening(e.Comment, len(b.Items) > 0)
	return b, nil
}
