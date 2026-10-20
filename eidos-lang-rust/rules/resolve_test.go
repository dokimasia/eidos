// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust"
	rustrules "go.dokimi.dev/eidos/lang/rust/rules"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// resolveAllocs is the ceiling of the resolution of a bare name that the
// module of the subject declares, which reads its declaration alone.
const resolveAllocs = 0

// The subjects of the resolutions and the declarations that they resolve to.
var (
	openID     = symbol.Identity{Lang: rust.Lang, Package: storePkg, Name: "open", Kind: symbol.KindFunction}
	belowID    = symbol.Identity{Lang: rust.Lang, Package: utilPkg, Name: "below", Kind: symbol.KindFunction}
	defaultsID = symbol.Identity{Lang: rust.Lang, Package: utilPkg, Name: "defaults", Kind: symbol.KindFunction}
	retriesID  = symbol.Identity{
		Lang: rust.Lang, Package: utilPkg + "/inner", Name: "retries", Kind: symbol.KindFunction,
	}
	clientID = symbol.Identity{Lang: rust.Lang, Package: "client", Name: "settings", Kind: symbol.KindFunction}
	maxID    = symbol.Identity{Lang: rust.Lang, Package: utilPkg, Name: "MAX_RETRIES", Kind: symbol.KindConstant}
	greetID  = symbol.Identity{Lang: rust.Lang, Package: utilPkg, Name: "GREETING", Kind: symbol.KindVariable}
	errorID  = symbol.Identity{Lang: rust.Lang, Package: errorsPkg, Name: "StoreError", Kind: symbol.KindSum}
	newID    = symbol.Identity{
		Lang:    rust.Lang,
		Package: storePkg,
		Owner:   "Memory",
		Name:    "new",
		Kind:    symbol.KindMethod,
	}
	noteID = symbol.Identity{
		Lang:    rust.Lang,
		Package: modelsPkg,
		Owner:   "Order",
		Name:    "note",
		Kind:    symbol.KindField,
	}
	statusOf = symbol.Identity{
		Lang:    rust.Lang,
		Package: modelsPkg,
		Owner:   "Order",
		Name:    "status",
		Kind:    symbol.KindField,
	}
	levelID = symbol.Identity{
		Lang:    rust.Lang,
		Package: modelsPkg,
		Owner:   "Settings",
		Name:    "level",
		Kind:    symbol.KindField,
	}
	secretOf = symbol.Identity{
		Lang:    rust.Lang,
		Package: modelsPkg,
		Owner:   "Settings",
		Name:    "secret",
		Kind:    symbol.KindField,
	}
)

