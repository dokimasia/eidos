// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/cli/internal/config"
)

// A list has at least one workspace, and every workspace has a root.
func TestList(t *testing.T) {
	t.Parallel()

	t.Run("Decode", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want []config.Error
		}{
			{
				name: "returns an Error at the key for an empty list",
				give: "version: 1\nworkspaces: []\n",
				want: []config.Error{{File: fileName, Line: 2, Msg: "the list has no workspaces"}},
			},
			{
				name: "returns an Error at the key for a list without a value",
				give: "version: 1\nworkspaces:\n",
				want: []config.Error{{File: fileName, Line: 2, Msg: "the list has no workspaces"}},
			},
			{
				name: "returns an Error at each workspace without a root",
				give: "version: 1\nworkspaces:\n    - {config: a.yaml}\n    - {root: b}\n    - {config: c.yaml}\n",
				want: []config.Error{
					{File: fileName, Line: 3, Msg: "the workspace has no root"},
					{File: fileName, Line: 5, Msg: "the workspace has no root"},
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := config.Decode(fileName, []byte(tt.give))
				assert.Equal(t, faultsOf(t, err), tt.want, "Decode returns the faults of the list")
			})
		}
	})
}
