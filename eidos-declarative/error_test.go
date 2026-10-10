// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/declarative"
)

// The text of an Error has the file of the fault and, where the Error has
// one, the line.
func TestError(t *testing.T) {
	t.Parallel()

	t.Run("Error", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give declarative.Error
			want string
		}{
			{
				name: "returns the file, the line and the message",
				give: declarative.Error{File: manifestFile, Line: 3, Msg: "the manifest has no version"},
				want: "plugin.yaml:3: the manifest has no version",
			},
			{
				name: "returns the file and the message for an Error without a line",
				give: declarative.Error{File: "templates/notes.md", Msg: "the file is directly below templates"},
				want: "templates/notes.md: the file is directly below templates",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.Error(), tt.want, "the text has the position of the fault")
			})
		}
	})
}
