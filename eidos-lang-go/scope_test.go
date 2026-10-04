// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The import paths and qualifiers the cases bind and probe.
const (
	ownPath    = "example.test/own"
	apiPath    = "example.test/fix/api"
	otherPath  = "example.test/fix/other"
	blankPath  = "example.test/fix/blank"
	dotPath    = "example.test/fix/dot"
	dottedPath = "example.test/fix/dotted"
	yamlPath   = "gopkg.in/yaml.v3"
	otherAlias = "ren"
)

// The allocations of a scope.
const (
	// scopeAllocs is the scope of a file without a dot import: the map
	// of qualifiers, two allocations up to eight imports, and the list of
	// unaliased imports.
	scopeAllocs = 2 + 1
	// candidatesAllocs is the list of candidates a spelling probes.
	candidatesAllocs = 1
)

// The load's resolution step, the rules and the frontend all resolve a
// spelling through a file's scope. What each import binds, and the probe
// order of every spelling, are pinned.
func TestScope(t *testing.T) {
	t.Parallel()

	t.Run("NewScope", func(t *testing.T) {
		t.Parallel()

		t.Run("binds the later path for a qualifier two imports bind", func(t *testing.T) {
			t.Parallel()

			s := golang.NewScope([]*node.Import{{Path: apiPath}, {Path: otherPath, Alias: "api"}})
			path, bound := s.Import("api")
			assert.True(t, bound, "the qualifier is bound")
			assert.Equal(t, path, otherPath, "to the later import")
		})

		t.Run("skips a nil import record", func(t *testing.T) {
			t.Parallel()

			s := golang.NewScope([]*node.Import{nil, {Path: apiPath}})
			path, bound := s.Import("api")
			assert.True(t, bound, "the record after the nil one binds")
			assert.Equal(t, path, apiPath, "its path")
		})
	})

	t.Run("Import", func(t *testing.T) {
		t.Parallel()

		s := golang.NewScope(imports())
		tests := []struct {
			name      string
			give      string
			wantPath  string
			wantBound bool
		}{
			{
				name: "returns the path of an unaliased import under its assumed name",
				give: "api", wantPath: apiPath, wantBound: true,
			},
			{
				name: "returns the path of an aliased import under its alias",
				give: otherAlias, wantPath: otherPath, wantBound: true,
			},
			{
				name: "returns the path of a versioned import under its assumed name",
				give: "yaml", wantPath: yamlPath, wantBound: true,
			},
			{name: "reports false for the assumed name of an aliased import", give: "other"},
			{name: "reports false for the blank alias", give: golang.BlankAlias},
			{name: "reports false for the assumed name of a blank import", give: "blank"},
			{name: "reports false for the assumed name of a wildcard import", give: "dot"},
			{name: "reports false for the assumed name of a dot-aliased import", give: "dotted"},
			{name: "reports false for the dot alias", give: golang.DotAlias},
			{name: "reports false for an empty qualifier", give: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				path, bound := s.Import(tt.give)
				assert.Equal(t, bound, tt.wantBound, "whether an import binds the qualifier")
				assert.Equal(t, path, tt.wantPath, "the path it binds")
			})
		}

		t.Run("reports false on the zero scope", func(t *testing.T) {
			t.Parallel()

			_, bound := golang.Scope{}.Import("api")
			assert.False(t, bound, "the zero scope binds nothing")
		})
	})

	t.Run("Candidates", func(t *testing.T) {
		t.Parallel()

		s := golang.NewScope(imports())
		tests := []struct {
			name string
			give string
			want []symbol.Identity
		}{
			{
				name: "returns the import a qualifier binds",
				give: "api.User", want: []symbol.Identity{at(apiPath, "User")},
			},
			{
				name: "returns the import an alias binds",
				give: otherAlias + ".Thing", want: []symbol.Identity{at(otherPath, "Thing")},
			},
			{
				name: "returns the import a versioned path binds",
				give: "yaml.Node", want: []symbol.Identity{at(yamlPath, "Node")},
			},
			{
				name: "returns every unaliased import in source order for an unbound qualifier",
				give: "ghost.X", want: []symbol.Identity{at(apiPath, "X"), at(yamlPath, "X")},
			},
			{
				name: "returns every unaliased import for the qualifier of a blank import",
				give: "blank.X", want: []symbol.Identity{at(apiPath, "X"), at(yamlPath, "X")},
			},
			{
				name: "returns the own package then each dot import for an exported bare name",
				give: "Thing",
				want: []symbol.Identity{at(ownPath, "Thing"), at(dotPath, "Thing"), at(dottedPath, "Thing")},
			},
			{
				name: "returns the own package alone for an unexported bare name",
				give: "thing", want: []symbol.Identity{at(ownPath, "thing")},
			},
			{
				name: "returns the named type under a pointer to a slice",
				give: "*[]api.User", want: []symbol.Identity{at(apiPath, "User")},
			},
			{
				name: "returns the named type under an array length",
				give: "[4]api.User", want: []symbol.Identity{at(apiPath, "User")},
			},
			{
				name: "returns the named type under a variadic mark",
				give: "...api.User", want: []symbol.Identity{at(apiPath, "User")},
			},
			{
				name: "returns the named type inside parentheses",
				give: "( thing )", want: []symbol.Identity{at(ownPath, "thing")},
			},
			{
				name: "returns the named type without its instantiation",
				give: "*api.List[api.User]", want: []symbol.Identity{at(apiPath, "List")},
			},
			{
				name: "returns the named type of a spelling inside spaces",
				give: " api.User ", want: []symbol.Identity{at(apiPath, "User")},
			},
			{name: "returns nothing for a predeclared type", give: "int"},
			{name: "returns nothing for a map", give: "map[string]int"},
			{name: "returns nothing for a func type", give: "func(int) error"},
			{name: "returns nothing for a channel", give: "chan int"},
			{name: "returns nothing for an inline struct", give: "struct{}"},
			{name: "returns nothing for a composite keyword alone", give: "interface"},
			{name: "returns nothing for an approximation", give: "~int"},
			{name: "returns nothing for a union", give: "A|B"},
			{name: "returns nothing for an array whose bracket never closes", give: "[4api.User"},
			{name: "returns nothing for an empty spelling", give: ""},
			{name: "returns nothing for a pointer mark alone", give: "*"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, s.Candidates(ownPath, tt.give), tt.want, "the candidates in probe order")
			})
		}

		t.Run("returns nothing for an unbound qualifier on the zero scope", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, golang.Scope{}.Candidates(ownPath, "api.User"), "no import can declare the name")
		})

		t.Run("returns the own package for a bare name on the zero scope", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, golang.Scope{}.Candidates(ownPath, "Thing"),
				[]symbol.Identity{at(ownPath, "Thing")}, "the own package alone")
		})
	})
}

