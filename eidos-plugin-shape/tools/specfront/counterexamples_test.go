// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
)

// The texts of the counterexamples of the fixture.
const (
	invalidText = "an invalid input"
	unsafeText  = "an unsafe input"
	refusedText = "a refused callable"
)

// The counterexamples of a spec name the inputs that a check covers.
func TestCounterexamples(t *testing.T) {
	t.Parallel()

	t.Run("Counterexamples", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes every kind of counterexample", func(t *testing.T) {
			t.Parallel()

			got := decodeAs[specfront.Counterexamples](t, "invalid: an invalid input\nunsafe: an unsafe input\n"+
				"edge: an edge\nrefused: a refused callable\n")
			assert.Equal(t, got, specfront.Counterexamples{
				Invalid: invalidText, Unsafe: unsafeText, Edge: edgeText, Refused: refusedText,
			}, "the counterexamples have every kind of the text")
		})
	})
}
