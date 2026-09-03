// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/eidos/sdk/rulestest"
)

// The Go rules meet the projection contract over the fixture tree.
func TestSuite(t *testing.T) {
	t.Parallel()

	rulestest.RunRulesSuite(t, setup)
}

func BenchmarkRules(b *testing.B) {
	rulestest.BenchRules(b, setup, rulestest.Budget{MaxAllocs: 400})
}
