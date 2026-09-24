// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"testing"

	"go.dokimi.dev/assert"

	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// Vet is the Go-only assertion: what it passes and what it reports
// are contract for every generator the harness checks.
func TestAssertVets(t *testing.T) {
	t.Parallel()

	t.Run("refuses a fixture carrying no output", func(t *testing.T) {
		t.Parallel()

		var r recorder
		gotesting.AssertVets(&r, adapter(), toolchain.Generated{})
		assert.True(t, r.says("carries no generated output"),
			"a vet over nothing passes while proving nothing")
	})

	t.Run("runs go vet over the generated output", func(t *testing.T) {
		t.Parallel()

		var probe recorder
		if !toolchain.Require(&probe, adapter()) {
			t.Skip("the Go toolchain is absent, so vet is skipped here and required in CI: " +
				probe.skipped)
		}

		t.Run("passes output vet accepts", func(t *testing.T) {
			t.Parallel()

			var r recorder
			gotesting.AssertVets(&r, adapter(), healthy())
			assert.False(t, r.failed(), "the healthy output vets clean")
		})

		t.Run("reports what compiles and is still wrong", func(t *testing.T) {
			t.Parallel()

			var r recorder
			gotesting.AssertVets(&r, adapter(), only(rowFile,
				"package harness\n\nimport \"fmt\"\n\n"+
					"// Print misuses its verb.\nfunc Print() string { return fmt.Sprintf(\"%d\", \"text\") }\n"))
			assert.True(t, r.says("go vet refused"), "the wrong verb is caught")
		})
	})
}
