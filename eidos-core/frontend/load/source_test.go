// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/load"
)

// A warm load's graph decodes a kept unit's region the first time a
// read needs it, so a run reads only the regions its reads touch.
func TestSource(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes no region of an unchanged tree", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), stdTree())
			assert.Equal(t, warm.report.Decoded(), 0, "every unit is kept, and nothing read a region")
		})

		t.Run("decodes a kept region the first time the graph reads it", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), stdTree())
			_, held := warm.g.Lookup(rowID())
			assert.True(t, held, "the kept declaration reads")
			assert.Equal(t, warm.report.Decoded(), 1, "through its unit's region alone")
		})

		t.Run("decodes a kept region once whatever reads it", func(t *testing.T) {
			t.Parallel()

			warm, _ := warmCold(t, stdTree(), stdTree())
			warm.g.Lookup(rowID())
			warm.g.PackageOf(rowID())
			assert.Equal(t, warm.report.Decoded(), 1, "the second read takes the decoded region")
		})

		t.Run("reports the failure of a kept region that does not read as damage", func(t *testing.T) {
			t.Parallel()

			prior := countedOf(committed(t, loadOf(t, stdTree()).report))
			prior.fail = storeFile
			warm := loadOf(t, stdTree(), func(cfg *load.Config) { cfg.Prior = prior })
			_, held := warm.g.Lookup(rowID())
			assert.False(t, held, "the region reads as absent")
			assert.ErrorIs(t, warm.g.Damaged(), errUnreadable, "and the graph reports why")
		})
	})
}
