// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/cli/internal/config"
	"go.dokimi.dev/eidos/core/layout"
)

// A policy in a config file is the name of a layout policy.
func TestPolicy(t *testing.T) {
	t.Parallel()

	t.Run("UnmarshalYAML", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes the name of each layout policy", func(t *testing.T) {
			t.Parallel()

			for p := layout.PolicyInherit; p.Valid(); p++ {
				var got config.Policy
				expect.NoError(t, yaml.Unmarshal([]byte(p.String()), &got), "the name of the policy decodes")
				expect.Equal(t, got, config.Policy(p), "the policy has the name")
			}
		})

		faults := []struct {
			name string
			give string
			want string
		}{
			{
				name: "returns a TypeError for a name in capitals",
				give: "Centralised",
				want: `line 1: "Centralised" is not a layout policy: use inherit, alongside-source or centralised`,
			},
			{
				name: "returns a TypeError for a list",
				give: "[centralised]",
				want: `line 1: "" is not a layout policy: use inherit, alongside-source or centralised`,
			},
		}
		for _, tt := range faults {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var got config.Policy
				typed := assert.ErrorAs[*yaml.TypeError](t, yaml.Unmarshal([]byte(tt.give), &got),
					"the decoder reports the fault as a type error")
				assert.Equal(t, typed.Errors, []string{tt.want}, "the error lists the names of the policies")
			})
		}
	})
}
