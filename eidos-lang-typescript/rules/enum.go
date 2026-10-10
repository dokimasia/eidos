// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"cmp"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/numeric"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
)

// EnumOf projects a TypeScript enum. A string enum, in which every
// member has a text, is value-formed, and the text of a member is its
// value. Every other enum is identifier-formed, and the text of a member
// is its name. EnumOf computes the values as TypeScript does. A member
// with an initializer has the value of the initializer, which is a
// constant enum expression. A member without one has the value of the
// member before it plus one, and the first member has 0.
//
// EnumOf also sets these fields of the returned [rules.EnumInfo]:
//
//   - Zero is the first member whose value is 0, or the empty text in a
//     string enum.
//   - Duplicate is the name of the earlier member at the first
//     repetition of a value.
//   - OutOfRange is one more than the largest value, asserted to the
//     enum's type. In a string enum, it is the empty text when no member
//     has that text.
//   - Foreign is the sorted list of the packages other than the enum's
//     own that declare a member.
//
// A member whose initializer is not a constant enum expression gets its
// value when the program runs. Such a member neither repeats nor bounds
// another member. EnumOf returns an identifier-formed info
// without members for a nil enum. It does not read the view, because
// every value is on the declaration.
//
// # Allocation contract
//
// EnumOf allocates the list of members, the list of variants, and the
// out-of-range value. The out-of-range value costs the reference to the
// enum, the conversion and the decimal text of its number. A text with
// an escape allocates its content, and a foreign package grows the list
// of foreign packages.
func (Rules) EnumOf(e *node.Enum, _ rules.View) rules.EnumInfo {
	info := rules.EnumInfo{Form: rules.EnumIdentifier}
	if e == nil {
		return info
	}
	ms := members(e)
	textual := len(ms) > 0 && !slices.ContainsFunc(ms, func(m member) bool { return !m.known || !m.value.isText })
	if textual {
		info.Form = rules.EnumValue
	}
	info.Variants = make([]rules.VariantText, 0, len(ms))
	var largest float64
	ranged, empty := false, false
	for i, m := range ms {
		text := m.name
		if textual {
			text = m.value.text
		}
		info.Variants = append(info.Variants,
			rules.VariantText{Name: m.name, Text: emit.Literal(emit.LiteralString, text)})
		if !m.known {
			continue
		}
		first := slices.IndexFunc(ms[:i], func(earlier member) bool {
			return earlier.known && earlier.value == m.value
		})
		if first >= 0 && info.Duplicate == "" {
			info.Duplicate = ms[first].name
		}
		switch {
		case textual:
			if text == "" {
				empty = true
				info.Zero = cmp.Or(info.Zero, m.name)
			}
		case m.value.isText:
		default:
			if m.value.number == 0 {
				info.Zero = cmp.Or(info.Zero, m.name)
			}
			if !ranged || m.value.number > largest {
				largest, ranged = m.value.number, true
			}
		}
	}
	for _, variant := range e.Variants {
		if variant != nil && !variant.ID.IsZero() && variant.ID.Package != e.ID.Package &&
			!slices.Contains(info.Foreign, variant.ID.Package) {

			info.Foreign = append(info.Foreign, variant.ID.Package)
		}
	}
	slices.Sort(info.Foreign)
	t := &node.TypeRef{Spelling: e.Name, Target: e.ID}
	switch {
	case textual && !empty:
		info.OutOfRange = emit.Conversion(rules.EmitRef(t), emit.Literal(emit.LiteralString, ""))
	case !textual && ranged:
		info.OutOfRange = emit.Conversion(rules.EmitRef(t),
			emit.Number(emit.LiteralFloat, numeric.Decimal(largest+1, numberBits), numberBits))
	}
	return info
}

// members returns the members of an enum in declaration order. A member
// has a known value when a constant enum expression decides it. members
// leaves out a nil variant.
func members(e *node.Enum) []member {
	out := make([]member, 0, len(e.Variants))
	for _, variant := range e.Variants {
		if variant == nil {
			continue
		}
		m := member{name: variant.Name}
		switch {
		case strings.TrimSpace(variant.Value) != "":
			m.value, m.known = evaluate(variant.Value, e.Name, out)
		case len(out) == 0:
			m.known = true
		default:
			prev := out[len(out)-1]
			m.value.number, m.known = prev.value.number+1, prev.known && !prev.value.isText
		}
		out = append(out, m)
	}
	return out
}
