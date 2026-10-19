// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// importName is the name of the header marker of an import.
const importName = string(protobuf.ImportMarker)

// fileOptionsAllocs is the options of a header with two options: the sorted
// options and the statements.
const fileOptionsAllocs = 2

// optionsFile is a header of proto3 with two options, which the allocation
// cases write.
var optionsFile = &emit.File{Annotations: symbol.Annotations{
	{Name: syntaxName, Args: []string{protobuf.Proto3}},
	{Name: importName, Args: []string{typesProto}},
	{Name: "(acme.file)", Args: []string{trueValue}},
	{Name: goPackageOption, Args: []string{`"x"`}},
}}

// optionsText is the options of optionsFile.
const optionsText = "option go_package = \"x\";\noption (acme.file) = true;\n"

// TestHeader pins the parts of the file skeleton that the header states:
// the version statement, the imports and the file options.
func TestHeader(t *testing.T) {
	t.Parallel()

	t.Run("Syntax", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give *emit.File
			want string
		}{
			{
				name: "returns the syntax statement of proto3 for a header without a version",
				give: &emit.File{},
				want: `syntax = "proto3";`,
			},
			{name: "returns the syntax statement of proto2", give: proto2File, want: `syntax = "proto2";`},
			{name: "returns the edition statement of edition 2023", give: edition2023File, want: `edition = "2023";`},
			{name: "returns the edition statement of edition 2024", give: edition2024File, want: `edition = "2024";`},
			{name: "returns the edition statement of edition 2026", give: edition2026File, want: `edition = "2026";`},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Syntax(tt.give)
				assert.NoError(t, err, "the header states a version")
				assert.Equal(t, got, tt.want, "the version statement")
			})
		}

		t.Run("returns an error for a header without a version of protobuf", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Syntax(
				&emit.File{Annotations: symbol.Annotations{{Name: syntaxName, Args: []string{"2025"}}}},
			)
			assert.HasError(t, err, "protobuf has no version 2025")
			assert.Equal(t, err.Error(), "protobuf: the header states the version 2025, which protobuf does not have",
				"the error states the version")
		})
	})

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("UseImports", func(t *testing.T) {
			t.Parallel()

			t.Run("returns nothing and records each import with its modifier", func(t *testing.T) {
				t.Parallel()

				set := &render.ImportSet{}
				got, err := backend.NewVocabulary(set).UseImports(&emit.File{Annotations: symbol.Annotations{
					{Name: syntaxName, Args: []string{protobuf.Edition2024}},
					{Name: importName, Args: []string{typesProto}},
					{Name: importName, Args: []string{"public " + commonProto}},
					{Name: importName, Args: []string{"option " + rulesProto}},
					{Name: importName},
					{Name: goPackageOption, Args: []string{`"x"`}},
				}})
				assert.NoError(t, err, "edition 2024 has every import of the header")
				assert.Empty(t, got, "the helper writes nothing")
				assert.Equal(t, set.Entries(), []render.Entry{
					{Path: commonProto, Name: publicMod}, {Path: rulesProto, Name: optionMod}, {Path: typesProto},
				}, "the imports of the header", assert.EquateEmpty())
			})

			refusals := []struct {
				name string
				give symbol.Annotations
				want string
			}{
				{
					name: "returns an error for a weak import of edition 2024",
					give: symbol.Annotations{
						{
							Name: syntaxName,
							Args: []string{protobuf.Edition2024},
						},
						{Name: importName, Args: []string{"weak " + legacyProto}},
					},
					want: "protobuf: edition 2024 removes the weak import, and the header imports acme/legacy.proto weak",
				},
				{
					name: "returns an error for an option import of proto3",
					give: symbol.Annotations{{Name: importName, Args: []string{"option " + rulesProto}}},
					want: "protobuf: proto3 has no option import, and the header imports acme/rules.proto for its options",
				},
				{
					name: "returns an error for a file that the header imports public and weak",
					give: symbol.Annotations{
						{Name: syntaxName, Args: []string{protobuf.Proto2}},
						{Name: goPackageOption, Args: []string{`"x"`}},
						{Name: importName},
						{Name: importName, Args: []string{"public " + typesProto}},
						{Name: importName, Args: []string{"weak " + typesProto}},
					},
					want: "protobuf: the header imports acme/types.proto as public and as weak, and an import has one modifier",
				},
				{
					name: "returns an error for a header without a version of protobuf",
					give: symbol.Annotations{{Name: syntaxName, Args: []string{"2025"}}},
					want: "protobuf: the header states the version 2025, which protobuf does not have",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					_, err := backend.NewVocabulary(&render.ImportSet{}).UseImports(&emit.File{Annotations: tt.give})
					assert.HasError(t, err, "protobuf does not write the imports")
					assert.Equal(t, err.Error(), tt.want, "the error states the import")
				})
			}

			t.Run("returns no error for a file that the header imports twice with one modifier", func(t *testing.T) {
				t.Parallel()

				set := &render.ImportSet{}
				_, err := backend.NewVocabulary(set).UseImports(&emit.File{Annotations: symbol.Annotations{
					{
						Name: importName,
						Args: []string{typesProto},
					},
					{Name: importName, Args: []string{"public " + typesProto}},
				}})
				assert.NoError(t, err, "a plain import and a public import of one file agree")
				assert.Equal(
					t,
					backend.Imports(set),
					"import public \""+typesProto+"\";\n",
					"the file takes the public import",
				)
			})
		})
	})

	t.Run("FileOptions", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Annotations
			want string
		}{
			{
				name: "returns nothing for a header without an option",
				give: symbol.Annotations{{Name: syntaxName, Args: []string{protobuf.Proto3}}},
			},
			{
				name: "returns the plain options and then the custom options in the order of their names",
				give: symbol.Annotations{
					{Name: syntaxName, Args: []string{protobuf.Proto3}},
					{Name: "java_package", Args: []string{`"y"`}},
					{Name: "(z.opt)", Args: []string{"1"}},
					{Name: goPackageOption, Args: []string{`"x"`}},
					{Name: "(a.opt)", Args: []string{"2"}},
					{Name: "(a.opt)", Args: []string{"3"}},
				},
				want: "option go_package = \"x\";\noption java_package = \"y\";\noption (a.opt) = 2;\noption (a.opt) = 3;\n" +
					"option (z.opt) = 1;\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, backend.FileOptions(&emit.File{Annotations: tt.give}), tt.want, "the file options")
			})
		}
	})
}

