// Copyright Dokimasia B.V. 2026
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

// These constants are the import paths and the qualifiers that the cases
// bind and probe.
const (
	ownPath    = "example.test/own"
	apiPath    = "example.test/fix/api"
	otherPath  = "example.test/fix/other"
	blankPath  = "example.test/fix/blank"
	dotPath    = "example.test/fix/dot"
	dottedPath = "example.test/fix/dotted"
	yamlPath   = "gopkg.in/yaml.v3"
	secondPath = "example.test/fix/second"
	otherAlias = "ren"
)

// These constants are the allocations of the bindings and of a probe.
const (
	// bindingsAllocs is the count of the bindings of a file without a dot
	// import. The map of qualifiers costs two allocations up to eight
	// imports, and the list of unaliased imports costs one.
	bindingsAllocs = 2 + 1
	// candidatesAllocs is the count of the list of candidates that a probe
	// returns.
	candidatesAllocs = 1
)

// The resolution step of the load, the rules and the frontend resolve a
// spelling through the bindings of a file, so the contract covers what each
// import binds and the probe order of every spelling.
func TestScope(t *testing.T) {
	t.Parallel()

	t.Run("NewBindings", func(t *testing.T) {
		t.Parallel()

		t.Run("binds the later path for a qualifier that two imports bind", func(t *testing.T) {
			t.Parallel()

			b := golang.NewBindings([]*node.Import{{Path: apiPath}, {Path: otherPath, Alias: "api"}})
			path, bound := b.Import("api")
			assert.True(t, bound, "an import binds the qualifier")
			assert.Equal(t, path, otherPath, "the later import binds the qualifier")
		})

		t.Run("skips a nil import record", func(t *testing.T) {
			t.Parallel()

			b := golang.NewBindings([]*node.Import{nil, {Path: apiPath}})
			path, bound := b.Import("api")
			assert.True(t, bound, "the record after the nil record binds its qualifier")
			assert.Equal(t, path, apiPath, "the qualifier is bound to the path of that record")
		})
	})

	t.Run("Import", func(t *testing.T) {
		t.Parallel()

		b := golang.NewBindings(imports())
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

				path, bound := b.Import(tt.give)
				assert.Equal(t, bound, tt.wantBound, "Import reports whether an import binds the qualifier")
				assert.Equal(t, path, tt.wantPath, "Import returns the path that the import binds")
			})
		}

		t.Run("reports false on the zero Bindings", func(t *testing.T) {
			t.Parallel()

			_, bound := golang.Bindings{}.Import("api")
			assert.False(t, bound, "the zero Bindings binds nothing")
		})
	})

	t.Run("Candidates", func(t *testing.T) {
		t.Parallel()

		b := golang.NewBindings(imports())
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

				assert.Equal(t, b.Candidates(ownPath, tt.give), tt.want,
					"Candidates returns the candidates in probe order")
			})
		}

		t.Run("returns nothing for an unbound qualifier on the zero Bindings", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, golang.Bindings{}.Candidates(ownPath, "api.User"), "no import can declare the name")
		})

		t.Run("returns the own package for a bare name on the zero Bindings", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, golang.Bindings{}.Candidates(ownPath, "Thing"),
				[]symbol.Identity{at(ownPath, "Thing")}, "the zero Bindings probes the own package alone")
		})
	})

	t.Run("PackageBindings", func(t *testing.T) {
		t.Parallel()

		b := golang.PackageBindings(packageFiles())

		t.Run("Import", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name      string
				give      string
				wantPath  string
				wantBound bool
			}{
				{
					name: "returns the path that a file of the package binds to a qualifier",
					give: "yaml", wantPath: yamlPath, wantBound: true,
				},
				{
					name: "returns the path that two files bind to a qualifier",
					give: "api", wantPath: apiPath, wantBound: true,
				},
				{name: "reports false for a qualifier that two files bind to different paths", give: otherAlias},
				{name: "reports false for the assumed name of a blank import", give: "blank"},
				{name: "reports false for the assumed name of a wildcard import", give: "dot"},
				{name: "reports false for a qualifier that no file binds", give: "ghost"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					path, bound := b.Import(tt.give)
					assert.Equal(t, bound, tt.wantBound, "Import reports whether one path is bound to the qualifier")
					assert.Equal(t, path, tt.wantPath, "Import returns the path that the files bind")
				})
			}

			t.Run("reports false on a nil PackageBindings", func(t *testing.T) {
				t.Parallel()

				_, bound := golang.PackageBindings(nil).Import("api")
				assert.False(t, bound, "a nil PackageBindings binds nothing")
			})
		})

		t.Run("Candidates", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name          string
				give          string
				want          []symbol.Identity
				wantAmbiguous bool
			}{
				{
					name: "returns the import path that a file of the package binds",
					give: "yaml.Node", want: []symbol.Identity{at(yamlPath, "Node")},
				},
				{
					name: "returns one candidate for a qualifier that two files bind to one path",
					give: "api.User", want: []symbol.Identity{at(apiPath, "User")},
				},
				{
					name:          "returns each path of a qualifier that two files bind to different paths",
					give:          otherAlias + ".Thing",
					want:          []symbol.Identity{at(otherPath, "Thing"), at(secondPath, "Thing")},
					wantAmbiguous: true,
				},
				{name: "returns nothing for a qualifier that no file binds", give: "ghost.X"},
				{
					name: "returns the named type under a pointer to a slice",
					give: "*[]api.User", want: []symbol.Identity{at(apiPath, "User")},
				},
				{name: "returns nothing for a bare name", give: "Thing"},
				{name: "returns nothing for an empty spelling", give: ""},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, ambiguous := b.Candidates(golang.Bindings{}, tt.give)
					assert.Equal(t, got, tt.want, "Candidates returns the candidates in file order")
					assert.Equal(t, ambiguous, tt.wantAmbiguous, "Candidates reports a qualifier of two paths")
				})
			}

			t.Run("returns nothing for a qualifier that the imports of the file bind", func(t *testing.T) {
				t.Parallel()

				got, _ := b.Candidates(golang.NewBindings([]*node.Import{{Path: secondPath, Alias: "api"}}), "api.User")
				assert.Empty(t, got, "the import of the file takes precedence over the imports of the package")
			})

			t.Run("returns each path once for a qualifier that three files bind to two paths", func(t *testing.T) {
				t.Parallel()

				again := &node.File{Imports: []*node.Import{{Path: otherPath, Alias: otherAlias}}}
				got, ambiguous := golang.PackageBindings(append(packageFiles(), again)).
					Candidates(golang.Bindings{}, otherAlias+".Thing")
				assert.Equal(t, got, []symbol.Identity{at(otherPath, "Thing"), at(secondPath, "Thing")},
					"Candidates returns each path in the order of its first file")
				assert.True(t, ambiguous, "Candidates reports a qualifier of two paths")
			})

			t.Run("returns nothing for an unbound qualifier on a nil PackageBindings", func(t *testing.T) {
				t.Parallel()

				got, _ := golang.PackageBindings(nil).Candidates(golang.Bindings{}, "api.User")
				assert.Empty(t, got, "no file can declare the name")
			})
		})
	})
}