// TestResolve pins the scope order of Rust in which the spelling of a
// directive resolves, and the refusals of the spellings that name nothing.
func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		getID := symbol.Identity{
			Lang:    rust.Lang,
			Package: storePkg,
			Owner:   "Store",
			Name:    "get",
			Kind:    symbol.KindMethod,
		}
		tests := []struct {
			name    string
			subject symbol.Identity
			give    string
			kind    directive.ResolutionKind
			want    symbol.Identity
		}{
			{
				name: "resolves a bare name through a glob import", subject: storeID, give: "Order",
				kind: directive.ResolveTypeInScope, want: orderID,
			},
			{
				name: "resolves a bare name through a use binding", subject: storeID, give: "StoreError",
				kind: directive.ResolveTypeInScope, want: errorID,
			},
			{
				name: "resolves an item of the module", subject: storeID, give: "Memory",
				kind: directive.ResolveTypeInScope, want: memoryID,
			},
			{
				name: "resolves a path from the crate root", subject: storeID, give: "crate::models::Item",
				kind: directive.ResolveTypeInScope, want: itemID,
			},
			{
				name: "resolves a path from the parent module", subject: storeID, give: "super::util::Ids",
				kind: directive.ResolveTypeInScope, want: idsID,
			},
			{
				name: "resolves a path from the module itself", subject: storeID, give: "self::Memory",
				kind: directive.ResolveTypeInScope, want: memoryID,
			},
			{
				name: "resolves a path through a child module", subject: belowID, give: "inner::retries",
				kind: directive.ResolveCallableInScope, want: retriesID,
			},
			{
				name: "resolves a path to a name that the module of the path imports", subject: storeID,
				give: "crate::Order", kind: directive.ResolveTypeInScope, want: orderID,
			},
			{
				name: "resolves a path to a name that a glob import of the module imports", subject: belowID,
				give: "crate::store::Item", kind: directive.ResolveTypeInScope, want: itemID,
			},
			{
				name: "resolves a constant through a use binding of an inline module", subject: retriesID,
				give: "MAX_RETRIES", kind: directive.ResolvePackageVar, want: maxID,
			},
			{
				name: "resolves a static", subject: belowID, give: "GREETING",
				kind: directive.ResolvePackageVar, want: greetID,
			},
			{
				name: "resolves an associated function of a type", subject: openID, give: "Memory::new",
				kind: directive.ResolveCallableInScope, want: newID,
			},
			{
				name: "resolves a type of the standard library to a stand-in", subject: storeID, give: "io::Error",
				kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: rust.Lang, Package: "std/io", Name: errorName, Kind: symbol.KindAlias},
			},
			{
				name: "resolves a name of the prelude to a stand-in without a package", subject: storeID,
				give: stringName, kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: rust.Lang, Name: stringName, Kind: symbol.KindAlias},
			},
			{
				name: "resolves the unit type to a stand-in", subject: storeID, give: "()",
				kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: rust.Lang, Name: "()", Kind: symbol.KindAlias},
			},
			{
				name:    "resolves a name that a use binds from a crate outside the view to a stand-in",
				subject: errorID, give: errorName, kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: rust.Lang, Package: "thiserror", Name: errorName, Kind: symbol.KindAlias},
			},
			{
				name: "resolves a path of a crate outside the view to a stand-in", subject: storeID,
				give: "chrono::Utc", kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: rust.Lang, Package: "chrono", Name: "Utc", Kind: symbol.KindAlias},
			},
			{
				name:    "resolves a path through an extern crate under another name",
				subject: belowID,
				give:    "standard::collections::HashMap",
				kind:    directive.ResolveTypeInScope,
				want: symbol.Identity{
					Lang:    rust.Lang,
					Package: collectionsPkg,
					Name:    "HashMap",
					Kind:    symbol.KindAlias,
				},
			},
			{
				name: "resolves a public field on the value of an option", subject: findID, give: "note",
				kind: directive.ResolveValueField, want: noteID,
			},
			{
				name: "resolves a field on the value of a result", subject: getID, give: "status",
				kind: directive.ResolveMemberOnHandle, want: statusOf,
			},
			{
				name: "resolves a field of crate visibility in the crate", subject: defaultsID, give: "level",
				kind: directive.ResolveMemberOnHandle, want: levelID,
			},
			{
				name: "resolves a private field in the module of its type", subject: settingsID, give: "secret",
				kind: directive.ResolveMemberOnHandle, want: secretOf,
			},
			{
				name:    "resolves a method on the value of an option",
				subject: findID,
				give:    "total",
				kind:    directive.ResolveMemberOnHandle,
				want: symbol.Identity{
					Lang:    rust.Lang,
					Package: modelsPkg,
					Owner:   "Order",
					Name:    "total",
					Kind:    symbol.KindMethod,
				},
			},
			{
				name:    "resolves an associated type of a trait",
				subject: storeID,
				give:    "Named::Key",
				kind:    directive.ResolveTypeInScope,
				want: symbol.Identity{
					Lang:    rust.Lang,
					Package: storePkg,
					Owner:   "Named",
					Name:    "Key",
					Kind:    symbol.KindAlias,
				},
			},
			{
				name: "resolves Self to the type that declares the subject", subject: newID, give: "Self",
				kind: directive.ResolveTypeInScope, want: memoryID,
			},
			{
				name: "resolves Self to a subject that is a type", subject: memoryID, give: "Self",
				kind: directive.ResolveTypeInScope, want: memoryID,
			},
			{
				name:    "resolves an associated function of Self",
				subject: noteID,
				give:    "Self::new",
				kind:    directive.ResolveCallableInScope,
				want: symbol.Identity{
					Lang:    rust.Lang,
					Package: modelsPkg,
					Owner:   "Order",
					Name:    "new",
					Kind:    symbol.KindMethod,
				},
			},
			{
				name: "resolves an associated type of Self",
				subject: symbol.Identity{
					Lang:    rust.Lang,
					Package: storePkg,
					Owner:   "Named",
					Name:    "name",
					Kind:    symbol.KindMethod,
				},
				give: "Self::Key",
				kind: directive.ResolveTypeInScope,
				want: symbol.Identity{
					Lang:    rust.Lang,
					Package: storePkg,
					Owner:   "Named",
					Name:    "Key",
					Kind:    symbol.KindAlias,
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				got, err := rustrules.Rules{}.Resolve(f.view.ScopeOf(tt.subject), tt.give, tt.kind, f.view)
				assert.NoError(t, err, tt.give+" resolves")
				decl, named := got.(node.Declaration)
				assert.True(t, named, "the result is a declaration")
				assert.Equal(t, decl.Identity(), tt.want, "the declaration of "+tt.give)
			})
		}

		t.Run("resolves a path from the crate root without the module that the load stamps", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			reads := store.NewReadSet()
			reader, err := f.Graph.Reader(reads, nil)
			assert.NoError(t, err, "the sealed graph returns a reader")
			view := rules.NewView(reader, nil, reads, f.Keys)
			got, err := rustrules.Rules{}.Resolve(view.ScopeOf(storeID), "crate::models::Item",
				directive.ResolveTypeInScope, view)
			assert.NoError(t, err, "the path resolves")
			decl, named := got.(node.Declaration)
			assert.True(t, named, "the result is a declaration")
			assert.Equal(t, decl.Identity(), itemID, "the crate root is the first segment of the package")
		})

		t.Run("resolves a name past a nil import", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			file := &node.File{Imports: []*node.Import{
				nil, {Path: modelsPkg, Names: []*node.Binding{nil, {Name: "Order"}}},
			}}
			got, err := rustrules.Rules{}.Resolve(rules.Scope{Subject: storeID, File: file}, "Order",
				directive.ResolveTypeInScope, f.view)
			assert.NoError(t, err, "the name resolves")
			decl, named := got.(node.Declaration)
			assert.True(t, named, "the result is a declaration")
			assert.Equal(t, decl.Identity(), orderID, "the binding of the import")
		})

		t.Run("resolves a name that the crate root imports", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			root := symbol.Identity{Lang: rust.Lang, Package: shopPkg, Kind: symbol.KindPackage}
			pkg, held := f.view.PackageOf(root)
			assert.True(t, held, "the view contains the crate root")
			assert.NotEmpty(t, pkg.Files, "the crate root has its file")
			got, err := rustrules.Rules{}.Resolve(rules.Scope{Subject: root, File: pkg.Files[0]}, "Order",
				directive.ResolveTypeInScope, f.view)
			assert.NoError(t, err, "the name resolves")
			decl, named := got.(node.Declaration)
			assert.True(t, named, "the result is a declaration")
			assert.Equal(t, decl.Identity(), orderID, "the declaration that the use publishes")
		})

		refusals := []struct {
			name    string
			subject symbol.Identity
			give    string
			kind    directive.ResolutionKind
			want    string
		}{
			{
				name: "returns an error for a name that is no path", subject: storeID, give: "a b",
				kind: directive.ResolveTypeInScope, want: "rust: a b is not a path of Rust",
			},
			{
				name:    "returns an error for a name that nothing declares",
				subject: storeID,
				give:    "Missing",
				kind:    directive.ResolveTypeInScope,
				want:    "rust: no Struct or Interface or Alias or Enum or Sum named Missing is in scope of " + storeID.String(),
			},
			{
				name: "returns an error for a path above the crate root", subject: storeID, give: "super::super::Order",
				kind: directive.ResolveTypeInScope, want: "named super::super::Order is in scope",
			},
			{
				name: "returns an error for an associated item that Self does not declare", subject: storeID,
				give: "Self::Item", kind: directive.ResolveTypeInScope, want: "named Self::Item is in scope",
			},
			{
				name: "returns an error for Self outside a type", subject: openID, give: "Self",
				kind: directive.ResolveTypeInScope, want: "named Self is in scope",
			},
			{
				name:    "returns an error for an associated function that the type does not declare",
				subject: openID,
				give:    "Memory::missing",
				kind:    directive.ResolveCallableInScope,
				want:    "rust: no Function named Memory::missing",
			},
			{
				name:    "returns an error for an associated method at the level of an instance",
				subject: openID,
				give:    "Memory::len",
				kind:    directive.ResolveCallableInScope,
				want:    "rust: no Function named Memory::len",
			},
			{
				name: "returns an error for a path whose prefix names no type", subject: openID, give: "nothing::open",
				kind: directive.ResolveCallableInScope, want: "rust: no Function named nothing::open",
			},
			{
				name: "returns an error for an associated item of another kind", subject: openID,
				give: "Memory::orders", kind: directive.ResolvePackageVar,
				want: "rust: no Constant or Variable named Memory::orders",
			},
			{
				name: "returns an error for a private field outside the module of its type", subject: defaultsID,
				give: "secret", kind: directive.ResolveMemberOnHandle, want: "that " + defaultsID.String() + " can use",
			},
			{
				name: "returns an error for a field of crate visibility in another crate", subject: clientID,
				give: "level", kind: directive.ResolveMemberOnHandle, want: "that " + clientID.String() + " can use",
			},
			{
				name: "returns an error for a member of a callable whose value is the unit type",
				subject: symbol.Identity{
					Lang:    rust.Lang,
					Package: storePkg,
					Owner:   "Store",
					Name:    "save",
					Kind:    symbol.KindMethod,
				},
				give: "id",
				kind: directive.ResolveMemberOnHandle,
				want: "has no value of a type that the view declares",
			},
			{
				name:    "returns an error for a member of a value nested deeper than the rules follow",
				subject: symbol.Identity{Lang: rust.Lang, Package: utilPkg, Name: "nested", Kind: symbol.KindFunction},
				give:    "id",
				kind:    directive.ResolveMemberOnHandle,
				want:    "has no value of a type that the view declares",
			},
			{
				name: "returns an error for an item of an associated type", subject: openID, give: "Named::Key::new",
				kind: directive.ResolveCallableInScope, want: "rust: no Function named Named::Key::new",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				got, err := rustrules.Rules{}.Resolve(f.view.ScopeOf(tt.subject), tt.give, tt.kind, f.view)
				refused := assert.ErrorAs[*rules.ResolveError](t, err, "the refusal is a resolve error")
				assert.Equal(t, refused.Lang, rust.Lang, "the refusal names the language")
				assert.Contains(t, err.Error(), tt.want, "the refusal names the reason")
				assert.Nil(t, got, "a refusal returns no declaration")
			})
		}
	})
}

// TestResolveAllocs checks the ceiling of a resolution in scope in the
// ordinary run, which does not run benchmarks.
func TestResolveAllocs(t *testing.T) {
	checkAllocs(t, resolveCalls(t))
}

// BenchmarkResolve measures the resolution of a bare name that the module of
// the subject declares under its ceiling.
func BenchmarkResolve(b *testing.B) {
	benchCalls(b, resolveCalls(b))
}

// resolveCalls returns the resolution that the allocation test and the
// benchmark measure. The view records the read of the declaration before the
// count, so each counted read finds its edge in the read set.
func resolveCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	scope := f.view.ScopeOf(storeID)
	var got symbol.Symbol
	return []allocCall{{
		name: "Resolve", allocs: resolveAllocs,
		call: func() { got, _ = rustrules.Rules{}.Resolve(scope, "Memory", directive.ResolveTypeInScope, f.view) },
		check: func(tb assert.TB) {
			decl, named := got.(node.Declaration)
			assert.True(tb, named, "Resolve returns a declaration")
			assert.Equal(tb, decl.Identity(), memoryID, "Resolve returns the item of the module")
		},
	}}
}
