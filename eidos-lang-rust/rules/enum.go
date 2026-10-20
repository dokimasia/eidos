// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// EnumOf projects an enum of Rust, whose variants have no payload. The enum is
// identifier-formed, and the text of a variant is its name, because Rust
// writes a variant by its name and gives it an integer discriminant and no
// text. EnumOf also sets these fields of the returned [rules.EnumInfo]:
//
//   - Zero is the variant that the attribute #[default] marks, where the enum
//     derives Default, because Default::default returns that variant.
//   - Duplicate is empty, because Rust refuses two variants of one
//     discriminant with error E0081.
//   - OutOfRange is the zero value, because safe Rust cannot build a value of
//     an enum outside its variants.
//   - Foreign is nil, because the body of an enum declares every variant.
//
// A nil enum returns an identifier-formed info without variants. The view is
// unread.
//
// # Allocation contract
//
// EnumOf allocates the list of variants, and nothing for a nil enum.
func (Rules) EnumOf(e *node.Enum, _ rules.View) rules.EnumInfo {
	info := rules.EnumInfo{Form: rules.EnumIdentifier}
	if e == nil {
		return info
	}
	defaulted := derives(e.Annotations, defaultTrait)
	info.Variants = make([]rules.VariantText, 0, len(e.Variants))
	for _, variant := range e.Variants {
		if variant == nil {
			continue
		}
		info.Variants = append(info.Variants, rules.VariantText{
			Name: variant.Name, Text: emit.Literal(emit.LiteralString, variant.Name),
		})
		marksDefault := slices.ContainsFunc(variant.Annotations, func(an symbol.Annotation) bool {
			return an.Name == defaultAttr
		})
		if defaulted && info.Zero == "" && marksDefault {
			info.Zero = variant.Name
		}
	}
	return info
}
