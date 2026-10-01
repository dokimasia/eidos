// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// A Java file declares its package in its clause, so a routed file
// keeps the package its declarations derive from wherever it is
// written.
func TestPackage(t *testing.T) {
	t.Parallel()

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		origin := symbol.Identity{Lang: java.Lang, Package: "com.acme.store", Kind: symbol.KindPackage}

		t.Run("returns the origin's package for a file in another directory", func(t *testing.T) {
			t.Parallel()

			got, err := spell.Package(plugin.Placement{Path: "gen/com/acme/store/RowStub.java", Origin: origin})
			assert.NoError(t, err, "the origin names the package")
			assert.Equal(t, got, origin, "the declarations' package")
		})

		t.Run("returns an error for a plan file", func(t *testing.T) {
			t.Parallel()

			_, err := spell.Package(plugin.Placement{Path: "gen/Registry.java"})
			assert.HasError(t, err, "a plan file derives from no package")
		})
	})
}
