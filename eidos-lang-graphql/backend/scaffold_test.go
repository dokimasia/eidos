// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// TestScaffold pins the refusal of every statement, because a GraphQL
// document declares no statement.
func TestScaffold(t *testing.T) {
	t.Parallel()

	t.Run("Scaffold", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for every statement", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Scaffold(emit.Stmt{Kind: emit.StmtReturn}, &render.ImportSet{})
			assert.HasError(t, err, "GraphQL writes no statement")
			assert.Equal(t, err.Error(), "graphql: a document declares no statement, so GraphQL writes none",
				"the error states the reason")
			assert.Nil(t, got, "a refusal returns no text")
		})
	})
}
