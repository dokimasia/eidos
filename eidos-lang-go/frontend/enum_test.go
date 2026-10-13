// Copyright Dokimasia B.V. 2026
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

// TestPromoteEnums pins what the promotion collapses, what it moves onto
// the enum and what it keeps as parsed.
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

	t.Run("sets the target of the defined type as the underlying type of the enum", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype Level uint8\n\nconst (\n\tLow Level = iota\n\tHigh\n)\n"))
		enum, promoted := file.Decls[0].(*node.Enum)
		assert.True(t, promoted, "the defined type and its constants promote to an enum")
		assert.NotNil(t, enum.Underlying, "the enum has an underlying type")
		assert.Equal(t, enum.Underlying.Spelling, "uint8", "the underlying type is the target of the defined type")
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

	t.Run("promotes only a defined type with typed constants", func(t *testing.T) {
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

	t.Run("keeps a constant whose type the package does not declare", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\nconst Max Elsewhere = 3\n"))
		_, kept := file.Decls[0].(*node.Constant)
		assert.True(t, kept, "the constant stays a constant")
	})

	t.Run("collects the constants of the type from every file of the package", func(t *testing.T) {
		t.Parallel()

		files := parsedUnit(t, map[string]string{
			"p/a.go": "package p\n\ntype Color int\n\nconst Red Color = 1\n",
			"p/b.go": "package p\n\n// Blue is cold.\nconst Blue Color = 2\n",
		})
		enum, promoted := files[0].Decls[0].(*node.Enum)
		assert.True(t, promoted, "the type promotes")
		assert.Equal(t, variantNames(enum), []string{"Red", "Blue"},
			"the variants are in the order of the files and their declarations")
		assert.Equal(t, enum.Variants[1].Doc, []string{"Blue is cold."}, "a variant of another file keeps its doc")
		assert.Empty(t, files[1].Decls, "the constant leaves its file")
	})

	t.Run("promotes a type whose constants are all in another file", func(t *testing.T) {
		t.Parallel()

		files := parsedUnit(t, map[string]string{
			"p/a.go": "package p\n\ntype Mode int\n",
			"p/b.go": "package p\n\nconst (\n\tModeRead Mode = iota\n\tModeWrite\n)\n",
		})
		enum, promoted := files[0].Decls[0].(*node.Enum)
		assert.True(t, promoted, "the type promotes")
		assert.Equal(t, variantNames(enum), []string{"ModeRead", "ModeWrite"}, "both constants are variants")
	})

	t.Run("promotes a defined type over bool", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype Switch bool\n\nconst (\n\tOn Switch = true\n\tOff Switch = false\n)\n"))
		enum, promoted := file.Decls[0].(*node.Enum)
		assert.True(t, promoted, "the type promotes")
		assert.Equal(t, enum.Underlying.Spelling, "bool", "the underlying type is bool")
	})

	t.Run("promotes a defined type over another defined type", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\ntype Base int\n\ntype Level Base\n\nconst Low Level = 1\n"))
		_, base := file.Decls[0].(*node.Alias)
		assert.True(t, base, "a type without constants stays a defined type")
		enum, promoted := file.Decls[1].(*node.Enum)
		assert.True(t, promoted, "the type with a constant promotes")
		assert.Equal(t, enum.Underlying.Spelling, "Base", "the underlying type is the other defined type")
	})

	t.Run("keeps a constant of a test file out of a type of another file", func(t *testing.T) {
		t.Parallel()

		files := parsedUnit(t, map[string]string{
			"p/a.go":      "package p\n\ntype Color int\n\nconst Red Color = 1\n",
			"p/a_test.go": "package p\n\nconst extra Color = 9\n",
		})
		enum, promoted := files[0].Decls[0].(*node.Enum)
		assert.True(t, promoted, "the type promotes")
		assert.Equal(t, variantNames(enum), []string{"Red"}, "the constant of the test file is no variant")
		_, kept := files[1].Decls[0].(*node.Constant)
		assert.True(t, kept, "the constant of the test file stays a constant")
	})

	t.Run("collects the constants of test files into a type of a test file", func(t *testing.T) {
		t.Parallel()

		files := parsedUnit(t, map[string]string{
			"p/a_test.go": "package p\n\ntype probe int\n\nconst first probe = 1\n",
			"p/b_test.go": "package p\n\nconst second probe = 2\n",
		})
		enum, promoted := files[0].Decls[0].(*node.Enum)
		assert.True(t, promoted, "the type of the test file promotes")
		assert.Equal(t, variantNames(enum), []string{"first", "second"}, "both constants are variants")
	})

	unpromoted := []struct {
		name string
		give string
	}{
		{
			name: "promotes no generic type",
			give: "package p\n\ntype Opt[T any] int\n\nconst None Opt[int] = 0\n",
		},
		{
			name: "promotes no type over a composite target",
			give: "package p\n\ntype List []int\n\nconst Empty List = nil\n",
		},
		{
			name: "promotes no type over an instantiated target",
			give: "package p\n\ntype Box[T any] int\n\ntype Ints Box[int]\n\nconst Zero Ints = 0\n",
		},
	}
	for _, tt := range unpromoted {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull, tt.give))
			assert.False(t, slices.ContainsFunc(file.Decls, func(d symbol.Symbol) bool {
				return d.Kind() == symbol.KindEnum
			}), "nothing promotes")
		})
	}
}

// variantNames returns the names of the variants of an enumeration, in
// order.
func variantNames(e *node.Enum) []string {
	names := make([]string, 0, len(e.Variants))
	for _, v := range e.Variants {
		names = append(names, v.Name)
	}
	return names
}
