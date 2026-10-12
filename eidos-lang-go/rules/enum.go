// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"math"
	"slices"
	"strconv"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// platformBits is the width that the range of an integer of the platform
// width takes. Every platform of Go holds 32 bits in an int.
const platformBits = 32

// constValue is the handle of the exact constant values that the Go
// frontend stamps. A read resolves the handle in the registry of the
// facts of the view, and a composition that did not register the key
// reads no value.
var constValue = meta.Named[string](golang.ConstValueKey)

// EnumOf projects a Go enumeration, which is a defined type with typed
// constants. The projection has these parts:
//   - The form is identifier-formed, and the text of each variant is its
//     name.
//   - The exact values come from the constant-value stamps of the
//     frontend, so the frontend evaluates iota arithmetic once at parse
//     time.
//   - The zero variant is the variant whose value is 0.
//   - The duplicate is the name of the first variant whose value a later
//     variant repeats.
//   - The out-of-range value is one past the largest integer value where
//     the underlying type holds it, and one below the smallest integer
//     value otherwise. A set whose values reach both ends of its
//     underlying type has no out-of-range value.
//   - The foreign packages are the packages other than the package of
//     the type that declare a variant.
//
// EnumOf compares values as int64. A value that int64 does not hold,
// such as a uint64 value from 2^63 up, takes no part in the range.
//
// # Allocation contract
//
// EnumOf allocates the variant list, sized to the enumeration. Where a
// variant has a stamped integer, it allocates the out-of-range value,
// which is the reference to the type and the converted integer. Each
// foreign package grows the list of foreign packages.
func (r Rules) EnumOf(e *node.Enum, v rules.View) rules.EnumInfo {
	info := rules.EnumInfo{Form: rules.EnumIdentifier}
	if e == nil {
		return info
	}
	info.Variants = make([]rules.VariantText, 0, len(e.Variants))
	seen := map[string]string{}
	var smallest, largest int64
	var ranged bool
	for _, variant := range e.Variants {
		if variant == nil {
			continue
		}
		info.Variants = append(info.Variants, rules.VariantText{
			Name: variant.Name, Text: emit.Literal(emit.LiteralString, variant.Name),
		})
		if !variant.ID.IsZero() && variant.ID.Package != e.ID.Package &&
			!slices.Contains(info.Foreign, variant.ID.Package) {

			info.Foreign = append(info.Foreign, variant.ID.Package)
		}
		value, stamped := rules.Fact(v, variant.ID, constValue)
		if !stamped {
			continue
		}
		if value == sampleZero && info.Zero == "" {
			info.Zero = variant.Name
		}
		if first, dup := seen[value]; dup && info.Duplicate == "" {
			info.Duplicate = first
		} else if !dup {
			seen[value] = variant.Name
		}
		n, err := strconv.ParseInt(value, 0, 64)
		if err != nil {
			continue
		}
		if !ranged || n < smallest {
			smallest = n
		}
		if !ranged || n > largest {
			largest = n
		}
		ranged = true
	}
	slices.Sort(info.Foreign)
	if !ranged {
		return info
	}
	lowest, highest := bounds(r.Builtin(e.Underlying, v))
	var outside int64
	switch {
	case largest < highest:
		outside = largest + 1
	case smallest > lowest:
		outside = smallest - 1
	default:
		return info
	}
	ref := &node.TypeRef{Spelling: e.Name, Target: e.ID}
	info.OutOfRange = emit.Conversion(rules.EmitRef(ref),
		emit.Literal(emit.LiteralInt, strconv.FormatInt(outside, 10)))
	return info
}

// bounds returns the smallest and the largest value of an integer of the
// shape s, within the range of int64. An integer of the platform width has
// the range of 32 bits, because Go defines int and uint as 32 or 64 bits
// wide. A float and every shape that is not a scalar have the range of
// int64. bounds allocates nothing.
func bounds(s rules.TypeShape) (int64, int64) {
	if s.Form != symbol.FormScalar || s.Class == rules.ScalarFloat {
		return math.MinInt64, math.MaxInt64
	}
	bits := s.Bits
	if bits == 0 {
		bits = platformBits
	}
	switch {
	case s.Class == rules.ScalarUint && bits >= 64:
		return 0, math.MaxInt64
	case s.Class == rules.ScalarUint:
		return 0, 1<<bits - 1
	case bits >= 64:
		return math.MinInt64, math.MaxInt64
	default:
		return -1 << (bits - 1), 1<<(bits-1) - 1
	}
}
