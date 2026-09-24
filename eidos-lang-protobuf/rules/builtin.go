// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// wellKnownPrefix opens the fully-qualified name of every well-known
// type.
const wellKnownPrefix = protobuf.WellKnownPackage + protobuf.NameSep

// The well-known types with a shape of their own, by their
// fully-qualified names.
const (
	wellKnownTimestamp = wellKnownPrefix + "Timestamp"
	wellKnownDuration  = wellKnownPrefix + "Duration"
	wellKnownEmpty     = wellKnownPrefix + "Empty"
	wellKnownFieldMask = wellKnownPrefix + "FieldMask"
	wellKnownStruct    = wellKnownPrefix + "Struct"
	wellKnownValue     = wellKnownPrefix + "Value"
	wellKnownListValue = wellKnownPrefix + "ListValue"
)

// wrappers maps each well-known type that wraps one scalar to give it
// explicit presence onto that scalar. Each projects as the optional
// form over the scalar, which is what the wrapper is for: a
// StringValue is a string that may be absent.
var wrappers = map[string]string{
	wellKnownPrefix + "DoubleValue": "double",
	wellKnownPrefix + "FloatValue":  "float",
	wellKnownPrefix + "Int64Value":  "int64",
	wellKnownPrefix + "UInt64Value": "uint64",
	wellKnownPrefix + "Int32Value":  "int32",
	wellKnownPrefix + "UInt32Value": "uint32",
	wellKnownPrefix + "BoolValue":   "bool",
	wellKnownPrefix + "StringValue": "string",
	wellKnownPrefix + "BytesValue":  "bytes",
}

// scalar is one proto number's projection: its class and the width
// the wire gives it.
type scalar struct {
	class rules.ScalarClass
	bits  int
}

// scalars is the wire table of the numbers. A width is the wire's,
// so int32 projects as thirty-two bits in every target. The zigzag
// and fixed spellings differ in encoding and not in range, so each
// projects as the width its name states.
var scalars = map[string]scalar{
	"int32":    {rules.ScalarInt, 32},
	"sint32":   {rules.ScalarInt, 32},
	"sfixed32": {rules.ScalarInt, 32},
	"int64":    {rules.ScalarInt, 64},
	"sint64":   {rules.ScalarInt, 64},
	"sfixed64": {rules.ScalarInt, 64},
	"uint32":   {rules.ScalarUint, 32},
	"fixed32":  {rules.ScalarUint, 32},
	"uint64":   {rules.ScalarUint, 64},
	"fixed64":  {rules.ScalarUint, 64},
	"float":    {rules.ScalarFloat, 32},
	"double":   {rules.ScalarFloat, 64},
}

// The three scalars that are not numbers.
const (
	spellBool   = "bool"
	spellString = "string"
	spellBytes  = "bytes"
)

// Builtin returns the shape of a named reference the resolution step
// left without a target.
//
// A number classifies from the wire table, so its width is the
// wire's: int32 is thirty-two bits everywhere. bool and string are
// their leaves and bytes is the byte string. A well-known type,
// spelled with or without its leading dot, projects from the table
// [protobuf.WellKnown] reports on.
//
// Every other spelling returns [symbol.FormOpaque] with the
// spelling, which is a message this workspace does not declare. That
// is degradation a reader can ask about, not failure: the reference
// keeps its name and a consumer decides what to do. A nil reference
// returns Opaque too. The view is unread, so the shape depends on
// the spelling alone.
func (Rules) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	if ref == nil {
		return rules.Opaque(nil)
	}
	if s, held := scalars[ref.Spelling]; held {
		return rules.Scalar(ref.Spelling, s.class, s.bits)
	}
	switch ref.Spelling {
	case spellBool:
		return rules.Leaf(symbol.FormBool, ref.Spelling)
	case spellString:
		return rules.Leaf(symbol.FormText, ref.Spelling)
	case spellBytes:
		// A proto bytes field is the byte string the fold's own rule
		// derives from a list of eight-bit scalars, stated outright
		// because protobuf spells it as one word.
		return rules.Leaf(symbol.FormBytes, ref.Spelling)
	}
	if name, known := protobuf.WellKnown(ref.Spelling); known {
		return wellKnownShape(ref, name)
	}
	return rules.Opaque(ref)
}

// wellKnownShape projects a well-known type as the shape it
// represents, not as a message the workspace may not declare.
//
// Timestamp and Duration are the kernel registry's. A wrapper is its
// scalar under the optional form, which is the presence it exists to
// give. Empty is the empty record, FieldMask the list of paths it
// contains, ListValue a list and Struct a map, both onto values
// decided at run time. Any, Value and NullValue are decided at run
// time too, so the projection keeps them opaque and a consumer reads
// the spelling.
func wellKnownShape(ref *node.TypeRef, name string) rules.TypeShape {
	if wrapped, wraps := wrappers[name]; wraps {
		inner := Rules{}.Builtin(&node.TypeRef{Spelling: wrapped}, rules.View{})
		return rules.TypeShape{
			Form:     symbol.FormOptional,
			Spelling: ref.Spelling,
			Elems:    []rules.TypeShape{inner},
		}
	}
	dynamic := rules.Opaque(&node.TypeRef{Spelling: wellKnownValue})
	switch name {
	case wellKnownTimestamp:
		return rules.Reference(ref.Spelling, rules.WellKnownTimestamp)
	case wellKnownDuration:
		return rules.Reference(ref.Spelling, rules.WellKnownDuration)
	case wellKnownEmpty:
		return rules.TypeShape{Form: symbol.FormInline, Spelling: ref.Spelling}
	case wellKnownFieldMask:
		return rules.TypeShape{
			Form:     symbol.FormList,
			Spelling: ref.Spelling,
			Elems:    []rules.TypeShape{rules.Leaf(symbol.FormText, spellString)},
		}
	case wellKnownListValue:
		return rules.TypeShape{
			Form:     symbol.FormList,
			Spelling: ref.Spelling,
			Elems:    []rules.TypeShape{dynamic},
		}
	case wellKnownStruct:
		return rules.TypeShape{
			Form:     symbol.FormMap,
			Spelling: ref.Spelling,
			Elems:    []rules.TypeShape{rules.Leaf(symbol.FormText, spellString), dynamic},
		}
	default:
		return rules.Opaque(ref)
	}
}

// wrapped returns the scalar a well-known wrapper spelling wraps, and
// false for every other spelling.
func wrapped(spelling string) (string, bool) {
	name, known := protobuf.WellKnown(spelling)
	if !known {
		return "", false
	}
	inner, wraps := wrappers[name]
	return inner, wraps
}
