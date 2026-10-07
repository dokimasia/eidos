// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The name the resolution cases probe, and the package every compilation
// unit imports on demand.
const (
	probeName = "X"
	javaLang  = "java/lang"
)

// Resolution follows the Java Language Specification's shadowing order
// through the bindings the parse recorded, so the tiers each spelling
// probes are pinned.
func TestScope(t *testing.T) {
	t.Parallel()

	t.Run("resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves nothing for a file whose parse recorded no scope", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, resolved(plugin.ImportScope{}, probeName), "no bindings to resolve through")
		})
	})

	t.Run("simple", func(t *testing.T) {
		t.Parallel()

		t.Run("probes the package and then java.lang for a name nothing imports", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, resolved(plainScope(t), probeName), plugin.Candidates{
				{id(pkgPath, probeName)}, {id(javaLang, probeName)},
			}, "the file's package shadows java.lang")
		})

		t.Run("probes the member types of the enclosing type first", func(t *testing.T) {
			t.Parallel()

			scope := plainScope(t)
			scope.Owner = nested(pkgPath, "", "A")
			assert.Equal(t, resolved(scope, probeName)[0], []symbol.Identity{nested(pkgPath, "A", probeName)},
				"a member type of A")
		})

		t.Run("probes each enclosing type innermost first", func(t *testing.T) {
			t.Parallel()

			scope := plainScope(t)
			scope.Owner = nested(pkgPath, "A", "B")
			assert.Equal(t, resolved(scope, probeName)[:2], plugin.Candidates{
				{nested(pkgPath, "A.B", probeName)}, {nested(pkgPath, "A", probeName)},
			}, "B's member types, then A's")
		})

		t.Run("probes a single-type import before the package", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, pkgClause+"import a.b.X;\n"+publicClass, pkgPath)
			assert.Equal(t, resolved(scope, probeName), plugin.Candidates{
				{id("a/b", probeName), nested("a", "b", probeName)},
				{id(pkgPath, probeName)},
				{id(javaLang, probeName)},
			}, "the import's readings, the type of a package before the nested type")
		})

		t.Run("probes the on-demand imports with java.lang in the last tier", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, pkgClause+"import java.util.*;\n"+publicClass, pkgPath)
			got := resolved(scope, probeName)
			assert.Equal(t, got[len(got)-1], []symbol.Identity{
				id("java/util", probeName), nested("java", "util", probeName), id(javaLang, probeName),
			}, "the on-demand imports compete with java.lang")
		})

		t.Run("probes the static on-demand imports with java.lang in the last tier", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, pkgClause+"import static a.B.*;\n"+publicClass, pkgPath)
			got := resolved(scope, probeName)
			assert.Equal(t, got[len(got)-1], []symbol.Identity{
				id("a/B", probeName), nested("a", "B", probeName), id(javaLang, probeName),
			}, "a member type of B competes with java.lang")
		})

		t.Run("probes the unnamed package for a name in a compact source file", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, compactSource, "")
			assert.Equal(t, resolved(scope, implicitName), plugin.Candidates{
				{id("", implicitName)}, {id(javaLang, implicitName)},
			}, "the file's class is in the unnamed package")
		})
	})

	t.Run("qualified", func(t *testing.T) {
		t.Parallel()

		t.Run("probes a member type of a first name a single-type import binds first", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, pkgClause+"import a.Map;\n"+publicClass, pkgPath)
			assert.Equal(t, resolved(scope, "Map.Entry")[0], []symbol.Identity{nested("a", "Map", "Entry")},
				"Entry is a member of the imported Map")
		})

		t.Run("probes a member type of a member type of a first name an import binds first", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, pkgClause+"import a.Map;\n"+publicClass, pkgPath)
			assert.Equal(t, resolved(scope, "Map.Entry.Key")[0], []symbol.Identity{nested("a", "Map.Entry", "Key")},
				"Key is a member of the imported Map's Entry")
		})

		t.Run("probes a member type of an enclosing type's member type first", func(t *testing.T) {
			t.Parallel()

			scope := plainScope(t)
			scope.Owner = nested(pkgPath, "", "A")
			assert.Equal(t, resolved(scope, "Inner.X")[0], []symbol.Identity{nested(pkgPath, "A.Inner", probeName)},
				"Inner is a member of A")
		})

		t.Run("probes a type of the package a qualified name spells before a nested type", func(t *testing.T) {
			t.Parallel()

			got := resolved(plainScope(t), "a.b.X")
			assert.Equal(t, got[len(got)-2:], plugin.Candidates{
				{id("a/b", probeName)}, {nested("a", "b", probeName)},
			}, "the canonical name's readings come last")
		})

		t.Run("probes each nesting a canonical name spells with the longest package first", func(t *testing.T) {
			t.Parallel()

			got := resolved(plainScope(t), "a.b.C.X")
			assert.Equal(t, got[len(got)-3:], plugin.Candidates{
				{id("a/b/C", probeName)}, {nested("a/b", "C", probeName)}, {nested("a", "b.C", probeName)},
			}, "one reading per split of the name")
		})
	})

	t.Run("packageOf", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{
				name: "records the package a single-type import imports a simple name from",
				give: "import java.util.List;\n\npublic class A {\n    List<String> f;\n}\n", want: "java/util",
			},
			{
				name: "records no package for a simple name no import binds",
				give: "public class A {\n    String f;\n}\n", want: "",
			},
			{
				name: "records a qualified name's prefix as its package",
				give: "public class A {\n    java.util.List f;\n}\n", want: "java/util",
			},
			{
				name: "records the package a qualified name's first name is imported from",
				give: "import java.util.Map;\n\npublic class A {\n    Map.Entry f;\n}\n", want: "java/util",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, classOf(t, tt.give, "A").Fields[0].Type.Package, tt.want, "what the import names")
			})
		}
	})
}

// plainScope returns the scope of a file of com.acme that imports
// nothing and declares the class A.
func plainScope(tb testing.TB) plugin.ImportScope {
	tb.Helper()

	return scopeIn(tb, pkgClause+publicClass, pkgPath)
}

// scopeIn parses one file of a source and returns the scope the File
// node of one package recorded.
func scopeIn(tb testing.TB, src, pkg string) plugin.ImportScope {
	tb.Helper()

	gb, _ := parsedSource(tb, src)
	file := fileIn(tb, gb, pkg)
	for _, rec := range gb.Scopes() {
		if rec.File == file {
			return plugin.ImportScope{Bindings: rec.Bindings}
		}
	}
	tb.Fatalf("the File node of %q recorded no scope", pkg)
	return plugin.ImportScope{}
}

// resolved resolves a spelling in a scope through the frontend.
func resolved(scope plugin.ImportScope, spelling string) plugin.Candidates {
	return frontend.New(nil).Resolve(scope, spelling)
}

// id spells one candidate a package declares.
func id(pkg, name string) symbol.Identity {
	return symbol.Identity{Lang: frontend.Lang, Package: pkg, Name: name}
}

// nested spells one candidate an owner chain in a package declares.
func nested(pkg, owner, name string) symbol.Identity {
	return symbol.Identity{Lang: frontend.Lang, Package: pkg, Owner: owner, Name: name}
}
