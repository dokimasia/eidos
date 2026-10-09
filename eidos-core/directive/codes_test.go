// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
)

// A code is API: consumers script against it and the published
// index anchors to it, so its number and meaning are contract.
func TestCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		code    diag.Code
		spelled string
		meaning string
	}{
		{
			name: "UnclaimedName", code: directive.UnclaimedName, spelled: "EID-0004",
			meaning: "a directive names no registered schema",
		},
		{
			name: "AmbiguousName", code: directive.AmbiguousName, spelled: "EID-0005",
			meaning: "a bare directive name has two claimants",
		},
		{
			name: "UnknownKey", code: directive.UnknownKey, spelled: "EID-0006",
			meaning: "a directive writes a key its schema does not accept",
		},
		{
			name: "DuplicateKey", code: directive.DuplicateKey, spelled: "EID-0007",
			meaning: "a directive writes one key twice",
		},
		{
			name: "TypeMismatch", code: directive.TypeMismatch, spelled: "EID-0008",
			meaning: "a directive value has the wrong shape for its param",
		},
		{
			name: "BadSpelling", code: directive.BadSpelling, spelled: "EID-0009",
			meaning: "a directive value is outside its type's spelling",
		},
		{
			name: "ExtraPositional", code: directive.ExtraPositional, spelled: "EID-0010",
			meaning: "a directive writes more positional arguments than its schema declares",
		},
		{
			name: "MissingParam", code: directive.MissingParam, spelled: "EID-0011",
			meaning: "a directive omits a param its schema requires",
		},
		{
			name: "UnknownRole", code: directive.UnknownRole, spelled: "EID-0012",
			meaning: "a directive's role is outside its schema's declared set",
		},
		{
			name: "MissingRole", code: directive.MissingRole, spelled: "EID-0013",
			meaning: "a directive omits the role its schema demands",
		},
		{
			name: "DuplicateInstance", code: directive.DuplicateInstance, spelled: "EID-0014",
			meaning: "a directive appears on one subject more often than its schema admits",
		},
		{
			name: "RequirementUnmet", code: directive.RequirementUnmet, spelled: "EID-0015",
			meaning: "a directive requires another directive the subject does not have",
		},
		{
			name: "Conflict", code: directive.Conflict, spelled: "EID-0016",
			meaning: "two directives on one subject conflict",
		},
		{
			name: "UnknownMetadataKey", code: directive.UnknownMetadataKey, spelled: "EID-0017",
			meaning: "a directive names a metadata key or group nothing registered",
		},
		{
			name: "DanglingSubject", code: directive.DanglingSubject, spelled: "EID-0018",
			meaning: "a directive or stamp is attached to a subject the graph does not contain",
		},
		{
			name: "UnsealedRegistry", code: directive.UnsealedRegistry, spelled: "EID-0019",
			meaning: "directive validation ran before the registry sealed",
		},
		{
			name: "UnresolvedReference", code: directive.UnresolvedReference, spelled: "EID-0040",
			meaning: "a directive's reference param resolves to nothing",
		},
		{
			name: "NegationRefused", code: directive.NegationRefused, spelled: "EID-0043",
			meaning: "a directive is negated, and its schema does not accept the negated form",
		},
		{
			name: "MixedCarriers", code: directive.MixedCarriers, spelled: "EID-0045",
			meaning: "a repeatable directive mixes carriers a formatter may move with carriers it keeps",
		},
		{
			name: "UnknownCode", code: directive.UnknownCode, spelled: "EID-0066",
			meaning: "a directive names a diagnostic code nothing registered",
		},
		{
			name: "DeprecatedDirective", code: directive.DeprecatedDirective, spelled: "EID-0067",
			meaning: "a directive or a parameter that its schema deprecates is used",
		},
		{
			name: "UnknownVariant", code: directive.UnknownVariant, spelled: "EID-0068",
			meaning: "a directive has no variant, or a name that no variant of its schema has",
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
