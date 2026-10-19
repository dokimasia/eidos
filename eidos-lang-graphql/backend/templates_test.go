// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestTemplates pins the kinds that GraphQL declares at file level, the
// writer that the template of each kind calls, and the kinds that GraphQL
// refuses.
func TestTemplates(t *testing.T) {
	t.Parallel()

	t.Run("KindTemplates", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a template that calls the writer of each kind that GraphQL declares", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.KindTemplates(), map[symbol.Kind]string{
				symbol.KindStruct:    "{{" + backend.FuncStruct + " .}}",
				symbol.KindInterface: "{{" + backend.FuncInterface + " .}}",
				symbol.KindEnum:      "{{" + backend.FuncEnum + " .}}",
				symbol.KindAlias:     "{{" + backend.FuncAlias + " .}}",
				symbol.KindFunction:  "{{" + backend.FuncFunction + " .}}",
			}, "the kind templates")
		})

		t.Run("returns a map of the caller's own", func(t *testing.T) {
			t.Parallel()

			delete(backend.KindTemplates(), symbol.KindStruct)
			assert.Length(t, backend.KindTemplates(), 5, "a change of one map leaves the next map unchanged")
		})
	})

	t.Run("RefusedKinds", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a reason for each kind that GraphQL does not declare", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.RefusedKinds(), map[symbol.Kind]string{
				symbol.KindConstant: "a GraphQL schema declares no constant",
				symbol.KindVariable: "a GraphQL schema declares no variable",
			}, "the refused kinds")
		})

		t.Run("returns a map of the caller's own", func(t *testing.T) {
			t.Parallel()

			delete(backend.RefusedKinds(), symbol.KindConstant)
			assert.Length(t, backend.RefusedKinds(), 2, "a change of one map leaves the next map unchanged")
		})
	})
}
