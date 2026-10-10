// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package catalog_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/plugin/shape/catalog"
	"go.dokimi.dev/eidos/sdk/diag"
)

// A code is API: consumers script against it and the published index
// anchors to it, so its number and its meaning are contract.
func TestCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		code    diag.Code
		spelled string
		meaning string
	}{
		{
			name: "RoleArity", code: catalog.RoleArity, spelled: "SHAPE-0001",
			meaning: "a role of a contract instance has a number of callables that its arity does not admit",
		},
		{
			name: "ParamRange", code: catalog.ParamRange, spelled: "SHAPE-0002",
			meaning: "an int param of a classification's directive is below the minimum of its spec",
		},
		{
			name: "ExclusiveParams", code: catalog.ExclusiveParams, spelled: "SHAPE-0003",
			meaning: "a classification's directive writes two params that its spec excludes from each other",
		},
		{
			name:    "UnsharedParam",
			code:    catalog.UnsharedParam,
			spelled: "SHAPE-0004",
			meaning: "the callable of a param lacks the parameter that a host-param reference of the directive resolves to",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("spells its prefix and padded number", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.code.String(), tt.spelled, "the spelling is pinned")
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
