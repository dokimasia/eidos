// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
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
	// type: the list of the types it searches, the binding the member
	// walk runs on, and the walk's member list.
	memberAllocs = 3
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
	var (
		got symbol.Symbol
		err error
	)
	return []allocCall{
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
	}
}