// Syntax writes a version statement without allocating, UseImports records
// the imports of a header into a set that holds them without allocating,
// and FileOptions allocates the sorted options and their statements. The
// ordinary run, which runs no benchmark, checks those ceilings here.
func TestHeaderAllocs(t *testing.T) {
	var (
		got string
		err error
	)
	assert.MaxAllocs(t, func() { got, err = backend.Syntax(edition2024File) }, 0, "Syntax allocates nothing")
	assert.NoError(t, err, "Syntax reads the header")
	assert.Equal(t, got, `edition = "2024";`, "Syntax writes the edition statement")

	set := &render.ImportSet{}
	v := backend.NewVocabulary(set)
	assert.MaxAllocs(t, func() { got, err = v.UseImports(optionsFile) }, 0,
		"UseImports allocates nothing for imports that the set holds")
	assert.NoError(t, err, "UseImports reads the header")
	assert.Equal(t, set.Paths(), []string{typesProto}, "UseImports records the import")

	assert.MaxAllocs(t, func() { got = backend.FileOptions(optionsFile) }, fileOptionsAllocs,
		"FileOptions allocates the sorted options and the statements")
	assert.Equal(t, got, optionsText, "FileOptions writes the options")
}

// BenchmarkHeader measures the version statement, the imports and the
// options that every file writes.
func BenchmarkHeader(b *testing.B) {
	b.Run("Syntax", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			got string
			err error
		)
		for c.Loop() {
			got, err = backend.Syntax(edition2024File)
		}
		assert.NoError(b, err, "Syntax reads the header")
		assert.Equal(b, got, `edition = "2024";`, "Syntax writes the edition statement")
	})

	b.Run("Vocabulary", func(b *testing.B) {
		b.Run("UseImports", func(b *testing.B) {
			v := backend.NewVocabulary(&render.ImportSet{})
			c := bench.Start(b).Warmup(1).MaxAllocs(0)
			defer c.End()
			var err error
			for c.Loop() {
				_, err = v.UseImports(optionsFile)
			}
			assert.NoError(b, err, "UseImports reads the header")
		})
	})

	b.Run("FileOptions", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(fileOptionsAllocs)
		defer c.End()
		var got string
		for c.Loop() {
			got = backend.FileOptions(optionsFile)
		}
		assert.Equal(b, got, optionsText, "FileOptions writes the options")
	})
}
