// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The annotations that a header of the cases states: the marker of a
// syntax, the same marker with another argument, and a file option.
var (
	syntaxMarker = symbol.Annotation{Name: "protobuf:syntax", Args: []string{"proto3"}}
	proto2Marker = symbol.Annotation{Name: "protobuf:syntax", Args: []string{"proto2"}}
	goPackage    = symbol.Annotation{Name: "go_package", Args: []string{`"example.com/svc/storepb"`}}
)

// The documentation lines that a header of the cases states.
const (
	storeDoc     = "Package store serves the rows."
	generatedDoc = "The stubs of the store."
)

// spelling is a fixture target that files every unit whole and names
// each file after its file key and word.
type spelling struct {
	named
}

// SplitUnit returns the unit whole.
func (spelling) SplitUnit(u plugin.Unit) []plugin.Unit { return []plugin.Unit{u} }

// FileName joins the unit's file key and word.
func (spelling) FileName(u plugin.Unit) string { return u.FileKey() + "." + u.Word }

// TestFile pins the routed file, the header that the emit.File
// declarations of its units state, and the filename half that a target
// opts into.
func TestFile(t *testing.T) {
	t.Parallel()

	t.Run("File", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the zero package for a file no target derived a package for", func(t *testing.T) {
			t.Parallel()

			var f plugin.File
			expect.Equal(t, f.Pkg, symbol.Identity{}, "a zero file declares no package")
			expect.Empty(t, f.Units, "and assembles no unit")
		})
	})

	t.Run("HeaderOf", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give []plugin.Unit
			want emit.File
		}{
			{
				name: "returns the zero header for units without an emit.File",
				give: []plugin.Unit{headerUnit()},
			},
			{
				name: "returns the documentation and the annotations of one emit.File",
				give: []plugin.Unit{headerUnit(
					&emit.File{Doc: []string{storeDoc}, Annotations: symbol.Annotations{syntaxMarker}})},
				want: emit.File{Doc: []string{storeDoc}, Annotations: symbol.Annotations{syntaxMarker}},
			},
			{
				name: "returns a header without the path of an emit.File",
				give: []plugin.Unit{headerUnit(&emit.File{Path: "svc/store/store.proto", Doc: []string{storeDoc}})},
				want: emit.File{Doc: []string{storeDoc}},
			},
			{
				name: "returns the annotations of two emit.File declarations in unit order",
				give: []plugin.Unit{
					headerUnit(&emit.File{Annotations: symbol.Annotations{syntaxMarker}}),
					headerUnit(&emit.File{Annotations: symbol.Annotations{goPackage}}),
				},
				want: emit.File{Annotations: symbol.Annotations{syntaxMarker, goPackage}},
			},
			{
				name: "returns one copy of an annotation that two emit.File declarations state alike",
				give: []plugin.Unit{
					headerUnit(&emit.File{Annotations: symbol.Annotations{syntaxMarker}}),
					headerUnit(&emit.File{Annotations: symbol.Annotations{goPackage, syntaxMarker}}),
				},
				want: emit.File{Annotations: symbol.Annotations{syntaxMarker, goPackage}},
			},
			{
				name: "returns each annotation that one emit.File states twice",
				give: []plugin.Unit{headerUnit(&emit.File{Annotations: symbol.Annotations{goPackage, goPackage}})},
				want: emit.File{Annotations: symbol.Annotations{goPackage, goPackage}},
			},
			{
				name: "returns two annotations of one name with different arguments",
				give: []plugin.Unit{
					headerUnit(&emit.File{Annotations: symbol.Annotations{syntaxMarker}}),
					headerUnit(&emit.File{Annotations: symbol.Annotations{proto2Marker}}),
				},
				want: emit.File{Annotations: symbol.Annotations{syntaxMarker, proto2Marker}},
			},
			{
				name: "returns two annotations that differ in the assigned form",
				give: []plugin.Unit{
					headerUnit(&emit.File{Annotations: symbol.Annotations{goPackage}}),
					headerUnit(&emit.File{Annotations: symbol.Annotations{{
						Name: goPackage.Name, Args: goPackage.Args, Assigned: true,
					}}}),
				},
				want: emit.File{Annotations: symbol.Annotations{
					goPackage, {Name: goPackage.Name, Args: goPackage.Args, Assigned: true},
				}},
			},
			{
				name: "returns the documentation of two emit.File declarations in unit order",
				give: []plugin.Unit{
					headerUnit(&emit.File{Doc: []string{storeDoc}}),
					headerUnit(&emit.File{Doc: []string{generatedDoc}}),
				},
				want: emit.File{Doc: []string{storeDoc, generatedDoc}},
			},
			{
				name: "returns one copy of a documentation that two emit.File declarations state alike",
				give: []plugin.Unit{
					headerUnit(&emit.File{Doc: []string{storeDoc}}),
					headerUnit(&emit.File{Doc: []string{storeDoc}}),
				},
				want: emit.File{Doc: []string{storeDoc}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, plugin.HeaderOf(tt.give), tt.want, "HeaderOf returns the header of the units")
			})
		}

		t.Run("returns a header that writes nothing into the lists of an emit.File", func(t *testing.T) {
			t.Parallel()

			// The lists of the first emit.File end before the end of their
			// arrays, so a merge that appends to them overwrites the line
			// and the annotation after them.
			lines := []string{storeDoc, generatedDoc}
			annotations := symbol.Annotations{syntaxMarker, syntaxMarker}
			second := &emit.File{Doc: []string{storeDoc + " Again."}, Annotations: symbol.Annotations{goPackage}}
			plugin.HeaderOf([]plugin.Unit{
				headerUnit(&emit.File{Doc: lines[:1], Annotations: annotations[:1]}),
				headerUnit(second),
			})
			expect.Equal(t, lines, []string{storeDoc, generatedDoc}, "the array of the documentation")
			expect.Equal(t, annotations, symbol.Annotations{syntaxMarker, syntaxMarker}, "the array of the annotations")
		})
	})

	t.Run("FileSpeller", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name a target's speller spells", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = spelling{name: "speller"}
			s, spells := p.(plugin.FileSpeller)
			assert.True(t, spells, "the filename surface asserts")
			u := plugin.Unit{Per: plugin.PerSource, Key: "svc/store.go", Word: "stub"}
			assert.Length(t, s.SplitUnit(u), 1, "the fixture files the unit whole")
			assert.Equal(t, s.FileName(u), "svc/store.go.stub", "the name joins the file key and the word")
		})

		t.Run("reports false asserting a plugin that spells nothing", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = named{name: "bare"}
			_, spells := p.(plugin.FileSpeller)
			assert.False(t, spells, "the filename surface is opt-in")
		})
	})
}

