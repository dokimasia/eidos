// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
)

// The precedence of a detected shape lists the shapes that rank before it.
func TestPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("Precedence", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes the shapes that rank before a shape in order", func(t *testing.T) {
			t.Parallel()

			got := decodeAs[specfront.Precedence](t, "{yields_to: [writer, reader]}\n")
			assert.Equal(t, got, specfront.Precedence{YieldsTo: []specfront.Name{writerName, readerName}},
				"the precedence lists the shapes of the text in order")
		})
	})
}
