// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// yamlPath is a versioned import path outside the fixture's view.
const yamlPath = "gopkg.in/yaml.v3"

// The packages of the tree of the visibility cases, and its file.
const (
	visPath    = "vis"
	visDepPath = "vis/dep"
	visFile    = "vis/a.go"
)

// The packages of the tree of the package scope cases, and the files of
// their subjects. The package wide imports vol, v3 and yaml in one file, and
// one and two under one alias in two files. The file of Bound imports two
// under the name w, and another file imports vol under the name w. Its
// external test package is wide_test.
const (
	widePath     = "wide"
	wideTestPath = "wide" + golang.TestSuffix
	volPath      = "wide/vol"
	onePath      = "wide/one"
	twoPath      = "wide/two"
	wideFile     = "wide/a.go"
	boundFile    = "wide/e.go"
	wideTestFile = "wide/g_test.go"
)

// The routes of the package scope cases: a directory below the directory of
// Subject and a directory beside it, each with an external test package.
const (
	kitDir        = "wide/kit"
	kitTestPath   = kitDir + golang.TestSuffix
	outerDir      = "outer"
	outerTestPath = outerDir + golang.TestSuffix
)

// refusalPrefix pins the opening of every refusal the Go rules
// return: the language's identity.
const refusalPrefix = string(golang.Lang) + ": "

// The allocations of a resolution.
const (
	// probeAllocs is a resolution through the probe of the subject's
	// file, whose four imports bind no dot import: the file's scope,
	// three allocations, and the list of candidates.
	probeAllocs = 3 + 1
	// memberAllocs is a resolution among the members of the subject's
	// type: the list of the types that it searches and the member list of
	// the walk.
	memberAllocs = 2
	// wideAllocs is a resolution through the import of another file of the
	// package: the map of the scope of the subject's file, which imports
	// nothing, and the list of the one candidate in the imports of the
	// package.
	wideAllocs = 1 + 1
	// routedAllocs is a resolution in the external test package of a route:
	// the map of the scope of the subject's file, which imports nothing, the
	// list of candidates, and the path of the package.
	routedAllocs = 1 + 1 + 3
)

