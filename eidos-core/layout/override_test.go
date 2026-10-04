// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The reserved keys and the kernel out directive's keys, as an author
// writes them.
var (
	keyOut  = string(directive.ReservedOut)
	keyTag  = string(directive.ReservedTag)
	keyPath = string(directive.OutPath)
	keyKTag = string(directive.OutTag)
)

// An author routes one declaration's output with the reserved keys on
// a plugin's own directive, or with the kernel out directive for every
// plugin's output.
func TestOverride(t *testing.T) {
	t.Parallel()

	t.Run("Route", func(t *testing.T) {
		t.Parallel()

		layouts := []struct {
			name string
			give func() *fixture
			want []string
		}{
			{
				name: "moves a primary declaration to the family its plugin's tag names",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID, written(stubDirective, 1, keyTag, tagTest))
				},
				want: []string{"svc/store/store_stub_test.go: StoreStub"},
			},
			{
				name: "moves a primary declaration to the family the kernel out directive's tag names",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID, written(directive.KernelOut, 1, keyKTag, tagTest))
				},
				want: []string{"svc/store/store_stub_test.go: StoreStub"},
			},
			{
				name: "moves a primary declaration by its plugin's tag over the kernel out directive's",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID,
						written(directive.KernelOut, 1, keyKTag, tagPkg),
						written(stubDirective, 2, keyTag, tagTest))
				},
				want: []string{"svc/store/store_stub_test.go: StoreStub"},
			},
			{
				name: "moves a primary declaration by the first instance's tag",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID,
						written(stubDirective, 1, keyTag, tagTest),
						written(stubDirective, 2, keyTag, tagPkg))
				},
				want: []string{"svc/store/store_stub_test.go: StoreStub"},
			},
			{
				name: "keeps a declaration in the tagged family its handler addressed",
				give: func() *fixture {
					return newFixture(storeStubTest()).on(storeID, written(stubDirective, 1, keyTag, tagPkg))
				},
				want: []string{"svc/store/store_stub_test.go: StoreStubTest"},
			},
			{
				name: "ignores the routing of a negated directive",
				give: func() *fixture {
					d := written(stubDirective, 1, keyTag, tagTest)
					d.Negated = true
					return newFixture(storeStub()).on(storeID, d)
				},
				want: []string{"svc/store/store_stub.go: StoreStub"},
			},
			{
				name: "ignores the reserved keys of another plugin's directive",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID, written(docDirective, 1, keyTag, tagTest))
				},
				want: []string{"svc/store/store_stub.go: StoreStub"},
			},
			{
				name: "moves a declaration into the directory a path with a trailing slash names",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID, written(stubDirective, 1, keyOut, "mocks/"))
				},
				want: []string{"svc/store/mocks/store_stub.go: StoreStub"},
			},
			{
				name: "moves a declaration to the file a path names",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID, written(stubDirective, 1, keyOut, "mocks/fake.go"))
				},
				want: []string{"svc/store/mocks/fake.go: StoreStub"},
			},
			{
				name: "moves a declaration into the parent directory two dots name",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID, written(stubDirective, 1, keyOut, ".."))
				},
				want: []string{"svc/store_stub.go: StoreStub"},
			},
			{
				name: "moves a declaration by the kernel out directive's path",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID, written(directive.KernelOut, 1, keyPath, "mocks/"))
				},
				want: []string{"svc/store/mocks/store_stub.go: StoreStub"},
			},
			{
				name: "resolves a path against the directory of the origin's source",
				give: func() *fixture {
					return newFixture(stubOf(storeFile, storePkg, generated(cacheID, "CacheStub"))).
						on(cacheID, written(stubDirective, 1, keyOut, "mocks/"))
				},
				want: []string{"svc/cache/mocks/store_stub.go: CacheStub"},
			},
			{
				name: "moves the declarations of two families to the one file a tagged path names",
				give: func() *fixture {
					return newFixture(storeStub(), storeStubTest()).
						on(storeID, written(directive.KernelOut, 1, keyPath, "fake.go", keyKTag, tagTest))
				},
				want: []string{"svc/store/fake.go: StoreStub StoreStubTest"},
			},
			{
				name: "moves the declarations of two families into the directory a path names",
				give: func() *fixture {
					return newFixture(storeStub(), storeStubTest()).
						on(storeID, written(stubDirective, 1, keyOut, "mocks/"))
				},
				want: []string{
					"svc/store/mocks/store_stub.go: StoreStub",
					"svc/store/mocks/store_stub_test.go: StoreStubTest",
				},
			},
			{
				name: "moves a declaration to its package's file of a per-package family",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID, written(stubDirective, 1, keyTag, tagPkg))
				},
				want: []string{"svc/store/suite_pkg.go: StoreStub"},
			},
			{
				name: "moves a declaration to the plan's file of a per-plan family",
				give: func() *fixture {
					f := newFixture(storeStub()).on(storeID, written(stubDirective, 1, keyTag, tagPlan))
					f.config = layout.Config{Dir: genDir}
					return f
				},
				want: []string{"gen/index_plan.go: StoreStub"},
			},
			{
				name: "moves a declaration into its package's file of a per-package family",
				give: func() *fixture {
					return newFixture(rowStub(), unitOf(stubgen, families()[stubgen][2], storePkg,
						coretest.PackageID(storePkg), generated(storeID, "StoreSuite"))).
						on(rowID, written(stubDirective, 1, keyTag, tagPkg))
				},
				want: []string{"svc/store/suite_pkg.go: RowStub StoreSuite"},
			},
			{
				name: "moves a declaration from a per-package family to its source's file",
				give: func() *fixture {
					return newFixture(unitOf(pkggen, families()[pkggen][0], storePkg, coretest.PackageID(storePkg),
						generated(storeID, "StoreReg"))).
						on(storeID, written(regDirective, 1, keyTag, tagFile))
				},
				want: []string{"svc/store/store_part_file.go: StoreReg"},
			},
			{
				name: "resolves a path against the unit's source directory for an origin the graph lacks",
				give: func() *fixture {
					return newFixture(stubOf(storeFile, storePkg, generated(ghostID, "GhostStub"))).
						on(ghostID, written(stubDirective, 1, keyOut, "mocks/"))
				},
				want: []string{"svc/store/mocks/store_stub.go: GhostStub"},
			},
			{
				name: "routes by the kernel out directive alone without a directive registry",
				give: func() *fixture {
					f := newFixture(storeStub()).on(storeID,
						written(stubDirective, 1, keyTag, tagTest),
						written(directive.KernelOut, 2, keyPath, "mocks/"))
					f.unregistered = true
					return f
				},
				want: []string{"svc/store/mocks/store_stub.go: StoreStub"},
			},
		}
		for _, tt := range layouts {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				files, sink := tt.give().route(t)
				assert.Equal(t, layoutOf(files), tt.want, "the routed files")
				coretest.AssertCodes(t, sink)
			})
		}

		refusals := []struct {
			name string
			give func() *fixture
			want diag.Code
		}{
			{
				name: "reports UnknownTag for a tag no family of the plugin declares",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID, written(stubDirective, 1, keyTag, "bogus"))
				},
				want: layout.UnknownTag,
			},
			{
				name: "reports EscapingPath for an absolute path",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID, written(stubDirective, 1, keyOut, "/etc/stub.go"))
				},
				want: layout.EscapingPath,
			},
			{
				name: "reports EscapingPath for a path that leaves the workspace root",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID, written(stubDirective, 1, keyOut, "../../../stub.go"))
				},
				want: layout.EscapingPath,
			},
			{
				name: "reports EscapingPath for a path with a backslash",
				give: func() *fixture {
					return newFixture(storeStub()).on(storeID, written(stubDirective, 1, keyOut, `mocks\stub.go`))
				},
				want: layout.EscapingPath,
			},
			{
				name: "reports NoDestination for a path on a declaration without a source directory",
				give: func() *fixture {
					return newFixture(stubOf(depFile, depPkg, generated(depID, "DepStub"))).
						on(depID, written(stubDirective, 1, keyOut, "mocks/"))
				},
				want: layout.NoDestination,
			},
			{
				name: "reports NoDestination for a moved declaration whose origin the graph lacks",
				give: func() *fixture {
					return newFixture(stubOf(storeFile, storePkg, generated(ghostID, "GhostStub"))).
						on(ghostID, written(stubDirective, 1, keyTag, tagPkg))
				},
				want: layout.NoDestination,
			},
			{
				name: "reports NoDestination for a tagged declaration of a dependency source",
				give: func() *fixture {
					return newFixture(stubOf(depFile, depPkg, generated(depID, "DepStub"))).
						on(depID, written(stubDirective, 1, keyTag, tagTest))
				},
				want: layout.NoDestination,
			},
			{
				name: "reports NoDestination for a path on a plan declaration whose origin the graph lacks",
				give: func() *fixture {
					f := newFixture(unitOf(stubgen, families()[stubgen][3], "", symbol.Identity{},
						generated(ghostID, "GhostIndex"))).
						on(ghostID, written(stubDirective, 1, keyOut, "mocks/"))
					f.config = layout.Config{Dir: genDir}
					return f
				},
				want: layout.NoDestination,
			},
			{
				name: "reports NoDestination for a declaration moved to a source file of a dependency",
				give: func() *fixture {
					return newFixture(unitOf(pkggen, families()[pkggen][0], depPkg, coretest.PackageID(depPkg),
						generated(depID, "DepReg"))).
						on(depID, written(regDirective, 1, keyTag, tagFile))
				},
				want: layout.NoDestination,
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				files, sink := tt.give().route(t)
				assert.Empty(t, files, "the declaration is refused")
				coretest.AssertCodes(t, sink, tt.want)
			})
		}

		t.Run("reports AmbiguousOverride for a filename on an origin of two families", func(t *testing.T) {
			t.Parallel()

			files, sink := newFixture(storeStub(), storeStubTest()).
				on(storeID, written(stubDirective, 1, keyOut, "fake.go")).route(t)
			assert.Empty(t, files, "both declarations are refused")
			coretest.AssertCodes(t, sink, layout.AmbiguousOverride, layout.AmbiguousOverride)
		})

		names := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{
				name: "names a refused declaration by its kind and name",
				give: generated(storeID, "StoreStub"), want: "Struct StoreStub is refused",
			},
			{
				name: "names a refused method by its kind and name",
				give: &emit.Method{Origin: storeID, Name: "Get"}, want: "Method Get is refused",
			},
			{
				name: "names a refused declaration without a name by its kind",
				give: &emit.Struct{Origin: storeID}, want: "Struct is refused",
			},
		}
		for _, tt := range names {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, sink := newFixture(stubOf(storeFile, storePkg, tt.give)).
					on(storeID, written(stubDirective, 1, keyOut, "/etc/stub.go")).route(t)
				coretest.AssertCodes(t, sink, layout.EscapingPath)
				for d := range sink.All() {
					assert.Contains(t, d.Msg, tt.want, "the finding names the declaration")
				}
			})
		}

		t.Run("reports an override's finding at its carrier line", func(t *testing.T) {
			t.Parallel()

			_, sink := newFixture(storeStub()).
				on(storeID, written(stubDirective, 7, keyOut, "/etc/stub.go")).route(t)
			for d := range sink.All() {
				assert.Equal(t, d.Pos.Line, 7, "the finding is at the directive")
			}
		})

		t.Run("routes the other plugin's output of an origin by its own directive", func(t *testing.T) {
			t.Parallel()

			files, sink := newFixture(storeStub(),
				unitOf(docgen, families()[docgen][0], storeFile, coretest.PackageID(storePkg),
					generated(storeID, "StoreDoc"))).
				on(storeID, written(docDirective, 1, keyOut, "docs/")).route(t)
			assert.Equal(t, layoutOf(files), []string{
				"svc/store/docs/store_doc.go: StoreDoc",
				"svc/store/store_stub.go: StoreStub",
			}, "docgen's directive routes docgen's output alone")
			coretest.AssertCodes(t, sink)
		})

		t.Run("routes every plugin's output by the kernel out directive", func(t *testing.T) {
			t.Parallel()

			files, _ := newFixture(storeStub(),
				unitOf(docgen, families()[docgen][0], storeFile, coretest.PackageID(storePkg),
					generated(storeID, "StoreDoc"))).
				on(storeID, written(directive.KernelOut, 1, keyPath, "gen/")).route(t)
			assert.Equal(t, layoutOf(files), []string{
				"svc/store/gen/store_doc.go: StoreDoc",
				"svc/store/gen/store_stub.go: StoreStub",
			}, "the kernel directive routes both plugins")
		})

		t.Run("keeps a declaration without an origin in its unit", func(t *testing.T) {
			t.Parallel()

			files, _ := newFixture(stubOf(storeFile, storePkg, generated(symbol.Identity{}, "Loose"))).route(t)
			assert.Equal(t, layoutOf(files), []string{"svc/store/store_stub.go: Loose"},
				"the unit's key decides")
		})
	})
}

// storeStub returns stubgen's primary unit of store.go with one stub
// of Store.
func storeStub() plugin.Unit {
	return stubOf(storeFile, storePkg, generated(storeID, "StoreStub"))
}

// storeStubTest returns stubgen's test unit of store.go with one test
// stub of Store.
func storeStubTest() plugin.Unit {
	return unitOf(stubgen, families()[stubgen][1], storeFile, coretest.PackageID(storePkg),
		generated(storeID, "StoreStubTest"))
}
