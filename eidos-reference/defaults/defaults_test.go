// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package defaults_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/reference/defaults"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages of the tree, the struct Address and its fields, and the
// TypeScript interface Product and its field.
const (
	catalogPackage = "example.com/shop/catalog"
	webPackage     = "web/store"
	addressName    = "Address"
	streetName     = "Street"
	cityName       = "City"
	countryName    = "Country"
	floorName      = "Floor"
	linesName      = "Lines"
	productName    = "Product"
	idName         = "id"
)

// The defaults that the tree declares on the fields of Address.
const (
	streetDefault  = "Dam 1"
	cityDefault    = "Amsterdam"
	countryDefault = "NL"
	floorDefault   = "0"
)

// The tree of a field with a default under two tag keys, its package, its
// struct and its field, the second key, and the default under each key.
const (
	taggedTree      = "testdata/tagged"
	taggedPackage   = "example.com/tagged"
	configName      = "Config"
	regionName      = "Region"
	fallbackTag     = "fallback"
	fallbackDefault = "west"
	regionDefault   = "east"
)

// otherRegistrant is the registrant that claims the namespace of defaults
// before defaults registers its keys.
const otherRegistrant = "other"

// stamp is what defaults stamps on one field: the text of the value, the
// source, and whether it stamps the value at all.
type stamp struct {
	value   string
	source  string
	stamped bool
}

// defaults stamps the declared defaults of the fields, the directive before
// the tag, and passes the plugin suite.
func TestDefaults(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the package catalog of the tree", func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := defaults.New()
				return p, fixture(ctx, tb, referencetest.Tree, catalogPackage, p).Fixture
			})
		})

		tests := []struct {
			name string
			give string
			want stamp
		}{
			{
				name: "stamps the value of the directive on a field under the directive", give: cityName,
				want: stamp{value: cityDefault, source: defaults.SourceDirective, stamped: true},
			},
			{
				name: "stamps the value of the directive on a field with the tag too", give: streetName,
				want: stamp{value: streetDefault, source: defaults.SourceDirective, stamped: true},
			},
			{
				name: "stamps the value of the tag on a field with the tag", give: countryName,
				want: stamp{value: countryDefault, source: defaults.SourceTag, stamped: true},
			},
			{
				name: "stamps the text of a number in the tag", give: floorName,
				want: stamp{value: floorDefault, source: defaults.SourceTag, stamped: true},
			},
			{name: "stamps nothing on a field without a default", give: linesName},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				p := defaults.New()
				f := fixture(t.Context(), t, referencetest.Tree, catalogPackage, p)
				assert.Equal(t, stamped(t, f, p, addressName, tt.give), tt.want,
					"defaults stamps the field "+tt.give+" as the case states")
			})
		}

		t.Run("reads the tag default without a config", func(t *testing.T) {
			t.Parallel()

			p := defaults.New()
			f := fixture(t.Context(), t, taggedTree, taggedPackage, p)
			assert.Equal(t, stamped(t, f, p, configName, regionName),
				stamp{value: regionDefault, source: defaults.SourceTag, stamped: true},
				"defaults reads the tag of the key default")
		})

		t.Run("reads the tag of the key that the option tag sets", func(t *testing.T) {
			t.Parallel()

			p := defaults.New()
			p.(plugin.OptionsProvider).Options().(*defaults.Options).Tag = fallbackTag
			f := fixture(t.Context(), t, taggedTree, taggedPackage, p)
			assert.Equal(t, stamped(t, f, p, configName, regionName),
				stamp{value: fallbackDefault, source: defaults.SourceTag, stamped: true},
				"defaults reads the tag of the key that the option sets")
		})

		t.Run("stamps nothing on a field of a language whose rules read no tags", func(t *testing.T) {
			t.Parallel()

			p := defaults.New()
			f := referencetest.New(t.Context(), t, referencetest.Spec{
				Dir:      referencetest.Tree,
				Packages: []string{webPackage},
				Target:   typescript.Target,
				Plugins:  []plugin.Plugin{p},
			})
			assert.Equal(t, stamped(t, f, p, productName, idName), stamp{},
				"defaults stamps nothing on a field of TypeScript")
		})

		t.Run("returns from Keys the error of a namespace that another registrant claimed", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.For(otherRegistrant).ClaimNamespace(defaults.KeyValue.Namespace()),
				"another registrant claims the namespace of defaults")
			keys, provides := defaults.New().(plugin.KeyProvider)
			assert.True(t, provides, "defaults registers keys")
			assert.HasError(t, keys.Keys(r.For(string(defaults.ID))), "defaults cannot claim its namespace")
		})
	})
}

// fixture returns a fixture over the Go package pkg of the tree dir, loaded
// under ctx, with the target golang and the plugin defaults p.
func fixture(ctx context.Context, tb assert.TB, dir, pkg string, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(ctx, tb, referencetest.Spec{
		Dir:      dir,
		Packages: []string{pkg},
		Target:   golang.Target,
		Plugins:  []plugin.Plugin{p},
	})
}

// stamped runs defaults p over the fixture f, and returns what p stamps on
// the field named name of the declaration named owner.
func stamped(t *testing.T, f *referencetest.Fixture, p plugin.Plugin, owner, name string) stamp {
	t.Helper()

	assert.NoError(t, f.Annotate(t, p).Err, "the annotator runs whole")
	valueKey, registered := meta.Lookup[string](f.Keys, defaults.KeyValue)
	assert.True(t, registered, "defaults registers the key of the value")
	sourceKey, registered := meta.Lookup[string](f.Keys, defaults.KeySource)
	assert.True(t, registered, "defaults registers the key of the source")
	id := f.Find(t, symbol.KindField, owner, name).Identity()
	value, held := meta.Get(f.Facts, id, valueKey)
	source, _ := meta.Get(f.Facts, id, sourceKey)
	return stamp{value: value, source: source, stamped: held}
}