// A directive names a declaration by a spelling, and the Go rules
// resolve it the way Go scopes the spelling. Every resolution kind, and
// every spelling that names nothing, is pinned.
func TestResolve(t *testing.T) {
	t.Parallel()

	r := gorules.New()
	row := id(fxPath, "Row", symbol.KindStruct)
	target := id(depPath, "Target", symbol.KindStruct)
	load := id(fxPath, "Load", symbol.KindFunction)
	find := id(fxPath, "Find", symbol.KindFunction)
	put := method(storeName, "Put")
	get := method(storeName, "Get")

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		// A case resolves from Row in the fixture file, or in a file
		// importing imports alone where it states them.
		scope := func(f *fixture, imports []*node.Import) rules.Scope {
			if imports == nil {
				return f.scope(row)
			}
			return rules.Scope{Subject: row, File: &node.File{Imports: imports}}
		}

		resolves := []struct {
			name    string
			imports []*node.Import
			give    string
			kind    directive.ResolutionKind
			want    symbol.Identity
		}{
			{
				name: "returns the function a bare name declares in the subject's package",
				give: "Find", kind: directive.ResolveCallableInScope,
				want: id(fxPath, "Find", symbol.KindFunction),
			},
			{
				name: "returns the variable a bare name declares in the subject's package",
				give: "Registry", kind: directive.ResolvePackageVar,
				want: id(fxPath, "Registry", symbol.KindVariable),
			},
			{
				name: "returns the constant a bare name declares in the subject's package",
				give: "Limit", kind: directive.ResolvePackageVar,
				want: id(fxPath, "Limit", symbol.KindConstant),
			},
			{
				name: "returns the variant of an enumeration for the name of its constant",
				give: "Green", kind: directive.ResolvePackageVar,
				want: symbol.Identity{
					Lang: golang.Lang, Package: fxPath, Owner: colorType, Name: "Green", Kind: symbol.KindEnumVariant,
				},
			},
			{
				name: "returns the type a bare name declares in the subject's package",
				give: "Row", kind: directive.ResolveTypeInScope, want: row,
			},
			{
				name: "returns the type the package a qualifier binds declares",
				give: "dep.Target", kind: directive.ResolveTypeInScope, want: target,
			},
			{
				name: "returns the type an unaliased import declares for an unbound qualifier",
				give: "ghost.Target", kind: directive.ResolveTypeInScope, want: target,
			},
			{
				name:    "returns the type a dot import declares for a bare name",
				imports: []*node.Import{{Path: depPath, Wildcard: true}},
				give:    "Target", kind: directive.ResolveTypeInScope, want: target,
			},
			{
				name: "returns a stand-in naming itself for a predeclared type",
				give: "int", kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: golang.Lang, Name: "int", Kind: symbol.KindAlias},
			},
			{
				name: "returns a stand-in named by its path for a standard library type",
				give: "time.Duration", kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: golang.Lang, Package: "time", Name: "Duration", Kind: symbol.KindAlias},
			},
			{
				name:    "returns a stand-in named by its path for a versioned import outside the view",
				imports: []*node.Import{{Path: yamlPath}},
				give:    "yaml.Node", kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: golang.Lang, Package: yamlPath, Name: "Node", Kind: symbol.KindAlias},
			},
			{
				name:    "returns the type that a full import path names from a file that does not import it",
				imports: []*node.Import{},
				give:    depPath + ".Target", kind: directive.ResolveTypeInScope, want: target,
			},
			{
				name: "returns the constant that a full import path names",
				give: depPath + ".Ratio", kind: directive.ResolvePackageVar,
				want: id(depPath, "Ratio", symbol.KindConstant),
			},
			{
				name: "returns a stand-in named by its path for a type under a full import path outside the view",
				give: yamlPath + ".Node", kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: golang.Lang, Package: yamlPath, Name: "Node", Kind: symbol.KindAlias},
			},
			{
				name:    "returns a stand-in for a type under a full import path from a file without imports",
				imports: []*node.Import{},
				give:    seedPath + ".Region", kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: golang.Lang, Package: seedPath, Name: "Region", Kind: symbol.KindAlias},
			},
		}
		for _, tt := range resolves {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				got, err := r.Resolve(scope(f, tt.imports), tt.give, tt.kind, f.view)
				assert.NoError(t, err, "the spelling resolves")
				assert.Equal(t, got.(node.Declaration).Identity(), tt.want, "to the declaration it names")
			})
		}

		refuses := []struct {
			name    string
			imports []*node.Import
			subject symbol.Identity
			give    string
			kind    directive.ResolutionKind
		}{
			{
				name: "returns an error for a callable nothing declares",
				give: "Ghost", kind: directive.ResolveCallableInScope,
			},
			{
				name: "returns an error for a name the qualified package does not declare",
				give: "dep.Ghost", kind: directive.ResolveCallableInScope,
			},
			{
				name: "returns an error for an unbound qualifier no unaliased import resolves",
				give: "ghost.Ghost", kind: directive.ResolveTypeInScope,
			},
			{
				name:    "returns an error for the assumed name of a dot import",
				imports: []*node.Import{{Path: depPath, Wildcard: true}},
				subject: target,
				give:    "dep.Target", kind: directive.ResolveTypeInScope,
			},
			{
				name:    "returns an error for the assumed name of a blank import",
				imports: []*node.Import{{Path: depPath, Alias: golang.BlankAlias}},
				subject: target,
				give:    "dep.Target", kind: directive.ResolveTypeInScope,
			},
			{
				name: "returns an error for the constant of an enumeration as a type",
				give: "Green", kind: directive.ResolveTypeInScope,
			},
			{
				name: "returns an error for a type spelling under a slice",
				give: "[]Row", kind: directive.ResolveTypeInScope,
			},
			{
				name: "returns an error for a qualified type spelling under a slice",
				give: "[]dep.Target", kind: directive.ResolveTypeInScope,
			},
			{
				name: "returns an error for an instantiated type spelling",
				give: "dep.Target[int]", kind: directive.ResolveTypeInScope,
			},
			{
				name: "returns an error for a name that the package of a full import path does not declare",
				give: depPath + ".Ghost", kind: directive.ResolveTypeInScope,
			},
			{
				name: "returns an error for a field of a type under a full import path",
				give: depPath + ".Target.V", kind: directive.ResolveTypeInScope,
			},
			{
				name: "returns an error for a callable under a full import path outside the view",
				give: yamlPath + ".Marshal", kind: directive.ResolveCallableInScope,
			},
			{
				name: "returns an error for an import path with an empty element",
				give: "fx//dep.Target", kind: directive.ResolveTypeInScope,
			},
			{
				name: "returns an error for a callable spelling under a pointer",
				give: "*Find", kind: directive.ResolveCallableInScope,
			},
			{
				name: "returns an error for a method named as a field",
				give: "Rename", kind: directive.ResolveValueField,
			},
			{
				name:    "returns an error for a field of a callable whose value has no declaration",
				subject: id(fxPath, "All", symbol.KindFunction),
				give:    "ID", kind: directive.ResolveValueField,
			},
			{
				name:    "returns an error for a field of the receiver of a method",
				subject: put,
				give:    "Size", kind: directive.ResolveValueField,
			},
			{
				name:    "returns an error for a member of a handle the callable does not return",
				subject: put,
				give:    "Rename", kind: directive.ResolveMemberOnHandle,
			},
			{
				name:    "returns an error for a callable that neither the receiver nor the scope declares",
				subject: put,
				give:    "Ghost", kind: directive.ResolveCallableInScope,
			},
			{
				name:    "returns an error for a field of a subject that belongs to no type",
				subject: id(fxPath, "Registry", symbol.KindVariable),
				give:    "ID", kind: directive.ResolveValueField,
			},
			{
				name:    "returns an error for a field of a subject outside the view",
				subject: id(fxPath, "Ghost", symbol.KindStruct),
				give:    "ID", kind: directive.ResolveValueField,
			},
			{
				name:    "returns an error for a parameter the callable does not declare",
				subject: load,
				give:    "ghost", kind: directive.ResolveHostParam,
			},
			{
				name: "returns an error for a parameter of a subject that is no callable",
				give: "id", kind: directive.ResolveHostParam,
			},
			{
				name:    "returns an error for a parameter of a subject outside the view",
				subject: id(fxPath, "Ghost", symbol.KindFunction),
				give:    "id", kind: directive.ResolveHostParam,
			},
			{
				name: "returns an error for a blank spelling",
				give: "  ", kind: directive.ResolveTypeInScope,
			},
			{
				name: "returns an error for the metadata resolution kind",
				give: "ID", kind: directive.ResolveMetadataKey,
			},
		}
		for _, tt := range refuses {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				s := scope(f, tt.imports)
				if !tt.subject.IsZero() {
					s.Subject = tt.subject
				}
				_, err := r.Resolve(s, tt.give, tt.kind, f.view)
				assert.HasError(t, err, "the spelling names nothing at the kind")
			})
		}

		subject := id(widePath, "Subject", symbol.KindStruct)
		bound := id(widePath, "Bound", symbol.KindStruct)
		tested := id(wideTestPath, "Tested", symbol.KindStruct)
		widely := []struct {
			name    string
			subject symbol.Identity
			file    string
			give    string
			kind    directive.ResolutionKind
			want    symbol.Identity
		}{
			{
				name:    "returns the variable that a qualifier names through the import of another file of the package",
				subject: subject,
				file:    wideFile,
				give:    "vol.ErrTooLarge",
				kind:    directive.ResolvePackageVar,
				want:    id(volPath, "ErrTooLarge", symbol.KindVariable),
			},
			{
				name:    "returns the variable of the import of the subject's file over the import of another file",
				subject: bound, file: boundFile, give: "w.Err", kind: directive.ResolvePackageVar,
				want: id(twoPath, "Err", symbol.KindVariable),
			},
			{
				name:    "returns the variable that a qualifier names through the import of an internal test file",
				subject: subject, file: wideFile, give: "fixture.Err", kind: directive.ResolvePackageVar,
				want: id(onePath, "Err", symbol.KindVariable),
			},
			{
				name:    "returns a stand-in named by the path that another file of the package imports",
				subject: subject, file: wideFile, give: "yaml.Node", kind: directive.ResolveTypeInScope,
				want: symbol.Identity{Lang: golang.Lang, Package: yamlPath, Name: "Node", Kind: symbol.KindAlias},
			},
			{
				name:    "returns the function that a bare name declares in the external test package",
				subject: subject, file: wideFile, give: "newSubject", kind: directive.ResolveCallableInScope,
				want: id(wideTestPath, "newSubject", symbol.KindFunction),
			},
			{
				name:    "returns the type that a bare name declares in the external test package",
				subject: subject, file: wideFile, give: "Tested", kind: directive.ResolveTypeInScope,
				want: tested,
			},
			{
				name:    "returns the function of the package over the function of the external test package",
				subject: subject, file: wideFile, give: "build", kind: directive.ResolveCallableInScope,
				want: id(widePath, "build", symbol.KindFunction),
			},
		}
		for _, tt := range widely {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := wideOver(t, tt.subject, tt.file)
				got, err := r.Resolve(f.scope(tt.subject), tt.give, tt.kind, f.view)
				assert.NoError(t, err, "the spelling resolves")
				assert.Equal(t, got.(node.Declaration).Identity(), tt.want, "to the declaration it names")
			})
		}

		widelyRefused := []struct {
			name    string
			subject symbol.Identity
			file    string
			give    string
			kind    directive.ResolutionKind
		}{
			{
				name:    "returns an error for a name that the import of the subject's file does not declare",
				subject: bound, file: boundFile, give: "w.ErrTooLarge", kind: directive.ResolvePackageVar,
			},
			{
				name:    "returns an error for a qualifier that only the package clause of an import of another file names",
				subject: subject,
				file:    wideFile,
				give:    "node.Leaf",
				kind:    directive.ResolveTypeInScope,
			},
			{
				name:    "returns an error for a qualified name that no import of the package declares",
				subject: subject, file: wideFile, give: "ghost.Missing", kind: directive.ResolvePackageVar,
			},
			{
				name:    "returns an error for a bare name that neither the package nor its external test package declares",
				subject: subject,
				file:    wideFile,
				give:    "missing",
				kind:    directive.ResolveCallableInScope,
			},
			{
				name:    "returns an error for a bare name of the package under test from its external test package",
				subject: tested, file: wideTestFile, give: "Subject", kind: directive.ResolveTypeInScope,
			},
		}
		for _, tt := range widelyRefused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := wideOver(t, tt.subject, tt.file)
				_, err := r.Resolve(f.scope(tt.subject), tt.give, tt.kind, f.view)
				assert.HasError(t, err, "the spelling names nothing at the kind")
			})
		}

		routed := []struct {
			name  string
			route string
			give  string
			want  symbol.Identity
		}{
			{
				name:  "returns the function that a bare name declares in the external test package of the route",
				route: kitDir, give: "newKit", want: id(kitTestPath, "newKit", symbol.KindFunction),
			},
			{
				name:  "returns the function of the external test package of the subject's directory over the route's",
				route: kitDir, give: "newSubject", want: id(wideTestPath, "newSubject", symbol.KindFunction),
			},
			{
				name:  "returns the function of the external test package of a route beside the subject's directory",
				route: outerDir, give: "newOuter", want: id(outerTestPath, "newOuter", symbol.KindFunction),
			},
		}
		for _, tt := range routed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := wideOver(t, subject, wideFile)
				scope := f.scope(subject)
				scope.Route = tt.route
				got, err := r.Resolve(scope, tt.give, directive.ResolveCallableInScope, f.view)
				assert.NoError(t, err, "the spelling resolves")
				assert.Equal(t, got.(node.Declaration).Identity(), tt.want, "to the declaration it names")
			})
		}

		routedRefused := []struct {
			name  string
			route string
			give  string
		}{
			{
				name: "returns an error for a bare name of the external test package of a route without the route",
				give: "newKit",
			},
			{
				name:  "returns an error for a bare name that the external test package of the route does not declare",
				route: kitDir, give: "missing",
			},
			{
				name:  "returns an error for a bare name of another directory where the route is the subject's directory",
				route: widePath,
				give:  "newKit",
			},
		}
		for _, tt := range routedRefused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := wideOver(t, subject, wideFile)
				scope := f.scope(subject)
				scope.Route = tt.route
				_, err := r.Resolve(scope, tt.give, directive.ResolveCallableInScope, f.view)
				assert.HasError(t, err, "the spelling names nothing at the kind")
			})
		}

		t.Run("returns an error for a bare name of a route of a subject outside the workspace root",
			func(t *testing.T) {
				t.Parallel()

				f := wideOver(t, subject, wideFile)
				outside := &node.File{Pos: position.Pos{File: "../outside/a.go"}}
				scope := rules.Scope{Subject: subject, File: outside, Route: kitDir}
				_, err := r.Resolve(scope, "newKit", directive.ResolveCallableInScope, f.view)
				assert.HasError(t, err, "the route has no path relative to a file outside the workspace root")
			})

		t.Run("returns an error for a bare name of a route without the file of the subject", func(t *testing.T) {
			t.Parallel()

			f := wideOver(t, subject, wideFile)
			scope := rules.Scope{Subject: subject, Route: kitDir}
			_, err := r.Resolve(scope, "newKit", directive.ResolveCallableInScope, f.view)
			assert.HasError(t, err, "the route resolves against the directory of the file of the subject")
		})

		t.Run("returns an error naming both paths of a qualifier that two other files bind", func(t *testing.T) {
			t.Parallel()

			f := wideOver(t, subject, wideFile)
			_, err := r.Resolve(f.scope(subject), "alt.Err", directive.ResolvePackageVar, f.view)
			assert.HasError(t, err, "the qualifier binds two packages")
			expect.That(t, err.Error()).
				Contains(onePath, "the refusal names the first path").
				Contains(twoPath, "the refusal names the second path")
		})

		t.Run("returns an error naming the spelling nothing declares", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, err := r.Resolve(f.scope(row), "Ghost", directive.ResolveCallableInScope, f.view)
			assert.Contains(t, err.Error(), "Ghost", "the refusal names the spelling")
		})

		t.Run("returns an error that opens with the language's identity", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, err := r.Resolve(f.scope(row), "Ghost", directive.ResolveCallableInScope, f.view)
			assert.HasError(t, err, "a name the scope does not bind is refused")
			assert.HasPrefix(t, err.Error(), refusalPrefix, "every satellite's refusals open alike")
		})

		t.Run("returns the type that an import of the package binds for a subject without a file", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, err := r.Resolve(rules.Scope{Subject: row}, "dep.Target", directive.ResolveTypeInScope, f.view)
			assert.NoError(t, err, "a file of the package binds the qualifier")
			assert.Equal(t, got.(node.Declaration).Identity(), target, "to the type that the import binds")
		})

		t.Run("returns an error for a qualified name from a subject of a package outside the view", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			ghost := id("ghost.test/absent", "Row", symbol.KindStruct)
			_, err := r.Resolve(rules.Scope{Subject: ghost}, "dep.Target", directive.ResolveTypeInScope, f.view)
			assert.HasError(t, err, "no file of the view binds the qualifier")
		})

		t.Run("returns the field a name declares on the subject's type", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, err := r.Resolve(f.scope(row), "ID", directive.ResolveValueField, f.view)
			assert.NoError(t, err, "ID is a field of Row")
			assert.Equal(t, got.(node.Declaration).Identity(), f.field(t, "Row", "ID").ID, "Row's own field")
		})

		t.Run("returns a promoted field on the type that declares it", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			derived := id(fxPath, "Derived", symbol.KindStruct)
			got, err := r.Resolve(f.scope(derived), "Kind", directive.ResolveValueField, f.view)
			assert.NoError(t, err, "the member walk promotes Kind")
			assert.Equal(t, got.(node.Declaration).Identity(), f.field(t, "Base", "Kind").ID, "Base's field")
		})

		t.Run("returns the method the type of a member subject declares", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			field := f.field(t, "Row", "ID")
			got, err := r.Resolve(f.scope(field.ID), "Rename", directive.ResolveMemberOnHandle, f.view)
			assert.NoError(t, err, "Rename is a method of Row")
			assert.Equal(t, got.Kind(), symbol.KindMethod, "a method is a member")
		})

		t.Run("returns the field of the type a function returns", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, err := r.Resolve(f.scope(find), "ID", directive.ResolveValueField, f.view)
			assert.NoError(t, err, "Find returns a Row")
			assert.Equal(t, got.(node.Declaration).Identity(), f.field(t, "Row", "ID").ID, "Row's own field")
		})

		t.Run("returns the field of the type a method takes through a pointer", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, err := r.Resolve(f.scope(put), "ID", directive.ResolveValueField, f.view)
			assert.NoError(t, err, "Put takes a *Row")
			assert.Equal(t, got.(node.Declaration).Identity(), f.field(t, "Row", "ID").ID, "Row's own field")
		})

		t.Run("returns the method of the handle a method returns", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, err := r.Resolve(f.scope(get), "Rename", directive.ResolveMemberOnHandle, f.view)
			assert.NoError(t, err, "Get returns a *Row")
			assert.Equal(t, got.(node.Declaration).Identity(), method("Row", "Rename"), "Row's method")
		})

		t.Run("returns a method of the type a method subject belongs to", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, err := r.Resolve(f.scope(put), "Get", directive.ResolveCallableInScope, f.view)
			assert.NoError(t, err, "Store declares Get")
			assert.Equal(t, got.(node.Declaration).Identity(), get, "the sibling method")
		})

		t.Run("returns a function in scope of a method subject", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, err := r.Resolve(f.scope(put), "Find", directive.ResolveCallableInScope, f.view)
			assert.NoError(t, err, "the package declares Find")
			assert.Equal(t, got.(node.Declaration).Identity(), find, "the function")
		})

		t.Run("returns an error naming the receiver's type for a callable nothing declares", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, err := r.Resolve(f.scope(put), "Ghost", directive.ResolveCallableInScope, f.view)
			assert.HasError(t, err, "neither Store nor the package declares Ghost")
			assert.Contains(t, err.Error(), storeName, "the refusal names the receiver's type")
		})

		t.Run("returns the parameter the subject's signature declares", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, err := r.Resolve(f.scope(load), "id", directive.ResolveHostParam, f.view)
			assert.NoError(t, err, "id is a parameter of Load")
			assert.Equal(t, got.(*node.Param).Name, "id", "the parameter named id")
		})

		t.Run("returns the parameter a method's signature declares", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, err := r.Resolve(f.scope(get), "id", directive.ResolveHostParam, f.view)
			assert.NoError(t, err, "id is a parameter of Get")
			assert.Equal(t, got.(*node.Param).Name, "id", "the parameter named id")
		})

		members := []struct {
			name    string
			subject string
			give    string
			want    symbol.Identity
		}{
			{
				name:    "returns an unexported field of a type of the subject's package",
				subject: "Own", give: "id", want: fieldOf(visPath, "Row", "id"),
			},
			{
				name:    "returns an exported field of a type of another package",
				subject: "Theirs", give: "Key", want: fieldOf(visDepPath, "Item", "Key"),
			},
			{
				name:    "returns the field of the first input that declares it",
				subject: "Save", give: "Seq", want: fieldOf(visPath, "Head", "Seq"),
			},
			{
				name:    "returns the field of a later input where the first declares none",
				subject: "Save", give: "Text", want: fieldOf(visPath, "Body", "Text"),
			},
		}
		for _, tt := range members {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				v := visible(t)
				subject := v.scope(id(visPath, tt.subject, symbol.KindFunction))
				got, err := r.Resolve(subject, tt.give, directive.ResolveValueField, v.view)
				assert.NoError(t, err, "the field resolves from "+tt.subject)
				assert.Equal(t, got.(node.Declaration).Identity(), tt.want, "to the field that the package can use")
			})
		}

		t.Run("returns a method of the interface that a method subject belongs to", func(t *testing.T) {
			t.Parallel()

			v := visible(t)
			source := symbol.Identity{Lang: golang.Lang, Package: visPath, Owner: "Source", Kind: symbol.KindMethod}
			closeMethod, next := source, source
			closeMethod.Name, next.Name = "Close", "Next"
			got, err := r.Resolve(v.scope(closeMethod), "Next", directive.ResolveCallableInScope, v.view)
			assert.NoError(t, err, "Source declares Next")
			assert.Equal(t, got.(node.Declaration).Identity(), next, "the sibling method of the interface")
		})

		t.Run("returns an error for an unexported field of a type of another package", func(t *testing.T) {
			t.Parallel()

			v := visible(t)
			subject := v.scope(id(visPath, "Theirs", symbol.KindFunction))
			_, err := r.Resolve(subject, "value", directive.ResolveValueField, v.view)
			assert.HasError(t, err, "the package vis cannot use the field value of dep.Item")
			assert.Contains(t, err.Error(), visPath, "the refusal names the package of the subject")
		})
	})
}

