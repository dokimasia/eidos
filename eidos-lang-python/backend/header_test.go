// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The allocations of the head of a file: its text, and the list of __all__.
const (
	headAllocs = 1
	allAllocs  = 1
)

// The header markers that the specs of the header state: the lazy import of
// json, and the all marker of Store and Row.
var (
	lazyJSON  = symbol.Annotation{Name: string(python.LazyMarker), Args: []string{"json"}}
	allMarker = symbol.Annotation{Name: string(python.AllMarker), Args: []string{"Store", rowName}}
)

// TestHeader pins the head of a module, its __all__, and the record of the
// public names that __all__ lists.
func TestHeader(t *testing.T) {
	t.Parallel()

	t.Run("Writer", func(t *testing.T) {
		t.Parallel()

		t.Run("Head", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name        string
				give        *emit.File
				giveVersion plugin.Choice
				want        string
				wantImports []render.Entry
			}{
				{name: "returns nothing for no header", giveVersion: python.Version311},
				{
					name: "returns the comments of the head marker, each after a hash",
					give: &emit.File{Annotations: symbol.Annotations{{
						Name: string(python.HeadMarker), Args: []string{"!/usr/bin/env python3", " Copyright 2026"},
					}}},
					giveVersion: python.Version311, want: "#!/usr/bin/env python3\n# Copyright 2026\n",
				},
				{
					name:        "returns the docstring of the header",
					give:        &emit.File{Doc: []string{"Shop models."}},
					giveVersion: python.Version311, want: "\"\"\"Shop models.\"\"\"\n",
				},
				{
					name: "returns the comments before the docstring",
					give: &emit.File{Doc: []string{"Shop models."}, Annotations: symbol.Annotations{{
						Name: string(python.HeadMarker), Args: []string{" generated"},
					}}},
					giveVersion: python.Version311, want: "# generated\n\"\"\"Shop models.\"\"\"\n",
				},
				{
					name: "returns nothing for the future marker and records the import of each feature",
					give: &emit.File{Annotations: symbol.Annotations{{
						Name: string(python.FutureMarker), Args: []string{"annotations"},
					}}},
					giveVersion: python.Version311,
					wantImports: []render.Entry{{Path: "__future__", Name: "annotations"}},
				},
				{
					name:        "returns nothing for the import marker and binds its imports",
					give:        &emit.File{Annotations: symbol.Annotations{jsonImport}},
					giveVersion: python.Version311, wantImports: []render.Entry{{Path: "json"}},
				},
				{
					name:        "returns nothing for the lazy marker from Python 3.15",
					give:        &emit.File{Annotations: symbol.Annotations{lazyJSON}},
					giveVersion: python.Version315,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, tt.giveVersion, false)
					got, err := w.Head(tt.give)
					assert.NoError(t, err, "Python writes the head")
					assert.Equal(t, got, tt.want, "the head")
					assert.Equal(t, importsOf(set), tt.wantImports, "the imports of the head", assert.EquateEmpty())
				})
			}

			refusals := []struct {
				name        string
				give        *emit.File
				giveVersion plugin.Choice
				want        string
			}{
				{
					name:        "returns an error for an annotation of a header that is no marker",
					give:        &emit.File{Annotations: symbol.Annotations{{Name: "dataclass"}}},
					giveVersion: python.Version311,
					want:        "python: a Python file states the header markers alone, and its header states dataclass",
				},
				{
					name:        "returns an error for a marker that is no marker of a header",
					give:        &emit.File{Annotations: symbol.Annotations{typedDictMarker}},
					giveVersion: python.Version311,
					want: "python: a Python file states the header markers alone, " +
						"and its header states python:typeddict",
				},
				{
					name:        "returns an error for the lazy marker before Python 3.15",
					give:        &emit.File{Annotations: symbol.Annotations{lazyJSON}},
					giveVersion: python.Version314,
					want:        "python: lazy imports are a construct of Python 3.15, and the file runs on Python 3.14",
				},
				{
					name:        "returns the error of an import of the header",
					give:        &emit.File{Annotations: symbol.Annotations{badImport}},
					giveVersion: python.Version311,
					want:        badImportRefusal,
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, tt.giveVersion, false)
					_, err := w.Head(tt.give)
					assert.HasError(t, err, "Python refuses the header")
					assert.Equal(t, err.Error(), tt.want, "the refusal names the annotation")
				})
			}
		})

		t.Run("All", func(t *testing.T) {
			t.Parallel()

			// hidden is a public declaration whose name starts with an
			// underscore, so __all__ lists the public names of the file, and
			// hiddenAll is the __all__ that lists it alone.
			hidden := emit.Constant{Name: "_Row", Visibility: symbol.VisibilityPublic}
			hiddenAll := "__all__ = [" + listMark + "\"_Row\"" + closeMark + "]\n"
			tests := []struct {
				name        string
				give        *emit.File
				giveDeclare []emit.Constant
				giveNested  string
				want        string
			}{
				{name: "returns nothing for a file without a header and without public names"},
				{
					name: "returns the names of the all marker in their order",
					give: &emit.File{Annotations: symbol.Annotations{allMarker}},
					want: "__all__ = [" + listMark + "\"Store\"" + sepMark + "\"Row\"" + closeMark + "]\n",
				},
				{
					name: "returns the empty list for the all marker without names",
					give: &emit.File{Annotations: symbol.Annotations{{Name: string(python.AllMarker)}}},
					want: "__all__ = [" + listMark + closeMark + "]\n",
				},
				{
					name: "returns an escaped string for a name of the all marker that is no identifier",
					give: &emit.File{Annotations: symbol.Annotations{{
						Name: string(python.AllMarker), Args: []string{`row"s`},
					}}},
					want: "__all__ = [" + listMark + "'row\"s'" + closeMark + "]\n",
				},
				{
					name:        "returns nothing where no public name starts with an underscore",
					giveDeclare: []emit.Constant{{Name: rowName}, {Name: "load"}},
				},
				{
					name: "returns the public names in the order of isort where one starts with an underscore",
					giveDeclare: []emit.Constant{
						{Name: "load"}, hidden, {Name: "Store"}, {Name: "LIMIT"},
					},
					want: "__all__ = [" + listMark + "\"LIMIT\"" + sepMark + "\"Store\"" + sepMark + "\"_Row\"" + sepMark +
						"\"load\"" + closeMark + "]\n",
				},
				{
					name:        "returns no private declaration among the public names",
					giveDeclare: []emit.Constant{hidden, {Name: "load", Visibility: symbol.VisibilityPrivate}},
					want:        hiddenAll,
				},
				{
					name:        "returns no special name among the public names",
					giveDeclare: []emit.Constant{hidden, {Name: "__version__", Visibility: symbol.VisibilityPublic}},
					want:        hiddenAll,
				},
				{
					name:        "returns no nested type among the public names",
					giveDeclare: []emit.Constant{hidden},
					giveNested:  "Inner",
					want:        hiddenAll,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, python.Version311, false)
					for _, c := range tt.giveDeclare {
						set.Reserve(c.Name)
						c.Value = "8"
						_, err := w.Constant(nil, &c)
						assert.NoError(t, err, "the declaration records its name")
					}
					if tt.giveNested != "" {
						_, err := w.Struct(nil, &emit.Struct{Name: tt.giveNested})
						assert.NoError(t, err, "the nested type renders")
					}
					assert.Equal(t, w.All(tt.give), tt.want, "the __all__ of the file")
				})
			}
		})
	})
}

