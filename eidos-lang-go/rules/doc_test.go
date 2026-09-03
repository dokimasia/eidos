// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/rules"
	sdkrules "go.dokimi.dev/eidos/sdk/rules"
)

// The package documentation states the capability set, which is
// the contract a consumer asserts against.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("satisfies every capability the documentation names", func(t *testing.T) {
		t.Parallel()

		r := rules.New()
		_, enum := r.(sdkrules.EnumRules)
		_, errs := r.(sdkrules.ErrorValueRules)
		_, tags := r.(sdkrules.TagRules)
		_, generics := r.(sdkrules.GenericsRules)
		_, promotion := r.(sdkrules.PromotionRules)
		_, equality := r.(sdkrules.EqualityRules)
		assert.True(
			t,
			enum && errs && tags && generics && promotion && equality,
			"the six named capabilities",
		)
		_, properties := r.(sdkrules.PropertyRules)
		_, constructs := r.(sdkrules.ConstructRules)
		_, throws := r.(sdkrules.ThrowsRules)
		_, ownership := r.(sdkrules.OwnershipRules)
		assert.False(
			t,
			properties || constructs || throws || ownership,
			"and none of the four Go lacks",
		)
	})
}
