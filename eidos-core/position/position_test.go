// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package position_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/position"
)

func TestPos(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		p := position.Pos{File: "svc/store.go", Line: 41, Col: 2}
		assert.Equal(t, p.String(), "svc/store.go:41:2",
			"String renders the position as file:line:col")
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		assert.True(t, (position.Pos{}).IsZero(),
			"the zero Pos carries no source position")
		assert.False(t, (position.Pos{File: "a.go", Line: 1, Col: 1}).IsZero(),
			"a located Pos is not the absence marker")
	})
}
