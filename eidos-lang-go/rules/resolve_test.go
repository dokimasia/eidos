// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// yamlPath is a versioned import path outside the fixture's view.
const yamlPath = "gopkg.in/yaml.v3"

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
	// type: the binding the member walk runs on, and the walk's member
	// list.
	memberAllocs = 2
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
				give:    "dep.Target", kind: directive.ResolveTypeInScope,
			},
			{
				name:    "returns an error for the assumed name of a blank import",
				imports: []*node.Import{{Path: depPath, Alias: golang.BlankAlias}},
				give:    "dep.Target", kind: directive.ResolveTypeInScope,
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
				name: "returns an error for a callable spelling under a pointer",
				give: "*Find", kind: directive.ResolveCallableInScope,
			},
			{
				name: "returns an error for a method named as a field",
				give: "Rename", kind: directive.ResolveValueField,
			},
			{
				name:    "returns an error for a field of a subject that belongs to no type",
				subject: id(fxPath, "Find", symbol.KindFunction),
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
			assert.True(t, strings.HasPrefix(err.Error(), refusalPrefix), "every satellite's refusals open alike")
		})

		t.Run("returns an error for a qualified name from a subject without a file", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, err := r.Resolve(rules.Scope{Subject: row}, "dep.Target", directive.ResolveTypeInScope, f.view)
			assert.HasError(t, err, "no import binds the qualifier")
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

		t.Run("returns the parameter the subject's signature declares", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, err := r.Resolve(f.scope(load), "id", directive.ResolveHostParam, f.view)
			assert.NoError(t, err, "id is a parameter of Load")
			assert.Equal(t, got.(*node.Param).Name, "id", "the parameter named id")
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

// resolveCalls returns a call of Resolve through the probe and through
// the member walk.
func resolveCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := gorules.New()
	scope := f.scope(id(fxPath, "Row", symbol.KindStruct))
	var (
		got symbol.Symbol
		err error
	)
	return []allocCall{
		{
			name: "Resolve", allocs: probeAllocs,
			call: func() { got, err = r.Resolve(scope, "Find", directive.ResolveCallableInScope, f.view) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Resolve finds Find")
				assert.Equal(tb, got.Kind(), symbol.KindFunction, "Resolve returns the function")
			},
		},
		{
			name: "Resolve/a member of the subject's type", allocs: memberAllocs,
			call: func() { got, err = r.Resolve(scope, "ID", directive.ResolveValueField, f.view) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Resolve finds ID")
				assert.Equal(tb, got.Kind(), symbol.KindField, "Resolve returns the field")
			},
		},
	}
}
