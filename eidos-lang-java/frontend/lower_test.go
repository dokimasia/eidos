// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The files that document a package and declare a module, in the
// directory of the fixture file.
const (
	packageInfo = "src/main/java/com/acme/package-info.java"
	moduleInfo  = "src/main/java/module-info.java"
)

// The messages that the syntax cases expect: the pattern of the finding
// that counts the errors past the cap, the suffix of the finding of a
// missing node, and the mark of a quoted line that the finding cuts short.
const (
	moreSyntax  = `^and \d+ more syntax errors$`
	missingMark = "is missing here"
	cutMark     = "..."
)

// The annotations of the generated cases, as a generator writes them on a
// top-level type, and the imports that bind their simple name.
const (
	generatedMark    = `@javax.annotation.processing.Generated("com.acme.Gen")`
	legacyMark       = `@javax.annotation.Generated("com.acme.Gen")`
	simpleMark       = `@Generated("com.acme.Gen")`
	generatedImport  = "import javax.annotation.processing.Generated;\n"
	processingImport = "import javax.annotation.processing.*;\n"
	foreignImport    = "import com.acme.meta.Generated;\n"
)

// One file lowers into its package through one walk. The cases pin the
// package that the file declares into, its imports, its generated stamp
// and the findings of its syntax errors.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("lower", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the types of a file into the package that its package clause names", func(t *testing.T) {
			t.Parallel()

			named[*node.Struct](t, declsOf(t, publicClass), "A")
		})

		t.Run("names a package after the last name of its package clause", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+publicClass)
			assert.Equal(t, packageIn(t, gb, pkgPath).Name, "acme", "the last name")
		})

		t.Run("declares a file without a package clause into the unnamed package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, publicClass)
			named[*node.Struct](t, fileIn(t, gb, "").Decls, "A")
		})

		t.Run("gives the unnamed package no name", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, publicClass)
			assert.Equal(t, packageIn(t, gb, "").Name, "", "the unnamed package has no name")
		})

		t.Run("reads a package clause written across lines", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "package com.\n    acme;\n\n"+publicClass)
			named[*node.Struct](t, fileIn(t, gb, pkgPath).Decls, "A")
		})

		t.Run("documents a package with the Javadoc of its package-info.java", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedPackageInfo(t, "/** The acme package. */\npackage com.acme;\n")
			assert.Equal(t, packageIn(t, gb, pkgPath).Doc, []string{"The acme package."}, "the Javadoc")
		})

		t.Run("attaches the carriers of package-info.java to the package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedPackageInfo(t, carrierLine+"package com.acme;\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onPackage := gb.Attachments()[0].Subject.(*node.Package)
			assert.True(t, onPackage, "the subject is the package")
		})

		t.Run("annotates the File node of package-info.java with the annotations of the package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedPackageInfo(t, "@Deprecated\npackage com.acme;\n")
			assert.Equal(t, fileIn(t, gb, pkgPath).Annotations, symbol.Annotations{{Name: "Deprecated"}},
				"the package annotation")
		})

		t.Run("documents no package with the Javadoc above the package clause of another file", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "/** Not the package's. */\n"+pkgClause+publicClass)
			assert.Empty(t, packageIn(t, gb, pkgPath).Doc, "package-info.java alone documents a package")
		})

		t.Run("stamps java.module with the name of the module on the File node of module-info.java",
			func(t *testing.T) {
				t.Parallel()

				gb, _ := parsedModuleInfo(t)
				assert.Equal(t, stampsOf(gb, java.ModuleKey), []any{"com.acme.store"}, "the name of the module")
			})

		t.Run("declares no type for module-info.java", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedModuleInfo(t)
			assert.Empty(t, fileIn(t, gb, "").Decls, "a module declaration is not a type")
		})

		t.Run("leaves out the members of a compact source file at signature depth", func(t *testing.T) {
			t.Parallel()

			gb, found := shallowSource(t, carrierLine+compactSource)
			assert.Empty(t, fileIn(t, gb, "").Decls, "the class has package access")
			assert.Empty(t, found, "the carrier leaves with its method")
		})

		t.Run("reports UnaddressedCarrier for a carrier that no declaration takes", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+carrierLine+"\n"+publicClass)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier},
				"a blank line separates the carrier from the class")
		})

		t.Run("describes a comment that no declaration takes as the subject of its carrier", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+carrierLine+"\n"+publicClass)
			assert.NotEmpty(t, found, "the carrier reports")
			assert.Contains(t, found[0].Msg, onNothing, "the comment is apart from every declaration")
		})

		t.Run("quotes a carrier that no declaration takes as the author wrote it", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+carrierLine+"\n"+publicClass)
			assert.NotEmpty(t, found, "the carrier reports")
			assert.Contains(t, found[0].Msg, carrierText, "the finding quotes the carrier")
		})

		t.Run("reports UnaddressedCarrier for a carrier in a class body that no member takes", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, pkgClause+"public class A {\n    int a;\n\n    "+carrierLine+"}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier},
				"the carrier is on no member")
		})

		generated := []struct {
			name string
			give string
			want []any
		}{
			{
				name: "stamps java.generated with javax.annotation.processing.Generated as written",
				give: generatedMark + "\n" + publicClass, want: []any{generatedMark},
			},
			{
				name: "stamps java.generated with javax.annotation.Generated as written",
				give: legacyMark + "\n" + publicClass, want: []any{legacyMark},
			},
			{
				name: "stamps java.generated for a simple name that a single-type import binds",
				give: generatedImport + "\n" + simpleMark + "\n" + publicClass, want: []any{simpleMark},
			},
			{
				name: "stamps java.generated for a simple name that an on-demand import imports",
				give: processingImport + "\n" + simpleMark + "\n" + publicClass, want: []any{simpleMark},
			},
			{
				name: "stamps java.generated once for two generated types",
				give: generatedMark + "\n" + publicClass + "\n" + legacyMark + "\nclass B {}\n",
				want: []any{generatedMark},
			},
			{
				name: "stamps no java.generated for a simple name that a single-type import binds to another type",
				give: foreignImport + processingImport + "\n" + simpleMark + "\n" + publicClass,
			},
			{
				name: "stamps no java.generated for a simple name that no import binds",
				give: simpleMark + "\n" + publicClass,
			},
			{
				name: "stamps no java.generated for the annotation on a member type",
				give: "public class A {\n    " + generatedMark + "\n    public static class B {}\n}\n",
			},
		}
		for _, tt := range generated {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, _ := parsedSource(t, pkgClause+tt.give)
				assert.Equal(t, stampsOf(gb, java.GeneratedKey), tt.want, "the stamps of the file")
			})
		}

		t.Run("stamps java.generated where the signature depth leaves the generated type out", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, fstest.MapFS{srcFile: {Data: []byte(pkgClause + generatedMark + "\nclass A {}\n")}},
				srcFile, plugin.DepthSignatures)
			assert.Equal(t, stampsOf(gb, java.GeneratedKey), []any{generatedMark}, "the stamp of the file")
		})

		t.Run("reports a MISSING node as missing", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "public class A {\n    int a\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnparsedFile}, "the missing semicolon reports")
			assert.HasSuffix(t, found[0].Msg, missingMark, "the finding states that the node is missing")
		})

		t.Run("reports an ERROR node with a quote of its source", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "public class A {\n    %% bad\n}\n")
			assert.NotEmpty(t, found, "the error reports")
			assert.Contains(t, found[0].Msg, "%%", "the finding quotes the source")
		})

		t.Run("quotes the first line of an ERROR node that spans two lines", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "public class A {\n    %% one\n    %% two\n}\n")
			assert.NotEmpty(t, found, "the error reports")
			assert.NotContains(t, found[0].Msg, "two", "the finding leaves out the second line")
		})

		t.Run("cuts a quoted line at its cap", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "public class A {\n    %% "+strings.Repeat("x", 40)+"\n}\n")
			assert.NotEmpty(t, found, "the error reports")
			assert.Contains(t, found[0].Msg, cutMark, "the finding cuts the quote")
		})

		t.Run("reports one finding more than the cap for the syntax errors of a file", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, strings.Repeat("class A { %% }\n", 12))
			assert.Length(t, found, plugin.MaxSyntaxFindings+1, "the findings up to the cap and the count of the rest")
		})

		t.Run("reports the syntax errors past the cap in one finding", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, strings.Repeat("class A { %% }\n", 12))
			assert.Matches(t, found[len(found)-1].Msg, moreSyntax, "the last finding counts the rest")
		})
	})

	t.Run("importDecl", func(t *testing.T) {
		t.Parallel()

		t.Run("records the package of a single-type import as the path of its Import", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importOf(t, "import java.util.List;\n").Path, "java/util", "the package")
		})

		t.Run("binds the simple name of a single-type import", func(t *testing.T) {
			t.Parallel()

			imp := importOf(t, "import java.util.List;\n")
			assert.Equal(t, imp.Names[0].Name, "List", "the simple name")
		})

		t.Run("records an on-demand import as a wildcard Import of its package", func(t *testing.T) {
			t.Parallel()

			imp := importOf(t, "import java.util.*;\n")
			assert.Equal(t, imp.Path, "java/util", "the package")
			assert.True(t, imp.Wildcard, "the Import covers every type of the package")
		})

		t.Run("records the type of a static import as the path of its Import", func(t *testing.T) {
			t.Parallel()

			imp := importOf(t, "import static java.util.Collections.emptyList;\n")
			assert.Equal(t, imp.Path, "java/util/Collections", "the type that declares the member")
		})

		t.Run("records a static on-demand import as a wildcard Import of its type", func(t *testing.T) {
			t.Parallel()

			imp := importOf(t, "import static java.util.Collections.*;\n")
			assert.Equal(t, imp.Path, "java/util/Collections", "the type")
			assert.True(t, imp.Wildcard, "the Import covers every static member of the type")
		})

		t.Run("keeps the first single-type import of a simple name", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, pkgClause+"import a.X;\nimport b.X;\n"+publicClass, pkgPath)
			assert.Equal(t, resolved(scope, probeName)[0], []symbol.Identity{id("a", probeName)},
				"Java refuses the second import of X")
		})
	})
}

// parsedPackageInfo parses a package-info.java with a source in the
// directory of the fixture file, and returns the builder and the findings
// of the unit.
func parsedPackageInfo(tb testing.TB, src string) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	return parsedTree(tb, fstest.MapFS{packageInfo: {Data: []byte(src)}}, packageInfo, plugin.DepthFull)
}

// parsedModuleInfo parses a module-info.java that declares the module
// com.acme.store, and returns the builder and the findings of the unit.
func parsedModuleInfo(tb testing.TB) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	return parsedTree(tb, fstest.MapFS{
		moduleInfo: {Data: []byte("module com.acme.store {\n    requires java.sql;\n}\n")},
	}, moduleInfo, plugin.DepthFull)
}

// importOf parses one import declaration below the package clause of
// com.acme and returns the Import that the File node records.
func importOf(tb testing.TB, decl string) *node.Import {
	tb.Helper()

	gb, _ := parsedSource(tb, pkgClause+decl+publicClass)
	file := fileIn(tb, gb, pkgPath)
	assert.Length(tb, file.Imports, 1, "the one import")
	return file.Imports[0]
}
