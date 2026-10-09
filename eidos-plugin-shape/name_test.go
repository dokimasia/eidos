// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shape_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/plugin/shape/catalog"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// capability is the spelling of the capability of the catalog, which a
// composition orders its annotators by.
const capability = "shape.classified"

// The capability orders the annotators that read a classification after
// the plugin that stamps it.
func TestName(t *testing.T) {
	t.Parallel()

	t.Run("Capability", func(t *testing.T) {
		t.Parallel()

		t.Run("has the spelling shape.classified", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, string(shape.Capability), capability, "the capability keeps its spelling")
		})

		t.Run("is the capability that the plugin shape provides", func(t *testing.T) {
			t.Parallel()

			provider, provides := catalog.Annotators()[0].(plugin.CapabilityProvider)
			assert.True(t, provides, "the plugin shape declares its capabilities")
			assert.Equal(t, provider.Provides(), []plugin.Capability{shape.Capability},
				"the plugin shape provides the capability")
		})

		t.Run("is the capability that the plugin shapecheck requires", func(t *testing.T) {
			t.Parallel()

			provider, provides := catalog.Annotators()[1].(plugin.CapabilityProvider)
			assert.True(t, provides, "the plugin shapecheck declares its capabilities")
			assert.Equal(t, provider.Requires(), []plugin.Capability{shape.Capability},
				"the plugin shapecheck requires the capability")
		})
	})
}
