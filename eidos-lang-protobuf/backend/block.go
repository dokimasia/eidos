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

// The keywords of the blocks and of the visibility of a declaration.
const (
	messageKeyword = "message "
	groupKeyword   = "group "
	repeatedGroup  = "repeated "
	localKeyword   = "local "
	exportKeyword  = "export "
)

// reservesAll is the one reserved range of a message in which the strict
// visibility of an edition lets an enum be exported.
const reservesAll = "1 to max"

// Block is the block that the template of a message, an enum, a oneof, a
// service or an extend block writes: its documentation, its head before the
// opening brace, its opening brace with its trailing comment, and its
// members in order.
type Block struct {
	// Doc is the documentation of the block, which the template writes
	// before its head.
	Doc []string
	// Head is the text before the opening brace, such as message Row or
	// optional group Result = 3.
	Head string
	// Open is the opening brace with the trailing comment of the block, and
	// the line break that starts its members where the block has members
	// or a comment. An empty block without a comment closes on the line of
	// its brace.
	Open string
	// Items are the members of the block in their order.
	Items []Item
	// Next are the declarations that the template renders after the
	// block, at the level of the block, through their kind templates. An
	// extend block at file level lists a message there that its cluster
	// claimed and whose version does not write it as a group.
	Next []symbol.Symbol
}

// Item is one member of a block: text that the vocabulary writes, or a
// nested declaration that the template renders through its kind template.
type Item struct {
	// Text is the lines of the member, each with its indentation and its
	// line break. It is empty for a nested declaration.
	Text string
	// Nested is a nested declaration, and nil for text.
	Nested symbol.Symbol
	// Indent is the indentation of each line of Nested.
	Indent string
}

// Block returns the block of the declaration d of the file whose header is h:
// a message, an enum, a oneof or a service. The block of a nested
// declaration takes the scope that the block of its message recorded, and
// the block of any other declaration takes the scope of the file.
//
// The block writes each construct in the version of the file. A message
// writes its options, its reserved and extension ranges, its fields, its
// nested declarations and its extend blocks in that order. A field that
// proto2 writes as a group renders as the block of the message of its group,
// at the position of the field. A nested declaration renders through its
// kind template, and the extensions of one message of a message render as
// one extend block at the position of the first of them.
//
// Error modes, each a refusal of the declaration that the template renders:
// a construct that the version of the file does not have, a feature that
// contradicts a shape or a marker, a header that states no version of
// protobuf, a reference that the writer refuses, and a declaration of
// another kind.
func (v *Vocabulary) Block(h *emit.File, d symbol.Symbol) (*Block, error) {
	p, nested := v.parents[d]
	if !nested {
		clear(v.parents)
		clear(v.groups)
		s, err := FileScope(h)
		if err != nil {
			return nil, err
		}
		p = parent{scope: s}
	}
	switch x := d.(type) {
	case *emit.Struct:
		return v.message(p, x)
	case *emit.Enum:
		return enum(p, x)
	case *emit.Sum:
		return v.oneof(p, x)
	case *emit.Interface:
		return v.service(p, x)
	default:
		return nil, fmt.Errorf(refusalPrefix+"protobuf writes no block of a declaration of the kind %s", d.Kind())
	}
}

