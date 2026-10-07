// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The files that document a package and declare a module, in the
// fixture file's directory.
const (
	packageInfo = "src/main/java/com/acme/package-info.java"
	moduleInfo  = "src/main/java/module-info.java"
)

// The messages the syntax cases expect: the one that counts the
// findings past the cap, the suffix of a missing node's, and the mark of
// a quoted line cut short.
const (
	moreSyntax  = "and more syntax errors"
	missingMark = "is missing here"
	cutMark     = "..."
)

// One file lowers into its package through one walk, so the package it
// names, its imports, and what a syntax error reports, is pinned.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("lower", func(t *testing.T) {
		t.Parallel()

		t.Run("declares a file's types into the package its package clause names", func(t *testing.T) {
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

		t.Run("names no unnamed package", func(t *testing.T) {
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
			assert.True(t, onPackage, "on the package")
		})

		t.Run("annotates the File node of package-info.java with the package's annotations", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedPackageInfo(t, "@Deprecated\npackage com.acme;\n")
			assert.Equal(t, fileIn(t, gb, pkgPath).Annotations, symbol.Annotations{{Name: "Deprecated"}},
				"the package annotation")
		})

		t.Run("documents no package with the Javadoc above another file's package clause", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "/** Not the package's. */\n"+pkgClause+publicClass)
			assert.Empty(t, packageIn(t, gb, pkgPath).Doc, "package-info.java alone documents a package")
		})

		t.Run("stamps the File node of module-info.java java.module with the module's name", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedModuleInfo(t)
			assert.Equal(t, stampsOf(gb, java.ModuleKey), []any{"com.acme.store"}, "the module's name")
		})

		t.Run("declares no type for module-info.java", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedModuleInfo(t)
			assert.Empty(t, fileIn(t, gb, "").Decls, "a module declaration is no type")
		})

		t.Run("leaves out the members of a compact source file at signature depth", func(t *testing.T) {
			t.Parallel()

			gb, found := shallowSource(t, carrierLine+compactSource)
			assert.Empty(t, fileIn(t, gb, "").Decls, "the class has package access")
			assert.Empty(t, found, "and the carrier leaves with its method")
		})
	})

	t.Run("importDecl", func(t *testing.T) {
		t.Parallel()

		t.Run("records a single-type import's package as its Import's path", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importOf(t, "import java.util.List;\n").Path, "java/util", "the package")
		})

		t.Run("binds a single-type import's simple name", func(t *testing.T) {
			t.Parallel()

			imp := importOf(t, "import java.util.List;\n")
			assert.Equal(t, imp.Names[0].Name, "List", "the simple name")
		})

		t.Run("records an on-demand import as a wildcard Import of its package", func(t *testing.T) {
			t.Parallel()

			imp := importOf(t, "import java.util.*;\n")
			assert.Equal(t, imp.Path, "java/util", "the package")
			assert.True(t, imp.Wildcard, "every type of it")
		})

		t.Run("records a static import's type as its Import's path", func(t *testing.T) {
			t.Parallel()

			imp := importOf(t, "import static java.util.Collections.emptyList;\n")
			assert.Equal(t, imp.Path, "java/util/Collections", "the type that declares the member")
		})

		t.Run("records a static on-demand import as a wildcard Import of its type", func(t *testing.T) {
			t.Parallel()

			imp := importOf(t, "import static java.util.Collections.*;\n")
			assert.Equal(t, imp.Path, "java/util/Collections", "the type")
			assert.True(t, imp.Wildcard, "every static member of it")
		})

		t.Run("keeps a simple name's first single-type import", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, pkgClause+"import a.X;\nimport b.X;\n"+publicClass, pkgPath)
			assert.Equal(t, resolved(scope, probeName)[0], []symbol.Identity{id("a", probeName)},
				"the second import of X is Java's error")
		})
	})

	t.Run("reportSyntax", func(t *testing.T) {
		t.Parallel()

		t.Run("reports a MISSING node as missing", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "public class A {\n    int a\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnparsedFile}, "the missing semicolon reports")
			assert.HasSuffix(t, found[0].Msg, missingMark, "as missing")
		})

		t.Run("reports an ERROR node quoting its source", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "public class A {\n    %% bad\n}\n")
			assert.NotEmpty(t, found, "the error reports")
			assert.Contains(t, found[0].Msg, "%%", "quoting the source")
		})

		t.Run("quotes the first line of an ERROR node that spans two", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "public class A {\n    %% one\n    %% two\n}\n")
			assert.NotEmpty(t, found, "the error reports")
			assert.NotContains(t, found[0].Msg, "two", "the second line is left out")
		})

		t.Run("cuts a quoted line at its cap", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "public class A {\n    %% "+strings.Repeat("x", 40)+"\n}\n")
			assert.NotEmpty(t, found, "the error reports")
			assert.Contains(t, found[0].Msg, cutMark, "the quote is cut")
		})

		t.Run("reports at most eleven findings for a file's syntax errors", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, strings.Repeat("class A { %% }\n", 12))
			assert.Length(t, found, 11, "ten findings and the remainder")
		})

		t.Run("reports the findings past the cap once", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, strings.Repeat("class A { %% }\n", 12))
			assert.Equal(t, found[len(found)-1].Msg, moreSyntax, "counted once")
		})
	})
}

// parsedPackageInfo parses a package-info.java of a source in the
// fixture file's directory, and returns the unit's builder and findings.
func parsedPackageInfo(tb testing.TB, src string) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	return parsedTree(tb, fstest.MapFS{packageInfo: {Data: []byte(src)}}, packageInfo, plugin.DepthFull)
}

// parsedModuleInfo parses a module-info.java that declares the module
// com.acme.store, and returns the unit's builder and findings.
func parsedModuleInfo(tb testing.TB) (*plugin.GraphBuilder, []diag.Diag) {
	tb.Helper()

	return parsedTree(tb, fstest.MapFS{
		moduleInfo: {Data: []byte("module com.acme.store {\n    requires java.sql;\n}\n")},
	}, moduleInfo, plugin.DepthFull)
}

// importOf parses one import declaration below the package clause of
// com.acme and returns the Import the File node records.
func importOf(tb testing.TB, decl string) *node.Import {
	tb.Helper()

	gb, _ := parsedSource(tb, pkgClause+decl+publicClass)
	file := fileIn(tb, gb, pkgPath)
	assert.Length(tb, file.Imports, 1, "the one import")
	return file.Imports[0]
}
