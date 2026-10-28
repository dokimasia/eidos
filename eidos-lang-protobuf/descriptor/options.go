// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor

import (
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"
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
			out = append(out, option{name: name, value: scalarText(fd, v), at: []int32{int32(fd.Number())}})
		}
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
	opts := optionsOf(m)
	pos := make(map[*option]position.Pos, len(opts))
	located := make(map[*option]bool, len(opts))
	for i := range opts {
		loc := l.loc.at(pathOf(path, opts[i].at...))
		if loc == nil && len(opts[i].at) > 1 {
			loc = l.loc.at(pathOf(path, opts[i].at[0]))
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
		text := prototext.MarshalOptions{}.Format(v.Message().Interface())
		return "{" + strings.TrimSpace(text) + "}"
	default:
		return v.String()
	}
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