// TestFileAllocs checks the ceilings of the header in the ordinary run,
// which runs no benchmark. The header of one emit.File shares its lists,
// and the header of two allocates the documentation and the annotations
// that it merges.
func TestFileAllocs(t *testing.T) {
	one := headerFile(1)
	var got emit.File
	assert.MaxAllocs(t, func() { got = plugin.HeaderOf(one) }, 0, "HeaderOf of one emit.File allocates nothing")
	assert.Length(t, got.Annotations, 1, "the header has the annotation of the emit.File")

	two := headerFile(2)
	assert.MaxAllocs(t, func() { got = plugin.HeaderOf(two) }, 2,
		"HeaderOf of two emit.File declarations allocates the merged lists")
	assert.Length(t, got.Annotations, 2, "the header has the annotations of both")
}

// BenchmarkFile measures the header of a file of 20 declarations of
// which the first states the header.
func BenchmarkFile(b *testing.B) {
	b.Run("HeaderOf", func(b *testing.B) {
		units := headerFile(1)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got emit.File
		for c.Loop() {
			got = plugin.HeaderOf(units)
		}
		assert.Length(b, got.Annotations, 1, "the header has the annotation of the emit.File")
	})
}

// headerUnit returns a unit of the emit.File declarations of headers,
// followed by one struct.
func headerUnit(headers ...*emit.File) plugin.Unit {
	decls := make([]symbol.Symbol, 0, len(headers)+1)
	for _, h := range headers {
		decls = append(decls, h)
	}
	return plugin.Unit{Decls: append(decls, &emit.Struct{Name: "Store"})}
}

// headerFile returns the n units of a file of 20 declarations. Each unit
// starts with an emit.File that states a documentation and an annotation
// of its own.
func headerFile(n int) []plugin.Unit {
	units := make([]plugin.Unit, 0, n)
	for i := range n {
		own := string(rune('a' + i))
		u := headerUnit(&emit.File{
			Doc: []string{own}, Annotations: symbol.Annotations{{Name: "option", Args: []string{own}}},
		})
		for range 20/n - 1 {
			u.Decls = append(u.Decls, &emit.Struct{Name: "Row"})
		}
		units = append(units, u)
	}
	return units
}
