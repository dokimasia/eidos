// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names the resolution cases probe.
const (
	probeName = "X"
	userName  = "User"
)

// Resolution follows TypeScript's scope order through the bindings the
// parse recorded, so the tiers each spelling probes are pinned.
func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("probes the file's own package and then the global package for a bare name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, resolved(scopeIn(t, "export {};\n", aPackage), probeName),
				plugin.Candidates{{id(aPackage, probeName)}, {id("", probeName)}},
				"the module first, the global scope last")
		})

		t.Run("probes the enclosing namespaces innermost first", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "export namespace A.B {}\n", nsPackage)
			assert.Equal(t, resolved(scope, probeName), plugin.Candidates{
				{id(nsPackage, probeName)},
				{id(outerPackage, probeName)},
				{id(aPackage, probeName)},
				{id("", probeName)},
			}, "B inside A inside the module")
		})

		t.Run("probes an imported name's module, the file before the directory index", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "import { User } from './models';\n", aPackage)
			assert.Equal(t, resolved(scope, userName), plugin.Candidates{
				{id(aPackage, userName)},
				{id("src/models", userName)},
				{id("src/models/index", userName)},
				{id("", userName)},
			}, "TypeScript tries the file before the directory")
		})

		t.Run("probes a renamed import under the name the module exports", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "import { Person as User } from './models';\n", aPackage)
			assert.Equal(t, resolved(scope, userName)[1], []symbol.Identity{id("src/models", "Person")},
				"the module declares Person")
		})

		t.Run("probes a default import's module under default", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "import User from './models';\n", aPackage)
			assert.Equal(t, resolved(scope, userName)[1], []symbol.Identity{id("src/models", "default")},
				"the re-export step follows the default export")
		})

		t.Run("probes the module a namespace import binds for a qualified name", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "import * as m from './models';\n", aPackage)
			assert.Equal(t, resolved(scope, "m."+userName), plugin.Candidates{
				{id("src/models", userName)}, {id("src/models/index", userName)},
			}, "m is the module")
		})

		t.Run("probes a namespace a named import binds inside its module", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "import { NS } from './models';\n", aPackage)
			assert.Equal(t, resolved(scope, "NS."+userName)[0], []symbol.Identity{id("src/models/NS", userName)},
				"NS is the module's namespace")
		})

		t.Run("probes a local namespace for a qualified name", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "export {};\n", aPackage)
			assert.Equal(t, resolved(scope, "A."+probeName), plugin.Candidates{
				{id(outerPackage, probeName)}, {id("A", probeName)},
			}, "a namespace of the module, then of the global scope")
		})

		t.Run("follows an import alias to the entity it names", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "import Q = A.B;\nexport {};\n", aPackage)
			assert.Equal(t, resolved(scope, "Q."+probeName)[0], []symbol.Identity{id(nsPackage, probeName)},
				"Q.X is A.B.X")
		})

		t.Run("resolves nothing through an alias that names itself", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "import A = A.B;\nexport {};\n", aPackage)
			assert.Empty(t, resolved(scope, "A."+probeName), "the alias is followed once")
		})

		t.Run("follows a chain of distinct aliases", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "import P = A;\nimport Q = P.B;\nexport {};\n", aPackage)
			assert.Equal(t, resolved(scope, "Q."+probeName)[0], []symbol.Identity{id(nsPackage, probeName)},
				"Q.X is P.B.X is A.B.X")
		})

		t.Run("resolves a .js specifier to the source file's package", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "import { User } from './models/user.js';\n", aPackage)
			assert.Equal(t, resolved(scope, userName)[1], []symbol.Identity{id("src/models/user", userName)},
				"TypeScript substitutes the source for the emitted file")
		})

		t.Run("resolves a bare specifier to the ambient module of its name", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "import { User } from 'lib';\n", aPackage)
			assert.Equal(t, resolved(scope, userName)[1], []symbol.Identity{id("lib", userName)},
				"a declare module 'lib' block declares it")
		})

		t.Run("resolves nothing for a file whose parse recorded no scope", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, resolved(plugin.ImportScope{}, probeName), "no bindings to resolve through")
		})

		t.Run("resolves nothing for an empty spelling", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, resolved(scopeIn(t, "export {};\n", aPackage), ""), "an empty spelling names nothing")
		})
	})

	t.Run("exports", func(t *testing.T) {
		t.Parallel()

		t.Run("publishes a re-export's source name, the file before the directory index", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "export { A as B } from './x';\n", aPackage)
			assert.Equal(t, published(t, scope, "B"), plugin.Candidates{
				{id("src/x", "A")}, {id("src/x/index", "A")},
			}, "B is the module's A")
		})

		t.Run("publishes only the re-export of the name asked for", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "export { A } from './x';\nexport { B } from './y';\n", aPackage)
			assert.Equal(t, published(t, scope, "B"), plugin.Candidates{
				{id("src/y", "B")}, {id("src/y/index", "B")},
			}, "A is another name")
		})

		t.Run("publishes a local export clause's name from the file's package", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "class A {}\nexport { A as B };\n", aPackage)
			assert.Equal(t, published(t, scope, "B"), plugin.Candidates{{id(aPackage, "A")}}, "B is the file's A")
		})

		t.Run("publishes a re-exported import from the import's module", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "import { A } from './x';\nexport { A as B };\n", aPackage)
			assert.Equal(t, published(t, scope, "B")[0], []symbol.Identity{id("src/x", "A")}, "B is x's A")
		})

		t.Run("publishes the default export's declaration under default", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "class A {}\nexport default A;\n", aPackage)
			assert.Equal(t, published(t, scope, "default"), plugin.Candidates{{id(aPackage, "A")}},
				"the default names A")
		})

		t.Run("publishes each export-star module's name in one tier", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "export * from './x';\nexport * from './y';\n", aPackage)
			assert.Equal(t, published(t, scope, probeName), plugin.Candidates{
				{id("src/x", probeName), id("src/y", probeName)},
				{id("src/x/index", probeName), id("src/y/index", probeName)},
			}, "two modules that both publish the name compete")
		})

		t.Run("publishes no default through an export star", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "export * from './x';\n", aPackage)
			assert.Empty(t, published(t, scope, "default"), "export * leaves the default out")
		})

		t.Run("publishes the explicit re-exports before the export-star modules", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "export * from './y';\nexport { X } from './x';\n", aPackage)
			assert.Equal(t, published(t, scope, probeName)[0], []symbol.Identity{id("src/x", probeName)},
				"an explicit export shadows a star")
		})

		t.Run("publishes nothing from a namespace's File node", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "export * from './x';\nexport namespace A.B {}\n", nsPackage)
			assert.Empty(t, published(t, scope, probeName), "the module's own File node publishes")
		})

		t.Run("publishes nothing for a file whose parse recorded no scope", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, published(t, plugin.ImportScope{}, probeName), "no bindings to publish through")
		})
	})
}

