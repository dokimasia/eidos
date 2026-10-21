// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The configuration of the directive cases, which selects the library of
// ES2024, the statement after the head of each source, and the directive
// that extends the library with the DOM.
const (
	es2024Config  = `{"compilerOptions": {"lib": ["ES2024"]}}`
	moduleBody    = "export {};\n"
	domDirective  = "/// <reference lib=\"dom\" />\n"
	eventName     = "Event"
	unknownLibDir = "/// <reference lib=\"es2099\" />\n"
)

// The packages of the library of ES2024 and of the library that the
// directive extends with the DOM.
var (
	es2024Package    = plugin.StorePath(typescript.LibraryStore, es2024Root)
	domES2024Package = plugin.StorePath(typescript.LibraryStore, domRoot+"+"+es2024Root)
)

// A lib directive at the head of a source file adds its library to the
// library of the file, as typescript-go reads the directive.
func TestDirective(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			giveHead    string
			wantPackage string
		}{
			{
				name:        "probes the library that a lib directive extends",
				giveHead:    domDirective,
				wantPackage: domES2024Package,
			},
			{
				name:        "probes the library of the file for a directive of a file of that library",
				giveHead:    "/// <reference lib=\"es2024\" />\n",
				wantPackage: es2024Package,
			},
			{
				name:        "reads a directive in single quotes",
				giveHead:    "/// <reference lib='dom' />\n",
				wantPackage: domES2024Package,
			},
			{
				name:        "reads the tag and the arguments of a directive without regard to case",
				giveHead:    "/// <Reference LIB=\"DOM\" />\n",
				wantPackage: domES2024Package,
			},
			{
				name:        "reads a directive after a block comment",
				giveHead:    "/* The licence. */\n" + domDirective,
				wantPackage: domES2024Package,
			},
			{
				name:        "reads a directive after a line comment",
				giveHead:    "// The module.\n" + domDirective,
				wantPackage: domES2024Package,
			},
			{
				name:        "reads a directive after a byte order mark and a shebang line",
				giveHead:    "\xef\xbb\xbf#!/usr/bin/env node\n" + domDirective,
				wantPackage: domES2024Package,
			},
			{
				name:        "does not read a directive after the first statement",
				giveHead:    "const first = 1;\n" + domDirective,
				wantPackage: es2024Package,
			},
			{
				name:        "does not read a directive with no-default-lib",
				giveHead:    "/// <reference no-default-lib=\"true\" lib=\"dom\" />\n",
				wantPackage: es2024Package,
			},
			{
				name:        "does not read a directive with a types argument",
				giveHead:    "/// <reference types=\"node\" lib=\"dom\" />\n",
				wantPackage: es2024Package,
			},
			{
				name:        "does not read a directive without a lib argument",
				giveHead:    "/// <reference path=\"./a.d.ts\" />\n",
				wantPackage: es2024Package,
			},
			{
				name:        "does not read a directive of another tag",
				giveHead:    "/// <amd-module name=\"dom\" />\n",
				wantPackage: es2024Package,
			},
			{
				name:        "does not read a directive of a comment of two slashes",
				giveHead:    "// <reference lib=\"dom\" />\n",
				wantPackage: es2024Package,
			},
			{
				name:        "does not read a directive without an opening of its tag",
				giveHead:    "/// reference lib=\"dom\"\n",
				wantPackage: es2024Package,
			},
			{
				name:        "does not read a tag past the end of a comment",
				giveHead:    "/// <\n",
				wantPackage: es2024Package,
			},
			{
				name:        "reads the arguments of a directive up to its end",
				giveHead:    "/// <reference /> lib=\"dom\"\n",
				wantPackage: es2024Package,
			},
			{
				name:        "reads the arguments of a directive up to an argument without a name",
				giveHead:    "/// <reference =\"dom\" lib=\"dom\" />\n",
				wantPackage: es2024Package,
			},
			{
				name:        "reads the arguments of a directive up to an argument without a value",
				giveHead:    "/// <reference lib \"dom\" />\n",
				wantPackage: es2024Package,
			},
			{
				name:        "reads the arguments of a directive up to a value without quotes",
				giveHead:    "/// <reference lib=dom />\n",
				wantPackage: es2024Package,
			},
			{
				name:        "reads the arguments of a directive up to a value that does not close",
				giveHead:    "/// <reference lib=\"dom />\n",
				wantPackage: es2024Package,
			},
			{
				name:        "reads the arguments of a directive up to a value at the end of the comment",
				giveHead:    "/// <reference lib=\n",
				wantPackage: es2024Package,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedWithLibrary(t, fstest.MapFS{
					rootConfig: {Data: []byte(es2024Config)},
					aFile:      {Data: []byte(tt.giveHead + moduleBody)},
				})
				assert.Empty(t, found, "the head of the file reads without a finding")
				file := fileIn(t, gb, aPackage)
				tiers := resolved(scopeOfFile(t, gb, file), eventName)
				expect.Equal(t, tiers[len(tiers)-1], []symbol.Identity{id(tt.wantPackage, eventName)},
					"a bare name probes the library of the file last")
				expect.Empty(t, file.Imports, "the file imports no library")
			})
		}

		t.Run("reports UnknownLibrary for a directive of a library that the release does not know", func(t *testing.T) {
			t.Parallel()

			_, found := parsedWithLibrary(t, fstest.MapFS{
				rootConfig: {Data: []byte(es2024Config)},
				aFile:      {Data: []byte("// The module.\n" + unknownLibDir + moduleBody)},
			})
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnknownLibrary}, "the parse reports the unknown name")
			assert.Contains(t, found[0].Msg, `"es2099"`, "the finding quotes the name")
			assert.Equal(t, found[0].Pos.Line, 2, "the finding is at the line of the directive")
		})

		t.Run("does not read a directive of a file without a library", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, unknownLibDir+domDirective+moduleBody)
			assert.Empty(t, found, "the parse does not report a directive of a file without a library")
			assert.Empty(t, fileIn(t, gb, aPackage).Imports, "a file without a library does not import a library")
		})
	})
}

// scopeOfFile returns the scope that a File node of a builder recorded.
func scopeOfFile(tb testing.TB, gb *plugin.GraphBuilder, file *node.File) plugin.ImportScope {
	tb.Helper()

	scopes := gb.Scopes()
	at := slices.IndexFunc(scopes, func(rec plugin.ScopeRecord) bool { return rec.File == file })
	assert.NotEqual(tb, at, -1, "the File node records a scope")
	return plugin.ImportScope{Bindings: scopes[at].Bindings}
}
