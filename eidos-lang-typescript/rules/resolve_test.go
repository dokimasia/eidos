// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/typescript"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// globalPkg is the package of TypeScript's global scope, which a script
// declares into. globalsFile is the fixture's script.
const (
	globalPkg   = ""
	globalsFile = "fx/globals.ts"
)

// resolveAllocs counts the resolution of a type that a named import
// binds from another module through a relative specifier. path.Join
// allocates the joined path, its cleaned copy and the text of the copy.
// The chain of the subject's packages is on the stack, and a view
// records nothing new for reads that it has recorded before.
const resolveAllocs = 3

// The subjects of the resolution cases that do not need a load of the
// fixture.
// rowFieldID is the field id of Row, and the other two are declarations
// that the view does not contain.
var (
	rowFieldID = symbol.Identity{
		Lang: typescript.Lang, Package: aPkg, Owner: rowName, Name: "id", Kind: symbol.KindField,
	}
	goneType     = symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: "Gone", Kind: symbol.KindStruct}
	goneFunction = symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: "Gone", Kind: symbol.KindFunction}
)

// TestResolve checks each resolution kind, each level of TypeScript's
// scope order, and each way in which a module publishes a name. A case
// leaves the language out of the identity that it expects, and the loop
// of its table sets the language.
func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		base := loaded(t)
		helper := methodID(t, base, derivedName, "helper")
		find := base.function(t, "find").ID
		load := base.function(t, loadName).ID
		touch := base.function(t, "touch").ID
		untyped := base.function(t, "untyped").ID
		repoGet := methodID(t, base, "Repo", "get")

		types := []struct {
			name string
			give string
			want symbol.Identity
		}{
			{name: "resolves a type of the subject's module", give: rowName, want: rowID},
			{name: "resolves a type that a named import binds", give: targetName, want: targetID},
			{name: "resolves a type that a namespace import qualifies", give: "dep." + targetName, want: targetID},
			{
				name: "resolves a type that an aliased import binds through an export clause", give: "Renamed",
				want: symbol.Identity{Package: morePkg, Name: "Again", Kind: symbol.KindStruct},
			},
			{
				name: "resolves a type that a default import binds through a default export", give: "Main",
				want: symbol.Identity{Package: barrelPkg, Name: "Main", Kind: symbol.KindStruct},
			},
			{
				name: "resolves a type that an import binds through an export-star statement", give: "Starred",
				want: symbol.Identity{Package: starPkg, Name: "Starred", Kind: symbol.KindInterface},
			},
			{
				name: "resolves a type that an export clause of the module's own scope renames", give: "Shown",
				want: symbol.Identity{Package: barrelPkg, Name: "Hidden2", Kind: symbol.KindStruct},
			},
			{
				name: "resolves a type that a default export of an import publishes", give: "Relayed",
				want: symbol.Identity{Package: morePkg, Name: "Again", Kind: symbol.KindStruct},
			},
			{
				name: "resolves a type of a namespace of the module", give: "N.Inner",
				want: symbol.Identity{Package: innerPkg, Name: "Inner", Kind: symbol.KindInterface},
			},
			{
				name: "resolves a type of the global scope", give: "GlobalThing",
				want: symbol.Identity{Package: globalPkg, Name: "GlobalThing", Kind: symbol.KindInterface},
			},
			{
				name: "resolves a global type as a stand-in", give: stringName,
				want: symbol.Identity{Name: stringName, Kind: symbol.KindAlias},
			},
			{
				name: "resolves a keyword type as a stand-in", give: "unknown",
				want: symbol.Identity{Name: "unknown", Kind: symbol.KindAlias},
			},
			{
				name: "resolves a type that an import binds from a package outside the view as a stand-in",
				give: "Outside",
				want: symbol.Identity{Package: outsideLib, Name: "Outside", Kind: symbol.KindAlias},
			},
			{
				name: "resolves a type that a namespace import qualifies outside the view as a stand-in",
				give: "lib.Thing",
				want: symbol.Identity{Package: outsideLib, Name: "Thing", Kind: symbol.KindAlias},
			},
			{
				name: "resolves a type that a relative import binds outside the view as a stand-in", give: "Lost",
				want: symbol.Identity{Package: "fx/lost", Name: "Lost", Kind: symbol.KindAlias},
			},
		}
		for _, tt := range types {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				scope := rules.Scope{Subject: rowFieldID, File: f.file(t, aPkg, aFile)}
				got, err := tsrules.New().Resolve(scope, tt.give, directive.ResolveTypeInScope, f.view)
				assert.NoError(t, err, "the type resolves")
				decl, names := got.(node.Declaration)
				assert.True(t, names, "the result is a declaration")
				want := tt.want
				want.Lang = typescript.Lang
				assert.Equal(t, decl.Identity(), want, "Resolve returns the declaration of the name")
			})
		}

		t.Run("resolves a type from inside a namespace through the namespace first", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			v := symbol.Identity{
				Lang: typescript.Lang, Package: innerPkg, Owner: "Inner", Name: "v", Kind: symbol.KindField,
			}
			scope := rules.Scope{Subject: v, File: f.file(t, innerPkg, aFile)}
			for name, want := range map[string]symbol.Identity{
				"Inner":    {Lang: typescript.Lang, Package: innerPkg, Name: "Inner", Kind: symbol.KindInterface},
				rowName:    rowID,
				targetName: targetID,
			} {
				got, err := tsrules.New().Resolve(scope, name, directive.ResolveTypeInScope, f.view)
				assert.NoError(t, err, name+" resolves from inside the namespace")
				decl, _ := got.(node.Declaration)
				expect.Equal(t, decl.Identity(), want, "Resolve returns the declaration of "+name)
			}
		})

		t.Run("resolves a type of the global scope from a script", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			g := symbol.Identity{Lang: typescript.Lang, Owner: "GlobalThing", Name: "g", Kind: symbol.KindField}
			scope := rules.Scope{Subject: g, File: f.file(t, globalPkg, globalsFile)}
			got, err := tsrules.New().Resolve(scope, "GlobalThing", directive.ResolveTypeInScope, f.view)
			assert.NoError(t, err, "the script's type resolves")
			decl, _ := got.(node.Declaration)
			want := symbol.Identity{
				Lang: typescript.Lang, Package: globalPkg, Name: "GlobalThing", Kind: symbol.KindInterface,
			}
			assert.Equal(t, decl.Identity(), want, "the declaration is in the global scope")
			_, err = tsrules.New().Resolve(scope, "Nope", directive.ResolveTypeInScope, f.view)
			assert.HasError(t, err, "a name that the global scope does not declare does not resolve")
		})

		t.Run("resolves a type of the module from a declare global block of the module", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s := symbol.Identity{Lang: typescript.Lang, Owner: "Shared", Name: "s", Kind: symbol.KindField}
			scope := rules.Scope{Subject: s, File: f.file(t, globalPkg, aFile)}
			got, err := tsrules.New().Resolve(scope, rowName, directive.ResolveTypeInScope, f.view)
			assert.NoError(t, err, "the module's type resolves after the global scope")
			decl, _ := got.(node.Declaration)
			assert.Equal(t, decl.Identity(), rowID, "the declaration is in the module")
		})

		t.Run("resolves a type of the subject's package from a file that is not a module", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			scope := rules.Scope{Subject: rowFieldID, File: &node.File{Path: "fx/a.mts"}}
			got, err := tsrules.New().Resolve(scope, rowName, directive.ResolveTypeInScope, f.view)
			assert.NoError(t, err, "the subject's own package declares Row")
			decl, _ := got.(node.Declaration)
			expect.Equal(t, decl.Identity(), rowID, "Resolve returns the declaration of Row")
			_, err = tsrules.New().Resolve(scope, targetName, directive.ResolveTypeInScope, f.view)
			assert.HasError(t, err, "the file's module has no imports to bind Target")
		})

		views := []struct {
			name string
			view func(f *fixture) rules.View
		}{
			{
				name: "resolves a type through a view without facts",
				view: func(f *fixture) rules.View {
					return rules.View{Decls: f.view.Decls}
				},
			},
			{
				name: "resolves a type through a view whose registry has no namespace key",
				view: func(f *fixture) rules.View {
					return rules.View{Decls: f.view.Decls, Facts: meta.NewFacts(meta.NewRegistry())}
				},
			},
		}
		for _, tt := range views {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				scope := rules.Scope{Subject: rowFieldID, File: f.file(t, aPkg, aFile)}
				got, err := tsrules.New().Resolve(scope, rowName, directive.ResolveTypeInScope, tt.view(f))
				assert.NoError(t, err, "the module's type resolves without the namespace stamps")
				decl, _ := got.(node.Declaration)
				assert.Equal(t, decl.Identity(), rowID, "Resolve returns the declaration of Row")
			})
		}

		values := []struct {
			name string
			give string
			want symbol.Identity
		}{
			{
				name: "resolves a constant", give: "limit",
				want: symbol.Identity{Package: aPkg, Name: "limit", Kind: symbol.KindConstant},
			},
			{
				name: "resolves a variable", give: "current",
				want: symbol.Identity{Package: aPkg, Name: "current", Kind: symbol.KindVariable},
			},
		}
		for _, tt := range values {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				scope := rules.Scope{Subject: rowFieldID, File: f.file(t, aPkg, aFile)}
				got, err := tsrules.New().Resolve(scope, tt.give, directive.ResolvePackageVar, f.view)
				assert.NoError(t, err, "the variable resolves")
				decl, _ := got.(node.Declaration)
				want := tt.want
				want.Lang = typescript.Lang
				assert.Equal(t, decl.Identity(), want, "Resolve returns the declaration of the variable")
			})
		}

		callables := []struct {
			name    string
			subject symbol.Identity
			give    string
			want    string
		}{
			{
				name: "resolves a method of the type that a method subject belongs to", give: "helper",
				subject: helper, want: "helper",
			},
			{
				name: "resolves a protected method that the subject's own type inherits", give: "guard",
				subject: helper, want: "guard",
			},
			{name: "resolves a function in scope of a method subject", give: "find", subject: helper, want: "find"},
			{name: "resolves a function in scope of a function subject", give: "find", subject: load, want: "find"},
		}
		for _, tt := range callables {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				scope := rules.Scope{Subject: tt.subject, File: f.file(t, aPkg, aFile)}
				got, err := tsrules.New().Resolve(scope, tt.give, directive.ResolveCallableInScope, f.view)
				assert.NoError(t, err, "the callable resolves")
				decl, _ := got.(node.Declaration)
				assert.Equal(t, decl.Identity().Name, tt.want, "Resolve returns the callable of the name")
			})
		}

		members := []struct {
			name    string
			kind    directive.ResolutionKind
			subject symbol.Identity
			give    string
			want    symbol.Identity
		}{
			{
				name: "resolves a field of the type that a field belongs to", kind: directive.ResolveValueField,
				subject: rowFieldID, give: "name",
				want: symbol.Identity{Package: aPkg, Owner: rowName, Name: "name", Kind: symbol.KindField},
			},
			{
				name: "resolves a private field of the subject's own type", kind: directive.ResolveValueField,
				subject: rowFieldID, give: "hidden",
				want: symbol.Identity{Package: aPkg, Owner: rowName, Name: "hidden", Kind: symbol.KindField},
			},
			{
				name: "resolves a protected field that a type inherits", kind: directive.ResolveValueField,
				subject: derivedID, give: "secret",
				want: symbol.Identity{Package: aPkg, Owner: "Base", Name: "secret", Kind: symbol.KindField},
			},
			{
				name: "resolves a field of a callable's optional result", kind: directive.ResolveValueField,
				subject: find, give: "tags",
				want: symbol.Identity{Package: aPkg, Owner: rowName, Name: "tags", Kind: symbol.KindField},
			},
			{
				name: "resolves a field of a callable's promised result", kind: directive.ResolveValueField,
				subject: load, give: "tags",
				want: symbol.Identity{Package: aPkg, Owner: rowName, Name: "tags", Kind: symbol.KindField},
			},
			{
				name: "resolves a field of a callable's input parameter", kind: directive.ResolveValueField,
				subject: touch, give: "v",
				want: symbol.Identity{Package: depPkg, Owner: targetName, Name: "v", Kind: symbol.KindField},
			},
			{
				name: "resolves a member on a callable's handle", kind: directive.ResolveMemberOnHandle,
				subject: find, give: "next",
				want: symbol.Identity{Package: aPkg, Owner: rowName, Name: "next", Kind: symbol.KindField},
			},
			{
				name: "resolves a field of a method's result", kind: directive.ResolveValueField,
				subject: repoGet, give: "tags",
				want: symbol.Identity{Package: aPkg, Owner: rowName, Name: "tags", Kind: symbol.KindField},
			},
		}
		for _, tt := range members {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				scope := rules.Scope{Subject: tt.subject, File: f.file(t, aPkg, aFile)}
				got, err := tsrules.New().Resolve(scope, tt.give, tt.kind, f.view)
				assert.NoError(t, err, "the member resolves")
				decl, _ := got.(node.Declaration)
				want := tt.want
				want.Lang = typescript.Lang
				assert.Equal(t, decl.Identity(), want, "Resolve returns the member of the name")
			})
		}

		t.Run("resolves a parameter of the subject's own signature", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			fn := f.function(t, loadName)
			scope := rules.Scope{Subject: fn.ID, File: f.file(t, aPkg, aFile)}
			got, err := tsrules.New().Resolve(scope, "id", directive.ResolveHostParam, f.view)
			assert.NoError(t, err, "the parameter resolves")
			assert.Equal(t, got, symbol.Symbol(fn.Params[1]), "Resolve returns the parameter id", assert.ByIdentity())
		})

		t.Run("resolves a parameter of a method subject", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			scope := rules.Scope{Subject: methodID(t, f, "Base", "constructor"), File: f.file(t, aPkg, aFile)}
			got, err := tsrules.New().Resolve(scope, "kind", directive.ResolveHostParam, f.view)
			assert.NoError(t, err, "the parameter resolves")
			param, _ := got.(*node.Param)
			assert.Equal(t, param.Name, "kind", "Resolve returns the constructor's parameter")
		})

		refusals := []struct {
			name    string
			kind    directive.ResolutionKind
			subject symbol.Identity
			give    string
			want    string
		}{
			{
				name: "refuses an empty name", kind: directive.ResolveTypeInScope, give: " ",
				want: "nothing to resolve",
			},
			{
				name: "refuses a resolution kind that TypeScript does not perform", kind: directive.ResolveMetadataKey,
				give: rowName, want: "is not a resolution that TypeScript performs",
			},
			{
				name: "refuses a spelling that is not a name", kind: directive.ResolveTypeInScope, give: "string[]",
				want: "is not a TypeScript name",
			},
			{
				name: "refuses a type that nothing in scope declares", kind: directive.ResolveTypeInScope, give: "Nope",
				want: "no Struct or Interface or Enum or Alias named Nope is in scope",
			},
			{
				name: "refuses a name that a cycle of re-exports publishes", kind: directive.ResolveTypeInScope,
				give: "Ghost", want: "named Ghost is in scope",
			},
			{
				name: "refuses a namespace import as a type", kind: directive.ResolveTypeInScope, give: "lib",
				want: "named lib is in scope",
			},
			{
				name: "refuses a class as a variable", kind: directive.ResolvePackageVar, give: rowName,
				want: "no Constant or Variable named Row",
			},
			{
				name: "refuses a callable that neither the type nor the scope declares",
				kind: directive.ResolveCallableInScope, give: "nope", subject: helper,
				want: "has no method named nope, and no function named nope is in scope",
			},
			{
				name: "refuses a private field of a type that the subject inherits", kind: directive.ResolveValueField,
				subject: derivedID, give: "own", want: "has no Field member named own",
			},
			{
				name: "refuses a private member of another type", kind: directive.ResolveMemberOnHandle,
				subject: find, give: "hidden", want: "has no Field or Method member named hidden",
			},
			{
				name: "refuses a member of a callable without a value of a declared type",
				kind: directive.ResolveMemberOnHandle, subject: touch, give: "v",
				want: "has no value of a type that the view declares",
			},
			{
				name: "refuses a field of a callable whose result and parameter have no type",
				kind: directive.ResolveValueField, subject: untyped, give: "v",
				want: "has no value of a type that the view declares",
			},
			{
				name: "refuses a member of a subject outside the view", kind: directive.ResolveValueField,
				subject: goneType, give: "v", want: "does not belong to a type that the view declares",
			},
			{
				name: "refuses a parameter that the signature does not declare", kind: directive.ResolveHostParam,
				subject: load, give: "nope", want: "has no parameter named nope",
			},
			{
				name: "refuses a parameter of a subject that is not a callable", kind: directive.ResolveHostParam,
				subject: rowID, give: "id", want: "is not a callable",
			},
			{
				name: "refuses a parameter of a subject outside the view", kind: directive.ResolveHostParam,
				subject: goneFunction, give: "id", want: "is outside the view",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				subject := rowFieldID
				if !tt.subject.IsZero() {
					subject = tt.subject
				}
				scope := rules.Scope{Subject: subject, File: f.file(t, aPkg, aFile)}
				_, err := tsrules.New().Resolve(scope, tt.give, tt.kind, f.view)
				assert.HasError(t, err, "the name does not resolve")
				expect.Contains(t, err.Error(), tt.want, "the refusal describes the search")
				expect.Contains(t, err.Error(), "typescript: ", "the refusal has the language's prefix")
			})
		}

		t.Run("refuses a qualified name of a namespace that nothing declares", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, err := tsrules.New().Resolve(rules.Scope{Subject: rowFieldID}, "Nowhere.Thing",
				directive.ResolveTypeInScope, f.view)
			assert.HasError(t, err, "the view does not declare a namespace Nowhere")
		})
	})
}

