// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/position"
)

// The option names that the lowering reads apart from the other options.
const (
	// featuresField is the field of an options message that contains the
	// features of a declaration.
	featuresField = "features"
	// mapEntryField marks the synthetic message of a map field, which the
	// lowering folds into the field.
	mapEntryField = "map_entry"
	// uninterpretedField contains the options that protoc did not
	// interpret, which a descriptor of protoc never has.
	uninterpretedField = "uninterpreted_option"
	// optionAssign joins the name of an option to its value.
	optionAssign = "="
)

// option is one option of a declaration: its name as a schema writes it,
// with an extension in parentheses, its value in the text format, and the
// path of its statement under the path of the options message.
type option struct {
	name  string
	value string
	at    []int32
	// msg is the value of an option of a message type that is not a list,
	// which [lowered.perField] spells one field at a time, and nil otherwise.
	msg protoreflect.Message
}

// optionsOf returns the options that an options message sets, one for each
// statement: one for each element of a repeated option, and one for each
// feature that the features of the message set, as
// features.field_presence=IMPLICIT. They are in the order of their field
// numbers, and in the order of their elements. A nil message sets none.
func optionsOf(m proto.Message) []option {
	if m == nil {
		return nil
	}
	msg := m.ProtoReflect()
	if !msg.IsValid() {
		return nil
	}
	var out []option
	for _, fd := range setFields(msg) {
		v := msg.Get(fd)
		name := string(fd.Name())
		if fd.IsExtension() {
			name = "(" + string(fd.FullName()) + ")"
		}
		switch {
		case name == featuresField:
			out = append(out, featuresOf(v.Message(), int32(fd.Number()))...)
		case name == mapEntryField, name == uninterpretedField:
		case fd.IsList():
			list := v.List()
			for i := range list.Len() {
				out = append(
					out,
					option{name: name, value: scalarText(fd, list.Get(i)), at: []int32{int32(fd.Number()), int32(i)}},
				)
			}
		default:
			out = append(
				out,
				option{name: name, value: scalarText(fd, v), at: []int32{int32(fd.Number())}, msg: messageOf(fd, v)},
			)
		}
	}
	return out
}

// messageOf returns the message of a value of a field of a message type,
// and nil for any other value.
func messageOf(fd protoreflect.FieldDescriptor, v protoreflect.Value) protoreflect.Message {
	if fd.IsList() || fd.IsMap() || (fd.Kind() != protoreflect.MessageKind && fd.Kind() != protoreflect.GroupKind) {
		return nil
	}
	return v.Message()
}

// perField spells an option of a message type that the source sets one field
// at a time, as (google.api.resource_reference).type = "*", as one option
// for each field, as the source writes it. protoc records the statement of
// such an option at the path of its field and none at the path of the
// option, under path. Every other option stays whole.
func (l *lowered) perField(o option, path []int32) []option {
	at := pathOf(path, o.at...)
	if o.msg == nil || l.loc.at(at) != nil || !l.loc.under(at) {
		return []option{o}
	}
	var out []option
	for _, fd := range setFields(o.msg) {
		name := string(fd.Name())
		if fd.IsExtension() {
			name = "(" + string(fd.FullName()) + ")"
		}
		v := o.msg.Get(fd)
		at := pathOf(o.at, int32(fd.Number()))
		if fd.IsList() {
			// An option statement takes one value, so the source sets each
			// element of a list in a statement of its own.
			list := v.List()
			for i := range list.Len() {
				out = append(
					out,
					option{name: o.name + "." + name, value: scalarText(fd, list.Get(i)), at: pathOf(at, int32(i))},
				)
			}
			continue
		}
		child := option{name: o.name + "." + name, value: fieldText(fd, v), at: at, msg: messageOf(fd, v)}
		out = append(out, l.perField(child, path)...)
	}
	return out
}

// featuresOf returns one entry for each feature that a feature set sets,
// in the order of their field numbers, with the name of the enum value.
// number is the field number of the features in the options message.
func featuresOf(m protoreflect.Message, number int32) []option {
	fields := setFields(m)
	out := make([]option, 0, len(fields))
	for _, fd := range fields {
		out = append(out, option{
			name:  protobuf.FeaturesOption + "." + string(fd.Name()),
			value: scalarText(fd, m.Get(fd)),
			at:    []int32{number, int32(fd.Number())},
		})
	}
	return out
}

// optionsAt returns the options of an options message at path in the
// order of their statements in the source: the options with a location by
// its position, and every other option after them in the order of
// [optionsOf].
func (l *lowered) optionsAt(m proto.Message, path []int32) []option {
	whole := optionsOf(m)
	opts := make([]option, 0, len(whole))
	for _, o := range whole {
		opts = append(opts, l.perField(o, path)...)
	}
	pos := make(map[*option]position.Pos, len(opts))
	located := make(map[*option]bool, len(opts))
	for i := range opts {
		// An element of a list falls back to the location of its list.
		loc := l.loc.at(pathOf(path, opts[i].at...))
		if loc == nil && len(opts[i].at) > 1 {
			loc = l.loc.at(pathOf(path, opts[i].at[:len(opts[i].at)-1]...))
		}
		if loc != nil {
			pos[&opts[i]], located[&opts[i]] = l.loc.start(loc), true
		}
	}
	order := make([]int, len(opts))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		pa, pb := pos[&opts[a]], pos[&opts[b]]
		switch la, lb := located[&opts[a]], located[&opts[b]]; {
		case la != lb && la:
			return -1
		case la != lb:
			return 1
		case pa.Line != pb.Line:
			return pa.Line - pb.Line
		default:
			return pa.Col - pb.Col
		}
	})
	out := make([]option, len(opts))
	for i, at := range order {
		out[i] = opts[at]
	}
	return out
}

