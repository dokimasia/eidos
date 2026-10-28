// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor

import (
	"slices"
	"strconv"
	"strings"
	"unicode"

	"google.golang.org/protobuf/types/descriptorpb"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings of proto that the lowering writes, as the source frontend
// writes them.
const (
	listSep        = ", "
	streamRequest  = "request"
	streamResponse = "response"
	streamBoth     = "both"
	streamKeyword  = "stream"
	requestParam   = "request"
	mapSpelling    = "map"
	labelRepeated  = "repeated"
	labelOptional  = "optional"
	labelRequired  = "required"
	localValue     = "local"
	rangeTo        = " to "
	rangeMax       = "max"
	scalarPrefix   = "TYPE_"
)

// The largest field number of a message and the largest number of an enum
// value, which a reserved range spells as max.
const (
	maxField     = 536870911
	maxEnumValue = 2147483647
)

// label is the label of a field as a schema writes it.
type label uint8

// The labels of a field. labelNone is a field that a schema writes without
// a label, such as a field of proto3 or of an edition.
const (
	labelNone label = iota
	labelOptionalKeyword
	labelRepeatedKeyword
	labelRequiredKeyword
)

// pending is one declaration of a scope that waits for its lowering, at
// the position of its location. A scope lowers its declarations in source
// order, so the declarations, the attachments and the stamps of the scope
// are in the order in which the source frontend records them.
type pending struct {
	pos   position.Pos
	lower func()
}

// pathOf returns base with nums appended, in a new slice.
func pathOf(base []int32, nums ...int32) []int32 {
	return append(slices.Clip(base), nums...)
}

// lowerInOrder runs the lowerings of a scope in source order. Two
// lowerings at one position, as in a descriptor without source code info,
// keep their order.
func lowerInOrder(steps []pending) {
	slices.SortStableFunc(steps, func(a, b pending) int {
		if a.pos.Line != b.pos.Line {
			return a.pos.Line - b.pos.Line
		}
		return a.pos.Col - b.pos.Col
	})
	for _, s := range steps {
		s.lower()
	}
}

// posAt returns the position of the location of a path.
func (l *lowered) posAt(path []int32) position.Pos { return l.loc.start(l.loc.at(path)) }

// decls lowers the declarations of the file in source order: its messages,
// enums, services and extensions. The message of a group of an extension
// is lowered with the group.
func (l *lowered) decls(scope protobuf.Features) []symbol.Symbol {
	groups := l.groupNames(l.fd.GetExtension())
	var (
		out   []symbol.Symbol
		steps []pending
	)
	for i, m := range l.fd.GetMessageType() {
		if groups[m.GetName()] {
			continue
		}
		path := pathOf(nil, fileMessage, int32(i))
		steps = append(steps, pending{pos: l.posAt(path), lower: func() {
			out = append(out, l.message(m, path, scope, false))
		}})
	}
	for i, e := range l.fd.GetEnumType() {
		path := pathOf(nil, fileEnum, int32(i))
		steps = append(steps, pending{pos: l.posAt(path), lower: func() {
			out = append(out, l.enum(e, path, scope, false))
		}})
	}
	for i, s := range l.fd.GetService() {
		path := pathOf(nil, fileService, int32(i))
		steps = append(steps, pending{pos: l.posAt(path), lower: func() {
			out = append(out, l.service(s, path))
		}})
	}
	ext := l.extensions(l.fd.GetExtension(), []int32{fileExtension}, scope,
		l.fd.GetMessageType(), []int32{fileMessage}, func(syms ...symbol.Symbol) { out = append(out, syms...) })
	lowerInOrder(append(steps, ext...))
	return out
}

// isGroup reports whether a field is a group of proto2, which declares a
// message and a field in one statement. A delimited field of an edition
// has the type of a group too, and refers to a message that the schema
// declares apart.
func (l *lowered) isGroup(f *descriptorpb.FieldDescriptorProto) bool {
	return f.GetType() == descriptorpb.FieldDescriptorProto_TYPE_GROUP && l.version.Name == protobuf.Proto2
}

// groupNames returns the names of the messages that the groups among
// fields declare.
func (l *lowered) groupNames(fields []*descriptorpb.FieldDescriptorProto) map[string]bool {
	out := map[string]bool{}
	for _, f := range fields {
		if l.isGroup(f) {
			out[lastName(f.GetTypeName())] = true
		}
	}
	return out
}

// lastName returns the last segment of a dotted name.
func lastName(name string) string {
	return name[strings.LastIndex(name, protobuf.NameSep)+1:]
}

// message lowers a message into a struct, with its comments, and its
// members through [lowered.members]. outer is the features of the scope
// that declares the message, and nested reports a message that another
// message declares.
func (l *lowered) message(
	m *descriptorpb.DescriptorProto,
	path []int32,
	outer protobuf.Features,
	nested bool,
) *node.Struct {
	loc := l.loc.at(path)
	s := &node.Struct{Pos: l.loc.start(loc), Name: m.GetName(), Visibility: symbol.VisibilityPublic}
	c := l.commentsOf(loc, s, true)
	s.Doc, s.Annotations, s.Comment = c.doc, c.annotations, c.comment
	l.members(m, path, s, outer, nested)
	return s
}

// members lowers the body of a message into a struct, as the source
// frontend does. Its fields lower as fields, its oneofs as nested sums, and
// its groups, nested messages and nested enums as nested types, in source
// order. The fields of its extensions are nested declarations too. A map
// entry and the oneof of an optional field of proto3 are synthetic, so they
// lower as the map field and the optional field. The reserved ranges, the
// extension ranges, the options and the visibility are stamps.
func (l *lowered) members(
	m *descriptorpb.DescriptorProto,
	path []int32,
	s *node.Struct,
	outer protobuf.Features,
	nested bool,
) {
	options := l.optionsAt(m.GetOptions(), pathOf(path, messageOptions))
	scope := with(outer, options)
	groups := l.groupNames(append(slices.Clone(m.GetField()), m.GetExtension()...))
	synthetic := syntheticOneofs(m)
	var steps []pending
	for j, f := range m.GetField() {
		fp := pathOf(path, messageField, int32(j))
		if f.OneofIndex != nil && !synthetic[f.GetOneofIndex()] {
			continue
		}
		steps = append(steps, pending{pos: l.posAt(fp), lower: func() {
			if !l.isGroup(f) {
				s.Fields = append(s.Fields, l.field(f, fp, scope, false, m))
				return
			}
			field, group := l.group(f, fp, scope, false, m.GetNestedType(), pathOf(path, messageNested))
			s.Fields = append(s.Fields, field)
			s.Types = append(s.Types, group)
		}})
	}
	for k, o := range m.GetOneofDecl() {
		if synthetic[int32(k)] {
			continue
		}
		index := int32(k)
		op := pathOf(path, messageOneof, index)
		steps = append(steps, pending{pos: l.posAt(op), lower: func() {
			sum, oneofGroups := l.oneof(o, op, index, m, path, scope)
			s.Types = append(append(s.Types, sum), oneofGroups...)
		}})
	}
	for k, n := range m.GetNestedType() {
		if groups[n.GetName()] || n.GetOptions().GetMapEntry() {
			continue
		}
		np := pathOf(path, messageNested, int32(k))
		steps = append(steps, pending{pos: l.posAt(np), lower: func() {
			s.Types = append(s.Types, l.message(n, np, scope, true))
		}})
	}
	for k, e := range m.GetEnumType() {
		ep := pathOf(path, messageEnum, int32(k))
		steps = append(steps, pending{pos: l.posAt(ep), lower: func() {
			s.Types = append(s.Types, l.enum(e, ep, scope, true))
		}})
	}
	ext := l.extensions(
		m.GetExtension(),
		pathOf(path, messageExtension),
		scope,
		m.GetNestedType(),
		pathOf(path, messageNested),
		func(syms ...symbol.Symbol) { s.Types = append(s.Types, syms...) },
	)
	lowerInOrder(append(steps, ext...))

	gb := l.unit.Graph()
	reserved := make([]string, 0, len(m.GetReservedRange())+len(m.GetReservedName()))
	for _, r := range m.GetReservedRange() {
		reserved = append(reserved, spelledRange(r.GetStart(), r.GetEnd()-1, maxField))
	}
	reserved = append(reserved, m.GetReservedName()...)
	if len(reserved) > 0 {
		gb.Stamp(s, meta.RawStamp{Key: protobuf.ReservedKey, Value: strings.Join(reserved, listSep), Pos: s.Pos})
	}
	if ranges := m.GetExtensionRange(); len(ranges) > 0 {
		extensions := make([]string, 0, len(ranges))
		for k, r := range ranges {
			text := spelledRange(r.GetStart(), r.GetEnd()-1, maxField)
			rangePath := pathOf(path, messageExtensionRange, int32(k), rangeOptions)
			if opts := l.optionsAt(r.GetOptions(), rangePath); len(opts) > 0 {
				entries := make([]string, len(opts))
				for i, o := range opts {
					entries[i] = o.name + " " + optionAssign + " " + o.value
				}
				text += " [" + strings.Join(entries, listSep) + "]"
			}
			extensions = append(extensions, text)
		}
		gb.Stamp(s, meta.RawStamp{Key: protobuf.ExtensionsKey, Value: extensions, Pos: s.Pos})
	}
	l.stampOptions(s, s.Pos, options)
	l.stampLocal(s, m.GetVisibility(), outer, nested)
}

// syntheticOneofs returns the indexes of the oneofs of a message that
// protoc made for an optional field of proto3. Such a oneof has the one
// field, which states proto3_optional.
func syntheticOneofs(m *descriptorpb.DescriptorProto) map[int32]bool {
	out := map[int32]bool{}
	for _, f := range m.GetField() {
		if f.GetProto3Optional() && f.OneofIndex != nil {
			out[f.GetOneofIndex()] = true
		}
	}
	return out
}

// labelOf returns the label of a field as the schema wrote it. proto2
// writes the optional label, and proto3 writes it where the field states
// proto3_optional.
func (l *lowered) labelOf(f *descriptorpb.FieldDescriptorProto) label {
	switch {
	case f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED:
		return labelRepeatedKeyword
	case f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REQUIRED:
		return labelRequiredKeyword
	case f.GetProto3Optional():
		return labelOptionalKeyword
	case l.version.Name == protobuf.Proto2:
		return labelOptionalKeyword
	default:
		return labelNone
	}
}

// field lowers one plain field: its type under its label and its
// presence, and then the parts that every field form shares. A field of a
// map entry of owner lowers through [lowered.mapField]. member reports a
// member of a oneof, which has presence through its oneof.
func (l *lowered) field(
	f *descriptorpb.FieldDescriptorProto, path []int32, outer protobuf.Features, member bool,
	owner *descriptorpb.DescriptorProto,
) *node.Field {
	options := l.optionsAt(f.GetOptions(), pathOf(path, fieldOptions))
	if entry := mapEntry(f, owner); entry != nil {
		return l.mapField(f, path, entry, options)
	}
	scope := with(outer, options)
	inner := l.typeRef(f, path)
	typ, required := fieldType(inner, l.labelOf(f), scope.Presence, member)
	out := l.fieldDecl(f, path, typ, options, false)
	gb := l.unit.Graph()
	if required {
		gb.Stamp(out, meta.RawStamp{Key: protobuf.LabelKey, Value: labelRequired, Pos: out.Pos})
	}
	if scope.Encoding == protobuf.EncodingDelimited && !protobuf.IsScalar(inner.Spelling) {
		gb.Stamp(
			out,
			meta.RawStamp{Key: protobuf.DelimitedKey, Value: string(protobuf.EncodingDelimited), Pos: out.Pos},
		)
	}
	return out
}

// mapEntry returns the map entry of owner that a repeated field refers to,
// and nil for every other field.
func mapEntry(f *descriptorpb.FieldDescriptorProto, owner *descriptorpb.DescriptorProto) *descriptorpb.DescriptorProto {
	if owner == nil || f.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		return nil
	}
	name := lastName(f.GetTypeName())
	for _, n := range owner.GetNestedType() {
		if n.GetName() == name && n.GetOptions().GetMapEntry() {
			return n
		}
	}
	return nil
}

