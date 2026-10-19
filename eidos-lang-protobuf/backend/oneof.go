// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strconv"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// oneofKeyword opens the head of a oneof.
const oneofKeyword = "oneof "

// oneof returns the block of the oneof o in the context p: its options and
// one member for each variant, under the name of the variant. A member that
// proto2 writes as a group renders as the block of the message of its group,
// which the block of the message of the oneof records.
//
// Error modes: a variant without exactly one field, and the refusals of
// each member.
func (v *Vocabulary) oneof(p parent, o *emit.Sum) (*Block, error) {
	inner, err := p.scope.Within(o.Annotations)
	if err != nil {
		return nil, err
	}
	b := &Block{Doc: o.Doc, Head: oneofKeyword + o.Name, Items: make([]Item, 0, 1+o.Variants.Len())}
	if text := optionStatements(o.Annotations, nil, memberIndent); text != "" {
		b.Items = append(b.Items, Item{Text: text})
	}
	for _, variant := range o.Variants.Items() {
		fields := variant.Fields.Items()
		if len(fields) != 1 {
			return nil, fmt.Errorf(refusalPrefix+"a member of a oneof is one field, and %s has %d",
				variant.Name, len(fields))
		}
		if g, grouped := v.groups[fields[0]]; grouped {
			b.Items = append(b.Items, Item{Nested: g, Indent: memberIndent})
			continue
		}
		text, err := v.member(inner, variant, fields[0])
		if err != nil {
			return nil, err
		}
		b.Items = append(b.Items, Item{Text: text})
	}
	b.Open = opening(o.Comment, len(b.Items) > 0)
	return b, nil
}

// member writes the field f in the scope s as a member of a oneof: its
// documentation, its type, the name of its variant, its number, its compact
// options and its trailing comment. variant is the variant of f. A member has
// presence through its oneof, so it takes no label and no presence, and the
// member of an optional writes the type of the optional. The documentation,
// the comment and the options of the member are those of the field, or
// those of the variant for a field that states none.
//
// Error modes: a member without a type, of a list or a map, required, and
// the refusals of its encoding and its number.
func (v *Vocabulary) member(s Scope, variant *emit.SumVariant, f *emit.Field) (string, error) {
	t := f.Type
	if t != nil && t.Form == symbol.FormOptional && len(t.Elems) == 1 {
		t = t.Elems[0]
	}
	switch {
	case t == nil:
		return "", errNoType
	case t.Form == symbol.FormList || t.Form == symbol.FormMap:
		return "", fmt.Errorf(refusalPrefix+"a member of a oneof is no list and no map, and %s states %s",
			variant.Name, t.Spelling)
	case marked(f.Annotations, protobuf.RequiredMarker):
		return "", fmt.Errorf(refusalPrefix+"a member of a oneof is not required, and %s states the marker %s",
			variant.Name, protobuf.RequiredMarker)
	case f.Value != "" && s.Version().Name == protobuf.Proto3:
		return "", fmt.Errorf(refusalPrefix+"proto3 has no default, and %s states one", variant.Name)
	}
	own, err := s.Within(f.Annotations)
	if err != nil {
		return "", err
	}
	added, err := encoding(s.Version(), own.Features(), variant.Name, f.Annotations, t)
	if err != nil {
		return "", err
	}
	n, err := fieldNumber(variant.Name, f)
	if err != nil {
		return "", err
	}
	spelled, err := v.writer.Spell(t)
	if err != nil {
		return "", err
	}
	doc, comment, as := f.Doc, f.Comment, f.Annotations
	if len(doc) == 0 {
		doc = variant.Doc
	}
	if comment == "" {
		comment = variant.Comment
	}
	if len(as) == 0 {
		as = variant.Annotations
	}
	var digits [numberDigits]byte
	return comments(doc, memberIndent) + memberIndent + spelled + space + variant.Name + numberAssign +
		string(strconv.AppendUint(digits[:0], uint64(n), 10)) + compactOptions(f.Value, as, added, memberIndent) +
		memberEnd + trailing(comment) + "\n", nil
}
