// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// TestScaffold pins the statement printer of protobuf, which refuses every
// statement, because a schema declares no body.
func TestScaffold(t *testing.T) {
	t.Parallel()

	t.Run("Scaffold", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give emit.Stmt
		}{
			{name: "returns an error for a return", give: emit.Stmt{Kind: emit.StmtReturn}},
			{
				name: "returns an error for an assignment",
				give: emit.Stmt{Kind: emit.StmtAssign, Names: []string{"state"}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Scaffold(tt.give, &render.ImportSet{})
				assert.HasError(t, err, "protobuf writes no statement")
				assert.Equal(t, err.Error(), "protobuf: a schema declares no body, so protobuf writes no statement",
					"the error states the reason")
				assert.Nil(t, got, "a refusal writes nothing")
			})
		}
	})
}

// Scaffold refuses without allocating. The ordinary run, which runs no
// benchmark, checks that ceiling here.
func TestScaffoldAllocs(t *testing.T) {
	set := &render.ImportSet{}
	var err error
	assert.MaxAllocs(t, func() { _, err = backend.Scaffold(emit.Stmt{Kind: emit.StmtReturn}, set) }, 0,
		"Scaffold allocates nothing")
	assert.HasError(t, err, "Scaffold refuses the statement")
}

// BenchmarkScaffold measures the refusal of a statement.
func BenchmarkScaffold(b *testing.B) {
	b.Run("Scaffold", func(b *testing.B) {
		set := &render.ImportSet{}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var err error
		for c.Loop() {
			_, err = backend.Scaffold(emit.Stmt{Kind: emit.StmtReturn}, set)
		}
		assert.HasError(b, err, "Scaffold refuses the statement")
	})
}
