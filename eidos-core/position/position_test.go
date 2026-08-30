// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package position_test

import (
	"testing"

	"go.dokimi.dev/eidos/core/position"
)

func TestPos(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		p := position.Pos{File: "svc/store.go", Line: 41, Col: 2}
		if got, want := p.String(), "svc/store.go:41:2"; got != want {
			t.Fatalf("String() = %q, want %q", got, want)
		}
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		if !(position.Pos{}).IsZero() {
			t.Fatal("zero Pos: IsZero() = false, want true")
		}
		if (position.Pos{File: "a.go", Line: 1, Col: 1}).IsZero() {
			t.Fatal("populated Pos: IsZero() = true, want false")
		}
	})
}
