// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/rules"
)

// The package's one code is registered under the kernel's prefix.
func TestCodes(t *testing.T) {
	t.Parallel()

	t.Run("AbsentRules", func(t *testing.T) {
		t.Parallel()

		t.Run("registers under the kernel prefix", func(t *testing.T) {
			t.Parallel()

			assert.True(t, strings.HasPrefix(rules.AbsentRules.String(), string(diag.KernelPrefix)),
				"the code is the kernel's")
			assert.Contains(t, rules.AbsentRules.String(), "39", "at its number")
		})
	})
}
