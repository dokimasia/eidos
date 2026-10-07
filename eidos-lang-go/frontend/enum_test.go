// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The schema names a Go constant group an enum, so the promotion
// is pinned: what collapses, what moves onto the enum, and what is
// kept exactly as parsed.
func TestPromoteEnums(t *testing.T) {
	t.Parallel()

	t.Run("collapses a typed group into the enum", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\n// Color is a palette.\ntype Color int\n\n"+
				"const (\n\tRed Color = iota // leads\n\tGreen\n\tBlue\n)\n\n"+
				"func (c Color) String() string { return \"\" }\n"))
		assert.Length(t, file.Decls, 1, "type, constants and method fold into one")
		enum := file.Decls[0].(*node.Enum)
		assert.Equal(t, enum.Name, "Color", "under the type's name")
		assert.Equal(t, enum.Doc, []string{"Color is a palette."}, "with its doc")
		assert.Length(t, enum.Variants, 3, "every constant is a variant")
		assert.Equal(t, enum.Variants[0].Value, "iota", "values verbatim")
		assert.Equal(t, enum.Variants[0].Comment, "leads",
			"the trailing comment follows its constant into the variant")
		assert.Equal(t, enum.Variants[1].Value, "", "an implicit row's value is empty")
		assert.Length(t, enum.Methods, 1, "the method set moves onto the enum")
		assert.Equal(t, enum.Methods[0].Name, "String", "by name")
	})

	t.Run("re-homes the underlying stamp onto the enum", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype Color int\n\nconst (\n\tRed Color = iota\n)\n")
		records := gb.StampRecords()
		at := slices.IndexFunc(records, func(s plugin.StampRecord) bool {
			return string(s.Stamp.Key) == "golang.underlyingKind"
		})
		assert.NotEqual(t, at, -1, "the underlying stamp survives the promotion")
		_, onEnum := records[at].Subject.(*node.Enum)
		assert.True(t, onEnum, "the shape is on the enum that replaces the type")
		assert.Equal(t, records[at].Stamp.Value, any("basic"), "the shape is the underlying's")
	})

	t.Run("re-homes carriers onto the enum and its variants", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\n// Color is a palette.\n// +fixture:gen:stringer\ntype Color int\n\n"+
				"const (\n\t// Red leads.\n\t// +fixture:gen:mark\n\tRed Color = iota\n\tGreen\n)\n")
		attached := gb.Attachments()
		assert.Length(t, attached, 2, "both carriers survive the promotion")
		var onEnum, onVariant bool
		for _, a := range attached {
			switch subject := a.Subject.(type) {
			case *node.Enum:
				onEnum = subject.Name == "Color"
			case *node.EnumVariant:
				onVariant = subject.Name == "Red"
			}
		}
		assert.True(t, onEnum, "the type's carrier follows the enum that replaces it")
		assert.True(t, onVariant, "the constant's carrier follows its variant")
	})

	t.Run("promotes only what the idiom states", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype Plain int\n\ntype Wide struct{}\n\nconst loose = 1\n"))
		kinds := map[symbol.Kind]int{}
		for _, d := range file.Decls {
			kinds[d.Kind()]++
		}
		assert.Equal(t, kinds[symbol.KindAlias], 1, "a type without constants is a defined type")
		assert.Equal(t, kinds[symbol.KindStruct], 1, "a struct is never a value set")
		assert.Equal(t, kinds[symbol.KindConstant], 1, "an untyped constant is a constant")
		assert.Equal(t, kinds[symbol.KindEnum], 0, "nothing promotes without the pairing")
	})

	t.Run("keeps the constants of a type another file declares", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\nconst Max Elsewhere = 3\n"))
		_, kept := file.Decls[0].(*node.Constant)
		assert.True(t, kept, "promotion is per file, and the type is not here")
	})
}
