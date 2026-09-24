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
		code     diag.Code
		spelled  string
		meaning  string
		describe string
	}{
		{
			code: render.UnspeltKind, spelled: "EID-0021", describe: "UnspeltKind",
			meaning: "a declaration's kind has no template in the target language",
		},
		{
			code: render.UnformattedFile, spelled: "EID-0022", describe: "UnformattedFile",
			meaning: "the language formatter refused a rendered file",
		},
		{
			code: render.RefusedTemplate, spelled: "EID-0023", describe: "RefusedTemplate",
			meaning: "a template refused a declaration at execute time",
		},
		{
			code: render.BodyConflict, spelled: "EID-0024", describe: "BodyConflict",
			meaning: "a body holds more than one content form",
		},
		{
			code: render.UnresolvedRef, spelled: "EID-0025", describe: "UnresolvedRef",
			meaning: "a template reference resolves to nothing in its emitting plugin's tree",
		},
		{
			code: render.DroppedSlots, spelled: "EID-0026", describe: "DroppedSlots",
			meaning: "a body-claiming template places no marker for pending contributions",
		},
		{
			code: render.UndeclaredOverride, spelled: "EID-0027", describe: "UndeclaredOverride",
			meaning: "a plugin shadows a shared template helper without declaring the override",
		},
		{
			code: render.UnknownGroup, spelled: "EID-0028", describe: "UnknownGroup",
			meaning: "a cluster names a group the target language declares no template for",
		},
		{
			code: render.HelperCollision, spelled: "EID-0036", describe: "HelperCollision",
			meaning: "two plugins register one template helper name",
		},
		{
			code: render.UnspeltValue, spelled: "EID-0042", describe: "UnspeltValue",
			meaning: "a scaffold value has no spelling in the target language",
		},
	}
	for _, tt := range codes {
		t.Run(tt.describe, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.code.String(), tt.spelled,
				"the code spells its prefix and padded number")
			meaning, held := diag.Kernel().Meaning(tt.code)
			assert.True(t, held, "the code registered at initialization")
			assert.Equal(t, meaning, tt.meaning, "the meaning anchors the published index")
		})
	}
}
