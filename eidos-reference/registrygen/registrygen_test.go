// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package registrygen_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/internal/callee"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/reference/registrygen"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package of the tree with the struct Item under the directive, the
// tree of a generic struct, its package and the struct, and the template
// of the family.
const (
	catalogPackage = "example.com/shop/catalog"
	itemName       = "Item"
	genericTree    = "testdata/generic"
	genericPackage = "example.com/generic"
	boxName        = "Box"
	fileTemplate   = "registry.tmpl"
)

// The name by which a file refers to the package of the default function,
// and the package outside the module of the tree, its name and the
// function that a config sets.
const (
	gobName      = "gob"
	codecPackage = "example.org/codec"
	codecName    = "codec"
	codecFunc    = "Add"
)

// A package of a config whose path ends in a keyword, the escaped name by
// which a file refers to it, and a package of the module of the tree that
// the tree does not contain.
const (
	keywordPackage = "example.org/shop/go"
	keywordName    = "go_"
	missingPackage = "example.com/shop/codec"
)

// registrygen records each struct under its directive into the registry of
// its package, and passes the plugin suite.
func TestRegistrygen(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the package catalog of the tree", func(t *testing.T) {
			t.Parallel()

			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := registrygen.New()
				return p, catalog(tb, p).Fixture
			})
		})

		t.Run("records each struct under the directive into the registry of its package", func(t *testing.T) {
			t.Parallel()

			p := registrygen.New()
			f := catalog(t, p)
			units, _ := registries(t, f, p)
			assert.Length(t, units, 1, "the package catalog has one registry")
			assert.Length(t, units[0].Decls, 1, "the registry has one record")
			record, isStruct := units[0].Decls[0].(*emit.Struct)
			assert.True(t, isStruct, "the record is a struct")
			item := f.Find(t, symbol.KindStruct, "", itemName).Identity()
			expect.That(t, record.Name).Equal(itemName, "the record has the name of Item")
			expect.That(t, record.Origin).Equal(item, "the record derives from Item")
			expect.Equal(
				t,
				units[0].Data[item],
				any(
					registrygen.Callee{
						Package: registrygen.DefaultPackage,
						Name:    gobName,
						Func:    registrygen.DefaultFunc,
					},
				),
				"the record of Item calls the default function",
			)
		})

		t.Run("records the function of the options", func(t *testing.T) {
			t.Parallel()

			p := registrygen.New()
			o := p.(plugin.OptionsProvider).Options().(*callee.Options)
			o.Package, o.Func = codecPackage, codecFunc
			f := catalog(t, p)
			units, _ := registries(t, f, p)
			assert.Length(t, units, 1, "the package catalog has one registry")
			item := f.Find(t, symbol.KindStruct, "", itemName).Identity()
			assert.Equal(t, units[0].Data[item],
				any(registrygen.Callee{Package: codecPackage, Name: codecName, Func: codecFunc}),
				"the record of Item calls the function of the options")
		})

		t.Run("records an escaped name for a package of the options whose path ends in a keyword", func(t *testing.T) {
			t.Parallel()

			p := registrygen.New()
			o := p.(plugin.OptionsProvider).Options().(*callee.Options)
			o.Package, o.Func = keywordPackage, codecFunc
			f := catalog(t, p)
			units, _ := registries(t, f, p)
			assert.Length(t, units, 1, "the package catalog has one registry")
			item := f.Find(t, symbol.KindStruct, "", itemName).Identity()
			assert.Equal(t, units[0].Data[item],
				any(registrygen.Callee{Package: keywordPackage, Name: keywordName, Func: codecFunc}),
				"go is a keyword, so the record refers to the package as go_")
		})

		t.Run("reports Missing at a struct for an empty option", func(t *testing.T) {
			t.Parallel()

			p := registrygen.New()
			p.(plugin.OptionsProvider).Options().(*callee.Options).Func = ""
			f := catalog(t, p)
			units, sink := registries(t, f, p)
			found := slices.Collect(sink.All())
			assert.Length(t, found, 1, "registrygen reports the one struct")
			expect.That(t, found[0].Code).Equal(callee.Missing, "the finding is Missing")
			expect.That(t, found[0].Pos.Line).Equal(f.Find(t, symbol.KindStruct, "", itemName).Position().Line,
				"the finding is at Item")
			expect.Empty(t, units, "registrygen records nothing")
		})

		t.Run("reports UnknownPackage at a struct for a package of its module that the load does not contain",
			func(t *testing.T) {
				t.Parallel()

				p := registrygen.New()
				o := p.(plugin.OptionsProvider).Options().(*callee.Options)
				o.Package, o.Func = missingPackage, codecFunc
				f := catalog(t, p)
				units, sink := registries(t, f, p)
				found := slices.Collect(sink.All())
				assert.Length(t, found, 1, "registrygen reports the one struct")
				expect.That(t, found[0].Code).Equal(callee.UnknownPackage, "the finding is UnknownPackage")
				expect.That(t, found[0].Pos.Line).Equal(f.Find(t, symbol.KindStruct, "", itemName).Position().Line,
					"the finding is at Item")
				expect.Empty(t, units, "registrygen records nothing")
			})

		t.Run("reports Generic at a struct with type parameters", func(t *testing.T) {
			t.Parallel()

			p := registrygen.New()
			f := referencetest.New(t, referencetest.Spec{
				Dir:      genericTree,
				Packages: []string{genericPackage},
				Target:   golang.Target,
				Plugins:  []plugin.Plugin{p},
			})
			units, sink := registries(t, f, p)
			found := slices.Collect(sink.All())
			assert.Length(t, found, 1, "registrygen reports the one struct")
			expect.That(t, found[0].Code).Equal(registrygen.Generic, "the finding is Generic")
			expect.That(t, found[0].Pos.Line).Equal(f.Find(t, symbol.KindStruct, "", boxName).Position().Line,
				"the finding is at Box")
			expect.Empty(t, units, "registrygen records nothing")
		})
	})
}

// catalog returns a fixture over the package catalog of the tree, loaded
// under the context of tb, with the target golang and the plugin
// registrygen p.
func catalog(tb assert.TB, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(tb, referencetest.Spec{
		Dir:      referencetest.Tree,
		Packages: []string{catalogPackage},
		Target:   golang.Target,
		Plugins:  []plugin.Plugin{p},
	})
}

// registries runs registrygen p over the fixture f. It returns the units
// that p flushes and the findings of the run. It checks that each unit is a
// registry of a package that claims its files through the template.
func registries(t *testing.T, f *referencetest.Fixture, p plugin.Plugin) ([]plugin.Unit, *diag.Sink) {
	t.Helper()

	res := f.Generate(t, p)
	assert.NoError(t, res.Err, "the generator runs whole")
	var units []plugin.Unit
	for u := range res.Emit.Units() {
		expect.That(t, u.Per).Equal(plugin.PerPackage, "the registry belongs to a package")
		expect.That(t, u.Template).Equal(fileTemplate, "the family claims its files through the template")
		units = append(units, u)
	}
	return units, res.Sink
}
