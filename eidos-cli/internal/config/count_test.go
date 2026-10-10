// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/cli/internal/config"
)

// A count is a whole number of 0 or more.
func TestCount(t *testing.T) {
	t.Parallel()

	t.Run("UnmarshalYAML", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want config.Count
		}{
			{name: "decodes 0", give: "0", want: 0},
			{name: "decodes a positive number", give: "16", want: 16},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var got config.Count
				assert.NoError(t, yaml.Unmarshal([]byte(tt.give), &got), "the count decodes")
				assert.Equal(t, got, tt.want, "the count has the value of the number")
			})
		}

		faults := []struct {
			name string
			give string
			want string
		}{
			{
				name: "returns a TypeError for a negative number",
				give: "-1",
				want: `line 1: "-1" is not a count: use a whole number of 0 or more`,
			},
			{
				name: "returns a TypeError for a fraction",
				give: "1.5",
				want: `line 1: "1.5" is not a count: use a whole number of 0 or more`,
			},
			{
				name: "returns a TypeError for a word",
				give: "four",
				want: `line 1: "four" is not a count: use a whole number of 0 or more`,
			},
			{
				name: "returns a TypeError for a number above the largest int",
				give: "9223372036854775808",
				want: `line 1: "9223372036854775808" is not a count: use a whole number of 0 or more`,
			},
		}
		for _, tt := range faults {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var got config.Count
				typed := assert.ErrorAs[*yaml.TypeError](t, yaml.Unmarshal([]byte(tt.give), &got),
					"the decoder reports the fault as a type error")
				assert.Equal(t, typed.Errors, []string{tt.want}, "the error has the line of the value")
			})
		}
	})
}
