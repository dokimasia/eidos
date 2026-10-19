// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
)

// EnumOf projects one enum for a generator that writes over it.
//
// The form is [rules.EnumIdentifier], because GraphQL serializes an enum
// value as its name: the text of a variant is its name. The variants are in
// declaration order, and a nil variant or one without a name is left out.
//
// The rest of the returned [rules.EnumInfo]:
//
//   - Zero is empty, because GraphQL gives an enum no zero value.
//   - Duplicate is the first name that a later variant repeats. A schema
//     with one does not validate, and an extension can add one.
//   - OutOfRange is the empty name converted to the enum type, because no
//     GraphQL name is empty. An enum without variants has none.
//   - Foreign lists, sorted, the packages that declare a variant outside
//     the package of the enum.
//
// A nil enum returns the zero info. The view is unread. EnumOf finds a
// duplicate in time quadratic in the variants and allocates nothing for it.
//
// # Allocation contract
//
// EnumOf allocates the list of variants, and the reference to the enum and
// the conversion of the value out of range. The list of foreign packages
// allocates as it grows.
func (Rules) EnumOf(e *node.Enum, _ rules.View) rules.EnumInfo {
	info := rules.EnumInfo{Form: rules.EnumIdentifier}
	if e == nil {
		return info
	}
	info.Variants = make([]rules.VariantText, 0, len(e.Variants))
	for i, variant := range e.Variants {
		if variant == nil || variant.Name == "" {
			continue
		}
		info.Variants = append(info.Variants, rules.VariantText{
			Name: variant.Name,
			Text: emit.Literal(emit.LiteralString, variant.Name),
		})
		repeats := slices.ContainsFunc(e.Variants[i+1:], func(later *node.EnumVariant) bool {
			return later != nil && later.Name == variant.Name
		})
		if repeats && info.Duplicate == "" {
			info.Duplicate = variant.Name
		}
		if !variant.ID.IsZero() && variant.ID.Package != e.ID.Package &&
			!slices.Contains(info.Foreign, variant.ID.Package) {

			info.Foreign = append(info.Foreign, variant.ID.Package)
		}
	}
	slices.Sort(info.Foreign)
	if len(info.Variants) > 0 {
		t := rules.EmitRef(&node.TypeRef{Spelling: e.Name, Target: e.ID})
		info.OutOfRange = emit.Conversion(t, emit.Literal(emit.LiteralString, ""))
	}
	return info
}