// TestResolveAllocs checks the allocation ceiling of a resolution
// through a relative import. The ordinary test run runs no benchmark, so
// this test checks the ceiling.
func TestResolveAllocs(t *testing.T) {
	checkAllocs(t, resolveCalls(t))
}

// BenchmarkResolve measures the resolution of a type that an import
// binds under its ceiling.
func BenchmarkResolve(b *testing.B) {
	benchCalls(b, resolveCalls(b))
}

// resolveCalls returns the measured call of resolve.go, which resolves a
// type that a named import binds from another module.
func resolveCalls(tb assert.TB) []allocCall {
	f := loaded(tb)
	r := tsrules.New()
	scope := rules.Scope{Subject: rowFieldID, File: f.file(tb, aPkg, aFile)}
	var got symbol.Symbol
	return []allocCall{{
		name:   "Resolve",
		allocs: resolveAllocs,
		call:   func() { got, _ = r.Resolve(scope, targetName, directive.ResolveTypeInScope, f.view) },
		check: func(tb assert.TB) {
			decl, _ := got.(node.Declaration)
			assert.Equal(tb, decl.Identity(), targetID, "Resolve finds the import")
		},
	}}
}

// methodID returns the identity of the first method with a name in a
// class of the fixture's module a. The identity has the discriminator
// that the frontend gave the method.
func methodID(tb assert.TB, f *fixture, host, name string) symbol.Identity {
	tb.Helper()

	var found symbol.Identity
	for _, m := range f.class(tb, host).Methods {
		if m.Name == name && found.IsZero() {
			found = m.ID
		}
	}
	assert.False(tb, found.IsZero(), host+" declares the method "+name)
	return found
}