// fieldType lowers the type of a field under its label and its resolved
// presence, and reports whether the field is required, as the source
// frontend does.
func fieldType(inner *node.TypeRef, l label, presence protobuf.Presence, member bool) (*node.TypeRef, bool) {
	switch {
	case l == labelRepeatedKeyword:
		return plugin.Composite(labelRepeated+" "+inner.Spelling, inner.Pos, symbol.FormList, inner), false
	case member:
		return inner, false
	case l == labelOptionalKeyword:
		return plugin.Composite(labelOptional+" "+inner.Spelling, inner.Pos, symbol.FormOptional, inner), false
	case l == labelRequiredKeyword:
		return inner, true
	case presence == protobuf.PresenceExplicit:
		return plugin.Composite(inner.Spelling, inner.Pos, symbol.FormOptional, inner), false
	default:
		return inner, presence == protobuf.PresenceLegacyRequired
	}
}

// typeRef returns the type of a field: the fully qualified name of a
// message or an enum, and the name of a scalar, at the location of the
// type in the source.
func (l *lowered) typeRef(f *descriptorpb.FieldDescriptorProto, path []int32) *node.TypeRef {
	if name := f.GetTypeName(); name != "" {
		at := pathOf(path, fieldTypeName)
		return namedRef(l.spelled(name, at), l.posOf(at, path))
	}
	scalar := strings.ToLower(strings.TrimPrefix(f.GetType().String(), scalarPrefix))
	return &node.TypeRef{Spelling: scalar, Pos: l.posOf(pathOf(path, fieldScalar), path)}
}

