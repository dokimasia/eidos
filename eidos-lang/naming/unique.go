// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming

import "strconv"

// scratchSize is the size of the stack buffer in which [Unique] spells a
// numbered name. A longer name spells in a buffer on the heap.
const scratchSize = 64

// Unique returns the first of candidates that taken does not contain.
// Where taken contains every candidate, it returns the last candidate
// followed by the smallest number from 0 that gives a name that taken
// does not contain. A nil taken contains no name.
//
// Unique panics without a candidate, which is a defect of the caller.
//
// # Allocation contract
//
// Unique allocates nothing for a free candidate. A numbered name
// allocates its text once, because the probes spell each number into a
// stack buffer and look the bytes up without a conversion.
func Unique(taken map[string]bool, candidates ...string) string {
	if len(candidates) == 0 {
		panic("naming: Unique needs a candidate")
	}
	for _, c := range candidates {
		if !taken[c] {
			return c
		}
	}
	last := candidates[len(candidates)-1]
	var scratch [scratchSize]byte
	name := append(scratch[:0], last...)
	for n := 0; ; n++ {
		name = strconv.AppendInt(name[:len(last)], int64(n), 10)
		if !taken[string(name)] {
			return string(name)
		}
	}
}
