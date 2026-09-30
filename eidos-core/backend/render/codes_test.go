// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
)

// A code is API: consumers script against it and the published
// index anchors to it, so each render code's number and meaning are
// contract.
func TestCodes(t *testing.T) {
	t.Parallel()

	codes := []struct {
		name    string
		code    diag.Code
		spelled string
		meaning string
	}{
		{
			name: "UnspeltKind", code: render.UnspeltKind, spelled: "EID-0021",
			meaning: "a declaration's kind has no template in the target language",
		},
		{
			name: "UnformattedFile", code: render.UnformattedFile, spelled: "EID-0022",
			meaning: "the language formatter refused a rendered file",
		},
		{
			name: "RefusedTemplate", code: render.RefusedTemplate, spelled: "EID-0023",
			meaning: "a template refused a declaration at execute time",
		},
		{
			name: "BodyConflict", code: render.BodyConflict, spelled: "EID-0024",
			meaning: "a body states more than one content form",
		},
		{
			name: "UnresolvedRef", code: render.UnresolvedRef, spelled: "EID-0025",
			meaning: "a template reference resolves to nothing in its emitting plugin's tree",
		},
		{
			name: "DroppedSlots", code: render.DroppedSlots, spelled: "EID-0026",
			meaning: "a body-claiming template places no marker for pending contributions",
		},
		{
			name: "UndeclaredOverride", code: render.UndeclaredOverride, spelled: "EID-0027",
			meaning: "a plugin shadows a shared template helper without declaring the override",
		},
		{
			name: "UnknownGroup", code: render.UnknownGroup, spelled: "EID-0028",
			meaning: "a cluster names a group the target language declares no template for",
		},
		{
			name: "HelperCollision", code: render.HelperCollision, spelled: "EID-0036",
			meaning: "two plugins register one template helper name",
		},
		{
			name: "UnspeltValue", code: render.UnspeltValue, spelled: "EID-0042",
			meaning: "a scaffold value has no spelling in the target language",
		},
		{
			name: "RefusedKind", code: render.RefusedKind, spelled: "EID-0046",
			meaning: "the target language refuses a declaration's kind",
		},
	}
	for _, tt := range codes {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("spells the kernel prefix with its padded number", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.code.String(), tt.spelled, "the spelling is pinned")
			})

			t.Run("registers its meaning in the kernel registry", func(t *testing.T) {
				t.Parallel()

				meaning, held := diag.Kernel().Meaning(tt.code)
				assert.True(t, held, "the code registered at initialization")
				assert.Equal(t, meaning, tt.meaning, "the meaning anchors the published index")
			})
		})
	}
}