// posOf returns the position of the location of path, and the position of
// the location of fallback where path has none.
func (l *lowered) posOf(path, fallback []int32) position.Pos {
	if loc := l.loc.at(path); loc != nil {
		return l.loc.start(loc)
	}
	return l.loc.start(l.loc.at(fallback))
}

// spelled returns a type name as the source writes it at the location of
// path. A name that the source qualifies partly resolves to a fully
// qualified name that ends with it, so the written name is the suffix of fq
// that the span of the location covers, from a dot on. spelled returns fq
// for a location that is absent or spans more than one line, and for a
// span whose length does not end fq at a dot.
func (l *lowered) spelled(fq string, path []int32) string {
	span := l.loc.at(path).GetSpan()
	if len(span) != 3 {
		return fq
	}
	if s, written := suffixOf(fq, int(span[2]-span[1])); written {
		return s
	}
	return fq
}

// mapValue returns the value type of a map field as the source writes it.
// The location of the type name of the field spans the value alone, or the
// whole map<key, value> from the start of the field, as buf records it.
// Then the value is what the span leaves after map<, the key, the comma
// with or without its space, and >. mapValue returns fq where no reading
// ends fq at a dot.
func (l *lowered) mapValue(fq, key string, path []int32) string {
	typeName := pathOf(path, fieldTypeName)
	span := l.loc.at(typeName).GetSpan()
	if len(span) != 3 || span[1] != l.loc.at(path).GetSpan()[1] {
		return l.spelled(fq, typeName)
	}
	whole := int(span[2] - span[1])
	for _, sep := range []string{listSep, ","} {
		if s, written := suffixOf(fq, whole-len(mapSpelling+"<"+key+sep+">")); written {
			return s
		}
	}
	return fq
}

