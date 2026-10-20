// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// TestImports pins the module boundary of a file: the Imports that its
// statements record and the names that they bind.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("importStatement", func(t *testing.T) {
		t.Parallel()

		t.Run("records an Import for each module of an import statement", func(t *testing.T) {
			t.Parallel()

			imports := importsOf(t, "import os.path as osp, sys\n")
			assert.Length(t, imports, 2, "one Import for each module")
			expect.Equal(t, imports[0].Path, "os/path", "the slash path of the module")
			expect.Equal(t, imports[0].Alias, "osp", "the alias of the module")
			expect.Equal(t, imports[1].Path, "sys", "the second module")
		})

		t.Run("records the names and the aliases of a from statement", func(t *testing.T) {
			t.Parallel()

			imports := importsOf(t, "from typing import (\n    Optional,\n    Protocol as P,\n)\n")
			assert.Length(t, imports, 1, "a from statement is one Import")
			expect.Equal(t, imports[0].Path, "typing", "the module of the statement")
			assert.Length(t, imports[0].Names, 2, "a Binding for each name")
			expect.Equal(t, imports[0].Names[0].Name, "Optional", "the first name")
			expect.Equal(t, imports[0].Names[1].Alias, "P", "the alias of the second name")
		})

		t.Run("records a wildcard for a star import", func(t *testing.T) {
			t.Parallel()

			imports := importsOf(t, "from shop.models import *\n")
			expect.True(t, imports[0].Wildcard, "every name of the module enters the scope")
			expect.Equal(t, imports[0].Path, "shop/models", "the module of the star import")
		})

		t.Run("records the trailing comment of an import", func(t *testing.T) {
			t.Parallel()

			imports := importsOf(t, "import os  # the system\n")
			assert.Equal(t, imports[0].Comment, "the system", "the comment after the import")
		})

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "resolves one dot against the package of a module", give: "from . import sibling\n", want: "shop"},
			{name: "resolves two dots against the package above", give: "from .. import top\n", want: ""},
			{
				name: "resolves a relative module below the package", give: "from .models import Order\n",
				want: "shop/models",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, importsOf(t, tt.give)[0].Path, tt.want, "the module of the relative import")
			})
		}

		t.Run("resolves one dot against the module of a package directory itself", func(t *testing.T) {
			t.Parallel()

			gbs, _ := parsedTree(t, fstest.MapFS{
				projectFile: {Data: []byte(rootProject)}, "shop/__init__.py": {Data: []byte("from . import models\n")},
			}, plugin.DepthFull)
			imports := gbs[0].Packages()[0].Files[0].Imports
			assert.Length(t, imports, 1, "the from statement is one Import")
			assert.Equal(t, imports[0].Path, "shop", "one dot is the package that the module is")
		})

		t.Run("stamps the modules of the lazy imports of the file", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "lazy import json\nlazy from os import path\n")
			assert.Equal(t, stampsOn(gb, fileOf(t, gb), python.LazyKey), []any{"json", "os"},
				"each module that the file imports lazily, as written")
			assert.Length(t, fileOf(t, gb).Imports, 2, "a lazy import is an Import")
		})

		t.Run("reports UnaddressedCarrier for a carrier on an import", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "x = 1\n"+carrier+"import os\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.UnaddressedCarrier}, "no rule takes an import")
		})
	})

	t.Run("futureImport", func(t *testing.T) {
		t.Parallel()

		t.Run("records no Import for a future statement", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, importsOf(t, "from __future__ import annotations\n"), "the statement imports no module")
		})
	})

	t.Run("bindModule", func(t *testing.T) {
		t.Parallel()

		t.Run("binds the first name of a module that an import names without an alias", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, "import collections.abc\n\nclass C:\n    x: collections.abc.Sequence[int]\n").Type
			assert.Equal(t, ref.Package, "collections/abc", "the reference reads through the imported module")
		})

		t.Run("binds an alias to the module that it names", func(t *testing.T) {
			t.Parallel()

			ref := classField(t, "import collections.abc as cabc\n\nclass C:\n    x: cabc.Sequence[int]\n").Type
			assert.Equal(t, ref.Package, "collections/abc", "the alias names the module")
		})
	})
}

// importsOf parses one source at appFile and returns the Imports of its
// file.
func importsOf(tb testing.TB, src string) []*node.Import {
	tb.Helper()

	gb, _ := parsedSource(tb, src)
	return fileOf(tb, gb).Imports
}
