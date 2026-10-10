// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/layout"
)

// A code is API: consumers script against it and the published index
// anchors to it, so its number and meaning are contract.
func TestCodes(t *testing.T) {
	t.Parallel()

	codes := []struct {
		name    string
		code    diag.Code
		spelt   string
		meaning string
	}{
		{
			name: "UndeclaredFamily", code: layout.UndeclaredFamily, spelt: "EID-0048",
			meaning: "a unit's family is not among its plugin's declared families",
		},
		{
			name: "UnknownTag", code: layout.UnknownTag, spelt: "EID-0049",
			meaning: "a tag override names no family the plugin declares",
		},
		{
			name: "AmbiguousOverride", code: layout.AmbiguousOverride, spelt: "EID-0050",
			meaning: "a filename override applies to more than one family of one plugin",
		},
		{
			name: "NoDestination", code: layout.NoDestination, spelt: "EID-0051",
			meaning: "no directory resolves for a declaration",
		},
		{
			name: "EscapingPath", code: layout.EscapingPath, spelt: "EID-0052",
			meaning: "a path override is absolute or leaves the workspace root",
		},
		{
			name: "PathCollision", code: layout.PathCollision, spelt: "EID-0053",
			meaning: "two routed paths collide",
		},
		{
			name: "UnderivedPackage", code: layout.UnderivedPackage, spelt: "EID-0054",
			meaning: "a reference needs the package of a routed file, and none derives",
		},
		{
			name: "UntranslatedReference", code: layout.UntranslatedReference, spelt: "EID-0070",
			meaning: "a translated reference refers to a declaration that the plan does not emit",
		},
	}
	for _, tt := range codes {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("spells the kernel prefix with its padded number", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.code.String(), tt.spelt, "the spelling is pinned")
			})

			t.Run("registers its meaning in the kernel registry", func(t *testing.T) {
				t.Parallel()

				meaning, held := diag.Kernel().Meaning(tt.code)
				assert.True(t, held, "the code registered at initialization")
				assert.Equal(t, meaning, tt.meaning, "the meaning is pinned")
			})
		})
	}
}
