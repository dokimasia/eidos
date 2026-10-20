// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestImports pins the binding of an import that a template states, and the
// import block of a file in the layout of isort's defaults, each the block
// that ruff's rules of isort leave as it is.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("TemplateImport", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			give      string
			wantLocal string
			wantItem  string
			wantBinds bool
		}{
			{
				name: "returns a name that the import binds as the local name and the item", give: "field",
				wantLocal: "field", wantItem: "field", wantBinds: true,
			},
			{name: "reports false for an import of the whole module"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				local, item, binds := backend.TemplateImport("dataclasses", tt.give)
				assert.Equal(t, local, tt.wantLocal, "the local name")
				assert.Equal(t, item, tt.wantItem, "the item")
				assert.Equal(t, binds, tt.wantBinds, "whether the import binds a name")
			})
		}
	})

	t.Run("Imports", func(t *testing.T) {
		t.Parallel()

		// Each case with header imports gives a variable uses whose value
		// uses their names, because the block leaves out an import that the
		// file does not use.
		tests := []struct {
			name        string
			giveVersion plugin.Choice
			giveHeader  *emit.File
			give        []*emit.TypeRef
			giveUses    string
			want        string
		}{
			{
				name: "writes no block for a file without imports", giveVersion: python.Version311,
				want: "def use() -> None: ...\n",
			},
			{
				name: "writes the sections of the future statement, the standard library, the third-party modules " +
					"and the project apart",
				giveVersion: python.Version311,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.FutureMarker), Args: []string{"annotations"},
				}}},
				give: []*emit.TypeRef{
					{Spelling: "Store", Package: "shop/other"},
					{Spelling: "ndarray", Package: "numpy"},
					{Spelling: "Decimal", Package: "decimal"},
				},
				want: "from __future__ import annotations\n\nfrom decimal import Decimal\n\nfrom numpy import ndarray\n\n" +
					"from shop.other import Store\n\n\ndef use(p0: Store, p1: ndarray, p2: Decimal) -> None: ...\n",
			},
			{
				name:        "writes the import statements before the from statements, each in the order of the modules",
				giveVersion: python.Version311,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{
						"import json", "from os import sep", "from os import PathLike", "from os import SEEK_SET",
						"from collections.abc import Mapping",
					},
				}}},
				give: []*emit.TypeRef{
					{Spelling: "Decimal", Package: "decimal"},
					{Spelling: "Any", Package: "typing"},
				},
				giveUses: "(json, sep, PathLike, SEEK_SET, Mapping)",
				want: "import json\nfrom collections.abc import Mapping\nfrom decimal import Decimal\n" +
					"from os import SEEK_SET, PathLike, sep\nfrom typing import Any\n\n\n" +
					"def use(p0: Decimal, p1: Any) -> None: ...\n\n\nuses = (json, sep, PathLike, SEEK_SET, Mapping)\n",
			},
			{
				name:        "writes a renamed member and a star import as statements of their own",
				giveVersion: python.Version311,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{
						"from decimal import Decimal as D", "from os import *", "import numpy as np",
					},
				}}},
				give:     []*emit.TypeRef{{Spelling: "Decimal", Package: "decimal"}},
				giveUses: "np",
				want: "from decimal import Decimal as D\nfrom os import *\n\nimport numpy as np\n\n\n" +
					"def use(p0: D) -> None: ...\n\n\nuses = np\n",
			},
			{
				name:        "leaves out an import of the header whose name the file does not use",
				giveVersion: python.Version311,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"import json", "from os import sep"},
				}}},
				give: []*emit.TypeRef{{Spelling: "Decimal", Package: "decimal"}},
				want: "from decimal import Decimal\n\n\ndef use(p0: Decimal) -> None: ...\n",
			},
			{
				name:        "keeps an import of the header whose name a string of the file contains",
				giveVersion: python.Version311,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"from os import sep"},
				}}},
				giveUses: `"sep"`,
				want:     "from os import sep\n\n\ndef use() -> None: ...\n\n\nuses = \"sep\"\n",
			},
			{
				name:        "leaves out an import of the header whose name occurs inside a longer name alone",
				giveVersion: python.Version311,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"from os import sep"},
				}}},
				giveUses: "separator",
				want:     "def use() -> None: ...\n\n\nuses = separator\n",
			},
			{
				name:        "keeps a re-export of the header with its alias that the file does not use",
				giveVersion: python.Version311,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{
						"import json as json", "from os import sep as sep", "from os import PathLike",
					},
				}}},
				giveUses: "PathLike",
				want: "import json as json\nfrom os import PathLike\nfrom os import sep as sep\n\n\n" +
					"def use() -> None: ...\n\n\nuses = PathLike\n",
			},
			{
				name:        "writes one member on each line of a from statement that is longer than the line",
				giveVersion: python.Version311,
				give: []*emit.TypeRef{
					{Spelling: "AVeryLongClassNameNumberOne", Package: "shop/other"},
					{Spelling: "AVeryLongClassNameNumberTwo", Package: "shop/other"},
					{Spelling: "AVeryLongClassNameNumberThree", Package: "shop/other"},
				},
				want: "from shop.other import (\n    AVeryLongClassNameNumberOne,\n    AVeryLongClassNameNumberThree,\n" +
					"    AVeryLongClassNameNumberTwo,\n)\n\n\ndef use(\n    p0: AVeryLongClassNameNumberOne,\n" +
					"    p1: AVeryLongClassNameNumberTwo,\n    p2: AVeryLongClassNameNumberThree,\n) -> None: ...\n",
			},
			{
				name:        "writes the word lazy before the statement of a module of the lazy marker",
				giveVersion: python.Version315,
				giveHeader: &emit.File{Annotations: symbol.Annotations{
					{Name: string(python.ImportMarker), Args: []string{"import json"}},
					{Name: string(python.LazyMarker), Args: []string{"json"}},
				}},
				give:     []*emit.TypeRef{{Spelling: "Decimal", Package: "decimal"}},
				giveUses: "json",
				want: "lazy import json\nfrom decimal import Decimal\n\n\ndef use(p0: Decimal) -> None: ...\n\n\n" +
					"uses = json\n",
			},
			{
				name:        "writes the word lazy before a from statement of a module of the lazy marker",
				giveVersion: python.Version315,
				giveHeader: &emit.File{Annotations: symbol.Annotations{
					{Name: string(python.ImportMarker), Args: []string{"from json import dumps"}},
					{Name: string(python.LazyMarker), Args: []string{"json"}},
				}},
				give:     []*emit.TypeRef{{Spelling: "Decimal", Package: "decimal"}},
				giveUses: "dumps",
				want: "from decimal import Decimal\nlazy from json import dumps\n\n\ndef use(p0: Decimal) -> None: ...\n\n\n" +
					"uses = dumps\n",
			},
			{
				name:        "writes a star import before the other members of its module",
				giveVersion: python.Version311,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"from os import sep", "from os import *"},
				}}},
				giveUses: "sep",
				want:     "from os import *\nfrom os import sep\n\n\ndef use() -> None: ...\n\n\nuses = sep\n",
			},
			{
				name:        "writes the import of a module before the import of the module under an alias",
				giveVersion: python.Version311,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"import numpy as np", "import numpy"},
				}}},
				giveUses: "(numpy, np)",
				want:     "import numpy\nimport numpy as np\n\n\ndef use() -> None: ...\n\n\nuses = (numpy, np)\n",
			},
			{
				name:        "writes the modules in the order of the numbers in their names",
				giveVersion: python.Version311,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"import pkg10", "import pkg2"},
				}}},
				giveUses: "(pkg2, pkg10)",
				want:     "import pkg2\nimport pkg10\n\n\ndef use() -> None: ...\n\n\nuses = (pkg2, pkg10)\n",
			},
			{
				name:        "writes the modules of one number in the order of the text after the number",
				giveVersion: python.Version311,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"import api2beta", "import api2alpha"},
				}}},
				giveUses: "(api2alpha, api2beta)",
				want:     "import api2alpha\nimport api2beta\n\n\ndef use() -> None: ...\n\n\nuses = (api2alpha, api2beta)\n",
			},
			{
				name:        "writes a module before its submodules",
				giveVersion: python.Version311,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"import json.decoder", "import json"},
				}}},
				giveUses: "json",
				want:     "import json\nimport json.decoder\n\n\ndef use() -> None: ...\n\n\nuses = json\n",
			},
			{
				name:        "writes typing_extensions and _typeshed among the third-party modules",
				giveVersion: python.Version312,
				give: []*emit.TypeRef{
					{Spelling: "ReadOnly", Package: "typing"},
					{Spelling: "StrPath", Package: "_typeshed"},
					{Spelling: "Decimal", Package: "decimal"},
				},
				want: "from decimal import Decimal\n\nfrom _typeshed import StrPath\nfrom typing_extensions import ReadOnly\n\n\n" +
					"def use(p0: ReadOnly, p1: StrPath, p2: Decimal) -> None: ...\n",
			},
			{
				name:        "writes a module of the standard library of the target version among the standard library",
				giveVersion: python.Version311,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"import asynchat"},
				}}},
				give:     []*emit.TypeRef{{Spelling: "Decimal", Package: "decimal"}},
				giveUses: "asynchat",
				want: "import asynchat\nfrom decimal import Decimal\n\n\ndef use(p0: Decimal) -> None: ...\n\n\n" +
					"uses = asynchat\n",
			},
			{
				name:        "writes a module that a later version removes among the third-party modules",
				giveVersion: python.Version312,
				giveHeader: &emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"import asynchat"},
				}}},
				give:     []*emit.TypeRef{{Spelling: "Decimal", Package: "decimal"}},
				giveUses: "asynchat",
				want: "from decimal import Decimal\n\nimport asynchat\n\n\ndef use(p0: Decimal) -> None: ...\n\n\n" +
					"uses = asynchat\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				use := &emit.Function{Name: "use"}
				for i, ref := range tt.give {
					use.Params.Append(&emit.Param{Name: "p" + strconv.Itoa(i), Type: ref})
				}
				decls := []symbol.Symbol{use}
				if tt.giveUses != "" {
					decls = append(decls, &emit.Variable{Name: "uses", Value: tt.giveUses})
				}
				if tt.giveHeader != nil {
					decls = append(decls, tt.giveHeader)
				}
				got, diags := rendered(t, backend.New(), versionPolicy(t, tt.giveVersion), decls...)
				assert.Empty(t, diags, "the render reports nothing")
				assert.Equal(t, got, tt.want, "the file")
			})
		}
	})
}

// TemplateImport allocates nothing. The ordinary run, which runs no
// benchmark, checks that ceiling here.
func TestImportsAllocs(t *testing.T) {
	checkAllocs(t, importsCalls())
}

// BenchmarkImports measures the binding of an import that a template
// states.
func BenchmarkImports(b *testing.B) {
	benchCalls(b, importsCalls())
}

// importsCalls returns the call of TemplateImport for a name of a module.
func importsCalls() []allocCall {
	var local string
	return []allocCall{
		{
			name: "TemplateImport", allocs: 0,
			call:  func() { local, _, _ = backend.TemplateImport("dataclasses", "field") },
			check: func(tb assert.TB) { assert.Equal(tb, local, "field", "TemplateImport binds the name") },
		},
	}
}
