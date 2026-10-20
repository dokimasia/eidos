// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// policiesAllocs is the list of specs and the list of choices of each of
// the two specs.
const policiesAllocs = 1 + 2

// TestPolicies pins the keys, the choices and the defaults of the policies,
// because a config file and a directive name them.
func TestPolicies(t *testing.T) {
	t.Parallel()

	t.Run("Policies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the version and the dynamic type", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, python.Policies(), []plugin.PolicySpec{
				{
					Key: python.VersionPolicy, Default: python.Version311,
					Choices: []plugin.Choice{
						python.Version311, python.Version312, python.Version313, python.Version314, python.Version315,
					},
					Doc:  "the version of Python that the written files run on",
					Plan: true,
				},
				{
					Key: python.DynamicPolicy, Default: python.Object,
					Choices: []plugin.Choice{python.Object, python.Any},
					Doc:     "the type of a dynamic value",
				},
			}, "the oldest supported version and the top type are the defaults")
		})

		t.Run("returns specs that a policy resolves", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.NewPolicy(python.Target, python.Policies(), nil)
			assert.NoError(t, err, "the specs are well formed")
		})
	})

	t.Run("Minor", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			give      plugin.Choice
			wantMinor int
			wantValid bool
		}{
			{name: "returns 11 for the version 3.11", give: python.Version311, wantMinor: 11, wantValid: true},
			{name: "returns 15 for the version 3.15", give: python.Version315, wantMinor: 15, wantValid: true},
			{name: "reports false for a version of Python 2", give: "2.7"},
			{name: "reports false for a version without a minor version", give: "3."},
			{name: "reports false for a minor version that is not a number", give: "3.x"},
			{name: "reports false for a negative minor version", give: "3.-1"},
			{name: "reports false for a minor version with a plus sign", give: "3.+1"},
			{name: "reports false for a choice of the dynamic type", give: python.Any},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				minor, valid := python.Minor(string(tt.give))
				assert.Equal(t, valid, tt.wantValid, "Minor reports whether the text is a version of Python 3")
				assert.Equal(t, minor, tt.wantMinor, "Minor returns the minor version")
			})
		}
	})

	t.Run("PolicyKey", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.PolicyKey
			want string
		}{
			{name: "spells the policy of the version", give: python.VersionPolicy, want: "python.version"},
			{name: "spells the policy of a dynamic value", give: python.DynamicPolicy, want: "python.dynamic"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, string(tt.give), tt.want, "the key that a config file names")
			})
		}
	})

	t.Run("Choice", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Choice
			want string
		}{
			{name: "spells 3.11", give: python.Version311, want: "3.11"},
			{name: "spells 3.12", give: python.Version312, want: "3.12"},
			{name: "spells 3.13", give: python.Version313, want: "3.13"},
			{name: "spells 3.14", give: python.Version314, want: "3.14"},
			{name: "spells 3.15", give: python.Version315, want: "3.15"},
			{name: "spells object", give: python.Object, want: "object"},
			{name: "spells Any", give: python.Any, want: "Any"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, string(tt.give), tt.want, "the choice that a config file names")
			})
		}
	})
}

// TestPoliciesAllocs checks the ceilings of the specs and of the minor
// version in the ordinary run, which does not run benchmarks.
func TestPoliciesAllocs(t *testing.T) {
	t.Run("Policies", func(t *testing.T) {
		var got []plugin.PolicySpec
		assert.MaxAllocs(t, func() { got = python.Policies() }, policiesAllocs,
			"Policies allocates its list and the choices of each spec")
		assert.Length(t, got, 2, "Policies returns two specs")
	})

	t.Run("Minor", func(t *testing.T) {
		v := string(python.Version315)
		minor := 0
		assert.MaxAllocs(t, func() { minor, _ = python.Minor(v) }, 0, "Minor does not allocate")
		assert.Equal(t, minor, 15, "Minor returns the minor version")
	})
}

// BenchmarkPolicies measures the specs that a backend declares once per
// build, and the minor version that the lowering reads per declaration.
func BenchmarkPolicies(b *testing.B) {
	b.Run("Policies", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(policiesAllocs)
		defer c.End()
		var got []plugin.PolicySpec
		for c.Loop() {
			got = python.Policies()
		}
		assert.Length(b, got, 2, "Policies returns two specs")
	})

	b.Run("Minor", func(b *testing.B) {
		v := string(python.Version315)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		minor := 0
		for c.Loop() {
			minor, _ = python.Minor(v)
		}
		assert.Equal(b, minor, 15, "Minor returns the minor version")
	})
}
