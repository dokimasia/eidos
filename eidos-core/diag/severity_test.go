// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package diag_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
)

// A severity decides a run's outcome and is what the machine schema
// carries, so both the zero value and the spelling are contract.
func TestSeverity(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			severity diag.Severity
			want     string
		}{
			{name: "error", severity: diag.SeverityError, want: "error"},
			{name: "warning", severity: diag.SeverityWarning, want: "warning"},
			{name: "info", severity: diag.SeverityInfo, want: "info"},
			{
				name:     "names a severity nothing declares by its number",
				severity: diag.SeverityInfo + 1,
				want:     "Severity(3)",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, tt.severity.String(), tt.want,
					"the severity spells what machine output carries")
			})
		}
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		var got diag.Severity
		assert.Equal(t, got, diag.SeverityError,
			"a finding that returned no severity must not downgrade itself")
	})
}
