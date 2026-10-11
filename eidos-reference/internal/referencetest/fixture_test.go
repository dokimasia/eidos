// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package referencetest_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/builder"
	"go.dokimi.dev/eidos/reference/debugweaver"
	"go.dokimi.dev/eidos/reference/defaults"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/reference/mockgen"
	"go.dokimi.dev/eidos/reference/repogen"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The workspace tree of the module, relative to the directory of this
// package, its package that the cases load, and a package that the tree
// does not declare.
const (
	workspaceTree  = "../../testdata/tree"
	catalogPackage = "example.com/shop/catalog"
	missingPackage = "example.com/shop/missing"
)

// A tree that does not exist, and a tree whose carrier of builder names a
// function of defaults that its package does not declare, with that
// package.
const (
	missingTree       = "testdata/missing"
	unresolvedTree    = "testdata/unresolved"
	unresolvedPackage = "example.com/unresolved"
)

// The declarations that the cases find: the struct under builder, the
// function of its defaults, its field with a declared default and that
// default, and a name that the tree does not declare.
const (
	addressName  = "Address"
	defaultsName = "defaultAddress"
	streetName   = "Street"
	streetValue  = "Dam 1"
	missingName  = "Missing"
)

// bareID is the name of the plugin that provides no key and declares no
// directive.
const bareID plugin.ID = "bare"

// unresolvedContract opens the contract of the record that the fixture
// writes for an Error of the validation of the directives.
const unresolvedContract = "the validation of the directives reports no Error, and reports "

// named is a plugin that is a name and nothing else: it provides no key and
// declares no directive.
type named plugin.ID

// Name returns the name.
func (n named) Name() plugin.ID { return plugin.ID(n) }

// A fixture runs a plugin of the ensemble over the tree after the plugins
// that it builds on, so it loads, validates, stamps and seeds the tree as a
// run of the composition does.
func TestFixture(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("registers the keys of the plugins of the spec", func(t *testing.T) {
			t.Parallel()

			_, registered := meta.Lookup[string](address(t).Keys, defaults.KeyValue)
			assert.True(t, registered, "the fixture registers the key of defaults")
		})

		t.Run("binds the reference param of a carrier through the rules of its language", func(t *testing.T) {
			t.Parallel()

			f := address(t)
			ds := f.Directives[f.Find(t, symbol.KindStruct, "", addressName).Identity()]
			assert.Length(t, ds, 1, "the fixture validates the carrier of builder on Address")
			assert.Equal(t, ds[0].Params[builder.DefaultsParam].Target,
				f.Find(t, symbol.KindFunction, "", defaultsName).Identity(),
				"the param defaults refers to the function of the defaults of Address")
		})

		t.Run("stamps the fixture through the annotators of the spec", func(t *testing.T) {
			t.Parallel()

			f := address(t)
			key, registered := meta.Lookup[string](f.Keys, defaults.KeyValue)
			assert.True(t, registered, "the fixture registers the key of defaults")
			got, _ := meta.Get(f.Facts, f.Find(t, symbol.KindField, addressName, streetName).Identity(), key)
			assert.Equal(t, got, streetValue, "defaults stamps the declared default of Street")
		})

		t.Run("seeds the units of the hosts of the spec", func(t *testing.T) {
			t.Parallel()

			f := referencetest.New(t.Context(), t, referencetest.Spec{
				Dir:        workspaceTree,
				Packages:   []string{catalogPackage},
				Target:     golang.Target,
				Plugins:    []plugin.Plugin{debugweaver.New(), mockgen.New()},
				Annotators: []plugin.Annotator{repogen.New().(plugin.Annotator)},
				Hosts:      []plugin.Generator{repogen.New(), mockgen.New()},
			})
			res := f.Generate(t, debugweaver.New())
			assert.NoError(t, res.Err, "debugweaver generates over the fixture")
			var seeded []plugin.ID
			for u := range res.Emit.Units() {
				seeded = append(seeded, u.Plugin)
			}
			expect.That(t, seeded).
				Contains(repogen.ID, "the fixture has the units of repogen").
				Contains(mockgen.ID, "the fixture has the units of mockgen")
		})

		t.Run("stops at a tree that does not load", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "a tree that does not exist", func(tb assert.TB) {
				referencetest.New(
					t.Context(),
					tb,
					referencetest.Spec{Dir: missingTree, Packages: []string{catalogPackage}},
				)
			})
			assert.Length(t, records, 1, "the fixture stops at the load")
			assert.Equal(t, records[0].Contract, "the tree "+missingTree+" loads", "the record names the tree")
		})

		t.Run("stops at a package that the tree does not declare", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "a spec of a package that the tree lacks", func(tb assert.TB) {
				referencetest.New(t.Context(), tb, referencetest.Spec{
					Dir: workspaceTree, Packages: []string{missingPackage},
				})
			})
			assert.Length(t, records, 1, "the fixture stops at the packages")
			assert.Equal(t, records[0].Contract, "the tree declares each package of the spec",
				"the record names the packages of the spec")
		})

		t.Run("fails at a reference param that the rules do not resolve", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(
				t,
				"a carrier of builder with a function that does not exist",
				func(tb assert.TB) {
					referencetest.New(t.Context(), tb, referencetest.Spec{
						Dir:      unresolvedTree,
						Packages: []string{unresolvedPackage},
						Target:   golang.Target,
						Plugins:  []plugin.Plugin{builder.New()},
					})
				},
			)
			assert.Length(t, records, 1, "the fixture fails once, at the Error of the validation")
			assert.HasPrefix(t, records[0].Contract, unresolvedContract, "the record names the validation")
		})
	})

	t.Run("Find", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declaration of the kind with the owner and the name", func(t *testing.T) {
			t.Parallel()

			got := address(t).Find(t, symbol.KindField, addressName, streetName).Identity()
			expect.Equal(t, got.Kind, symbol.KindField, "the declaration is a field")
			expect.Equal(t, got.Owner, addressName, "the declaration is a member of Address")
			expect.Equal(t, got.Name, streetName, "the declaration is Street")
		})

		t.Run("stops at a name that the fixture does not declare", func(t *testing.T) {
			t.Parallel()

			f := address(t)
			records := assert.Rejects(t, "a name without a declaration", func(tb assert.TB) {
				f.Find(tb, symbol.KindStruct, "", missingName)
			})
			assert.Length(t, records, 1, "Find stops at the count of the declarations")
			assert.Equal(t, records[0].Contract,
				"the fixture declares one "+symbol.KindStruct.String()+" "+strconv.Quote(missingName)+` in ""`,
				"the record names the kind, the name and the owner")
		})
	})
}

// address returns the fixture of the package catalog for builder and a
// plugin that is only a name, stamped by defaults.
func address(t *testing.T) *referencetest.Fixture {
	t.Helper()

	return referencetest.New(t.Context(), t, referencetest.Spec{
		Dir:        workspaceTree,
		Packages:   []string{catalogPackage},
		Target:     golang.Target,
		Plugins:    []plugin.Plugin{builder.New(), named(bareID)},
		Annotators: []plugin.Annotator{defaults.New()},
	})
}
