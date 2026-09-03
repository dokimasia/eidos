// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest_test

import (
	"testing"

	"go.dokimi.dev/eidos/core/rules/rulestest"
)

// BenchmarkRules drives the four projections over the scripted
// tree under a ceiling pinned from measurement with headroom.
func BenchmarkRules(b *testing.B) {
	rulestest.BenchRules(b, setup, rulestest.Budget{MaxAllocs: 64})
}
