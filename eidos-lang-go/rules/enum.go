// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"
	"strconv"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
)

// EnumOf projects a Go enumeration, a defined type with typed
// constants: identifier-formed, each variant's text its name, the
// exact values read from the frontend's constValue stamps, so iota
// arithmetic is evaluated once at parse and read here. The zero
// variant is the one whose value is 0, a duplicate the first text
// two variants' values share, the out-of-range value one past the
// largest integer value, and the foreign packages those declaring
// a variant outside the type's own.
func (r Rules) EnumOf(e *node.Enum, v rules.View) rules.EnumInfo {
	info := rules.EnumInfo{Form: rules.EnumIdentifier}
	if e == nil {
		return info
	}
	key, keyed := r.constKey(v)
	seen := map[string]string{}
	var largest int64
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
		if !keyed {
			continue
		}
		value, stamped := rules.Fact(v, variant.ID, key)
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
		if n, err := strconv.ParseInt(value, 0, 64); err == nil && (!ranged || n > largest) {
			largest, ranged = n, true
		}
	}
	slices.Sort(info.Foreign)
	if ranged {
		ref := &node.TypeRef{Spelling: e.Name, Target: e.ID}
		info.OutOfRange = emit.Conversion(rules.EmitRef(ref),
			emit.Literal(emit.LiteralInt, strconv.FormatInt(largest+1, 10)))
	}
	return info
}

// constKey returns the handle the frontend's exact constant values
// stamp under, and false where the view carries no facts or the
// composition registered no such key.
func (Rules) constKey(v rules.View) (meta.Key[string], bool) {
	if v.Facts == nil {
		return meta.Key[string]{}, false
	}
	return meta.Lookup[string](v.Facts.Registry(), golang.ConstValueKey)
}
