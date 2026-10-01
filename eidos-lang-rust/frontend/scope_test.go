// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The name the resolution cases probe, and the packages of the inline
// modules they probe from.
const (
	probeName = "X"
	mPackage  = crateName + "/m"
	nPackage  = crateName + "/m/n"
)

// scopeIn parses the fixture crate whose library root is src and returns
// the scope the File node of one package recorded.
func scopeIn(tb assert.TB, src, pkg string) plugin.ImportScope {
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

// id spells one candidate.
func id(pkg, name string) symbol.Identity {
	return symbol.Identity{Lang: frontend.Lang, Package: pkg, Name: name}
}

// resolved resolves a spelling in a scope through the frontend.
func resolved(scope plugin.ImportScope, spelling string) plugin.Candidates {
	return frontend.New(nil).Resolve(scope, spelling)
}

// published returns what a scope's module publishes under a name.
func published(tb assert.TB, scope plugin.ImportScope, name string) plugin.Candidates {
	tb.Helper()

	exporter, exports := frontend.New(nil).(plugin.Exporter)
	assert.True(tb, exports, "the frontend is in the exporter role")
	return exporter.Exports(scope, name)
}

// Resolution follows Rust's scope order through the bindings the parse
// recorded, so the tiers each spelling probes are pinned.
func TestScope(t *testing.T) {
	t.Parallel()

	t.Run("resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("probes the module's own item for a bare name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, resolved(scopeIn(t, "", crateName), probeName),
				plugin.Candidates{{id(crateName, probeName)}}, "the module alone")
		})

		t.Run("probes no item of the parent for a bare name in a child module", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, resolved(scopeIn(t, "pub mod m {}\n", mPackage), probeName),
				plugin.Candidates{{id(mPackage, probeName)}}, "a module's items are not in its children's scope")
		})

		t.Run("probes a use binding in the module's own tier", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "use crate::store::Table;\n", crateName)
			assert.Equal(t, resolved(scope, "Table"), plugin.Candidates{
				{id(crateName, "Table"), id(crateName+"/store", "Table")},
			}, "Rust refuses a module that declares and imports one name")
		})

		t.Run("probes a renamed use binding under the name its path names", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "use crate::store::Table as Row;\n", crateName)
			assert.Equal(t, resolved(scope, "Row")[0][1], id(crateName+"/store", "Table"), "the module declares Table")
		})

		t.Run("probes an external crate and then an undeclared child module for a use binding", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "use serde::Deserialize;\n", crateName)
			assert.Equal(t, resolved(scope, "Deserialize"), plugin.Candidates{
				{id(crateName, "Deserialize"), id("serde", "Deserialize")},
				{id(crateName+"/serde", "Deserialize")},
			}, "serde is no mod item of the module")
		})

		t.Run("probes the glob imports in the tier after the module's own", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "use crate::a::*;\n", crateName)
			assert.Equal(t, resolved(scope, probeName), plugin.Candidates{
				{id(crateName, probeName)}, {id(crateName+"/a", probeName)},
			}, "an item or a binding shadows a glob")
		})

		t.Run("probes two glob imports in one tier", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "use crate::a::*;\nuse crate::b::*;\n", crateName)
			assert.Equal(t, resolved(scope, probeName)[1], []symbol.Identity{
				id(crateName+"/a", probeName), id(crateName+"/b", probeName),
			}, "two globs that both name it compete")
		})

		t.Run("probes the crate root for a path from crate", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, resolved(scopeIn(t, "pub mod m {}\n", mPackage), "crate::a::X"),
				plugin.Candidates{{id(crateName+"/a", probeName)}}, "below the crate root")
		})

		t.Run("probes the module for a path from self", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, resolved(scopeIn(t, "pub mod m {}\n", mPackage), "self::a::X"),
				plugin.Candidates{{id(mPackage+"/a", probeName)}}, "below the module")
		})

		t.Run("probes the parent for a path from super", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, resolved(scopeIn(t, "pub mod m {}\n", mPackage), "super::X"),
				plugin.Candidates{{id(crateName, probeName)}}, "the module above")
		})

		t.Run("probes the grandparent for a path from super::super", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, resolved(scopeIn(t, "pub mod m {\n    pub mod n {}\n}\n", nPackage), "super::super::X"),
				plugin.Candidates{{id(crateName, probeName)}}, "two modules above")
		})

		t.Run("resolves nothing for super at the crate root", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, resolved(scopeIn(t, "", crateName), "super::X"), "the crate root has no parent")
		})

		t.Run("resolves nothing for a path from Self", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, resolved(scopeIn(t, "", crateName), "Self::Item"), "an associated item of a type")
		})

		t.Run("probes a declared child module for a path from its name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, resolved(scopeIn(t, "pub mod a {}\n", crateName), "a::X"),
				plugin.Candidates{{id(crateName+"/a", probeName)}}, "the mod item declares a")
		})

		t.Run("probes the module a use declaration binds for a path from its name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, resolved(scopeIn(t, "use crate::store;\n", crateName), "store::X"),
				plugin.Candidates{{id(crateName+"/store", probeName)}}, "the binding names the module")
		})

		t.Run("probes the crate an extern crate declaration binds under another name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, resolved(scopeIn(t, "extern crate serde as sd;\n", crateName), "sd::de::X"),
				plugin.Candidates{{id("serde/de", probeName)}}, "sd is serde")
		})

		t.Run("probes an external crate and then an undeclared child module for any other path", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, resolved(scopeIn(t, "", crateName), "std::io::Read"), plugin.Candidates{
				{id("std/io", "Read")}, {id(crateName+"/std/io", "Read")},
			}, "a macro can declare the child module")
		})

		t.Run("probes an external crate for a path from ::", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, resolved(scopeIn(t, "", crateName), "::std::io::Read"),
				plugin.Candidates{{id("std/io", "Read")}}, "a leading :: names a crate")
		})

		t.Run("resolves nothing through a cycle of use bindings", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, resolved(scopeIn(t, "use b as a;\nuse a as b;\n", crateName), "a::X"),
				"each binding is followed at most once")
		})

		t.Run("probes the module's own item for a bare name a cycle of use bindings binds", func(t *testing.T) {
			t.Parallel()

			got := resolved(scopeIn(t, "use b as a;\nuse a as b;\n", crateName), "a")
			assert.Length(t, got, 1, "the cycle ends in one tier")
			assert.Equal(t, got[0][0], id(crateName, "a"), "the module's own item first")
		})

		t.Run("resolves nothing for a file whose parse recorded no scope", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, resolved(plugin.ImportScope{}, probeName), "no bindings to resolve through")
		})
	})

	t.Run("exports", func(t *testing.T) {
		t.Parallel()

		t.Run("publishes what a pub use binding of the name names", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "pub use crate::store::Table;\n", crateName)
			assert.Equal(t, published(t, scope, "Table"), plugin.Candidates{{id(crateName+"/store", "Table")}},
				"the declaring module")
		})

		t.Run("publishes a renamed binding under its local name", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "pub use crate::store::Table as Row;\n", crateName)
			assert.Equal(t, published(t, scope, "Row"), plugin.Candidates{{id(crateName+"/store", "Table")}},
				"Row is the store's Table")
		})

		t.Run("publishes what a private use binding names", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "use crate::store::Table;\n", crateName)
			assert.Equal(t, published(t, scope, "Table"), plugin.Candidates{{id(crateName+"/store", "Table")}},
				"a descendant reads it through super")
		})

		t.Run("publishes through a child module the module declares", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "mod person {}\npub use person::Person;\n", crateName)
			assert.Equal(t, published(t, scope, "Person"), plugin.Candidates{{id(crateName+"/person", "Person")}},
				"the child declares it")
		})

		t.Run("publishes what the glob imports name after the use bindings", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "pub use crate::b::*;\npub use crate::a::X;\n", crateName)
			assert.Equal(t, published(t, scope, probeName), plugin.Candidates{
				{id(crateName+"/a", probeName)}, {id(crateName+"/b", probeName)},
			}, "a binding shadows a glob")
		})

		t.Run("publishes nothing for a name the module binds nothing for", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, published(t, scopeIn(t, "pub use crate::a::X;\n", crateName), "Y"), "Y is not bound")
		})

		t.Run("publishes nothing for a file whose parse recorded no scope", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, published(t, plugin.ImportScope{}, probeName), "no bindings to publish through")
		})
	})

	t.Run("bind", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps a name's first binding", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "use crate::a::X;\nuse crate::b::X;\n", crateName)
			assert.Equal(t, resolved(scope, probeName), plugin.Candidates{
				{id(crateName, probeName), id(crateName+"/a", probeName)},
			}, "the second binding of X is Rust's error")
		})

		t.Run("binds no name for an import as _", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "use std::fmt::Write as _;\n", crateName)
			assert.Equal(t, resolved(scope, "_"), plugin.Candidates{{id(crateName, "_")}}, "_ binds nothing")
		})
	})
}
