// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import "slices"

// The closed vocabulary's runtime behaviour lives here: four terms
// are plain comparable values, and []string is the one that has to
// be copied at the boundary and compared element-wise.

// cloneValue returns a value safe to hold or hand out: a slice is
// copied, everything else is a value already.
func cloneValue(v any) any {
	if list, isList := v.([]string); isList {
		return slices.Clone(list)
	}
	return v
}

// equalValue compares two held values per vocabulary term; slices
// compare element-wise.
func equalValue(a, b any) bool {
	if left, isList := a.([]string); isList {
		right, isRight := b.([]string)
		return isRight && slices.Equal(left, right)
	}
	return a == b
}
