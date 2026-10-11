// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package double_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/reference/internal/double"
)

// The targets of a double plugin are the targets of the Go and the
// TypeScript satellites.
func TestTargets(t *testing.T) {
	t.Parallel()

	t.Run("GoTarget", func(t *testing.T) {
		t.Parallel()

		t.Run("is the target of the Go satellite", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, double.GoTarget, golang.Target, "the Go target has the spelling of the satellite")
		})
	})

	t.Run("TypeScriptTarget", func(t *testing.T) {
		t.Parallel()

		t.Run("is the target of the TypeScript satellite", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, double.TypeScriptTarget, typescript.Target,
				"the TypeScript target has the spelling of the satellite")
		})
	})
}