// setFields returns the fields that a message sets, in the order of their
// field numbers. [protoreflect.Message.Range] visits them in an undefined
// order.
func setFields(m protoreflect.Message) []protoreflect.FieldDescriptor {
	var out []protoreflect.FieldDescriptor
	m.Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		out = append(out, fd)
		return true
	})
	slices.SortFunc(out, func(a, b protoreflect.FieldDescriptor) int { return int(a.Number() - b.Number()) })
	return out
}

// scalarText spells one value of an option in the text format: a string
// and bytes with quotes, an enum by the name of its value, a message as a
// text-format literal in braces, and any other value as Go prints it.
func scalarText(fd protoreflect.FieldDescriptor, v protoreflect.Value) string {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return strconv.Quote(v.String())
	case protoreflect.BytesKind:
		return strconv.Quote(string(v.Bytes()))
	case protoreflect.EnumKind:
		if ev := fd.Enum().Values().ByNumber(v.Enum()); ev != nil {
			return string(ev.Name())
		}
		return strconv.Itoa(int(v.Enum()))
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return messageText(v.Message())
	default:
		return v.String()
	}
}

// messageText spells a message value as a text-format literal on one
// line: {name: value, name: value} in the order of the field numbers, a
// list as [a, b], a map as the list of its entries in key order, and an
// extension by its full name in brackets. prototext varies the spaces of
// its output with the build of the binary, so the frontend spells the
// literal itself.
func messageText(m protoreflect.Message) string {
	fields := setFields(m)
	parts := make([]string, 0, len(fields))
	for _, fd := range fields {
		name := string(fd.Name())
		if fd.IsExtension() {
			name = "[" + string(fd.FullName()) + "]"
		}
		parts = append(parts, name+": "+fieldText(fd, m.Get(fd)))
	}
	return "{" + strings.Join(parts, listSep) + "}"
}

// fieldText spells the value of one field of a message: a map as the list
// of its entries, a list as [a, b], and any other value as [scalarText]
// spells it.
func fieldText(fd protoreflect.FieldDescriptor, v protoreflect.Value) string {
	switch {
	case fd.IsMap():
		return mapText(fd, v.Map())
	case fd.IsList():
		list := v.List()
		elems := make([]string, list.Len())
		for i := range elems {
			elems[i] = scalarText(fd, list.Get(i))
		}
		return "[" + strings.Join(elems, listSep) + "]"
	default:
		return scalarText(fd, v)
	}
}

// mapText spells the entries of a map field as a list of {key: k, value:
// v} literals, in key order.
func mapText(fd protoreflect.FieldDescriptor, m protoreflect.Map) string {
	keys := make([]protoreflect.MapKey, 0, m.Len())
	m.Range(func(k protoreflect.MapKey, _ protoreflect.Value) bool {
		keys = append(keys, k)
		return true
	})
	slices.SortFunc(keys, compareKeys)
	entries := make([]string, len(keys))
	for i, k := range keys {
		entries[i] = "{key: " + scalarText(fd.MapKey(), k.Value()) + listSep +
			"value: " + scalarText(fd.MapValue(), m.Get(k)) + "}"
	}
	return "[" + strings.Join(entries, listSep) + "]"
}

// compareKeys orders two keys of one map: a bool, an integer or a string,
// as protobuf allows for a map key.
func compareKeys(a, b protoreflect.MapKey) int {
	switch x := a.Interface().(type) {
	case bool:
		return cmp.Compare(boolRank(x), boolRank(b.Bool()))
	case int32, int64:
		return cmp.Compare(a.Int(), b.Int())
	case uint32, uint64:
		return cmp.Compare(a.Uint(), b.Uint())
	default:
		return cmp.Compare(a.String(), b.String())
	}
}

// boolRank orders false before true.
func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// with returns f after the features among opts, in order, as
// [protobuf.Features.Apply] applies each of them.
func with(f protobuf.Features, opts []option) protobuf.Features {
	for _, o := range opts {
		f = f.Apply(o.name, o.value)
	}
	return f
}

// split returns the features among opts and the other options, each as
// name=value entries in order.
func split(opts []option) (plain, features []string) {
	for _, o := range opts {
		entry := o.name + optionAssign + o.value
		if strings.HasPrefix(o.name, protobuf.FeaturesOption+".") {
			features = append(features, entry)
			continue
		}
		plain = append(plain, entry)
	}
	return plain, features
}
