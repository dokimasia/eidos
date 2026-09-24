// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/eidos/sdk/rulestest"
)

// protobuf's rules meet the projection contract over the loaded
// schema tree, which is the same contract every language's rules
// meet.
func TestSuite(t *testing.T) {
	t.Parallel()

	rulestest.RunRulesSuite(t, setup)
}

func BenchmarkRules(b *testing.B) {
	rulestest.BenchRules(b, setup, rulestest.Budget{MaxAllocs: 192})
}
