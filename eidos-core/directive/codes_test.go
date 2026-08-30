// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package directive_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
)

// validationCodes lists every code this package refuses under.
var validationCodes = []diag.Code{
	directive.UnclaimedName, directive.AmbiguousName, directive.UnknownKey,
	directive.DuplicateKey, directive.TypeMismatch, directive.BadSpelling,
	directive.ExtraPositional, directive.MissingParam, directive.UnknownRole,
	directive.MissingRole, directive.DuplicateInstance,
	directive.RequirementUnmet, directive.Conflict,
	directive.UnknownMetadataKey, directive.DanglingSubject,
	directive.UnsealedRegistry,
}

// Every failure class carries its own code, and each is API from
// the moment it registers.
func TestCodes(t *testing.T) {
	t.Parallel()

	t.Run("are registered in the kernel registry", func(t *testing.T) {
		t.Parallel()

		for _, code := range validationCodes {
			meaning, known := diag.Kernel().Meaning(code)
			assert.True(t, known, "every validation code is registered")
			assert.NotEqual(t, meaning, "", "with a meaning the index anchors to")
			assert.Equal(t, code.Prefix, diag.KernelPrefix, "under the kernel's prefix")
		}
	})

	t.Run("name distinct findings", func(t *testing.T) {
		t.Parallel()

		seen := make(map[diag.Code]struct{}, len(validationCodes))
		for _, code := range validationCodes {
			seen[code] = struct{}{}
		}
		assert.Length(t, seen, len(validationCodes),
			"a consumer scripting per class can tell every class apart")
	})
}
