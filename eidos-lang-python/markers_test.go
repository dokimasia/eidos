// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// decorator is the name of a decorator, which no marker has.
const decorator python.Marker = "dataclass"

// TestMarkers pins the spelling of each marker and the markers of the header,
// because a generator outside this module states them in the emit model.
func TestMarkers(t *testing.T) {
	t.Parallel()

	t.Run("Marker", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give python.Marker
			want string
		}{
			{name: "spells the TypedDict marker", give: python.TypedDictMarker, want: "python:typeddict"},
			{name: "spells the bases marker", give: python.BasesMarker, want: "python:bases"},
			{name: "spells the keywords marker", give: python.KeywordsMarker, want: "python:keywords"},
			{name: "spells the annotated marker", give: python.AnnotatedMarker, want: "python:annotated"},
			{name: "spells the InitVar marker", give: python.InitVarMarker, want: "python:initvar"},
			{name: "spells the all marker", give: python.AllMarker, want: "python:all"},
			{name: "spells the lazy marker", give: python.LazyMarker, want: "python:lazy"},
			{name: "spells the future marker", give: python.FutureMarker, want: "python:future"},
			{name: "spells the head marker", give: python.HeadMarker, want: "python:head"},
			{name: "spells the import marker", give: python.ImportMarker, want: "python:import"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, string(tt.give), tt.want, "the name of the annotation of the marker")
			})
		}

		t.Run("Valid", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give python.Marker
				want bool
			}{
				{name: "reports true for the TypedDict marker", give: python.TypedDictMarker, want: true},
				{name: "reports true for the bases marker", give: python.BasesMarker, want: true},
				{name: "reports true for the keywords marker", give: python.KeywordsMarker, want: true},
				{name: "reports true for the annotated marker", give: python.AnnotatedMarker, want: true},
				{name: "reports true for the InitVar marker", give: python.InitVarMarker, want: true},
				{name: "reports true for the all marker", give: python.AllMarker, want: true},
				{name: "reports true for the lazy marker", give: python.LazyMarker, want: true},
				{name: "reports true for the future marker", give: python.FutureMarker, want: true},
				{name: "reports true for the head marker", give: python.HeadMarker, want: true},
				{name: "reports true for the import marker", give: python.ImportMarker, want: true},
				{name: "reports false for a name with the prefix that no marker has", give: "python:protocol"},
				{name: "reports false for the prefix alone", give: python.MarkerPrefix},
				{name: "reports false for the name of a decorator", give: decorator},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.Valid(), tt.want, "Valid reports whether the satellite declares the marker")
				})
			}
		})

		t.Run("Header", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give python.Marker
				want bool
			}{
				{name: "reports true for the all marker", give: python.AllMarker, want: true},
				{name: "reports true for the lazy marker", give: python.LazyMarker, want: true},
				{name: "reports true for the future marker", give: python.FutureMarker, want: true},
				{name: "reports true for the head marker", give: python.HeadMarker, want: true},
				{name: "reports true for the import marker", give: python.ImportMarker, want: true},
				{name: "reports false for the TypedDict marker", give: python.TypedDictMarker},
				{name: "reports false for the bases marker", give: python.BasesMarker},
				{name: "reports false for the keywords marker", give: python.KeywordsMarker},
				{name: "reports false for the annotated marker", give: python.AnnotatedMarker},
				{name: "reports false for the InitVar marker", give: python.InitVarMarker},
				{name: "reports false for the name of a decorator", give: decorator},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.Header(), tt.want,
						"Header reports whether the marker is in the head of a module")
				})
			}
		})

		t.Run("Marks", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name     string
				give     python.Marker
				giveKind symbol.Kind
				want     bool
			}{
				{
					name: "reports true for the TypedDict marker of an interface",
					give: python.TypedDictMarker, giveKind: symbol.KindInterface, want: true,
				},
				{
					name: "reports false for the TypedDict marker of a struct",
					give: python.TypedDictMarker, giveKind: symbol.KindStruct,
				},
				{
					name: "reports true for the bases marker of an enum",
					give: python.BasesMarker, giveKind: symbol.KindEnum, want: true,
				},
				{
					name: "reports false for the bases marker of a struct",
					give: python.BasesMarker, giveKind: symbol.KindStruct,
				},
				{
					name: "reports true for the keywords marker of a struct",
					give: python.KeywordsMarker, giveKind: symbol.KindStruct, want: true,
				},
				{
					name: "reports true for the keywords marker of an interface",
					give: python.KeywordsMarker, giveKind: symbol.KindInterface, want: true,
				},
				{
					name: "reports true for the keywords marker of an enum",
					give: python.KeywordsMarker, giveKind: symbol.KindEnum, want: true,
				},
				{
					name: "reports false for the keywords marker of a function",
					give: python.KeywordsMarker, giveKind: symbol.KindFunction,
				},
				{
					name: "reports true for the annotated marker of a field",
					give: python.AnnotatedMarker, giveKind: symbol.KindField, want: true,
				},
				{
					name: "reports true for the annotated marker of a parameter",
					give: python.AnnotatedMarker, giveKind: symbol.KindParam, want: true,
				},
				{
					name: "reports true for the annotated marker of a variable",
					give: python.AnnotatedMarker, giveKind: symbol.KindVariable, want: true,
				},
				{
					name: "reports true for the annotated marker of a constant",
					give: python.AnnotatedMarker, giveKind: symbol.KindConstant, want: true,
				},
				{
					name: "reports true for the annotated marker of an alias",
					give: python.AnnotatedMarker, giveKind: symbol.KindAlias, want: true,
				},
				{
					name: "reports false for the annotated marker of a method",
					give: python.AnnotatedMarker, giveKind: symbol.KindMethod,
				},
				{
					name: "reports true for the InitVar marker of a field",
					give: python.InitVarMarker, giveKind: symbol.KindField, want: true,
				},
				{
					name: "reports false for the InitVar marker of a variable",
					give: python.InitVarMarker, giveKind: symbol.KindVariable,
				},
				{
					name: "reports true for a header marker of a file",
					give: python.ImportMarker, giveKind: symbol.KindFile, want: true,
				},
				{
					name: "reports false for a header marker of a struct",
					give: python.AllMarker, giveKind: symbol.KindStruct,
				},
				{name: "reports false for the name of a decorator", give: decorator, giveKind: symbol.KindStruct},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.Marks(tt.giveKind), tt.want,
						"Marks reports whether the marker marks a declaration of the kind")
				})
			}
		})
	})
}