// suffixOf returns the last n bytes of fq where they start after a dot of
// fq, and false otherwise.
func suffixOf(fq string, n int) (string, bool) {
	if n <= 0 || n >= len(fq) || fq[len(fq)-n-1] != '.' {
		return "", false
	}
	return fq[len(fq)-n:], true
}

// namedRef lowers one named reference, and a well-known type with the
// import path of the file that declares it, as the source frontend does.
func namedRef(spelling string, at position.Pos) *node.TypeRef {
	ref := &node.TypeRef{Spelling: spelling, Pos: at}
	if file, known := protobuf.WellKnownImport(spelling); known {
		ref.Package = file
	}
	return ref
}

// mapField lowers a map field into the Map form over the key and the value
// of its entry, and marks it as a map.
func (l *lowered) mapField(
	f *descriptorpb.FieldDescriptorProto, path []int32, entry *descriptorpb.DescriptorProto, options []option,
) *node.Field {
	at := l.posOf(pathOf(path, fieldTypeName), path)
	var (
		key, value *node.TypeRef
		valueName  string
	)
	for _, ef := range entry.GetField() {
		ref := &node.TypeRef{
			Spelling: strings.ToLower(strings.TrimPrefix(ef.GetType().String(), scalarPrefix)),
			Pos:      at,
		}
		switch ef.GetNumber() {
		case 1:
			key = ref
		case 2:
			value, valueName = ref, ef.GetTypeName()
		}
	}
	if valueName != "" {
		value = namedRef(l.mapValue(valueName, key.Spelling, path), at)
	}
	typ := plugin.Composite(mapSpelling+"<"+key.Spelling+listSep+value.Spelling+">", at, symbol.FormMap, key, value)
	out := l.fieldDecl(f, path, typ, options, false)
	l.unit.Graph().Stamp(out, meta.RawStamp{Key: protobuf.MapEntryKey, Value: mapSpelling, Pos: out.Pos})
	return out
}