// NewBindings allocates the map and the list of unaliased imports. Import
// allocates nothing, and Candidates allocates the list that it returns, in
// the scope of a file and in the scope of a package. The ordinary run
// checks these ceilings, because it runs no benchmark.
func TestScopeAllocs(t *testing.T) {
	plain, b := plainImports(), golang.NewBindings(imports())
	var (
		bindings golang.Bindings
		path     string
		bound    bool
		got      []symbol.Identity
	)
	assert.MaxAllocs(t, func() { bindings = golang.NewBindings(plain) }, bindingsAllocs,
		"NewBindings allocates the map and the list of unaliased imports")
	path, bound = bindings.Import("api")
	assert.True(t, bound, "NewBindings binds the imports")
	assert.Equal(t, path, apiPath, "NewBindings binds api to its path")
	assert.MaxAllocs(t, func() { path, bound = b.Import("api") }, 0, "Import allocates nothing")
	assert.True(t, bound, "Import finds the bound name")
	assert.Equal(t, path, apiPath, "Import returns the bound path")
	assert.MaxAllocs(t, func() { got = b.Candidates(ownPath, "Thing") }, candidatesAllocs,
		"Candidates allocates the list once")
	assert.Length(t, got, 3, "Candidates probes the own package and both dot imports")
	assert.MaxAllocs(t, func() { got = b.Candidates(ownPath, "ghost.X") }, candidatesAllocs,
		"Candidates allocates the list of unaliased imports once")
	assert.Length(t, got, 2, "Candidates probes both unaliased imports")

	pb := golang.PackageBindings(packageFiles())
	assert.MaxAllocs(t, func() { path, bound = pb.Import("yaml") }, 0, "PackageBindings.Import allocates nothing")
	assert.True(t, bound, "PackageBindings.Import finds the bound name")
	assert.Equal(t, path, yamlPath, "PackageBindings.Import returns the bound path")
	assert.MaxAllocs(t, func() { got, _ = pb.Candidates(golang.Bindings{}, "api.User") }, candidatesAllocs,
		"PackageBindings.Candidates allocates the list of one path once")
	assert.Length(t, got, 1, "PackageBindings.Candidates returns the one path of the qualifier")
	assert.MaxAllocs(t, func() { got, _ = pb.Candidates(golang.Bindings{}, otherAlias+".Thing") }, candidatesAllocs,
		"PackageBindings.Candidates allocates the list of the paths of the qualifier once")
	assert.Length(t, got, 2, "PackageBindings.Candidates returns both paths of the qualifier")
}

