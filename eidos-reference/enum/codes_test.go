// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package enum_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/reference/enum"
	"go.dokimi.dev/eidos/sdk/diag"
)

// The package registers its codes under its own prefix when it
// initializes, each with its number and its meaning.
func TestCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		give        diag.Code
		wantCode    string
		wantMeaning string
	}{
		{
			name: "DuplicateText", give: enum.DuplicateText, wantCode: "ENUM-0001",
			wantMeaning: "two variants of an enumeration have one text",
		},
		{
			name: "NoProjection", give: enum.NoProjection, wantCode: "ENUM-0002",
			wantMeaning: "the rules of the language of an enumeration do not project it",
		},
		{
			name: "UnknownText", give: enum.UnknownText, wantCode: "ENUM-0003",
			wantMeaning: "a variant of an enumeration of text has no text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("has the prefix of the package and its number", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.wantCode, "the code")
			})

			t.Run("has a meaning in the registry of the codes", func(t *testing.T) {
				t.Parallel()

				meaning, registered := diag.Kernel().Meaning(tt.give)
				assert.True(t, registered, "the package registers the code")
				assert.Equal(t, meaning, tt.wantMeaning, "the meaning anchors the code in the index")
			})
		})
	}
}