// Head allocates its text, and All its list. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestHeaderAllocs(t *testing.T) {
	checkAllocs(t, headerCalls(t))
}

// BenchmarkHeader measures the head and the __all__ of a module.
func BenchmarkHeader(b *testing.B) {
	b.Run("Writer", func(b *testing.B) {
		benchCalls(b, headerCalls(b))
	})
}

// headerCalls returns the calls of Head for a header with a comment and a
// docstring, and of All for a header with the all marker.
func headerCalls(tb assert.TB) []allocCall {
	w, _ := newWriter(tb, python.Version311, false)
	head := &emit.File{Doc: []string{"Shop models."}, Annotations: symbol.Annotations{{
		Name: string(python.HeadMarker), Args: []string{" generated"},
	}}}
	all := &emit.File{Annotations: symbol.Annotations{allMarker}}
	var got string
	return []allocCall{
		{
			name: "Head", allocs: headAllocs,
			call: func() { got, _ = w.Head(head) },
			check: func(tb assert.TB) {
				assert.Equal(tb, got, "# generated\n\"\"\"Shop models.\"\"\"\n", "Head writes the head")
			},
		},
		{
			name: "All", allocs: allAllocs,
			call: func() { got = w.All(all) },
			check: func(tb assert.TB) {
				assert.Equal(tb, got, "__all__ = ["+listMark+"\"Store\""+sepMark+"\"Row\""+closeMark+"]\n",
					"All writes the list")
			},
		},
	}
}
