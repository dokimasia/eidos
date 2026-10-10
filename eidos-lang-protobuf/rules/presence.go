// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Rules reports the presence of a message field, which its type does not
// contain where the field's field_presence is IMPLICIT.
var _ rules.PresenceRules = Rules{}

// Presence reports whether a field has presence that its type does not
// contain. Such a field is a singular field of a message type. protobuf
// gives that field presence in every version, also where its
// field_presence is IMPLICIT.
//
// A field whose type is not a named reference reports false: the load
// gives a field with explicit presence the optional form, and a repeated
// field and a map field have no presence. A member of a oneof reports
// false, because the oneof's sum contains its presence, and so does a
// required field, which the load marks with [protobuf.LabelKey]. Every
// well-known type is a message, except the enum NullValue. A wrapper
// reports false, because its shape is an optional already.
//
// Presence reads the field's host, its label and the declaration that its
// type resolves to through v, so the caller's read set records each
// dependency. It allocates nothing.
func (Rules) Presence(f *node.Field, v rules.View) bool {
	if f == nil || f.Type == nil || f.Type.Form != symbol.FormNamed {
		return false
	}
	if host, _ := v.Lookup(f.Host); host != nil {
		if _, member := host.(*node.SumVariant); member {
			return false
		}
	}
	if stamped(v, f.ID, protobuf.LabelKey) {
		return false
	}
	if name, known := protobuf.WellKnown(f.Type.Spelling); known {
		_, wraps := wrappers[name]
		return !wraps && name != wellKnownNullValue
	}
	target, _ := v.Lookup(f.Type.Target)
	_, message := target.(*node.Struct)
	return message
}

// stamped reports whether the load stamped a key on a subject. A view
// without facts and a key that the composition did not register report
// false. It reads the fact through v, so the caller's read set records
// it, and allocates nothing.
func stamped(v rules.View, id symbol.Identity, name meta.KeyName) bool {
	if v.Facts == nil {
		return false
	}
	key, registered := meta.Lookup[string](v.Facts.Registry(), name)
	if !registered {
		return false
	}
	_, held := rules.Fact(v, id, key)
	return held
}
