// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestScope pins the tiers of the candidates of a spelling in Python's
// order of scopes.
func TestScope(t *testing.T) {
	t.Parallel()

	outer := symbol.Identity{Lang: frontend.Lang, Package: appPkg, Name: "Outer", Kind: symbol.KindStruct}
	tests := []struct {
		name      string
		giveOwner symbol.Identity
		give      string
		want      plugin.Candidates
	}{
		{
			name: "returns the declaration that an import binds, the star imports and builtins",
			give: "Order",
			want: plugin.Candidates{
				{at("shop/models", "", "Order")},
				{at("typing", "", "Order")},
				{at("builtins", "", "Order")},
			},
		},
		{
			name: "returns the name that an alias binds",
			give: "L",
			want: plugin.Candidates{
				{at("shop/models", "", "Line")},
				{at("typing", "", "L")},
				{at("builtins", "", "L")},
			},
		},
		{
			name: "returns the declaration of the module for a name that it declares",
			give: "Outer",
			want: plugin.Candidates{
				{at(appPkg, "", "Outer")},
				{at("typing", "", "Outer")},
				{at("builtins", "", "Outer")},
			},
		},
		{
			name: "returns the name that a relative import binds",
			give: "Base",
			want: plugin.Candidates{
				{at("shop/base", "", "Base")},
				{at("typing", "", "Base")},
				{at("builtins", "", "Base")},
			},
		},
		{
			name:      "returns the body of the enclosing class first",
			giveOwner: outer,
			give:      "Inner",
			want: plugin.Candidates{
				{at(appPkg, "Outer", "Inner")},
				{at("typing", "", "Inner")},
				{at("builtins", "", "Inner")},
			},
		},
		{
			name:      "returns the body of each enclosing class, innermost first",
			giveOwner: symbol.Identity{Lang: frontend.Lang, Package: appPkg, Owner: "Outer", Name: "Inner"},
			give:      "Deep",
			want: plugin.Candidates{
				{at(appPkg, "Outer.Inner", "Deep")},
				{at(appPkg, "Outer", "Deep")},
				{at("typing", "", "Deep")},
				{at("builtins", "", "Deep")},
			},
		},
		{
			name: "returns a member of a class that the module declares", give: "Outer.Inner",
			want: plugin.Candidates{{at(appPkg, "Outer", "Inner")}, {at("typing", "Outer", "Inner")}},
		},
		{
			name: "returns the name in the longest module of a dotted name first", give: "collections.abc.Sequence",
			want: plugin.Candidates{
				{at("collections/abc", "", "Sequence")},
				{at("collections", "abc", "Sequence")},
				{at("typing", "collections.abc", "Sequence")},
			},
		},
		{
			name: "returns the name in a module that an alias binds", give: "osp.join",
			want: plugin.Candidates{{at("os/path", "", "join")}, {at("typing", "osp", "join")}},
		},
		{
			name: "returns a member of a class that an import binds, and then the name in its module",
			give: "Order.Item",
			want: plugin.Candidates{
				{at("shop/models", "Order", "Item")},
				{at("shop/models/Order", "", "Item")},
				{at("typing", "Order", "Item")},
			},
		},
		{
			name: "returns the name in each module of a dotted name below an import",
			give: "collections.abc.deep.Name",
			want: plugin.Candidates{
				{at("collections/abc/deep", "", "Name")},
				{at("collections/abc", "deep", "Name")},
				{at("collections", "abc.deep", "Name")},
				{at("typing", "collections.abc.deep", "Name")},
			},
		},
		{
			name: "returns the star imports alone for a dotted name that nothing binds", give: "unknown.Name",
			want: plugin.Candidates{{at("typing", "unknown", "Name")}},
		},
		{name: "returns nothing for the empty spelling", give: ""},
	}
	t.Run("candidates", func(t *testing.T) {
		t.Parallel()

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s := scopeOf(t, scopeSource)
				s.Owner = tt.giveOwner
				assert.Equal(t, frontend.New(nil).Resolve(s, tt.give), tt.want, "the tiers of the spelling")
			})
		}
	})

	t.Run("through", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name in a module that a relative import above the root binds", func(t *testing.T) {
			t.Parallel()

			got := frontend.New(nil).Resolve(scopeOf(t, "from .. import models\n"), "models.Order")
			want := plugin.Candidates{{at("", "models", "Order")}, {at("models", "", "Order")}}
			assert.Equal(t, got, want, "the class of the global package, and then the module models")
		})
	})

	t.Run("published", func(t *testing.T) {
		t.Parallel()

		exporter, _ := frontend.New(nil).(plugin.Exporter)
		published := []struct {
			name string
			give string
			want plugin.Candidates
		}{
			{
				name: "returns the declaration that an import binds and the star imports", give: "Order",
				want: plugin.Candidates{{at("shop/models", "", "Order")}, {at("typing", "", "Order")}},
			},
			{name: "returns nothing for a name that the module declares", give: "Outer"},
			{
				name: "returns the star imports alone for a module that an import binds", give: "osp",
				want: plugin.Candidates{{at("typing", "", "osp")}},
			},
		}
		for _, tt := range published {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := exporter.Exports(scopeOf(t, scopeSource), tt.give)
				assert.Equal(t, got, tt.want, "what the module publishes")
			})
		}
	})
}

// at returns the candidate of a name in a module, inside the owner chain
// owner.
func at(module, owner, name string) symbol.Identity {
	return symbol.Identity{Lang: frontend.Lang, Package: module, Owner: owner, Name: name}
}