// fieldDecl lowers what every field form states: the name, the comments,
// the wire number, a default, a json_name that differs from the default
// one, and the options. block reports a group, whose trailing comment
// protoc takes after the opening brace of its body.
func (l *lowered) fieldDecl(
	f *descriptorpb.FieldDescriptorProto, path []int32, typ *node.TypeRef, options []option, block bool,
) *node.Field {
	loc := l.loc.at(path)
	out := &node.Field{
		Pos:        l.loc.start(loc),
		Name:       f.GetName(),
		Visibility: symbol.VisibilityPublic,
		Mutability: symbol.MutabilityMutable,
		Level:      symbol.LevelInstance,
		Type:       typ,
	}
	c := l.commentsOf(loc, out, block)
	out.Doc, out.Annotations, out.Comment = c.doc, c.annotations, c.comment
	gb := l.unit.Graph()
	gb.Stamp(out, meta.RawStamp{Key: protobuf.FieldKey, Value: strconv.Itoa(int(f.GetNumber())), Pos: out.Pos})
	if f.DefaultValue != nil {
		out.Value = spelledDefault(f)
	}
	if f.JsonName != nil && f.GetJsonName() != jsonName(f.GetName()) {
		gb.Stamp(out, meta.RawStamp{Key: protobuf.JSONNameKey, Value: f.GetJsonName(), Pos: out.Pos})
	}
	l.stampOptions(out, out.Pos, options)
	return out
}

// spelledDefault returns the default of a field as a schema writes it: a
// string and bytes in quotes, and any other value as protoc records it.
func spelledDefault(f *descriptorpb.FieldDescriptorProto) string {
	switch f.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return strconv.Quote(f.GetDefaultValue())
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return `"` + f.GetDefaultValue() + `"`
	default:
		return f.GetDefaultValue()
	}
}

