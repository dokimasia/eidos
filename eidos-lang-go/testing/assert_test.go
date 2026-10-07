// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// Vet is the Go-only assertion: what it passes and what it reports
// are contract for every generator the harness checks.
func TestAssert(t *testing.T) {
	t.Parallel()

	t.Run("AssertVets", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a fixture carrying no output", func(t *testing.T) {
			t.Parallel()

			failure := rejection(t, func(tb assert.TB) {
				gotesting.AssertVets(t.Context(), tb, adapter(), toolchain.Generated{})
			})
			want := string(golang.Lang) +
				": the fixture carries generated output, without which a toolchain run proves nothing"
			assert.Equal(t, failure.Contract, want, "a vet over nothing passes while proving nothing")
		})

		t.Run("passes output that vets clean", func(t *testing.T) {
			t.Parallel()
			requireGo(t)

			gotesting.AssertVets(t.Context(), t, adapter(), healthy())
		})

		t.Run("fails compiling output that go vet reports on", func(t *testing.T) {
			t.Parallel()
			requireGo(t)

			failure := rejection(t, func(tb assert.TB) {
				gotesting.AssertVets(t.Context(), tb, adapter(), only(rowFile,
					"package harness\n\nimport \"fmt\"\n\n"+
						"// Print misuses its verb.\nfunc Print() string { return fmt.Sprintf(\"%d\", \"text\") }\n"))
			})
			assert.Equal(t, failure.Contract, string(golang.Lang)+": go vet accepts the generated output",
				"the wrong verb is caught")
			assert.Contains(t, reason(t, failure), "wrong type", "in go vet's own words")
		})

		t.Run("fails the run of a context that ended", func(t *testing.T) {
			t.Parallel()

			ended, cancel := context.WithCancel(t.Context())
			cancel()
			failure := rejection(t, func(tb assert.TB) { gotesting.AssertVets(ended, tb, adapter(), healthy()) })
			got, stated := failure.Got()
			assert.True(t, stated, "the record states the run's error")
			err, isErr := got.(error)
			assert.True(t, isErr, "as an error")
			assert.ErrorIs(t, err, context.Canceled, "which wraps the context's error")
		})
	})
}
