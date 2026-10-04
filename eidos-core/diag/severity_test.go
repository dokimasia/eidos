// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package diag_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

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
			{name: "returns error for SeverityError", severity: diag.SeverityError, want: "error"},
			{name: "returns warning for SeverityWarning", severity: diag.SeverityWarning, want: "warning"},
			{name: "returns info for SeverityInfo", severity: diag.SeverityInfo, want: "info"},
			{
				name:     "returns the number of a severity nothing declares",
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

// A declared severity spells without allocating in the ordinary run,
// which runs no benchmark.
func TestSeverityZeroAlloc(t *testing.T) {
	severity := diag.SeverityWarning
	var got string
	assert.MaxAllocs(t, func() { got = severity.String() }, 0, "String allocates nothing for a declared severity")
	assert.Equal(t, got, "warning", "String spells SeverityWarning")
}

// BenchmarkSeverity measures the spelling of a declared severity.
func BenchmarkSeverity(b *testing.B) {
	b.Run("String", func(b *testing.B) {
		severity := diag.SeverityWarning
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = severity.String()
		}
		assert.Equal(b, got, "warning", "String spells SeverityWarning")
	})
}
