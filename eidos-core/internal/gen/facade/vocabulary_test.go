// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade_test

import (
	"path"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/internal/gen/facade"
	"go.dokimi.dev/eidos/core/internal/genfile"
)

// The curated list is the supported surface, so its invariants are
// contract: the renderer's qualifier mapping and the mirror
// guard's stray hunt both read it.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("opens with the authoring root", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, facade.Surfaces[0], facade.Surface{Rel: "", Name: "sdk"},
			"the module root re-exports the kernel's authoring package as sdk")
	})

	t.Run("names every entry once, by its directory", func(t *testing.T) {
		t.Parallel()

		assert.NoDuplicates(t, func() ([]string, error) { return surfaceNames(), nil },
			"no two entries share a name")
		for _, s := range facade.Surfaces[1:] {
			expect.Equal(t, s.Name, path.Base(s.Rel),
				"a sibling's package name is its kernel directory's base, "+
					"which is what keeps the source qualifier the printed qualifier")
			expect.Equal(t, s.FacadeRel(), s.Name,
				"a sibling's facade directory is its name, flat under the "+
					"module root however the kernel groups its packages")
		}
	})

	t.Run("reserves the counterpart alias", func(t *testing.T) {
		t.Parallel()

		assert.NotContains(t, surfaceNames(), "core",
			"no curated package takes the alias every file binds the kernel under")
	})

	t.Run("keeps the guard able to see its output", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, facade.OwnedDirs, []string{facade.FacadeDir},
			"the guard hunts strays across the whole facade module")
		assert.HasSuffix(t, facade.FileName, genfile.GeneratedSuffix,
			"the facade file carries the suffix the stray hunt matches")
	})
}

// surfaceNames returns the package name of every curated surface, in
// curated order.
func surfaceNames() []string {
	names := make([]string, 0, len(facade.Surfaces))
	for _, s := range facade.Surfaces {
		names = append(names, s.Name)
	}
	return names
}
