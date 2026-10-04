// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages the namespace cases declare into.
const (
	nsPackage    = aPackage + "/A/B"
	outerPackage = aPackage + "/A"
	modulePkg    = "lib"
)

// The statement walk decides which package each declaration is in and
// what the module publishes, so its rules are pinned.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("lower", func(t *testing.T) {
		t.Parallel()

		t.Run("declares a script's declarations into the global package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "class A {}\n")
			assert.Equal(t, packagePaths(gb), []string{""}, "a file without import or export is a script")
		})

		t.Run("lowers a script's declarations as public", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "class A {}\n")
			assert.Equal(t, fileIn(t, gb, "").Decls[0].(*node.Struct).Visibility, symbol.VisibilityPublic,
				"the global scope is every file's")
		})
	})

	t.Run("record", func(t *testing.T) {
		t.Parallel()

		t.Run("records a named import with its renames", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "import { A, B as C } from './lib';\n")
			imp := fileIn(t, gb, aPackage).Imports[0]
			assert.Equal(t, imp.Path, "./lib", "the module specifier")
			assert.Equal(t, imp.Names[1].Alias, "C", "the rename")
		})

		t.Run("records a default and a namespace import", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "import D from './d';\nimport * as N from './n';\n")
			imports := fileIn(t, gb, aPackage).Imports
			assert.Equal(t, imports[0].Default, "D", "the default's local name")
			assert.Equal(t, imports[1].Alias, "N", "the namespace's local name")
		})

		t.Run("records an import-require as a namespace import", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "import F = require('f');\nexport {};\n")
			imp := fileIn(t, gb, aPackage).Imports[0]
			assert.Equal(t, imp.Alias, "F", "the module's local name")
			assert.Equal(t, imp.Path, "f", "the module")
		})

		t.Run("records a re-export from a module with its renames", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export { J, K as L } from './j';\n")
			export := fileIn(t, gb, aPackage).Exports[0]
			assert.Equal(t, export.Path, "./j", "the source module")
			assert.Equal(t, export.Names[1].Alias, "L", "the published name")
		})

		t.Run("records an export star as a wildcard", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export * from './star';\n")
			assert.True(t, fileIn(t, gb, aPackage).Exports[0].Wildcard, "every name of the module")
		})

		t.Run("records a namespace re-export as the whole module under its name", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export * as M from './m';\n")
			b := fileIn(t, gb, aPackage).Exports[0].Names[0]
			assert.Equal(t, b.Name+" "+b.Alias, "* M", "the module's namespace, published as M")
		})

		t.Run("records the default export's local name", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "class A {}\nexport default A;\n")
			assert.Equal(t, fileIn(t, gb, aPackage).Exports[0].Default, "A", "the default publishes A")
		})

		t.Run("records an export assignment as the default export", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "import x = require('x');\nexport = x;\n")
			assert.Equal(t, fileIn(t, gb, aPackage).Exports[0].Default, "x", "export = publishes its name")
		})

		t.Run("records nothing for an export statement that exports its own declaration", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, exportClass)
			assert.Empty(t, fileIn(t, gb, aPackage).Exports, "the declaration's export is its visibility")
		})

		t.Run("records an anonymous default export under default", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export default class {}\n")
			assert.Equal(t, fileIn(t, gb, aPackage).Exports[0].Default, "default", "the class's name is default")
		})

		t.Run("records a default-exported declaration's name as the default export", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export default class Foo {}\n")
			assert.Equal(t, fileIn(t, gb, aPackage).Exports[0].Default, "Foo", "the default publishes Foo")
		})

		nothings := []struct {
			name string
			give string
		}{
			{name: "records nothing for a default export of an expression", give: "export default 42;\n"},
			{name: "records nothing for an export assignment of an expression", give: "export = { a: 1 };\n"},
			{name: "records nothing for an export as namespace", give: "export as namespace Lib;\n"},
		}
		for _, tt := range nothings {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, _ := parsedSource(t, tt.give)
				assert.Empty(t, fileIn(t, gb, aPackage).Exports, "no declaration is published")
			})
		}

		t.Run("skips a comment among an import's names", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "import { A, /* and */ B } from './x';\n")
			assert.Length(t, fileIn(t, gb, aPackage).Imports[0].Names, 2, "a comment binds nothing")
		})

		t.Run("skips a comment among an export clause's names", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export { A, /* and */ B } from './x';\n")
			assert.Length(t, fileIn(t, gb, aPackage).Exports[0].Names, 2, "a comment publishes nothing")
		})
	})

	t.Run("namespace", func(t *testing.T) {
		t.Parallel()

		t.Run("declares a dotted namespace's members into its package below the file's", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export namespace A.B {\n  export interface I {}\n}\n")
			assert.Length(t, fileIn(t, gb, nsPackage).Decls, 1, "the interface is in src/a/A/B")
		})

		t.Run("stamps a namespace's package with its dotted name", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export namespace A.B {}\n")
			s := gb.StampRecords()[0].Stamp
			assert.Equal(t, s.Key, typescript.NamespaceKey, "under the namespace key")
			assert.Equal(t, s.Value, any("A.B"), "the dotted name")
		})

		t.Run("spells a nested namespace's dotted name through its parent", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export namespace A {\n  export namespace B {}\n}\n")
			assert.Equal(t, gb.StampRecords()[1].Stamp.Value, any("A.B"), "B inside A is A.B")
		})

		t.Run("merges two declarations of one namespace in a file into one File node", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export namespace A { export class X {} }\n"+
				"export namespace A { export class Y {} }\n")
			assert.Length(t, fileIn(t, gb, outerPackage).Decls, 2, "both classes are in one File node")
		})

		t.Run("lowers a namespace member it does not export as package-visible", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export namespace A {\n  class Hidden {}\n}\n")
			assert.Equal(t, fileIn(t, gb, outerPackage).Decls[0].(*node.Struct).Visibility,
				symbol.VisibilityPackage, "the namespace alone can name it")
		})

		t.Run("lowers an ambient namespace's members as public", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export declare namespace A {\n  class Shown {}\n}\n")
			assert.Equal(t, fileIn(t, gb, outerPackage).Decls[0].(*node.Struct).Visibility,
				symbol.VisibilityPublic, "an ambient namespace exports implicitly")
		})

		t.Run("declares an ambient module's members into the package its name spells", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "declare module 'lib' {\n  export interface I {}\n}\nexport {};\n")
			assert.Length(t, fileIn(t, gb, modulePkg).Decls, 1, "a bare import of lib names it")
		})

		t.Run("declares a module block's members into its namespace package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export {};\nmodule A.B {\n  export class X {}\n}\n")
			assert.Length(t, fileIn(t, gb, nsPackage).Decls, 1, "module A.B is namespace A.B")
		})

		t.Run("declares a script's namespace below the global package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "namespace A {\n  class X {}\n}\n")
			assert.Length(t, fileIn(t, gb, "A").Decls, 1, "the global package's namespace A")
		})

		t.Run("leaves out a namespace the module does not export at signature depth", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, fstest.MapFS{aFile: {Data: []byte(
				"export {};\nnamespace Hidden {\n  export class X {}\n}\n",
			)}}, aFile, plugin.DepthSignatures)
			assert.Equal(t, packagePaths(gb), []string{aPackage}, "no other module can name it")
		})

		t.Run("stamps typescript.ambient on a member of a declared namespace", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export declare namespace A {\n  class Shown {}\n}\n")
			_, stamped := stampOn(gb, fileIn(t, gb, outerPackage).Decls[0], typescript.AmbientKey)
			assert.True(t, stamped, "a declared namespace implements nothing")
		})

		t.Run("stamps typescript.ambient on a member of an ambient module", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "declare module 'lib' {\n  export interface I {}\n}\nexport {};\n")
			_, stamped := stampOn(gb, fileIn(t, gb, modulePkg).Decls[0], typescript.AmbientKey)
			assert.True(t, stamped, "an ambient module describes one implemented elsewhere")
		})

		t.Run("stamps no typescript.ambient on a member of a script's namespace", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "namespace A {\n  class X {}\n}\n")
			_, stamped := stampOn(gb, fileIn(t, gb, "A").Decls[0], typescript.AmbientKey)
			assert.False(t, stamped, "a script's namespace is public and implemented in place")
		})
	})

	t.Run("ambient", func(t *testing.T) {
		t.Parallel()

		t.Run("declares a declare global block's members into the global package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export {};\ndeclare global {\n  interface Window { x: number }\n}\n")
			assert.Length(t, fileIn(t, gb, "").Decls, 1, "the global scope gains the interface")
		})

		t.Run("lowers one ambient declaration in place", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, declsOf(t, "export declare class A {}\n"), 1, "declare states a declaration")
		})

		t.Run("lowers an ambient declaration after a comment inside the statement", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, declsOf(t, "export declare /* the A */ class A {}\n"), 1, "the comment declares nothing")
		})

		t.Run("stamps typescript.ambient on a declare global block's declaration", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export {};\ndeclare global {\n  interface Window { x: number }\n}\n")
			_, stamped := stampOn(gb, fileIn(t, gb, "").Decls[0], typescript.AmbientKey)
			assert.True(t, stamped, "the block describes the global scope's existing members")
		})
	})

	t.Run("reportSyntax", func(t *testing.T) {
		t.Parallel()

		t.Run("reports UnparsedFile at a syntax error", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export class A {\n  a: = ;\n}\n")
			assert.NotEmpty(t, found, "the error reports")
			assert.Equal(t, found[0].Code, frontend.UnparsedFile, "under the syntax error code")
			assert.Equal(t, found[0].Pos.Line, 2, "at the line it is on")
		})

		t.Run("reports a missing token as missing", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export let x = (1;\n")
			assert.Length(t, found, 1, "one finding")
			assert.True(t, strings.Contains(found[0].Msg, "missing"), "the parser inserted the parenthesis")
		})

		t.Run("quotes at most 32 bytes of the source that does not parse", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export class A {\n  "+strings.Repeat("#", 40)+"\n}\n")
			assert.NotEmpty(t, found, "the error reports")
			assert.True(t, strings.Contains(found[0].Msg, strings.Repeat("#", 32)+"..."), "the quote is cut")
		})

		t.Run("reports at most ten syntax errors and counts the remainder once", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export {};\n"+strings.Repeat("let = ;\n", 20))
			assert.Length(t, found, 11, "ten errors and the remainder")
			assert.True(t, strings.Contains(found[10].Msg, "more"), "the last finding counts the rest")
		})

		t.Run("keeps every declaration a broken file still parses", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "export class A {}\nexport let = ;\nexport class B {}\n")
			assert.Length(t, decls, 2, "one bad statement does not erase the file")
		})
	})

	t.Run("statements", func(t *testing.T) {
		t.Parallel()

		t.Run("reports no finding for an import or a comment", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "// A comment.\nimport { A } from './a';\n")
			assert.Empty(t, found, "neither declares anything")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a statement", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export {};\n"+carrierLine+"run();\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "a statement is no subject")
		})
	})
}

// packagePaths returns the paths of a builder's packages, in first
// touch order.
func packagePaths(gb *plugin.GraphBuilder) []string {
	var out []string
	for _, p := range gb.Packages() {
		out = append(out, p.ID.Package)
	}
	return out
}