// TestMarkersAllocs checks that the two checks of a marker do not allocate,
// in the ordinary run, which does not run benchmarks.
func TestMarkersAllocs(t *testing.T) {
	t.Run("Marker", func(t *testing.T) {
		t.Run("Valid", func(t *testing.T) {
			m := python.HeadMarker
			valid := false
			assert.MaxAllocs(t, func() { valid = m.Valid() }, 0, "Valid does not allocate")
			assert.True(t, valid, "the head marker is valid")
		})

		t.Run("Header", func(t *testing.T) {
			m := python.HeadMarker
			header := false
			assert.MaxAllocs(t, func() { header = m.Header() }, 0, "Header does not allocate")
			assert.True(t, header, "the head marker is in the head of a module")
		})

		t.Run("Marks", func(t *testing.T) {
			m := python.ImportMarker
			marks := false
			assert.MaxAllocs(t, func() { marks = m.Marks(symbol.KindFile) }, 0, "Marks does not allocate")
			assert.True(t, marks, "the import marker marks a file")
		})
	})
}

// BenchmarkMarkers measures the checks of the last marker of each switch.
func BenchmarkMarkers(b *testing.B) {
	b.Run("Marker", func(b *testing.B) {
		b.Run("Valid", func(b *testing.B) {
			m := python.HeadMarker
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			valid := false
			for c.Loop() {
				valid = m.Valid()
			}
			assert.True(b, valid, "the head marker is valid")
		})

		b.Run("Header", func(b *testing.B) {
			m := python.HeadMarker
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			header := false
			for c.Loop() {
				header = m.Header()
			}
			assert.True(b, header, "the head marker is in the head of a module")
		})

		b.Run("Marks", func(b *testing.B) {
			m := python.ImportMarker
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			marks := false
			for c.Loop() {
				marks = m.Marks(symbol.KindFile)
			}
			assert.True(b, marks, "the import marker marks a file")
		})
	})
}