// jsonName returns the JSON name that protoc gives a field without a
// json_name option: the name without its underscores, with each letter
// after an underscore in upper case.
func jsonName(name string) string {
	var b strings.Builder
	upper := false
	for _, r := range name {
		switch {
		case r == '_':
			upper = true
		case upper:
			b.WriteRune(unicode.ToUpper(r))
			upper = false
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// group lowers a group of proto2 into a field and the nested message of
// the group, as the source frontend does. The message is the nested type
// of siblings that the group names, at siblingsPath. protocompile records
// the comments of a group at the location of the message, and they belong
// to the field, before any comment at the location of the field. The
// message has the doc of the field.
func (l *lowered) group(
	f *descriptorpb.FieldDescriptorProto, path []int32, outer protobuf.Features, member bool,
	siblings []*descriptorpb.DescriptorProto, siblingsPath []int32,
) (*node.Field, *node.Struct) {
	options := l.optionsAt(f.GetOptions(), pathOf(path, fieldOptions))
	scope := with(outer, options)
	pos := l.loc.start(l.loc.at(path))
	// A group declares its message in place, so the field writes the name
	// of the group.
	typ, required := fieldType(namedRef(lastName(f.GetTypeName()), pos), l.labelOf(f), scope.Presence, member)
	field := l.fieldDecl(f, path, typ, options, true)
	gb := l.unit.Graph()
	if required {
		gb.Stamp(field, meta.RawStamp{Key: protobuf.LabelKey, Value: labelRequired, Pos: field.Pos})
	}
	gb.Stamp(
		field,
		meta.RawStamp{Key: protobuf.DelimitedKey, Value: string(protobuf.EncodingDelimited), Pos: field.Pos},
	)
	name := lastName(f.GetTypeName())
	message := &node.Struct{Pos: field.Pos, Name: name, Visibility: symbol.VisibilityPublic}
	for k, m := range siblings {
		if m.GetName() != name {
			continue
		}
		mp := pathOf(siblingsPath, int32(k))
		c := l.commentsOf(l.loc.at(mp), field, true)
		field.Doc = append(c.doc, field.Doc...)
		field.Annotations = append(c.annotations, field.Annotations...)
		if field.Comment == "" {
			field.Comment = c.comment
		}
		l.members(m, mp, message, outer, true)
	}
	message.Doc = slices.Clone(field.Doc)
	return field, message
}

// variantOf returns the variant of a member of a oneof, which repeats the
// documentation, the comment and the annotations of the field.
func variantOf(f *node.Field) *node.SumVariant {
	return &node.SumVariant{
		Pos:         f.Pos,
		Name:        f.Name,
		Fields:      []*node.Field{f},
		Doc:         slices.Clone(f.Doc),
		Comment:     f.Comment,
		Annotations: slices.Clone(f.Annotations),
	}
}

// oneof lowers a oneof into a sum whose variants are its members, and
// returns the messages of the groups among the members.
func (l *lowered) oneof(
	o *descriptorpb.OneofDescriptorProto, path []int32, index int32, m *descriptorpb.DescriptorProto,
	messagePath []int32, outer protobuf.Features,
) (*node.Sum, []symbol.Symbol) {
	loc := l.loc.at(path)
	s := &node.Sum{Pos: l.loc.start(loc), Name: o.GetName(), Visibility: symbol.VisibilityPublic}
	c := l.commentsOf(loc, s, true)
	s.Doc, s.Annotations, s.Comment = c.doc, c.annotations, c.comment
	options := l.optionsAt(o.GetOptions(), pathOf(path, oneofOptions))
	scope := with(outer, options)
	var groups []symbol.Symbol
	for j, f := range m.GetField() {
		if f.OneofIndex == nil || f.GetOneofIndex() != index {
			continue
		}
		fp := pathOf(messagePath, messageField, int32(j))
		if l.isGroup(f) {
			field, group := l.group(f, fp, scope, true, m.GetNestedType(), pathOf(messagePath, messageNested))
			s.Variants = append(s.Variants, variantOf(field))
			groups = append(groups, group)
			continue
		}
		s.Variants = append(s.Variants, variantOf(l.field(f, fp, scope, true, m)))
	}
	l.unit.Graph().Stamp(s, meta.RawStamp{Key: protobuf.OneofKey, Value: s.Name, Pos: s.Pos})
	l.stampOptions(s, s.Pos, options)
	return s, groups
}

// enum lowers an enum into the enum kind, each value a variant with its
// number. The reserved ranges, the options, the openness and the
// visibility of the enum are stamps.
func (l *lowered) enum(
	e *descriptorpb.EnumDescriptorProto,
	path []int32,
	outer protobuf.Features,
	nested bool,
) *node.Enum {
	loc := l.loc.at(path)
	out := &node.Enum{Pos: l.loc.start(loc), Name: e.GetName(), Visibility: symbol.VisibilityPublic}
	c := l.commentsOf(loc, out, true)
	out.Doc, out.Annotations, out.Comment = c.doc, c.annotations, c.comment
	options := l.optionsAt(e.GetOptions(), pathOf(path, enumOptions))
	scope := with(outer, options)
	for k, v := range e.GetValue() {
		vloc := l.loc.at(pathOf(path, enumValue, int32(k)))
		variant := &node.EnumVariant{Pos: l.loc.start(vloc), Name: v.GetName(), Value: strconv.Itoa(int(v.GetNumber()))}
		vc := l.commentsOf(vloc, variant, false)
		variant.Doc, variant.Annotations, variant.Comment = vc.doc, vc.annotations, vc.comment
		l.stampOptions(
			variant,
			variant.Pos,
			l.optionsAt(v.GetOptions(), pathOf(path, enumValue, int32(k), enumValueOptions)),
		)
		out.Variants = append(out.Variants, variant)
	}
	gb := l.unit.Graph()
	reserved := make([]string, 0, len(e.GetReservedRange())+len(e.GetReservedName()))
	for _, r := range e.GetReservedRange() {
		reserved = append(reserved, spelledRange(r.GetStart(), r.GetEnd(), maxEnumValue))
	}
	reserved = append(reserved, e.GetReservedName()...)
	if len(reserved) > 0 {
		gb.Stamp(out, meta.RawStamp{Key: protobuf.ReservedKey, Value: strings.Join(reserved, listSep), Pos: out.Pos})
	}
	l.stampOptions(out, out.Pos, options)
	if scope.EnumType == protobuf.EnumClosed {
		gb.Stamp(out, meta.RawStamp{Key: protobuf.ClosedKey, Value: string(protobuf.EnumClosed), Pos: out.Pos})
	}
	l.stampLocal(out, e.GetVisibility(), outer, nested)
	return out
}

// spelledRange spells one range of numbers from first to last, both
// included: one number, first to last, or first to max where last is the
// largest number.
func spelledRange(first, last, largest int32) string {
	switch {
	case first == last:
		return strconv.Itoa(int(first))
	case last == largest:
		return strconv.Itoa(int(first)) + rangeTo + rangeMax
	default:
		return strconv.Itoa(int(first)) + rangeTo + strconv.Itoa(int(last))
	}
}

// stampLocal marks a message or an enum that another file cannot
// reference: one that states local, or one that states neither local nor
// export under a default visibility of the scope that declares it which
// makes it local.
func (l *lowered) stampLocal(
	subject symbol.Symbol,
	visibility descriptorpb.SymbolVisibility,
	outer protobuf.Features,
	nested bool,
) {
	local := outer.LocalByDefault(nested)
	switch visibility {
	case descriptorpb.SymbolVisibility_VISIBILITY_LOCAL:
		local = true
	case descriptorpb.SymbolVisibility_VISIBILITY_EXPORT:
		local = false
	default:
	}
	if local {
		pos := l.loc.start(nil)
		switch s := subject.(type) {
		case *node.Struct:
			pos = s.Pos
		case *node.Enum:
			pos = s.Pos
		}
		l.unit.Graph().Stamp(subject, meta.RawStamp{Key: protobuf.LocalKey, Value: localValue, Pos: pos})
	}
}

// service lowers a service into an interface whose methods are its rpcs.
func (l *lowered) service(s *descriptorpb.ServiceDescriptorProto, path []int32) *node.Interface {
	loc := l.loc.at(path)
	out := &node.Interface{Pos: l.loc.start(loc), Name: s.GetName(), Visibility: symbol.VisibilityPublic}
	c := l.commentsOf(loc, out, true)
	out.Doc, out.Annotations, out.Comment = c.doc, c.annotations, c.comment
	for k, m := range s.GetMethod() {
		out.Methods = append(out.Methods, l.rpc(m, pathOf(path, serviceMethod, int32(k))))
	}
	l.stampOptions(out, out.Pos, l.optionsAt(s.GetOptions(), pathOf(path, serviceOptions)))
	return out
}

// rpc lowers one rpc into an abstract method with one parameter for the
// request and one return for the response, as the source frontend does.
func (l *lowered) rpc(m *descriptorpb.MethodDescriptorProto, path []int32) *node.Method {
	loc := l.loc.at(path)
	out := &node.Method{
		Pos:        l.loc.start(loc),
		Name:       m.GetName(),
		Visibility: symbol.VisibilityPublic,
		Level:      symbol.LevelInstance,
		Abstract:   true,
	}
	c := l.commentsOf(loc, out, false)
	out.Doc, out.Annotations, out.Comment = c.doc, c.annotations, c.comment
	inPath, outPath := pathOf(path, methodInput), pathOf(path, methodOutput)
	in := l.rpcType(l.spelled(m.GetInputType(), inPath), m.GetClientStreaming(), l.posOf(inPath, path))
	result := l.rpcType(l.spelled(m.GetOutputType(), outPath), m.GetServerStreaming(), l.posOf(outPath, path))
	out.Params = []*node.Param{{Pos: in.Pos, Name: requestParam, Type: in}}
	out.Returns = []*node.Return{{Pos: result.Pos, Type: result}}
	out.Async = result.Form != symbol.FormStream
	if side := streamSide(m); side != "" {
		l.unit.Graph().Stamp(out, meta.RawStamp{Key: protobuf.StreamKey, Value: side, Pos: out.Pos})
	}
	l.stampOptions(out, out.Pos, l.optionsAt(m.GetOptions(), pathOf(path, methodOptions)))
	return out
}

// rpcType lowers one side of an rpc to the message that it refers to,
// under the asynchronous stream form where that side streams.
func (*lowered) rpcType(name string, stream bool, at position.Pos) *node.TypeRef {
	inner := namedRef(name, at)
	if !stream {
		return inner
	}
	ref := plugin.Composite(streamKeyword+" "+inner.Spelling, at, symbol.FormStream, inner)
	ref.Async = true
	return ref
}

// streamSide returns the value of the stream stamp for the side of an rpc
// that streams, and the empty string for an rpc where neither side does.
func streamSide(m *descriptorpb.MethodDescriptorProto) string {
	switch in, out := m.GetClientStreaming(), m.GetServerStreaming(); {
	case in && out:
		return streamBoth
	case in:
		return streamRequest
	case out:
		return streamResponse
	default:
		return ""
	}
}

// extensions lowers the fields of the extensions of the file or of a
// message, at listPath. Each field has the extended message as its
// Receives. A group of an extension is a field with Receives and the
// message of siblings at siblingsPath that it names. The comments of an
// extend block belong to its first field, before the comments of the
// field, and they attach once the last field of the block is lowered, as
// the source frontend attributes them. emit receives the declarations of
// each field in the order of the lowerings, which the caller runs.
func (l *lowered) extensions(
	exts []*descriptorpb.FieldDescriptorProto, listPath []int32, scope protobuf.Features,
	siblings []*descriptorpb.DescriptorProto, siblingsPath []int32, emit func(...symbol.Symbol),
) []pending {
	blocks := l.loc.blocks[pathKey(listPath)]
	blockOf := make([]int, len(exts))
	last := make([]int, len(blocks))
	for i := range exts {
		blockOf[i] = -1
		loc := l.loc.at(pathOf(listPath, int32(i)))
		for b, block := range blocks {
			if loc != nil && blockOf[i] < 0 && contains(block, loc) {
				blockOf[i], last[b] = b, i
			}
		}
	}
	heads := make([]*node.Field, len(blocks))
	steps := make([]pending, 0, len(exts))
	for i, f := range exts {
		fp := pathOf(listPath, int32(i))
		steps = append(steps, pending{pos: l.posAt(fp), lower: func() {
			var field *node.Field
			if l.isGroup(f) {
				var group *node.Struct
				field, group = l.group(f, fp, scope, false, siblings, siblingsPath)
				emit(field, group)
			} else {
				field = l.field(f, fp, scope, false, nil)
				emit(field)
			}
			extendee := pathOf(fp, fieldExtendee)
			field.Receives = namedRef(l.spelled(f.GetExtendee(), extendee), l.posOf(extendee, fp))
			b := blockOf[i]
			if b < 0 {
				return
			}
			if heads[b] == nil {
				heads[b] = field
			}
			if last[b] == i {
				l.blockComments(blocks[b], heads[b])
			}
		}})
	}
	return steps
}

// blockComments attaches the comments of an extend block to its first
// field, before the comments of the field.
func (l *lowered) blockComments(block *descriptorpb.SourceCodeInfo_Location, head *node.Field) {
	c := l.commentsOf(block, head, true)
	head.Doc = append(c.doc, head.Doc...)
	head.Annotations = append(c.annotations, head.Annotations...)
	switch {
	case head.Comment == "":
		head.Comment = c.comment
	case c.comment != "":
		head.Comment = c.comment + " " + head.Comment
	}
}

// stampOptions stamps the plain options of a declaration under
// [protobuf.OptionsKey] and its edition features under
// [protobuf.FeaturesKey], each as a list of name=value entries.
func (l *lowered) stampOptions(subject symbol.Symbol, at position.Pos, opts []option) {
	plain, features := split(opts)
	gb := l.unit.Graph()
	if len(plain) > 0 {
		gb.Stamp(subject, meta.RawStamp{Key: protobuf.OptionsKey, Value: plain, Pos: at})
	}
	if len(features) > 0 {
		gb.Stamp(subject, meta.RawStamp{Key: protobuf.FeaturesKey, Value: features, Pos: at})
	}
}
