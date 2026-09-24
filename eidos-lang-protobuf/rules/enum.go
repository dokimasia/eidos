// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"math"
	"slices"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
)

// EnumOf projects one enumeration for a generator writing over it.
//
// The form is [rules.EnumIdentifier]: a variant's text is its
// declared name, since protobuf states no string form of its own.
// Variants are returned in declaration order, and a nil variant or
// one with no name is skipped.
//
// The rest of the returned [rules.EnumInfo]:
//
//   - Zero is the first declared variant, which is protobuf's default
//     for an enum field. proto3 requires its number to be zero.
//   - Duplicate is the first name another variant shares its number
//     with, the numbers read in any base protobuf writes. protobuf
//     admits that as an alias under allow_alias, and a consumer
//     switching on the number must know.
//   - OutOfRange is a number no variant declares, converted to the
//     enum type at the wire's thirty-two bits: one past the largest
//     number, or one below the smallest where the largest is the
//     int32 maximum.
//   - Foreign lists, sorted, the packages declaring a variant outside
//     the enum's own.
//
// A nil enum returns the zero info. The view is unread, because
// every number is on the declaration.
func (Rules) EnumOf(e *node.Enum, _ rules.View) rules.EnumInfo {
	info := rules.EnumInfo{Form: rules.EnumIdentifier}
	if e == nil {
		return info
	}
	seen := map[int64]string{}
	var smallest, largest int64
	numbered := false
	for _, variant := range e.Variants {
		if variant == nil || variant.Name == "" {
			continue
		}
		info.Variants = append(info.Variants, rules.VariantText{
			Name: variant.Name,
			Text: emit.Literal(emit.LiteralString, variant.Name),
		})
		if info.Zero == "" {
			info.Zero = variant.Name
		}
		if !variant.ID.IsZero() && variant.ID.Package != e.ID.Package &&
			!slices.Contains(info.Foreign, variant.ID.Package) {
			info.Foreign = append(info.Foreign, variant.ID.Package)
		}
		n, valid := enumNumber(variant)
		if !valid {
			continue
		}
		if first, dup := seen[n]; !dup {
			seen[n] = variant.Name
		} else if info.Duplicate == "" {
			info.Duplicate = first
		}
		if !numbered || n > largest {
			largest = n
		}
		if !numbered || n < smallest {
			smallest = n
		}
		numbered = true
	}
	slices.Sort(info.Foreign)
	if numbered {
		t := rules.EmitRef(&node.TypeRef{Spelling: e.Name, Target: e.ID})
		switch {
		case largest < math.MaxInt32:
			info.OutOfRange = enumValue(t, largest+1)
		case smallest > math.MinInt32:
			info.OutOfRange = enumValue(t, smallest-1)
		}
	}
	return info
}
