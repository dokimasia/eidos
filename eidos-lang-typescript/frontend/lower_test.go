// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"strings"
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

// The packages into which the namespace cases declare.
const (
	nsPackage    = aPackage + "/A/B"
	outerPackage = aPackage + "/A"
	modulePkg    = "lib"
)

// moreSyntax is the pattern of the finding that counts the syntax errors
// past the cap.
const moreSyntax = `^and \d+ more syntax errors$`

// relativeAugmentation is a module that augments the module of a relative
// specifier.
const relativeAugmentation = "export {};\ndeclare module './user' {\n  interface User { n: string }\n}\n"

// The statement walk decides the package of each declaration and what the
// module publishes. The cases pin its rules, the sweep over the comments
// that no declaration takes, and the findings of syntax errors.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("lower", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the declarations of a script into the global package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "class A {}\n")
			assert.Equal(t, packagePaths(gb), []string{""}, "a file without import or export is a script")
		})

		t.Run("lowers the declarations of a script as public", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "class A {}\n")
			assert.Equal(t, fileIn(t, gb, "").Decls[0].(*node.Struct).Visibility, symbol.VisibilityPublic,
				"every file shares the global scope")
		})

		t.Run("reports UnaddressedCarrier for a carrier that no declaration takes", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export class A {}\n\n"+carrierLine+"\nexport class B {}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier},
				"blank lines separate the carrier from both classes")
		})

		headers := []struct {
			name string
			give string
		}{
			{
				name: "attaches a carrier above a blank line at the top of a module to its package",
				give: carrierLine + "\nexport class A {}\n",
			},
			{
				name: "attaches a carrier below a license header to the package of the module",
				give: "// Copyright Acme.\n\n" + carrierLine + "\nexport class A {}\n",
			},
			{
				name: "attaches a carrier directly above the first import of a module to its package",
				give: "// The errors of the shop.\n" + carrierLine + "import { A } from './lib';\n",
			},
			{
				name: "attaches a carrier below the hashbang line of a module to its package",
				give: "#!/usr/bin/env node\n" + carrierLine + "\nexport class A {}\n",
			},
		}
		for _, tt := range headers {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, tt.give)
				assert.Empty(t, found, "the package takes the carrier")
				assert.Length(t, gb.Attachments(), 1, "one directive")
				assert.Equal(t, gb.Attachments()[0].Subject, symbol.Symbol(packageIn(t, gb, aPackage)),
					"the package of the module is the subject", assert.ByIdentity())
			})
		}

		t.Run("leaves the doc of the package unchanged by the header of a module", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "// The errors of the shop.\n"+carrierLine+"\nexport class A {}\n")
			assert.Empty(t, packageIn(t, gb, aPackage).Doc, "a header is often a license")
		})

		t.Run("leaves the carrier directly above the first declaration to the declaration", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "// Copyright Acme.\n\n"+carrierLine+"export class A {}\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onClass := gb.Attachments()[0].Subject.(*node.Struct)
			assert.True(t, onClass, "the class takes the carrier above it")
		})

		t.Run("reports UnaddressedCarrier for a carrier at the top of a script", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, carrierLine+"\nclass A {}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier},
				"a script declares into the global package of every script")
		})

		t.Run("reports UnaddressedCarrier for a carrier in a namespace that no declaration takes", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export namespace N {\n  "+carrierLine+"\n  export class A {}\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier},
				"the block of a namespace contains declarations, and the carrier is on none of them")
		})

		t.Run("reports UnaddressedCarrier for a carrier in a class body that no member takes", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export class A {\n  a = 1;\n\n  "+carrierLine+"\n  b = 2;\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "the carrier is on no member")
		})

		t.Run("reports UnparsedFile at a syntax error", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export class A {\n  a: = ;\n}\n")
			assert.NotEmpty(t, found, "the error reports")
			assert.Equal(t, found[0].Code, frontend.UnparsedFile, "the code of a syntax error")
			assert.Equal(t, found[0].Pos.Line, 2, "the line of the error")
		})

		t.Run("reports a missing token as missing", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export let x = (1;\n")
			assert.Length(t, found, 1, "one finding")
			assert.Contains(t, found[0].Msg, "missing", "the parser inserted the parenthesis")
		})

		t.Run("quotes at most 32 bytes of the source that does not parse", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export class A {\n  "+strings.Repeat("#", 40)+"\n}\n")
			assert.NotEmpty(t, found, "the error reports")
			assert.Contains(t, found[0].Msg, strings.Repeat("#", 32)+"...", "the finding cuts the quote")
		})

		t.Run("reports one finding more than the cap for the syntax errors of a file", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export {};\n"+strings.Repeat("let = ;\n", 20))
			assert.Length(t, found, plugin.MaxSyntaxFindings+1, "the findings up to the cap and the count of the rest")
		})

		t.Run("reports the syntax errors past the cap in one finding", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export {};\n"+strings.Repeat("let = ;\n", 20))
			assert.Matches(t, found[len(found)-1].Msg, moreSyntax, "the last finding counts the rest")
		})

		t.Run("keeps every declaration that the parser recovers from a broken file", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "export class A {}\nexport let = ;\nexport class B {}\n")
			assert.Length(t, decls, 2, "one bad statement does not erase the file")
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
			assert.Equal(t, imports[0].Default, "D", "the local name of the default")
			assert.Equal(t, imports[1].Alias, "N", "the local name of the namespace")
		})

		t.Run("records an import-require as a namespace import", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "import F = require('f');\nexport {};\n")
			imp := fileIn(t, gb, aPackage).Imports[0]
			assert.Equal(t, imp.Alias, "F", "the local name of the module")
			assert.Equal(t, imp.Path, "f", "the module")
		})

		mapped := fstest.MapFS{
			rootConfig: {Data: []byte(`{"compilerOptions": {"paths": {"@lib/*": ["lib/*"]}}}`)},
			aFile: {Data: []byte("import { A } from '@lib/x';\nimport { B } from './x';\nimport { C } from 'other';\n" +
				"export { D } from '@lib/x';\n")},
		}
		mappedModules := []string{"lib/x", "lib/x/index", "@lib/x"}

		t.Run("records the packages that a paths pattern maps the specifier of an import onto", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, mapped, aFile, plugin.DepthFull)
			assert.Equal(t, fileIn(t, gb, aPackage).Imports[0].Modules, mappedModules,
				"the substitution as a file and an index, then the specifier")
		})

		t.Run("records no packages for a relative specifier", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, mapped, aFile, plugin.DepthFull)
			assert.Nil(t, fileIn(t, gb, aPackage).Imports[1].Modules, "the rules resolve a relative specifier alike")
		})

		t.Run("records no packages for a specifier that the tsconfig does not map", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, mapped, aFile, plugin.DepthFull)
			assert.Nil(t, fileIn(t, gb, aPackage).Imports[2].Modules, "the specifier names its ambient module")
		})

		t.Run("records the packages that a paths pattern maps the source of a re-export onto", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, mapped, aFile, plugin.DepthFull)
			assert.Equal(t, fileIn(t, gb, aPackage).Exports[0].Modules, mappedModules,
				"a re-export resolves its source as an import does")
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
			assert.Equal(t, b.Name+" "+b.Alias, "* M", "the namespace of the module, published as M")
		})

		t.Run("records the local name of the default export", func(t *testing.T) {
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
			assert.Empty(t, fileIn(t, gb, aPackage).Exports, "the export of the declaration is its visibility")
		})

		t.Run("records an anonymous default export under default", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export default class {}\n")
			assert.Equal(t, fileIn(t, gb, aPackage).Exports[0].Default, "default", "the name of the class is default")
		})

		t.Run("records the name of a default-exported declaration as the default export", func(t *testing.T) {
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

		t.Run("records the doc and the trailing comment of an import", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "import { B } from './b';\n// The library.\nimport { A } from './lib'; // for A\n")
			imp := fileIn(t, gb, aPackage).Imports[1]
			expect.Equal(t, imp.Doc, []string{"The library."}, "the doc of the import")
			expect.Equal(t, imp.Comment, "for A", "the trailing comment of the import")
		})

		t.Run("leaves the comment above the first import to the header of the module", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "// The library.\nimport { A } from './lib';\n")
			assert.Empty(t, fileIn(t, gb, aPackage).Imports[0].Doc, "the first comment of a module is its header")
		})

		refused := []struct {
			name string
			give string
		}{
			{
				name: "reports UnaddressedCarrier for a carrier on an import",
				give: "import { B } from './b';\n" + carrierLine + "import { A } from './lib';\n",
			},
			{
				name: "reports UnaddressedCarrier for a trailing carrier on the first import",
				give: "import { A } from './lib'; " + carrierLine,
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, tt.give)
				assert.Length(t, found, 1, "the carrier refuses")
				assert.Equal(t, found[0].Code, frontend.UnaddressedCarrier, "as a carrier that no rule takes")
				assert.Empty(t, gb.Attachments(), "nothing attaches")
			})
		}

		t.Run("records the doc of a re-export", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "// The names of the library.\nexport { J } from './j';\n")
			assert.Equal(t, fileIn(t, gb, aPackage).Exports[0].Doc, []string{"The names of the library."},
				"the doc of the export")
		})

		t.Run("leaves the doc of an exported declaration to the declaration", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "// Foo is the default.\nexport default class Foo {}\n")
			file := fileIn(t, gb, aPackage)
			expect.Empty(t, file.Exports[0].Doc, "the export has no doc of its own")
			expect.Equal(t, named[*node.Struct](t, file.Decls, "Foo").Doc, []string{"Foo is the default."},
				"the class has the doc")
		})

		t.Run("leaves the doc of an anonymous default class to the class", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export {};\n// The default.\nexport default class {}\n")
			file := fileIn(t, gb, aPackage)
			expect.Empty(t, file.Exports[1].Doc, "the export has no doc of its own")
			expect.Equal(t, named[*node.Struct](t, file.Decls, typescript.DefaultExport).Doc, []string{"The default."},
				"the class has the doc")
		})

		t.Run("attaches a carrier above an anonymous default class to the class", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, "export {};\n"+carrierLine+"export default class {}\n")
			assert.Empty(t, found, "the class takes the carrier")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onClass := gb.Attachments()[0].Subject.(*node.Struct)
			assert.True(t, onClass, "the anonymous class is the subject")
		})

		t.Run("skips a comment among the names of an import", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "import { A, /* and */ B } from './x';\n")
			assert.Length(t, fileIn(t, gb, aPackage).Imports[0].Names, 2, "a comment binds nothing")
		})

		t.Run("skips a comment among the names of an export clause", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export { A, /* and */ B } from './x';\n")
			assert.Length(t, fileIn(t, gb, aPackage).Exports[0].Names, 2, "a comment publishes nothing")
		})
	})

	t.Run("namespace", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the members of a dotted namespace into its package below the package of the file",
			func(t *testing.T) {
				t.Parallel()

				gb, _ := parsedSource(t, "export namespace A.B {\n  export interface I {}\n}\n")
				assert.Length(t, fileIn(t, gb, nsPackage).Decls, 1, "the interface is in src/a/A/B")
			})

		t.Run("stamps the package of a namespace with its dotted name", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export namespace A.B {}\n")
			s := gb.StampRecords()[0].Stamp
			assert.Equal(t, s.Key, typescript.NamespaceKey, "the namespace key")
			assert.Equal(t, s.Value, any("A.B"), "the dotted name")
		})

		t.Run("spells the dotted name of a nested namespace through its parent", func(t *testing.T) {
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

		t.Run("lowers a namespace member that the namespace does not export as package-visible", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export namespace A {\n  class Hidden {}\n}\n")
			assert.Equal(t, fileIn(t, gb, outerPackage).Decls[0].(*node.Struct).Visibility,
				symbol.VisibilityPackage, "only the namespace can refer to it")
		})

		t.Run("lowers the members of an ambient namespace as public", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export declare namespace A {\n  class Shown {}\n}\n")
			assert.Equal(t, fileIn(t, gb, outerPackage).Decls[0].(*node.Struct).Visibility,
				symbol.VisibilityPublic, "an ambient namespace exports implicitly")
		})

		t.Run("declares the members of an ambient module into the package of its name", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "declare module 'lib' {\n  export interface I {}\n}\nexport {};\n")
			assert.Length(t, fileIn(t, gb, modulePkg).Decls, 1, "a bare import of lib refers to it")
		})

		t.Run("reports RelativeAugmentation for an augmentation of a relative module", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, relativeAugmentation)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.RelativeAugmentation},
				"the parse of one file cannot name the module of a relative specifier")
		})

		t.Run("loads nothing from an augmentation of a relative module", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, relativeAugmentation)
			assert.Equal(t, packagePaths(gb), []string{aPackage}, "no package has the spelling of the specifier")
		})

		t.Run("declares the members of a module block into its namespace package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export {};\nmodule A.B {\n  export class X {}\n}\n")
			assert.Length(t, fileIn(t, gb, nsPackage).Decls, 1, "module A.B is namespace A.B")
		})

		t.Run("declares the namespace of a script below the global package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "namespace A {\n  class X {}\n}\n")
			assert.Length(t, fileIn(t, gb, "A").Decls, 1, "the namespace A of the global package")
		})

		t.Run("lowers a member that the namespace of a script does not export as package-visible", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "namespace A {\n  class Hidden {}\n}\n")
			assert.Equal(t, fileIn(t, gb, "A").Decls[0].(*node.Struct).Visibility, symbol.VisibilityPackage,
				"another file cannot refer to A.Hidden")
		})

		t.Run("leaves out a namespace that the module does not export at signature depth", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, fstest.MapFS{aFile: {Data: []byte(
				"export {};\nnamespace Hidden {\n  export class X {}\n}\n",
			)}}, aFile, plugin.DepthSignatures)
			assert.Equal(t, packagePaths(gb), []string{aPackage}, "no other module can refer to it")
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

		t.Run("stamps no typescript.ambient on a member of the namespace of a script", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "namespace A {\n  class X {}\n}\n")
			_, stamped := stampOn(gb, fileIn(t, gb, "A").Decls[0], typescript.AmbientKey)
			assert.False(t, stamped, "the namespace of a script is public and implemented in place")
		})
	})

	t.Run("ambient", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the members of a declare global block into the global package", func(t *testing.T) {
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

		t.Run("stamps typescript.ambient on the declaration of a declare global block", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export {};\ndeclare global {\n  interface Window { x: number }\n}\n")
			_, stamped := stampOn(gb, fileIn(t, gb, "").Decls[0], typescript.AmbientKey)
			assert.True(t, stamped, "the block describes the existing members of the global scope")
		})
	})

	t.Run("statements", func(t *testing.T) {
		t.Parallel()

		t.Run("reports no finding for the statements that declare nothing", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "// A comment.\nimport { A } from './a';\n")
			assert.Empty(t, found, "neither the comment nor the import declares anything")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a statement", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export {};\n"+carrierLine+"run();\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "a statement is not a subject")
		})
	})
}

// packagePaths returns the paths of the packages of a builder, in the order
// of first use.
func packagePaths(gb *plugin.GraphBuilder) []string {
	pkgs := gb.Packages()
	out := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		out = append(out, p.ID.Package)
	}
	return out
}
