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

	t.Run("Compare", func(t *testing.T) {
		t.Parallel()

		at := position.Pos{File: "b.go", Line: 5, Col: 3}
		cases := []struct {
			name  string
			other position.Pos
			want  int
		}{
			{name: "an equal position compares equal", other: at, want: 0},
			{name: "the file decides first", other: position.Pos{File: "a.go", Line: 9, Col: 9}, want: 1},
			{name: "then the line", other: position.Pos{File: "b.go", Line: 6, Col: 1}, want: -1},
			{name: "then the column", other: position.Pos{File: "b.go", Line: 5, Col: 2}, want: 1},
		}
		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, at.Compare(tt.other), tt.want, tt.name)
				assert.Equal(t, tt.other.Compare(at), -tt.want, "and the order is antisymmetric")
			})
		}
	})
}
