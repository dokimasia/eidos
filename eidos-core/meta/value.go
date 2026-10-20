// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"slices"

	"go.dokimi.dev/eidos/core/symbol"
)

// This file implements the run-time behaviour of the closed vocabulary.
// Four terms are plain comparable values. The fact store copies the two
// lists at its boundary and compares them element by element.

// cloneValue returns a value that the store may keep or hand out. It
// copies a list and returns every other term as it is.
func cloneValue(v any) any {
	switch list := v.(type) {
	case []string:
		return slices.Clone(list)
	case []symbol.Identity:
		return slices.Clone(list)
	default:
		return v
	}
}

// equalValue reports whether two held values are equal. It compares two
// lists element by element and every other term with ==.
func equalValue(a, b any) bool {
	switch left := a.(type) {
	case []string:
		right, isRight := b.([]string)
		return isRight && slices.Equal(left, right)
	case []symbol.Identity:
		right, isRight := b.([]symbol.Identity)
		return isRight && slices.Equal(left, right)
	default:
		return a == b
	}
}
