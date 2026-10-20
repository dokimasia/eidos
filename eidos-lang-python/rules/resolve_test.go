// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python"
	pythonrules "go.dokimi.dev/eidos/lang/python/rules"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The files of the fixture that the resolutions read the imports of.
const (
	modelsFile = "shop/models.py"
	apiFile    = "shop/api.py"
	loopAPkg   = "shop/loop_a"
)

// The ceilings of Resolve: a class of the module of the subject allocates
// nothing, and a dotted name through an import allocates the path of the
// module that its names name.
const (
	resolveAllocs = 0
	throughAllocs = 1
)

// The subjects and the declarations of the resolutions.
var (
	idFieldID = symbol.Identity{
		Lang: python.Lang, Package: modelsPkg, Owner: "Order", Name: "id", Kind: symbol.KindField,
	}
	sizeFieldID = symbol.Identity{
		Lang: python.Lang, Package: modelsPkg, Owner: "Outer.Inner", Name: "size", Kind: symbol.KindField,
	}
	outerID      = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Outer", Kind: symbol.KindStruct}
	accountID    = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Account", Kind: symbol.KindStruct}
	accountFnID  = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "account", Kind: symbol.KindFunction}
	maybeOrderID = symbol.Identity{
		Lang: python.Lang, Package: modelsPkg, Name: "maybe_order", Kind: symbol.KindFunction,
	}
	taggedID   = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "tagged", Kind: symbol.KindFunction}
	vagueID    = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "vague", Kind: symbol.KindFunction}
	vaultID    = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Vault", Kind: symbol.KindStruct}
	safeID     = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Safe", Kind: symbol.KindStruct}
	hereID     = symbol.Identity{Lang: python.Lang, Package: loopAPkg, Name: "Here", Kind: symbol.KindStruct}
	limitGetID = symbol.Identity{
		Lang: python.Lang, Package: modelsPkg, Owner: "Account", Name: "limit", Kind: symbol.KindMethod,
	}
)

