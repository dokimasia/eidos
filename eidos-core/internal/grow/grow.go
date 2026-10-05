// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package grow

import "slices"

// Room returns s with room for n more elements: s itself where its
// capacity has the room, and otherwise a copy whose capacity is at least
// the larger of twice the length of s, len(s)+n and least. The copy keeps
// the elements and the length of s. A caller appends after Room, as in
// append(grow.Room(s, 1, 64), v).
//
// A negative n needs no room and returns s. Room cannot fail.
//
// # Allocation contract
//
// Room allocates nothing where s has the room, and one copy otherwise.
func Room[T any](s []T, n, least int) []T {
	if cap(s)-len(s) >= n {
		return s
	}
	return slices.Grow(s, max(n, len(s), least-len(s)))
}