// scopeIn parses a module's source at src/a.ts and returns the scope the
// File node of one package recorded.
func scopeIn(tb testing.TB, src, pkg string) plugin.ImportScope {
	tb.Helper()

	gb, _ := parsedSource(tb, src)
	file := fileIn(tb, gb, pkg)
	scopes := gb.Scopes()
	at := slices.IndexFunc(scopes, func(rec plugin.ScopeRecord) bool { return rec.File == file })
	assert.NotEqual(tb, at, -1, "the File node of "+pkg+" records a scope")
	return plugin.ImportScope{Bindings: scopes[at].Bindings}
}

// id spells one candidate.
func id(pkg, name string) symbol.Identity {
	return symbol.Identity{Lang: frontend.Lang, Package: pkg, Name: name}
}

// resolved resolves a spelling in a scope through the frontend.
func resolved(scope plugin.ImportScope, spelling string) plugin.Candidates {
	return frontend.New().Resolve(scope, spelling)
}

// published returns what a scope's file publishes under a name.
func published(tb testing.TB, scope plugin.ImportScope, name string) plugin.Candidates {
	tb.Helper()

	exporter, exports := frontend.New().(plugin.Exporter)
	assert.True(tb, exports, "the frontend is in the exporter role")
	return exporter.Exports(scope, name)
}
