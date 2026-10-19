// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The suffixes of the names of the wrapper messages, which follow the name
// of the field in Pascal case: the item of a list, the value of a map or of
// a member, and the entry of a map whose key protobuf does not take.
const (
	itemSuffix  = "Item"
	valueSuffix = "Value"
	entrySuffix = "Entry"
)

// The fields of the wrapper messages: the list or the map that a wrapper
// wraps, the optional value that it wraps, and the key and the value of an
// entry.
const (
	valuesField = "values"
	valueField  = "value"
	keyField    = "key"
)

// optionalLabel opens the spelling of an optional that the lowering writes.
const optionalLabel = "optional "

// mapKeys are the scalar types that protobuf takes as the key of a map:
// the integer types, bool and string.
var mapKeys = map[string]bool{
	"int32": true, "int64": true, "uint32": true, "uint64": true,
	"sint32": true, "sint64": true, "fixed32": true, "fixed64": true,
	"sfixed32": true, "sfixed64": true, protobuf.TypeBool: true, protobuf.TypeString: true,
}

// shape returns the type t of the field name in a form that protobuf
// writes. It declares through add each wrapper message that the form
// needs, as the protobuf ecosystem writes it, with the name of the field in
// Pascal case before the suffix of the wrapper:
//
//   - a list of a list or of a map is a list of the wrapper Item, whose
//     field values is the element;
//   - a list of an optional scalar is a list of the well-known wrapper of
//     the scalar, such as google.protobuf.Int32Value;
//   - a list of any other optional is a list of the wrapper Item, whose
//     field value is the optional;
//   - a map to a list or to a map is a map to the wrapper Value, whose field
//     values is the value of the map;
//   - a map to an optional scalar is a map to the well-known wrapper of the
//     scalar, and a map to any other optional is a map to the wrapper Value,
//     whose field value is the optional;
//   - a map whose key is not an integer, a bool or a string is a list of the
//     wrapper Entry, whose fields key and value are the key and the value;
//   - an optional of a list or of a map is the list or the map, whose
//     absent value is the empty one.
//
// Each field of a wrapper takes its shape by the same rules, and the
// wrapper declares the wrapper of its field. shape returns every other type
// as it is, and a form with the wrong number of children too, because the
// writer refuses it.
//
// # Allocation contract
//
// A type that protobuf writes as it is allocates nothing. A wrapper
// allocates its message, its field, its reference and the form that refers
// to it.
func shape(name string, t *emit.TypeRef, add func(*emit.Struct)) *emit.TypeRef {
	if t == nil {
		return nil
	}
	switch {
	case t.Form == symbol.FormOptional && len(t.Elems) == 1:
		if elem := t.Elems[0]; elem.Form == symbol.FormList || elem.Form == symbol.FormMap {
			return shape(name, elem, add)
		}
	case t.Form == symbol.FormList && len(t.Elems) == 1:
		elem := listElement(name, t.Elems[0], add)
		if elem != t.Elems[0] {
			return &emit.TypeRef{
				Spelling: repeatedLabel + elem.Spelling,
				Form:     symbol.FormList,
				Elems:    []*emit.TypeRef{elem},
			}
		}
	case t.Form == symbol.FormMap && len(t.Elems) == 2:
		return mapShape(name, t, add)
	}
	return t
}

// listElement returns the element of a list of the field name in a form
// that protobuf writes, by the rules of [shape].
func listElement(name string, elem *emit.TypeRef, add func(*emit.Struct)) *emit.TypeRef {
	switch {
	case elem.Form == symbol.FormList || elem.Form == symbol.FormMap:
		return wrapped(naming.Pascal(name)+itemSuffix, valuesField, elem, add)
	case elem.Form == symbol.FormOptional && len(elem.Elems) == 1:
		inner := elem.Elems[0]
		if wrapper, scalar := protobuf.Wrapper(inner.Spelling); scalar && inner.Form == symbol.FormNamed {
			return &emit.TypeRef{Spelling: wrapper, Package: protobuf.WrappersFile}
		}
		return wrapped(naming.Pascal(name)+itemSuffix, valueField, elem, add)
	default:
		return elem
	}
}

// mapShape returns the map t of the field name in a form that protobuf
// writes, by the rules of [shape].
func mapShape(name string, t *emit.TypeRef, add func(*emit.Struct)) *emit.TypeRef {
	key, value := t.Elems[0], t.Elems[1]
	if key.Form != symbol.FormNamed || !mapKeys[key.Spelling] {
		entry := &emit.Struct{Name: naming.Pascal(name) + entrySuffix}
		nest := func(m *emit.Struct) { entry.Types.Append(m) }
		entry.Fields.Append(
			&emit.Field{
				Name: keyField, Type: shape(keyField, key, nest),
				Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "1"}),
			},
			&emit.Field{
				Name: valueField, Type: shape(valueField, value, nest),
				Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "2"}),
			},
		)
		add(entry)
		ref := &emit.TypeRef{Spelling: entry.Name}
		return &emit.TypeRef{Spelling: repeatedLabel + entry.Name, Form: symbol.FormList, Elems: []*emit.TypeRef{ref}}
	}
	var v *emit.TypeRef
	switch {
	case value.Form == symbol.FormList || value.Form == symbol.FormMap:
		v = wrapped(naming.Pascal(name)+valueSuffix, valuesField, value, add)
	case value.Form == symbol.FormOptional && len(value.Elems) == 1:
		inner := value.Elems[0]
		if wrapper, scalar := protobuf.Wrapper(inner.Spelling); scalar && inner.Form == symbol.FormNamed {
			v = &emit.TypeRef{Spelling: wrapper, Package: protobuf.WrappersFile}
		} else {
			v = wrapped(naming.Pascal(name)+valueSuffix, valueField, value, add)
		}
	default:
		return t
	}
	return &emit.TypeRef{
		Spelling: mapOpen + key.Spelling + mapSep + v.Spelling + mapClose,
		Form:     symbol.FormMap, Elems: []*emit.TypeRef{key, v},
	}
}

// wrapped declares through add the wrapper message name, whose one field,
// numbered 1, has the name field and the type t in the shape that [shape]
// gives it. It returns the reference to the wrapper.
func wrapped(name, field string, t *emit.TypeRef, add func(*emit.Struct)) *emit.TypeRef {
	m := &emit.Struct{Name: name}
	m.Fields.Append(&emit.Field{
		Name: field, Type: shape(field, t, func(n *emit.Struct) { m.Types.Append(n) }),
		Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "1"}),
	})
	add(m)
	return &emit.TypeRef{Spelling: m.Name}
}
