// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	pyfrontend "go.dokimi.dev/eidos/lang/python/frontend"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages of the module whose stamps the cases restate and of a module
// without a header.
const (
	shopPkg  = "shop/app"
	plainPkg = "shop/plain"
)

// restatedTree is a module with every stamp of a declaration and of a
// file, beside a module without a header, below a project file whose module
// root is the root of the workspace.
var restatedTree = fstest.MapFS{
	"pyproject.toml": {Data: []byte("[project]\nname = \"shop\"\n[build-system]\nbuild-backend = \"uv_build\"\n" +
		"[tool.uv.build-backend]\nmodule-root = \"\"\n")},
	"shop/app.py": {Data: []byte(`#!/usr/bin/env python3
"""The shop."""

from __future__ import annotations

from dataclasses import InitVar, dataclass
from enum import Enum
from typing import Annotated, TypedDict
from typing import cast as as_type

import shop.plain as plain
from shop.plain import *

lazy import json

__all__ = ["Movie", "Color", "Box", "count"]


@dataclass
class Point:
    scale: InitVar[int]


class Movie(TypedDict, total=False):
    name: str


class Color(int, Enum):
    RED = 1


class Box(metaclass=Meta):
    pass


count: Annotated[int, "positive"] = 1
`)},
	"shop/plain.py": {Data: []byte("class Plain: ...\n")},
}

// TestStated pins the markers through which Stated and Header restate the
// stamps of a Python source.
func TestStated(t *testing.T) {
	t.Parallel()

	t.Run("Stated", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Identity
			want symbol.Annotations
		}{
			{
				name: "returns the TypedDict marker and the keywords of a TypedDict",
				give: symbol.Identity{Lang: python.Lang, Package: shopPkg, Name: "Movie", Kind: symbol.KindInterface},
				want: symbol.Annotations{
					{Name: string(python.TypedDictMarker)},
					{Name: string(python.KeywordsMarker), Args: []string{"total=False"}},
				},
			},
			{
				name: "returns the bases of an enum",
				give: symbol.Identity{Lang: python.Lang, Package: shopPkg, Name: "Color", Kind: symbol.KindEnum},
				want: symbol.Annotations{{Name: string(python.BasesMarker), Args: []string{"int", "Enum"}}},
			},
			{
				name: "returns the keywords of a class statement",
				give: symbol.Identity{Lang: python.Lang, Package: shopPkg, Name: "Box", Kind: symbol.KindStruct},
				want: symbol.Annotations{{Name: string(python.KeywordsMarker), Args: []string{"metaclass=Meta"}}},
			},
			{
				name: "returns the metadata of an Annotated",
				give: symbol.Identity{Lang: python.Lang, Package: shopPkg, Name: "count", Kind: symbol.KindVariable},
				want: symbol.Annotations{{Name: string(python.AnnotatedMarker), Args: []string{`"positive"`}}},
			},
			{
				name: "returns the InitVar marker of a field of a dataclass",
				give: symbol.Identity{
					Lang: python.Lang, Package: shopPkg, Owner: "Point", Name: "scale", Kind: symbol.KindField,
				},
				want: symbol.Annotations{{Name: string(python.InitVarMarker)}},
			},
			{
				name: "returns nothing for a declaration without stamps",
				give: symbol.Identity{Lang: python.Lang, Package: plainPkg, Name: "Plain", Kind: symbol.KindStruct},
			},
		}
		ids := make([]symbol.Identity, 0, len(tests))
		for _, tt := range tests {
			ids = append(ids, tt.give)
		}
		got, _ := restated(t, ids, nil)
		assert.Length(t, got, len(tests), "Stated returns the markers of each identity")
		for i, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, got[i], tt.want, "the markers of the stamps")
			})
		}
	})

	t.Run("Header", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Identity
			want *emit.File
		}{
			{
				name: "returns the docstring and the header markers of a module",
				give: symbol.Identity{Lang: python.Lang, Package: shopPkg, Name: "Movie", Kind: symbol.KindInterface},
				want: &emit.File{
					Doc: []string{"The shop."},
					Annotations: symbol.Annotations{
						{Name: string(python.AllMarker), Args: []string{"Movie", "Color", "Box", "count"}},
						{Name: string(python.FutureMarker), Args: []string{"annotations"}},
						{Name: string(python.LazyMarker), Args: []string{"json"}},
						{Name: string(python.HeadMarker), Args: []string{"!/usr/bin/env python3"}},
						{Name: string(python.ImportMarker), Args: []string{
							"from dataclasses import InitVar",
							"from dataclasses import dataclass",
							"from enum import Enum",
							"from typing import Annotated",
							"from typing import TypedDict",
							"from typing import cast as as_type",
							"import shop.plain as plain",
							"from shop.plain import *",
							"import json",
						}},
					},
				},
			},
			{
				name: "returns nil for a module without a header",
				give: symbol.Identity{Lang: python.Lang, Package: plainPkg, Name: "Plain", Kind: symbol.KindStruct},
			},
		}
		ids := make([]symbol.Identity, 0, len(tests))
		for _, tt := range tests {
			ids = append(ids, tt.give)
		}
		_, got := restated(t, nil, ids)
		assert.Length(t, got, len(tests), "Header returns the header of each declaration")
		for i, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, got[i], tt.want, "the header of the file of the declaration")
			})
		}
	})
}

// restated returns what Stated returns for each identity and what Header
// returns for each declaration, in one generate phase over restatedTree.
func restated(tb assert.TB, stated, headers []symbol.Identity) ([]symbol.Annotations, []*emit.File) {
	tb.Helper()

	f := plugintest.Loaded(tb, plugintest.Source{
		Tree: restatedTree, Brand: frontendtest.Brand, Frontends: []plugin.Frontend{pyfrontend.New(nil)},
	})
	var (
		markers []symbol.Annotations
		files   []*emit.File
	)
	restater := eidos.NewPlugin("restater").
		Output(plugin.Output{Per: plugin.PerPlan, Word: "restated"}).
		Handle(eidos.OnGraph(func(m *eidos.GraphMatch, _ *eidos.Emitter) error {
			for _, id := range stated {
				markers = append(markers, python.Stated(m, id))
			}
			for _, id := range headers {
				d, held := m.Reader().Lookup(id)
				assert.True(tb, held, "the graph declares "+id.String())
				decl, is := d.(node.Declaration)
				assert.True(tb, is, id.String()+" is a declaration")
				files = append(files, python.Header(m, decl))
			}
			return nil
		})).Generator()
	r := f.Generate(tb, restater)
	assert.NoError(tb, r.Err, "the generate phase completes")
	return markers, files
}
