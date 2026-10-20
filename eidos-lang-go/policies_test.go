// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// TestPolicies pins the spec of the Go policy, whose key and choices a
// config file writes.
func TestPolicies(t *testing.T) {
	t.Parallel()

	t.Run("Policies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the gRPC binding with none as its default", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, golang.Policies(), []plugin.PolicySpec{{
				Key:     "golang.grpc",
				Choices: []plugin.Choice{"none", "grpc-go", "connect-go"},
				Default: "none",
				Doc:     "the gRPC binding of the code that a plan writes against a protobuf service",
			}}, "the one policy of Go")
		})
	})
}
