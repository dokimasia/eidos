// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
	"go.dokimi.dev/eidos/sdk/directive"
)

// The spellings of the key layout of the catalog. A key that a run stamps
// is persisted, so its spelling is contract.
const (
	namespace      = "shape"
	detectedPart   = "detected"
	shapePart      = "shape"
	mixedPart      = "mixed"
	memberPart     = "member"
	classifiedPart = "classified"
	mixinPart      = "mixin"
	rolePart       = "role"
	idPart         = "id"
)

// The key layout of the catalog spells the keys that the generator
// writes into the registry, and reserves the keys that a spec cannot
// have.
func TestCatalog(t *testing.T) {
	t.Parallel()

	t.Run("Namespace", func(t *testing.T) {
		t.Parallel()

		t.Run("is the namespace shape", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, specfront.Namespace, namespace, "every key of the catalog is in the namespace shape")
		})
	})

	t.Run("Summaries", func(t *testing.T) {
		t.Parallel()

		t.Run("lists the parts of the summary keys in order", func(t *testing.T) {
			t.Parallel()

			parts := make([]string, 0, len(specfront.Summaries))
			for _, s := range specfront.Summaries {
				parts = append(parts, s.Part)
				expect.NotEmpty(t, s.Doc, "the summary key "+s.Part+" has a meaning")
			}
			assert.Equal(t, parts, []string{detectedPart, shapePart, mixedPart, memberPart, classifiedPart},
				"the summary keys are detected, shape, mixed, member and classified")
		})
	})

	t.Run("ReservedKeys", func(t *testing.T) {
		t.Parallel()

		t.Run("lists the routing keys and the parts of the family keys", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, specfront.ReservedKeys, []string{
				string(directive.ReservedOut), string(directive.ReservedTag), rolePart, idPart, shapePart, mixinPart,
			}, "no param and no binding has a key of the directive layer or of a family key")
		})
	})

	t.Run("Part", func(t *testing.T) {
		t.Parallel()

		t.Run("spells each part of a key", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, []string{
				specfront.PartDetected, specfront.PartShape, specfront.PartMixed, specfront.PartMember,
				specfront.PartClassified, specfront.PartMixin, specfront.PartRole, specfront.PartID,
			}, []string{detectedPart, shapePart, mixedPart, memberPart, classifiedPart, mixinPart, rolePart, idPart},
				"the parts keep their spellings")
		})
	})
}