// A resolution allocates the scope of the subject's file and the
// candidates it probes, or the member walk it reads. The ordinary run,
// which runs no benchmark, checks those ceilings here.
func TestResolveAllocs(t *testing.T) {
	checkAllocs(t, resolveCalls(t))
}

// BenchmarkResolve measures the resolution the kernel makes for every
// spelling a directive names.
func BenchmarkResolve(b *testing.B) {
	benchCalls(b, resolveCalls(b))
}

// visible returns a fixture over the tree of the visibility cases, with a
// tracked view over it.
func visible(tb assert.TB) *fixture {
	tb.Helper()

	f := rulestest.Loaded(tb, gofrontend.New(nil), visibilityTree())
	return viewOver(tb, f, id(visPath, "Row", symbol.KindStruct), visFile)
}

// visibilityTree is the Go source of the visibility cases and the alias
// cases. The package vis returns a type of its own and a type of the
// package vis/dep, each with an unexported field, Save takes two inputs
// that share a field name, and the interface Source declares two methods.
// Ping states its context and its error through aliases, Wait takes a
// type defined over context.Context, and Spin takes a parameter whose
// alias is a cycle.
func visibilityTree() fstest.MapFS {
	return fstest.MapFS{
		visFile: {Data: []byte(`package vis

import (
	"context"

	"vis/dep"
)

type Row struct{ id int }

type Head struct{ Seq int }

type Body struct {
	Seq  int
	Text string
}

type Source interface {
	Next() (int, error)
	Close() error
}

type Ctx = context.Context

type Fault = error

type Scope context.Context

type Loop1 = Loop2

type Loop2 = Loop1

func Ping(ctx Ctx) Fault { return nil }

func Wait(s Scope) error { return nil }

func Spin(l Loop1) error { return nil }

func Own() (Row, error) { return Row{}, nil }

func Theirs() (dep.Item, error) { return dep.Item{}, nil }

func Save(ctx context.Context, h Head, b Body) error { return nil }
`)},
		"vis/dep/d.go": {Data: []byte(`package dep

type Item struct {
	Key   string
	value int
}
`)},
	}
}

