// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shapewriter_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/internal/referencetest"
	"go.dokimi.dev/eidos/reference/shapewriter"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package of the tree with the writers, its struct with the method
// Write, its struct under the directive, and its struct under the negated
// directive.
const (
	catalogPackage = "example.com/shop/catalog"
	sinkName       = "Sink"
	journalName    = "Journal"
	mirrorName     = "Mirror"
)

// The tree of the structs with a method Write of each shape, its package,
// and its structs.
const (
	shapesTree    = "testdata/shapes"
	shapesPackage = "example.com/shapes"
	plainName     = "Plain"
	teeName       = "Tee"
	textName      = "Text"
	spreadName    = "Spread"
	multiName     = "Multi"
	singleName    = "Single"
	wideName      = "Wide"
	unsignedName  = "Unsigned"
	flaggedName   = "Flagged"
	closerName    = "Closer"
)

// otherRegistrant is the registrant that claims the namespace of
// shapewriter before shapewriter registers its keys.
const otherRegistrant = "other"

// shapewriter stamps the writers that it detects and the writers under its
// directive, and passes the plugin suite.
func TestShapewriter(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the plugin suite over the package catalog of the tree", func(t *testing.T) {
			t.Parallel()

			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := shapewriter.New()
				return p, fixture(tb, referencetest.Tree, catalogPackage, p).Fixture
			})
		})

		catalog := []struct {
			name string
			give string
			want bool
		}{
			{name: "stamps a struct that declares the method Write of a writer", give: sinkName, want: true},
			{name: "stamps a struct under the directive", give: journalName, want: true},
			{name: "stamps no struct under the negated directive", give: mirrorName},
		}
		for _, tt := range catalog {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, stamped(t, referencetest.Tree, catalogPackage, tt.give), tt.want,
					"shapewriter stamps "+tt.give+" as the case states")
			})
		}

		shapes := []struct {
			name string
			give string
			want bool
		}{
			{name: "stamps a struct whose method Write has the shape of io.Writer", give: plainName, want: true},
			{name: "stamps no struct whose method Write an embedded field promotes", give: teeName},
			{name: "stamps no struct whose method Write takes a string", give: textName},
			{name: "stamps no struct whose method Write takes the bytes one by one", give: spreadName},
			{name: "stamps no struct whose method Write takes a variadic list of byte slices", give: multiName},
			{name: "stamps no struct whose method Write returns the count alone", give: singleName},
			{name: "stamps no struct whose method Write returns a count of 64 bits", give: wideName},
			{name: "stamps no struct whose method Write returns an unsigned count", give: unsignedName},
			{name: "stamps no struct whose method Write returns a flag in place of the error", give: flaggedName},
			{name: "stamps no struct without a method Write", give: closerName},
		}
		for _, tt := range shapes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, stamped(t, shapesTree, shapesPackage, tt.give), tt.want,
					"shapewriter stamps "+tt.give+" as the case states")
			})
		}

		t.Run("returns from Keys the error of a namespace that another registrant claimed", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.For(otherRegistrant).ClaimNamespace(shapewriter.KeyWriter.Namespace()),
				"another registrant claims the namespace of shapewriter")
			keys, provides := shapewriter.New().(plugin.KeyProvider)
			assert.True(t, provides, "shapewriter registers keys")
			assert.HasError(t, keys.Keys(r.For(string(shapewriter.ID))), "shapewriter cannot claim its namespace")
		})
	})
}

// fixture returns a fixture over the package pkg of the tree dir, loaded
// under the context of tb, with the target golang and the plugin
// shapewriter p.
func fixture(tb assert.TB, dir, pkg string, p plugin.Plugin) *referencetest.Fixture {
	tb.Helper()

	return referencetest.New(tb, referencetest.Spec{
		Dir:      dir,
		Packages: []string{pkg},
		Target:   golang.Target,
		Plugins:  []plugin.Plugin{p},
	})
}

// stamped runs shapewriter over the package pkg of the tree dir, and
// reports whether it stamps the struct named name as a writer.
func stamped(t *testing.T, dir, pkg, name string) bool {
	t.Helper()

	p := shapewriter.New()
	f := fixture(t, dir, pkg, p)
	assert.NoError(t, f.Annotate(t, p).Err, "the annotator runs whole")
	key, registered := meta.Lookup[bool](f.Keys, shapewriter.KeyWriter)
	assert.True(t, registered, "shapewriter registers its key")
	writer, _ := meta.Get(f.Facts, f.Find(t, symbol.KindStruct, "", name).Identity(), key)
	return writer
}
