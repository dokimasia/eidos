// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest_test

import (
	"testing"

	"go.dokimi.dev/eidos/core/rules/rulestest"
)

// scriptedAllocs is one projection pass of the scripted rules over the
// fixture tree: 27 in each of 12 runs. A memory profile attributes 10
// to TypeOf, 7 to SamplesOf, 3 to MembersOf, 2 to CallableOf and 3 to
// the fresh view and its binding, and it samples the tiny allocations
// of the rest only in part. The 2 more are for the runtime's own
// allocations in a run of one iteration: 300 fresh processes counted 0
// or 1.
const scriptedAllocs = 27 + 2

// BenchmarkRules drives the four projections over the scripted tree
// under their ceiling.
func BenchmarkRules(b *testing.B) {
	rulestest.BenchRules(b, setup, rulestest.Budget{MaxAllocs: scriptedAllocs})
}