// wideOver returns a fixture over the tree of the package scope cases, with
// a tracked view over it and the file at path of the package of subject.
func wideOver(tb assert.TB, subject symbol.Identity, path string) *fixture {
	tb.Helper()

	return viewOver(tb, rulestest.Loaded(tb, gofrontend.New(nil), wideTree()), subject, path)
}

// wideTree is the Go source of the package scope cases. Subject in a.go
// imports nothing, and the other files of its package import what it names.
// Bound in e.go imports two under the name w, which h.go binds to vol. The
// package clause of v3 is node. The internal test file binds fixture, and
// the external test package declares newSubject, Tested and a build of its
// own beside the build of the package. The external test package of the
// route wide/kit declares newKit and a newSubject of its own, and the
// external test package of the route outer declares newOuter.
func wideTree() fstest.MapFS {
	return fstest.MapFS{
		wideFile: {Data: []byte(`package wide

// Subject names the declarations that other files of its package import.
type Subject struct{}
`)},
		"wide/b.go": {Data: []byte(`package wide

import (
	"wide/v3"
	"wide/vol"

	"gopkg.in/yaml.v3"
)

var _, _, _ = vol.ErrTooLarge, node.Leaf{}, yaml.Node{}
`)},
		"wide/c.go": {Data: []byte(`package wide

import alt "wide/one"

var _ = alt.Err
`)},
		"wide/d.go": {Data: []byte(`package wide

import alt "wide/two"

var _ = alt.Err
`)},
		boundFile: {Data: []byte(`package wide

import w "wide/two"

// Bound names the declarations of its own import of w.
type Bound struct{ T w.Thing }
`)},
		"wide/f_test.go": {Data: []byte(`package wide

import fixture "wide/one"

var _ = fixture.Err
`)},
		wideTestFile: {Data: []byte(`package wide_test

import "wide"

func newSubject() *wide.Subject { return &wide.Subject{} }

func build() {}

// Tested is a declaration of the external test package.
type Tested struct{}
`)},
		"wide/h.go": {Data: []byte(`package wide

import w "wide/vol"

var _ = w.Err

func build() {}
`)},
		"wide/vol/v.go": {Data: []byte(`package vol

var ErrTooLarge, Err error
`)},
		"wide/one/o.go": {Data: []byte(`package one

var Err error
`)},
		"wide/two/t.go": {Data: []byte(`package two

var Err error

type Thing struct{}
`)},
		"wide/v3/n.go": {Data: []byte(`package node

type Leaf struct{}
`)},
		kitDir + "/k_test.go": {Data: []byte(`package kit_test

import "wide"

func newKit() *wide.Subject { return &wide.Subject{} }

func newSubject() *wide.Subject { return &wide.Subject{} }
`)},
		outerDir + "/o_test.go": {Data: []byte(`package outer_test

func newOuter() {}
`)},
	}
}

