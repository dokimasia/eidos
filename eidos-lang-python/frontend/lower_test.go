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
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestLower pins the lowering of a module: the head of its file, its
// docstring, the carriers of its header, the walk of its statements, the
// names that it declares and their visibility.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("lower", func(t *testing.T) {
		t.Parallel()

		t.Run("takes the docstring of the module as the documentation of the file", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "\"\"\"The shop.\n\n    More.\n\"\"\"\n\nx = 1\n")
			assert.Equal(t, fileOf(t, gb).Doc, []string{"The shop.", "", "More."}, "the docstring trimmed by PEP 257")
		})

		t.Run("takes the docstring of the module as the documentation of the package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "\"\"\"The shop.\"\"\"\n")
			assert.Equal(t, gb.Packages()[0].Doc, []string{"The shop."}, "the module documents its package")
		})

		t.Run("lists the names of __all__ as the export of the file", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "__all__ = [\"Order\", \"total\"]\n__all__ += (\"more\",)\n"+
				"__all__.append(\"last\")\n\nclass Order: ...\n")
			exports := fileOf(t, gb).Exports
			assert.Length(t, exports, 1, "__all__ is one export of the file")
			want := []string{"Order", "total", "more", "last"}
			assert.Equal(t, bindingNames(exports[0]), want, "every name of __all__ in its order")
			assert.Equal(t, exports[0].Path, "", "the module publishes names of its own")
		})

		t.Run("declares nothing for __all__", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, declsOf(t, "__all__ = [\"x\"]\n"), "__all__ is the export of the file")
		})

		t.Run("stamps the features of the future statements of the file", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "from __future__ import annotations, division\n")
			assert.Equal(t, stampsOn(gb, fileOf(t, gb), python.FutureKey), []any{"annotations", "division"},
				"the features in statement order")
		})

		t.Run("stamps the feature of a future statement under an alias", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "from __future__ import annotations as deferred\n")
			assert.Equal(t, stampsOn(gb, fileOf(t, gb), python.FutureKey), []any{"annotations"},
				"the feature without its alias")
		})

		t.Run("reports UnaddressedCarrier for a carrier in a comment that no declaration takes", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "x = 1\n\n"+carrier+"\ny = 2\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.UnaddressedCarrier},
				"a carrier separated from its subject attaches nowhere")
		})

		t.Run("reports UnparsedFile for a syntax error", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "class :\n")
			assert.Contains(t, codesOf(sink), frontend.UnparsedFile, "the syntax error reports")
		})

		t.Run("loads the declarations outside the statement with a syntax error", func(t *testing.T) {
			t.Parallel()

			names := declNames(&node.File{Decls: declsOf(t, "class Order: ...\n\ndef broken(:\n    pass\n")})
			assert.Contains(t, names, "Order", "the class before the error loads")
		})
	})

	t.Run("headComments", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps the comments of the head of a module without their #", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#!/usr/bin/env python3\n# -*- coding: utf-8 -*-\n# type: ignore\n"+
				"# mypy: strict\n# pyright: basic\n# ruff: noqa\n\"\"\"Doc.\"\"\"\n")
			assert.Equal(t, stampsOn(gb, fileOf(t, gb), python.HeadKey), []any{
				"!/usr/bin/env python3", " -*- coding: utf-8 -*-", " type: ignore", " mypy: strict", " pyright: basic",
				" ruff: noqa",
			}, "each comment of the head as written")
		})

		t.Run("leaves out a comment of the head that is no directive", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "# Copyright the authors.\n\"\"\"Doc.\"\"\"\n")
			assert.Empty(t, stampsOn(gb, fileOf(t, gb), python.HeadKey), "a licence is no comment of the head")
		})

		t.Run("reports ForeignEncoding for an encoding declaration of another encoding", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "# -*- coding: latin-1 -*-\nx = 1\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.ForeignEncoding}, "the file parses as UTF-8")
		})

		t.Run("reads utf_8 as UTF-8", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "# coding=utf_8\nx = 1\n")
			assert.Empty(t, codesOf(sink), "the encoding is UTF-8")
		})
	})

	t.Run("header", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
		}{
			{name: "attaches a carrier above the docstring to the package", give: carrier + "\"\"\"Doc.\"\"\"\n"},
			{name: "attaches a carrier above the first import to the package", give: carrier + "import os\n"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, sink := parsedSource(t, tt.give)
				assert.Empty(t, codesOf(sink), "the carrier attaches")
				attachments := gb.Attachments()
				assert.Length(t, attachments, 1, "one directive attaches")
				_, isPackage := attachments[0].Subject.(*node.Package)
				assert.True(t, isPackage, "the package is the subject of the header")
			})
		}

		t.Run("leaves the run above a first declaration to the declaration", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, carrier+"class Row: ...\n")
			attachments := gb.Attachments()
			assert.Length(t, attachments, 1, "one directive attaches")
			assert.Equal(t, string(attachments[0].Raw.Name), carrierDirective, "the directive of the carrier")
			_, isStruct := attachments[0].Subject.(*node.Struct)
			assert.True(t, isStruct, "the class is the subject of the run above it")
		})
	})

	t.Run("statements", func(t *testing.T) {
		t.Parallel()

		t.Run("loads the declarations of every clause of a try statement", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "try:\n    a = 1\nexcept ImportError:\n    b = 2\nelse:\n    c = 3\n"+
				"finally:\n    d = 4\n")
			assert.Equal(t, declNames(&node.File{Decls: decls}), []string{"a", "b", "c", "d"}, "every clause runs")
		})

		t.Run("loads the declarations of a with statement and of a loop", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "with open(p) as f:\n    a = 1\nfor i in r:\n    b = 2\nelse:\n    c = 3\n"+
				"while x:\n    d = 4\n")
			assert.Equal(t, declNames(&node.File{Decls: decls}), []string{"a", "b", "c", "d"}, "every body loads")
		})

		t.Run("loads the declarations of the cases of a match statement", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "match x:\n    case 1:\n        a = 1\n    case _:\n        b = 2\n")
			assert.Equal(t, declNames(&node.File{Decls: decls}), []string{"a", "b"}, "every case loads")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a statement that declares nothing", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, carrier+"print(1)\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.UnaddressedCarrier}, "a call declares nothing")
		})
	})

	t.Run("declare", func(t *testing.T) {
		t.Parallel()

		t.Run("reports DuplicateName for a second declaration of a name", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "x = 1\nx = 2\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.DuplicateName}, "the second declaration reports")
		})

		t.Run("keeps the first declaration of a name", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "x = 1\nx = 2\n")
			assert.Length(t, decls, 1, "one declaration of the name")
			assert.Equal(t, named[*node.Variable](t, decls, "x").Value, "1", "the first value")
		})

		t.Run("declares the overloads of a name and its implementation", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "from typing import overload\n\n@overload\ndef f(x: int) -> int: ...\n"+
				"@overload\ndef f(x: str) -> str: ...\ndef f(x): return x\n")
			assert.Length(t, decls, 3, "each overload and the implementation")
		})

		t.Run("reports DuplicateName for a declaration after the implementation of overloads", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "from typing import overload\n\n@overload\ndef f(x: int) -> int: ...\n"+
				"def f(x): return x\ndef f(y): return y\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.DuplicateName}, "the third declaration reports")
		})
	})

	t.Run("moduleVisibility", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want symbol.Visibility
		}{
			{
				name: "returns public for a name without an underscore",
				give: "order = 1\n",
				want: symbol.VisibilityPublic,
			},
			{
				name: "returns package for a name with a leading underscore", give: "_order = 1\n",
				want: symbol.VisibilityPackage,
			},
			{name: "returns public for a dunder name", give: "__version__ = 1\n", want: symbol.VisibilityPublic},
			{
				name: "returns package for a name that __all__ does not list", give: "__all__ = []\norder = 1\n",
				want: symbol.VisibilityPackage,
			},
			{
				name: "returns public for a name with an underscore that __all__ lists",
				give: "__all__ = [\"_order\"]\n_order = 1\n", want: symbol.VisibilityPublic,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				decls := declsOf(t, tt.give)
				assert.Length(t, decls, 1, "the module declares one variable")
				v, _ := decls[0].(*node.Variable)
				assert.Equal(t, v.Visibility, tt.want, "the visibility of the name")
			})
		}

		t.Run("leaves out a declaration that is not public at signature depth", func(t *testing.T) {
			t.Parallel()

			gbs, _ := parsedTree(t, fstest.MapFS{
				projectFile: {Data: []byte(rootProject)}, appFile: {Data: []byte("_order = 1\npublic = 2\n")},
			}, plugin.DepthSignatures)
			expect.Equal(t, declNames(fileOf(t, gbs[0])), []string{"public"}, "the private variable is left out")
		})
	})

	t.Run("memberVisibility", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			give     string
			want     symbol.Visibility
			wantHard bool
		}{
			{name: "returns public for a name without an underscore", give: "order", want: symbol.VisibilityPublic},
			{
				name: "returns protected for a name with one leading underscore", give: "_order",
				want: symbol.VisibilityProtected,
			},
			{
				name: "returns private for a name that Python mangles", give: "__order",
				want: symbol.VisibilityPrivate, wantHard: true,
			},
			{name: "returns public for a dunder name", give: "__order__", want: symbol.VisibilityPublic},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				st := named[*node.Struct](t, declsOf(t, "class C:\n    "+tt.give+": int\n"), "C")
				assert.Length(t, st.Fields, 1, "the class declares one field")
				expect.Equal(t, st.Fields[0].Visibility, tt.want, "the visibility of the member")
				expect.Equal(t, st.Fields[0].Hard, tt.wantHard, "a mangled name is private at run time")
			})
		}
	})
}