// TestResolve pins the scopes of an annotation of Python, the visibility of
// a member, the stand-ins and the value of a type.
func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		types := []struct {
			name    string
			subject symbol.Identity
			file    string
			pkg     string
			give    string
			want    symbol.Identity
		}{
			{
				name:    "resolves a class of the module of the subject",
				subject: idFieldID,
				file:    modelsFile,
				pkg:     modelsPkg,
				give:    "Item",
				want:    itemID,
			},
			{
				name:    "resolves a class that an import binds through the module that publishes it",
				subject: fetchID,
				file:    apiFile,
				pkg:     apiPkg,
				give:    "Order",
				want:    orderID,
			},
			{
				name:    "resolves a dotted name through an import of a module",
				subject: fetchID,
				file:    apiFile,
				pkg:     apiPkg,
				give:    "shop.models.Order",
				want:    orderID,
			},
			{
				name:    "resolves a dotted name through the longest module that its names name",
				subject: fetchID,
				file:    apiFile,
				pkg:     apiPkg,
				give:    "shop.models.Outer.Inner",
				want:    innerID,
			},
			{
				name:    "resolves a name of a star import of the module",
				subject: fetchID,
				file:    apiFile,
				pkg:     apiPkg,
				give:    "Status",
				want:    statusID,
			},
			{
				name:    "resolves a nested class through its owner",
				subject: idFieldID,
				file:    modelsFile,
				pkg:     modelsPkg,
				give:    "Outer.Inner",
				want:    innerID,
			},
			{
				name:    "resolves a class of an enclosing class of a member",
				subject: sizeFieldID,
				file:    modelsFile,
				pkg:     modelsPkg,
				give:    "Inner",
				want:    innerID,
			},
			{
				name:    "resolves a class of the class that is the subject",
				subject: outerID,
				file:    modelsFile,
				pkg:     modelsPkg,
				give:    "Inner",
				want:    innerID,
			},
			{
				name:    "resolves a class of builtins",
				subject: idFieldID,
				file:    modelsFile,
				pkg:     modelsPkg,
				give:    "int",
				want: symbol.Identity{
					Lang:    python.Lang,
					Package: builtinsModule,
					Name:    "int",
					Kind:    symbol.KindStruct,
				},
			},
		}
		for _, tt := range types {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				scope := rules.Scope{Subject: tt.subject, File: f.file(t, tt.pkg, tt.file)}
				got := f.resolvedType(t, scope, tt.give)
				assert.Equal(t, got, tt.want, "Resolve returns the declaration of "+tt.give)
			})
		}

		t.Run("resolves a name of a star import of the file of the subject", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			scope := rules.Scope{Subject: idFieldID, File: &node.File{Imports: []*node.Import{
				nil, {Path: modelsPkg}, {Path: loopAPkg, Wildcard: true},
			}}}
			assert.Equal(t, f.resolvedType(t, scope, "Here"), hereID, "the star import publishes Here")
		})

		standIns := []struct {
			name    string
			imports []*node.Import
			give    string
			want    symbol.Identity
		}{
			{
				name:    "resolves a name that an import binds from a module outside the view to a stand-in",
				imports: []*node.Import{{Path: outsidePkg, Names: []*node.Binding{{Name: "Thing"}}}},
				give:    "Thing",
				want:    symbol.Identity{Lang: python.Lang, Package: outsidePkg, Name: "Thing", Kind: symbol.KindAlias},
			},
			{
				name:    "resolves a member of a name that an import binds outside the view to a stand-in",
				imports: []*node.Import{{Path: outsidePkg, Names: []*node.Binding{{Name: "Thing"}}}},
				give:    "Thing.Inner",
				want: symbol.Identity{
					Lang: python.Lang, Package: outsidePkg, Owner: "Thing", Name: "Inner", Kind: symbol.KindAlias,
				},
			},
			{
				name:    "resolves a name of a module that an import binds outside the view to a stand-in",
				imports: []*node.Import{{Path: outsidePkg, Alias: "lib"}},
				give:    "lib.Thing",
				want:    symbol.Identity{Lang: python.Lang, Package: outsidePkg, Name: "Thing", Kind: symbol.KindAlias},
			},
		}
		for _, tt := range standIns {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				scope := rules.Scope{Subject: fetchID, File: &node.File{Imports: tt.imports}}
				assert.Equal(t, f.resolvedType(t, scope, tt.give), tt.want, "Resolve returns the stand-in")
			})
		}

		t.Run("resolves a class of builtins outside the view to a stand-in", func(t *testing.T) {
			t.Parallel()

			got, err := pythonrules.Rules{}.Resolve(rules.Scope{Subject: fetchID}, "int", directive.ResolveTypeInScope,
				rules.View{})
			assert.NoError(t, err, "a class of builtins has a stand-in")
			alias, is := got.(*node.Alias)
			assert.True(t, is, "the stand-in is an alias")
			want := symbol.Identity{Lang: python.Lang, Package: builtinsModule, Name: "int", Kind: symbol.KindAlias}
			assert.Equal(t, alias.ID, want, "the stand-in has the name in builtins")
		})

		members := []struct {
			name    string
			kind    directive.ResolutionKind
			subject symbol.Identity
			give    string
			want    symbol.Identity
		}{
			{
				name:    "resolves a field of the type that is the subject",
				kind:    directive.ResolveValueField,
				subject: orderID,
				give:    "id",
				want:    idFieldID,
			},
			{
				name:    "resolves a protected field of the own type of the subject",
				kind:    directive.ResolveValueField,
				subject: accountID,
				give:    "_limit",
				want: symbol.Identity{
					Lang: python.Lang, Package: modelsPkg, Owner: "Account", Name: "_limit", Kind: symbol.KindField,
				},
			},
			{
				name:    "resolves a private field that the own type of the subject declares",
				kind:    directive.ResolveValueField,
				subject: vaultID,
				give:    "__secret",
				want: symbol.Identity{
					Lang: python.Lang, Package: modelsPkg, Owner: "Vault", Name: "__secret", Kind: symbol.KindField,
				},
			},
			{
				name:    "resolves a public field of the result of a callable",
				kind:    directive.ResolveValueField,
				subject: accountFnID,
				give:    "owner",
				want: symbol.Identity{
					Lang: python.Lang, Package: modelsPkg, Owner: "Account", Name: "owner", Kind: symbol.KindField,
				},
			},
			{
				name:    "resolves a field of the type of an optional result",
				kind:    directive.ResolveValueField,
				subject: maybeOrderID,
				give:    "id",
				want:    idFieldID,
			},
			{
				name:    "resolves a method of the type of a method subject",
				kind:    directive.ResolveCallableInScope,
				subject: limitGetID,
				give:    "__eq__",
				want: symbol.Identity{
					Lang: python.Lang, Package: modelsPkg, Owner: "Account", Name: "__eq__", Kind: symbol.KindMethod,
					Disc: "object",
				},
			},
			{
				name:    "resolves a function that a star import publishes",
				kind:    directive.ResolveCallableInScope,
				subject: fetchID,
				give:    "total",
				want:    totalID,
			},
			{
				name:    "resolves a function of the module whose identity has a discriminator",
				kind:    directive.ResolveCallableInScope,
				subject: idFieldID,
				give:    "total",
				want:    totalID,
			},
			{
				name:    "resolves a variable of the module",
				kind:    directive.ResolvePackageVar,
				subject: idFieldID,
				give:    "LIMIT",
				want: symbol.Identity{
					Lang:    python.Lang,
					Package: modelsPkg,
					Name:    "LIMIT",
					Kind:    symbol.KindVariable,
				},
			},
		}
		for _, tt := range members {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				scope := f.view.ScopeOf(tt.subject)
				got, err := pythonrules.Rules{}.Resolve(scope, tt.give, tt.kind, f.view)
				assert.NoError(t, err, tt.give+" resolves")
				decl, named := got.(node.Declaration)
				assert.True(t, named, "the result is a declaration")
				assert.Equal(t, decl.Identity(), tt.want, "Resolve returns the member "+tt.give)
			})
		}

		t.Run("resolves a method of the target of an alias that a callable returns", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, err := pythonrules.Rules{}.Resolve(f.view.ScopeOf(taggedID), "append", directive.ResolveMemberOnHandle,
				f.view)
			assert.NoError(t, err, "a list has the method append")
			decl, named := got.(node.Declaration)
			assert.True(t, named, "the result is a declaration")
			expect.Equal(t, decl.Identity().Package, builtinsModule, "the method of list of builtins")
			expect.Equal(t, decl.Identity().Owner, "list", "list declares the method")
			expect.Equal(t, decl.Identity().Name, "append", "the method of the name")
		})

		refusals := []struct {
			name    string
			kind    directive.ResolutionKind
			subject symbol.Identity
			imports []*node.Import
			give    string
			want    string
		}{
			{
				name:    "refuses a spelling that is no dotted name",
				kind:    directive.ResolveTypeInScope,
				subject: fetchID,
				give:    "list[int]",
				want:    "list[int] is not a dotted name of Python",
			},
			{
				name:    "refuses a name that no declaration in scope has",
				kind:    directive.ResolveTypeInScope,
				subject: fetchID,
				give:    "Nope",
				want:    "no Struct or Interface or Alias or Enum or Sum named Nope is in scope",
			},
			{
				name:    "refuses a name that a cycle of star imports publishes",
				kind:    directive.ResolveTypeInScope,
				subject: hereID,
				imports: []*node.Import{{Path: "shop/loop_b", Wildcard: true}},
				give:    "Missing",
				want:    "named Missing is in scope",
			},
			{
				name:    "refuses a name of a module that an import binds from the view",
				kind:    directive.ResolveTypeInScope,
				subject: fetchID,
				imports: []*node.Import{{Path: modelsPkg, Alias: "m"}},
				give:    "m.Nope",
				want:    "named m.Nope is in scope",
			},
			{
				name:    "refuses a module that an import binds outside the view",
				kind:    directive.ResolveTypeInScope,
				subject: fetchID,
				imports: []*node.Import{{Path: outsidePkg, Alias: "lib"}},
				give:    "lib",
				want:    "named lib is in scope",
			},
			{
				name:    "refuses a member of a declaration that an import binds without the member",
				kind:    directive.ResolveTypeInScope,
				subject: fetchID,
				imports: []*node.Import{{Path: modelsPkg, Names: []*node.Binding{{Name: "Outer"}}}},
				give:    "Outer.Nope",
				want:    "named Outer.Nope is in scope",
			},
			{
				name:    "refuses a name of no module of a dotted name",
				kind:    directive.ResolveTypeInScope,
				subject: fetchID,
				imports: []*node.Import{{Path: modelsPkg}},
				give:    "shop.Nope",
				want:    "named shop.Nope is in scope",
			},
			{
				name:    "refuses a dotted name whose first name nothing binds",
				kind:    directive.ResolveTypeInScope,
				subject: fetchID,
				give:    "lib.Thing",
				want:    "named lib.Thing is in scope",
			},
			{
				name:    "refuses a name of builtins without a class",
				kind:    directive.ResolveTypeInScope,
				subject: fetchID,
				give:    "Callable",
				want:    "named Callable is in scope",
			},
			{
				name:    "refuses a function of an owner chain",
				kind:    directive.ResolveCallableInScope,
				subject: idFieldID,
				give:    "Account.limit",
				want:    "no Function named Account.limit is in scope",
			},
			{
				name:    "refuses a function of a star import of a module outside the view",
				kind:    directive.ResolveCallableInScope,
				subject: idFieldID,
				imports: []*node.Import{{Path: outsidePkg, Wildcard: true}},
				give:    "nope",
				want:    "no Function named nope is in scope",
			},
			{
				name:    "refuses a protected field from outside the type",
				kind:    directive.ResolveValueField,
				subject: accountFnID,
				give:    "_limit",
				want:    "has no Field member named _limit",
			},
			{
				name:    "refuses a private field that a base declares",
				kind:    directive.ResolveValueField,
				subject: safeID,
				give:    "__secret",
				want:    "has no Field member named __secret",
			},
			{
				name:    "refuses a field of a callable whose value has no declared type",
				kind:    directive.ResolveValueField,
				subject: vagueID,
				give:    "x",
				want:    "has no value of a type that the view declares",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				scope := f.view.ScopeOf(tt.subject)
				if tt.imports != nil {
					scope.File = &node.File{Imports: tt.imports}
				}
				_, err := pythonrules.Rules{}.Resolve(scope, tt.give, tt.kind, f.view)
				assert.HasError(t, err, tt.give+" does not resolve")
				expect.Contains(t, err.Error(), tt.want, "the refusal describes the search")
				expect.Contains(t, err.Error(), "python: ", "the refusal has the prefix of the language")
			})
		}
	})
}

