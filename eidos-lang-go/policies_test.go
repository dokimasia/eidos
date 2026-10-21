// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// TestPolicies pins the specs of the Go policies, whose keys and choices a
// config file writes.
func TestPolicies(t *testing.T) {
	t.Parallel()

	t.Run("Policies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the gRPC binding with none as its default", func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, golang.Policies(), plugin.PolicySpec{
				Key:     "golang.grpc",
				Choices: []plugin.Choice{"none", "grpc-go", "connect-go"},
				Default: "none",
				Doc:     "the gRPC binding of the code that a plan writes against a protobuf service",
			}, "the policy of the binding")
		})

		t.Run("returns the width of a line as a policy of the plan with 100 as its default", func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, golang.Policies(), plugin.PolicySpec{
				Key:     "golang.width",
				Choices: []plugin.Choice{"80", "100", "120", "none"},
				Default: "100",
				Doc:     "the width in columns at which the written files wrap a line of code",
				Plan:    true,
			}, "the policy of the width")
		})

		t.Run("returns two policies", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, golang.Policies(), 2, "the binding and the width")
		})
	})
}