// message returns the block of the message s in the context p.
func (v *Vocabulary) message(p parent, s *emit.Struct) (*Block, error) {
	inner, err := p.scope.Within(s.Annotations)
	if err != nil {
		return nil, err
	}
	version := inner.Version()
	b := &Block{Doc: s.Doc, Items: make([]Item, 0, 1+s.Fields.Len()+s.Types.Len())}
	comment := s.Comment
	if p.group != nil {
		var head string
		if head, err = groupHead(p, s.Name); err != nil {
			return nil, err
		}
		b.Doc, b.Head, comment = p.group.Doc, head, p.group.Comment
	} else {
		var keyword string
		if keyword, err = visibility(p, symbol.KindStruct, s.Name, s.Annotations); err != nil {
			return nil, err
		}
		b.Head = keyword + messageKeyword + s.Name
	}
	if version.Name == protobuf.Proto2 {
		err = v.markGroups(inner, s)
		if err != nil {
			return nil, err
		}
	}
	reserved, err := reservedStatements(version, s.Name, s.Annotations, memberIndent)
	if err != nil {
		return nil, err
	}
	extensions, err := extensionStatements(version, s.Name, s.Annotations, memberIndent)
	if err != nil {
		return nil, err
	}
	if text := optionStatements(s.Annotations, nil, memberIndent) + reserved + extensions; text != "" {
		b.Items = append(b.Items, Item{Text: text})
	}
	types := s.Types.Items()
	// names lists the names of each member item after the options, and
	// stays nil for a message whose nested declarations have no follows
	// marker, which keeps the order of the fields and then the types.
	var names []memberNames
	if slices.ContainsFunc(types, func(t symbol.Symbol) bool { _, marked := followsOf(t); return marked }) {
		names = make([]memberNames, 0, s.Fields.Len()+s.Types.Len())
	}
	start := len(b.Items)
	for _, f := range s.Fields.Items() {
		if g, grouped := v.groups[f]; grouped {
			b.Items = append(b.Items, Item{Nested: g, Indent: memberIndent})
			if names != nil {
				names = append(names, memberNames{f.Name, groupName(g)})
			}
			continue
		}
		text, err := v.field(inner, f, memberIndent)
		if err != nil {
			return nil, err
		}
		b.Items = append(b.Items, Item{Text: text})
		if names != nil {
			names = append(names, memberNames{f.Name, ""})
		}
	}
	for i, t := range types {
		var (
			items []Item
			name  string
		)
		if f, extension := t.(*emit.Field); extension && f.Receives != nil {
			var err error
			if items, err = v.extendBlock(inner, types, i, f); err != nil {
				return nil, err
			}
			name = f.Name
		} else {
			if prior, grouped := v.parents[t]; grouped && prior.group != nil {
				continue
			}
			v.parents[t] = parent{scope: inner, host: s}
			items, name = []Item{{Nested: t, Indent: memberIndent}}, nestedName(t)
		}
		if names == nil {
			b.Items = append(b.Items, items...)
			continue
		}
		at := placeAt(names, t)
		b.Items = slices.Insert(b.Items, start+at, items...)
		for range items {
			names = slices.Insert(names, at, memberNames{})
		}
		names[at] = memberNames{name, ""}
	}
	b.Open = opening(comment, len(b.Items) > 0)
	return b, nil
}

// memberNames are the names that a follows marker refers to a member item
// by: the name of a field or a nested declaration, and for a group also
// the name of its message. An item that continues a member has none.
type memberNames [2]string

// placeAt returns the index among the member items at which the nested
// declaration t goes: after the item that its follows marker names, first
// for the empty name, and last for a declaration without the marker or
// with a name that no item before it has.
func placeAt(names []memberNames, t symbol.Symbol) int {
	after, marked := followsOf(t)
	switch {
	case !marked:
		return len(names)
	case after == "":
		return 0
	}
	for i, n := range names {
		if n[0] == after || n[1] == after {
			end := i + 1
			for end < len(names) && names[end] == (memberNames{}) {
				end++
			}
			return end
		}
	}
	return len(names)
}

// followsOf returns the argument of the follows marker of a nested
// declaration, and false for a declaration without one.
func followsOf(t symbol.Symbol) (string, bool) {
	var as symbol.Annotations
	switch n := t.(type) {
	case *emit.Struct:
		as = n.Annotations
	case *emit.Enum:
		as = n.Annotations
	case *emit.Sum:
		as = n.Annotations
	case *emit.Field:
		as = n.Annotations
	}
	for _, a := range as {
		if a.Name == string(protobuf.FollowsMarker) && len(a.Args) == 1 {
			return a.Args[0], true
		}
	}
	return "", false
}

// nestedName returns the name of a nested declaration of a message.
func nestedName(t symbol.Symbol) string {
	switch n := t.(type) {
	case *emit.Struct:
		return n.Name
	case *emit.Enum:
		return n.Name
	case *emit.Sum:
		return n.Name
	default:
		return ""
	}
}

// groupName returns the name of the message of a group, and the empty
// name for a nested declaration that is no message.
func groupName(g symbol.Symbol) string {
	if m, message := g.(*emit.Struct); message {
		return m.Name
	}
	return ""
}

// opening returns the opening brace of a block whose trailing comment is
// comment. A block with members or a comment ends the line of its brace, and
// a block without either closes on that line, because buf format writes an
// empty block as {}. opening allocates the brace with a comment, and
// nothing otherwise.
func opening(comment string, members bool) string {
	switch {
	case comment != "":
		return blockOpen + trailing(comment) + "\n"
	case members:
		return blockOpenLine
	default:
		return blockOpen
	}
}

