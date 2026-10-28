// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package edit

// The bound of a near spelling: at most maxEdits edits, and at most a
// third of the length of the longer of the two spellings.
const (
	maxEdits = 2
	portion  = 3
)

// Distance returns the optimal string alignment distance between two
// spellings: the count of byte insertions, deletions, substitutions and
// swaps of two adjacent bytes that turn a into b, where no byte is edited
// twice. It allocates three rows of the length of b.
func Distance(a, b string) int {
	before, prev, cur := make([]int, len(b)+1), make([]int, len(b)+1), make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				cur[j] = min(cur[j], before[j-2]+1)
			}
		}
		before, prev, cur = prev, cur, before
	}
	return prev[len(b)]
}

// Near returns the distance between a and b, and reports whether the two
// spellings are near: at most two edits apart, and at most a third of the
// length of the longer one. It reports false without a distance for two
// lengths that differ by more than that bound, and allocates nothing for
// them. Otherwise it allocates what [Distance] allocates.
func Near(a, b string) (int, bool) {
	longer := max(len(a), len(b))
	gap := max(len(a)-len(b), len(b)-len(a))
	if gap > maxEdits || gap*portion > longer {
		return 0, false
	}
	d := Distance(a, b)
	return d, d <= maxEdits && d*portion <= longer
}