// TestResolveAllocs checks the ceilings of Resolve in the ordinary run,
// which does not run benchmarks.
func TestResolveAllocs(t *testing.T) {
	checkAllocs(t, resolveCalls(t))
}

// BenchmarkResolve measures the resolutions of Resolve under their ceilings.
func BenchmarkResolve(b *testing.B) {
	benchCalls(b, resolveCalls(b))
}

// resolveCalls returns the resolution of a class of the module of the
// subject, and of a dotted name through an import of a module.
func resolveCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := pythonrules.Rules{}
	models := rules.Scope{Subject: idFieldID, File: f.file(tb, modelsPkg, modelsFile)}
	api := rules.Scope{Subject: fetchID, File: f.file(tb, apiPkg, apiFile)}
	var got symbol.Symbol
	return []allocCall{
		{
			name: "Resolve", caseName: "a class of the module", allocs: resolveAllocs,
			call: func() { got, _ = r.Resolve(models, "Item", directive.ResolveTypeInScope, f.view) },
			check: func(tb assert.TB) {
				decl, _ := got.(node.Declaration)
				assert.Equal(tb, decl.Identity(), itemID, "Resolve finds Item")
			},
		},
		{
			name: "Resolve", caseName: "a dotted name through an import", allocs: throughAllocs,
			call: func() { got, _ = r.Resolve(api, "shop.models.Order", directive.ResolveTypeInScope, f.view) },
			check: func(tb assert.TB) {
				decl, _ := got.(node.Declaration)
				assert.Equal(tb, decl.Identity(), orderID, "Resolve finds Order")
			},
		},
	}
}