// BenchmarkScope measures the bindings that the frontend and the rules
// derive once for each file, and the lookup and the probe that they make for
// each reference.
func BenchmarkScope(b *testing.B) {
	b.Run("NewBindings", func(b *testing.B) {
		plain := plainImports()
		c := bench.Start(b).MaxAllocs(bindingsAllocs)
		defer c.End()
		var bindings golang.Bindings
		for c.Loop() {
			bindings = golang.NewBindings(plain)
		}
		_, bound := bindings.Import("api")
		assert.True(b, bound, "NewBindings binds the imports")
	})

	b.Run("Import", func(b *testing.B) {
		bindings := golang.NewBindings(imports())
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var path string
		for c.Loop() {
			path, _ = bindings.Import("api")
		}
		assert.Equal(b, path, apiPath, "Import returns the bound path")
	})

	b.Run("Candidates", func(b *testing.B) {
		bindings := golang.NewBindings(imports())
		c := bench.Start(b).MaxAllocs(candidatesAllocs)
		defer c.End()
		var got []symbol.Identity
		for c.Loop() {
			got = bindings.Candidates(ownPath, "api.User")
		}
		assert.Equal(b, got, []symbol.Identity{at(apiPath, "User")}, "Candidates returns the bound import")
	})

	b.Run("PackageBindings", func(b *testing.B) {
		b.Run("Import", func(b *testing.B) {
			bindings := golang.PackageBindings(packageFiles())
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var path string
			for c.Loop() {
				path, _ = bindings.Import("yaml")
			}
			assert.Equal(b, path, yamlPath, "Import returns the bound path")
		})

		b.Run("Candidates", func(b *testing.B) {
			bindings := golang.PackageBindings(packageFiles())
			c := bench.Start(b).MaxAllocs(candidatesAllocs)
			defer c.End()
			var got []symbol.Identity
			for c.Loop() {
				got, _ = bindings.Candidates(golang.Bindings{}, "api.User")
			}
			assert.Equal(b, got, []symbol.Identity{at(apiPath, "User")}, "Candidates returns the bound import")
		})
	})
}

// imports returns the import records of a file in source order. They are
// two unaliased imports, one aliased import, one blank import, and a dot
// import in each of its two recorded forms.
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

// plainImports returns the import records of a file without a dot import.
// They are two unaliased imports and one aliased import.
func plainImports() []*node.Import {
	return []*node.Import{{Path: apiPath}, {Path: otherPath, Alias: otherAlias}, {Path: yamlPath}}
}

// packageFiles returns the files of a package in file order. The first file
// imports api and binds ren to other. The second imports yaml and api again,
// with a dot import, a blank import and a nil import record. A nil file
// follows. The last file binds ren to second, so two files bind ren to
// different paths.
func packageFiles() []*node.File {
	return []*node.File{
		{Imports: []*node.Import{{Path: apiPath}, {Path: otherPath, Alias: otherAlias}}},
		{Imports: []*node.Import{
			{Path: yamlPath},
			{Path: apiPath},
			{Path: dotPath, Wildcard: true},
			{Path: blankPath, Alias: golang.BlankAlias},
			nil,
		}},
		nil,
		{Imports: []*node.Import{{Path: secondPath, Alias: otherAlias}}},
	}
}

// at returns a candidate identity in Go with a package, a name and no kind.
func at(pkg, name string) symbol.Identity {
	return symbol.Identity{Lang: golang.Lang, Package: pkg, Name: name}
}
