// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/sdk/rules"
)

// The package documentation states the capability set, which is
// the contract a consumer asserts against.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("satisfies the one capability the documentation names", func(t *testing.T) {
		t.Parallel()

		r := protorules.New()
		_, enum := r.(rules.EnumRules)
		assert.True(t, enum, "protobuf projects enums")
		_, generics := r.(rules.GenericsRules)
		_, promotion := r.(rules.PromotionRules)
		_, equality := r.(rules.EqualityRules)
		_, tags := r.(rules.TagRules)
		assert.False(t, generics || promotion || equality || tags,
			"and none of the capabilities a schema language has no shape for")
	})
}