// markGroups records the message of each field of the message s that proto2
// writes as a group: a delimited field, a delimited member of a oneof and a
// delimited extension, whose type is a nested message of s. The scope of the
// members of s is inner.
func (v *Vocabulary) markGroups(inner Scope, s *emit.Struct) error {
	types := s.Types.Items()
	for _, f := range s.Fields.Items() {
		v.markGroup(parent{scope: inner, host: s, group: f}, f.Name, types)
	}
	for _, t := range types {
		switch n := t.(type) {
		case *emit.Field:
			v.markGroup(parent{scope: inner, host: s, group: n}, n.Name, types)
		case *emit.Sum:
			members, err := inner.Within(n.Annotations)
			if err != nil {
				return err
			}
			for _, variant := range n.Variants.Items() {
				for _, f := range variant.Fields.Items() {
					v.markGroup(parent{scope: members, host: s, group: f, member: true}, variant.Name, types)
				}
			}
		}
	}
	return nil
}

// markGroup records the message of the group of the field p.group, whose
// member takes the name name, among types, as [groupOf] finds it. markGroup
// records nothing for a field without such a message, and the writer of the
// field refuses a delimited one.
func (v *Vocabulary) markGroup(p parent, name string, types []symbol.Symbol) {
	if g := groupOf(p.group, name, types); g != nil {
		v.parents[g] = p
		v.groups[p.group] = g
	}
}

// groupHead returns the head of the group of the field p.group, whose
// message is name: the label of the field, the keyword group, the name, the
// number and the compact options of the field. A member of a oneof takes no
// label.
func groupHead(p parent, name string) (string, error) {
	f := p.group
	n, err := fieldNumber(f.Name, f)
	if err != nil {
		return "", err
	}
	var label string
	switch {
	case p.member:
	case f.Type.Form == symbol.FormList:
		label = repeatedGroup
	case f.Type.Form == symbol.FormOptional:
		label = optionalKeyword
	case marked(f.Annotations, protobuf.RequiredMarker):
		label = requiredKeyword
	default:
		return "", fmt.Errorf(refusalPrefix+"proto2 writes a singular field with a label, "+
			"and %s is neither optional nor required", f.Name)
	}
	var digits [numberDigits]byte
	return label + groupKeyword + name + numberAssign + string(strconv.AppendUint(digits[:0], uint64(n), 10)) +
		compactOptions(f.Value, f.Annotations, nil, ""), nil
}

// visibility returns the keyword of the visibility of the message or the
// enum name of the kind kind, with the annotations as, in the context p.
// The editions 2024 and 2026 write the keyword local for a declaration with
// the marker protobuf:local where the default visibility of its scope
// exports it, and the keyword export for a declaration without the marker
// where the default makes it local. A declaration whose visibility is the
// default takes no keyword.
//
// Error modes: the marker protobuf:local in a version before edition 2024,
// and an exported nested declaration under the visibility STRICT, which
// exports a nested enum of a message whose one reserved range is 1 to max
// alone.
func visibility(p parent, kind symbol.Kind, name string, as symbol.Annotations) (string, error) {
	local := marked(as, protobuf.LocalMarker)
	version := p.scope.Version()
	if version.Name != protobuf.Edition2024 && version.Name != protobuf.Edition2026 {
		if local {
			return "", fmt.Errorf(refusalPrefix+"%s marks no declaration local, and %s states the marker %s",
				version.Name, name, protobuf.LocalMarker)
		}
		return "", nil
	}
	features := p.scope.Features()
	nested := p.scope.Nested()
	switch byDefault := features.LocalByDefault(nested); {
	case local == byDefault:
		return "", nil
	case local:
		return localKeyword, nil
	case nested && features.Visibility == protobuf.VisibilityStrict && kind != symbol.KindEnum:
		return "", fmt.Errorf(refusalPrefix+"the visibility STRICT exports no nested message, "+
			"and %s is not marked local", name)
	case nested && features.Visibility == protobuf.VisibilityStrict && !reservesEvery(p.host):
		return "", fmt.Errorf(refusalPrefix+"the visibility STRICT exports the nested enum of a message "+
			"that reserves %s alone, and %s is not marked local", reservesAll, name)
	default:
		return exportKeyword, nil
	}
}

// reservesEvery reports whether the message host reserves every number of
// its fields in one reserved range, 1 to max. It allocates nothing.
func reservesEvery(host *emit.Struct) bool {
	n, first := 0, ""
	for _, a := range host.Annotations {
		if a.Name != string(protobuf.ReservedMarker) {
			continue
		}
		for _, arg := range a.Args {
			if n == 0 {
				first = arg
			}
			n++
		}
	}
	return n == 1 && first == reservesAll
}
