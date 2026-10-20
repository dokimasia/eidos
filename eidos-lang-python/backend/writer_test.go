// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The allocations of the writer: the writer, and nothing for the imports of
// a header that the file has bound before.
const (
	newWriterAllocs = 1
	bindAllocs      = 0
)

// TestWriter pins the writer of a module and of a stub file, and the binding
// of the import statements of the header of a file.
func TestWriter(t *testing.T) {
	t.Parallel()

	t.Run("NewWriter", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			giveStub bool
			want     string
		}{
			{
				name: "returns the writer of a module, which writes the body of a callable",
				want: "def load() -> None:\n    return\n",
			},
			{
				name: "returns the writer of a stub file, which writes the body ...", giveStub: true,
				want: "def load() -> None: ...\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				w := backend.NewWriter(&render.ImportSet{}, tt.giveStub)
				got, err := w.Function(nil, &emit.Function{Name: "load"}, "    return\n")
				assert.NoError(t, err, "the writer writes the function")
				assert.Equal(t, got, tt.want, "the function")
			})
		}

		t.Run("returns a writer for Python 3.11 under a policy that does not declare the version", func(t *testing.T) {
			t.Parallel()

			w := backend.NewWriter(&render.ImportSet{}, false)
			got, err := w.Alias(nil, &emit.Alias{Name: "Id", Target: strRef})
			assert.NoError(t, err, "the writer writes the alias")
			assert.Equal(t, got, "Id: TypeAlias = str\n", "the alias of Python 3.11")
		})
	})

	t.Run("Writer", func(t *testing.T) {
		t.Parallel()

		t.Run("Bind", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name        string
				give        []string
				giveVersion plugin.Choice
				want        []render.Entry
			}{
				{name: "binds nothing for a header without the import marker"},
				{
					name: "binds an import of a module",
					give: []string{"import json"}, want: []render.Entry{{Path: "json"}},
				},
				{
					name: "binds an import of a module of a package at its package path",
					give: []string{"import os.path"}, want: []render.Entry{{Path: "os/path"}},
				},
				{
					name: "binds an import of a name from a module",
					give: []string{"from decimal import Decimal"},
					want: []render.Entry{{Path: "decimal", Name: "Decimal"}},
				},
				{
					name: "binds an import of a name under its alias",
					give: []string{"from decimal import Decimal as D"},
					want: []render.Entry{{Path: "decimal", Name: "D", Item: "Decimal"}},
				},
				{
					name: "binds a star import",
					give: []string{"from os import *"}, want: []render.Entry{{Path: "os", Name: python.StarName}},
				},
				{
					name: "binds a name of typing that the version does not have from typing_extensions",
					give: []string{"from typing import override"}, giveVersion: python.Version311,
					want: []render.Entry{{Path: "typing_extensions", Name: "override"}},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, cmp.Or(tt.giveVersion, python.Version312), false)
					h := &emit.File{}
					if tt.give != nil {
						h.Annotations = symbol.Annotations{{Name: string(python.ImportMarker), Args: tt.give}}
					}
					got, err := w.Bind(h)
					assert.NoError(t, err, "the header binds its imports")
					assert.Empty(t, got, "Bind writes nothing")
					assert.Equal(t, set.Entries(), tt.want, "the bound imports", assert.EquateEmpty())
				})
			}

			t.Run("binds nothing for no header", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				_, err := w.Bind(nil)
				assert.NoError(t, err, "a file without a header binds nothing")
				assert.Empty(t, set.Entries(), "the import set")
			})

			t.Run("binds an import of a module under its alias", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				_, err := w.Bind(&emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"import numpy as np"},
				}}})
				assert.NoError(t, err, "the header binds its import")
				assert.Length(t, set.Entries(), 1, "the one bound import")
				assert.Equal(t, set.Entries()[0].Name, "np", "the alias that the import binds")
			})

			t.Run("binds nothing more for a header that the file bound before", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				h := &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker),
					Args: []string{"from decimal import Decimal", "import numpy as np"},
				}}}
				for range 2 {
					_, err := w.Bind(h)
					assert.NoError(t, err, "a second bind of one header binds the same imports")
				}
				assert.Length(t, set.Entries(), 2, "the bound imports")
			})

			t.Run("binds the imports of another header", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				for _, stmt := range []string{"import json", "import os"} {
					_, err := w.Bind(&emit.File{Annotations: symbol.Annotations{{
						Name: string(python.ImportMarker), Args: []string{stmt},
					}}})
					assert.NoError(t, err, "the header binds its import")
				}
				assert.Equal(
					t,
					set.Entries(),
					[]render.Entry{{Path: "json"}, {Path: "os"}},
					"the imports of both headers",
				)
			})

			refusals := []struct {
				name        string
				give        []string
				giveDeclare string
				want        string
			}{
				{
					name: "returns an error for an argument that is no import statement", give: []string{"print(1)"},
					want: `python: an argument of the import marker is no import statement: "print(1)"`,
				},
				{
					name: "returns an error for an argument of one word", give: []string{"import"},
					want: `python: an argument of the import marker is no import statement: "import"`,
				},
				{
					name: "returns an error for an argument of two words that is no import statement",
					give: []string{"from shop"},
					want: `python: an argument of the import marker is no import statement: "from shop"`,
				},
				{
					name: "returns an error for a name that a declaration of the file binds",
					give: []string{"from decimal import Decimal"}, giveDeclare: "Decimal",
					want: "python: the header binds Decimal to an import of decimal, " +
						"and the file binds the name or the import otherwise",
				},
				{
					name: "returns an error for an alias of a module that another import binds",
					give: []string{"from shop import np", "import numpy as np"},
					want: "python: the header binds np to an import of numpy, " +
						"and the file binds the name or the import otherwise",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, python.Version311, false)
					if tt.giveDeclare != "" {
						set.Reserve(tt.giveDeclare)
					}
					_, err := w.Bind(&emit.File{Annotations: symbol.Annotations{{
						Name: string(python.ImportMarker), Args: tt.give,
					}}})
					assert.HasError(t, err, "the header refuses its imports")
					assert.Equal(t, err.Error(), tt.want, "the refusal names the import")
				})
			}
		})
	})
}

// NewWriter allocates the writer, and Bind allocates nothing for a header
// that the file has bound before. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestWriterAllocs(t *testing.T) {
	checkAllocs(t, writerCalls(t))
}

// BenchmarkWriter measures the writer of each file and the binding of the
// header of each declaration.
func BenchmarkWriter(b *testing.B) {
	benchCalls(b, writerCalls(b))
}

// writerCalls returns the calls of NewWriter, and of Bind for a header that
// the file has bound before.
func writerCalls(tb assert.TB) []allocCall {
	set := &render.ImportSet{}
	w, _ := newWriter(tb, python.Version311, false)
	h := &emit.File{Annotations: symbol.Annotations{{
		Name: string(python.ImportMarker), Args: []string{"from decimal import Decimal", "import json"},
	}}}
	_, err := w.Bind(h)
	assert.NoError(tb, err, "the header binds its imports")
	var got *backend.Writer
	var bound error
	return []allocCall{
		{
			name: "NewWriter", allocs: newWriterAllocs,
			call:  func() { got = backend.NewWriter(set, false) },
			check: func(tb assert.TB) { assert.NotNil(tb, got, "NewWriter returns the writer") },
		},
		{
			name: "Bind", allocs: bindAllocs,
			call:  func() { _, bound = w.Bind(h) },
			check: func(tb assert.TB) { assert.NoError(tb, bound, "Bind binds the header again") },
		},
	}
}