// A scope allocates its map and its list of unaliased imports, a lookup
// nothing, and a probe the list it returns. The ordinary run, which runs
// no benchmark, checks those ceilings here.
func TestScopeAllocs(t *testing.T) {
	plain, s := plainImports(), golang.NewScope(imports())
	var (
		scope golang.Scope
		path  string
		bound bool
		got   []symbol.Identity
	)
	assert.MaxAllocs(t, func() { scope = golang.NewScope(plain) }, scopeAllocs,
		"NewScope allocates the map and the list of unaliased imports")
	path, bound = scope.Import("api")
	assert.True(t, bound && path == apiPath, "NewScope binds the imports")
	assert.MaxAllocs(t, func() { path, bound = s.Import("api") }, 0, "Import allocates nothing")
	assert.True(t, bound && path == apiPath, "Import returns the bound path")
	assert.MaxAllocs(t, func() { got = s.Candidates(ownPath, "Thing") }, candidatesAllocs,
		"Candidates allocates the list once")
	assert.Length(t, got, 3, "Candidates probes the own package and both dot imports")
	assert.MaxAllocs(t, func() { got = s.Candidates(ownPath, "ghost.X") }, candidatesAllocs,
		"Candidates allocates the list of unaliased imports once")
	assert.Length(t, got, 2, "Candidates probes both unaliased imports")
}

// BenchmarkScope measures the scope the frontend and the rules derive
// once per file, and the lookup and the probe they make per reference.
func BenchmarkScope(b *testing.B) {
	b.Run("NewScope", func(b *testing.B) {
		plain := plainImports()
		c := bench.Start(b).MaxAllocs(scopeAllocs)
		defer c.End()
		var scope golang.Scope
		for c.Loop() {
			scope = golang.NewScope(plain)
		}
		_, bound := scope.Import("api")
		assert.True(b, bound, "NewScope binds the imports")
	})

	b.Run("Import", func(b *testing.B) {
		s := golang.NewScope(imports())
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var path string
		for c.Loop() {
			path, _ = s.Import("api")
		}
		assert.Equal(b, path, apiPath, "Import returns the bound path")
	})

	b.Run("Candidates", func(b *testing.B) {
		s := golang.NewScope(imports())
		c := bench.Start(b).MaxAllocs(candidatesAllocs)
		defer c.End()
		var got []symbol.Identity
		for c.Loop() {
			got = s.Candidates(ownPath, "api.User")
		}
		assert.Equal(b, got, []symbol.Identity{at(apiPath, "User")}, "Candidates returns the bound import")
	})
}

// imports is a file's import records in source order: two
// unaliased, one aliased, one blank, and a dot import in each of its
// two recorded forms.
func imports() []*node.Import {
	return []*node.Import{
		{Path: apiPath},
		{Path: otherPath, Alias: otherAlias},
		{Path: blankPath, Alias: golang.BlankAlias},
		{Path: dotPath, Wildcard: true},
		{Path: dottedPath, Alias: golang.DotAlias},
		{Path: yamlPath},
	}
}

// plainImports is a file's import records without a dot import: two
// unaliased and one aliased.
func plainImports() []*node.Import {
	return []*node.Import{{Path: apiPath}, {Path: otherPath, Alias: otherAlias}, {Path: yamlPath}}
}

// at returns a candidate identity: a package and a name in Go, with
// no kind.
func at(pkg, name string) symbol.Identity {
	return symbol.Identity{Lang: golang.Lang, Package: pkg, Name: name}
}