// fieldOf returns the identity of a field of a struct in a package.
func fieldOf(path, host, name string) symbol.Identity {
	return symbol.Identity{Lang: golang.Lang, Package: path, Owner: host, Name: name, Kind: symbol.KindField}
}

// resolveCalls returns a call of Resolve through the probe and through
// the member walk.
func resolveCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := gorules.New()
	scope := f.scope(id(fxPath, "Row", symbol.KindStruct))
	subject := id(widePath, "Subject", symbol.KindStruct)
	w := wideOver(tb, subject, wideFile)
	wideScope := w.scope(subject)
	routedScope := w.scope(subject)
	routedScope.Route = kitDir
	var (
		got symbol.Symbol
		err error
	)
	return []allocCall{
		{
			name: "Resolve", caseName: "a function of the external test package of the route", allocs: routedAllocs,
			call: func() { got, err = r.Resolve(routedScope, "newKit", directive.ResolveCallableInScope, w.view) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Resolve finds newKit")
				assert.Equal(tb, got.Kind(), symbol.KindFunction, "Resolve returns the function")
			},
		},
		{
			name: "Resolve", caseName: "a function in scope", allocs: probeAllocs,
			call: func() { got, err = r.Resolve(scope, "Find", directive.ResolveCallableInScope, f.view) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Resolve finds Find")
				assert.Equal(tb, got.Kind(), symbol.KindFunction, "Resolve returns the function")
			},
		},
		{
			name: "Resolve", caseName: "a member of the subject's type", allocs: memberAllocs,
			call: func() { got, err = r.Resolve(scope, "ID", directive.ResolveValueField, f.view) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Resolve finds ID")
				assert.Equal(tb, got.Kind(), symbol.KindField, "Resolve returns the field")
			},
		},
		{
			name: "Resolve", caseName: "a type under a full import path",
			call: func() { got, err = r.Resolve(scope, depPath+".Target", directive.ResolveTypeInScope, f.view) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Resolve finds Target")
				assert.Equal(tb, got.Kind(), symbol.KindStruct, "Resolve returns the struct")
			},
		},
		{
			name: "Resolve", caseName: "a variable through the import of another file", allocs: wideAllocs,
			call: func() { got, err = r.Resolve(wideScope, "vol.ErrTooLarge", directive.ResolvePackageVar, w.view) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Resolve finds ErrTooLarge")
				assert.Equal(tb, got.Kind(), symbol.KindVariable, "Resolve returns the variable")
			},
		},
	}
}
