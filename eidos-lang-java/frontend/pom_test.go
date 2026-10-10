// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// parentModule is the module of a pom.xml that states no group of its
// own and inherits its parent's.
const parentModule = "org.parent:store"

// The module a unit's packages are in comes from the pom.xml's
// coordinates, so which elements the frontend reads is pinned.
func TestPom(t *testing.T) {
	t.Parallel()

	t.Run("parsePOM", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the coordinates of a pom.xml in the Maven namespace", func(t *testing.T) {
			t.Parallel()

			src := "<project xmlns=\"http://maven.apache.org/POM/4.0.0\"><groupId>com.acme</groupId>" +
				"<artifactId>store</artifactId></project>"
			assert.Equal(t, moduleOf(t, src), []any{storeModule}, "the namespace is the POM's")
		})

		t.Run("reads the module's own coordinates over a dependency's", func(t *testing.T) {
			t.Parallel()

			src := "<project><groupId>com.acme</groupId><artifactId>store</artifactId><dependencies><dependency>" +
				"<groupId>x</groupId><artifactId>y</artifactId></dependency></dependencies></project>"
			assert.Equal(t, moduleOf(t, src), []any{storeModule}, "a dependency is no coordinate of the module")
		})
	})

	t.Run("module", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the parent's group for a module that states none", func(t *testing.T) {
			t.Parallel()

			src := "<project><parent><groupId>org.parent</groupId></parent><artifactId>store</artifactId></project>"
			assert.Equal(t, moduleOf(t, src), []any{parentModule}, "the inherited group")
		})

		t.Run("returns the module's own group over its parent's", func(t *testing.T) {
			t.Parallel()

			src := "<project><parent><groupId>org.parent</groupId></parent><groupId>com.acme</groupId>" +
				"<artifactId>store</artifactId></project>"
			assert.Equal(t, moduleOf(t, src), []any{storeModule}, "the module's group")
		})

		t.Run("returns no module for a pom.xml without an artifact", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, pomTree("<project/>"), srcFile, plugin.DepthFull)
			assert.Empty(t, found, "an empty project reports nothing")
			assert.Empty(t, stampsOf(gb, meta.ModuleKey), "and states no module")
		})
	})
}

// moduleOf parses the fixture file under a pom.xml of a source and
// returns the module stamps the parse recorded.
func moduleOf(tb testing.TB, src string) []any {
	tb.Helper()

	gb, _ := parsedTree(tb, pomTree(src), srcFile, plugin.DepthFull)
	return stampsOf(gb, meta.ModuleKey)
}
