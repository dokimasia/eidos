// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/golden"

	"go.dokimi.dev/eidos/declarative"
)

// schemaFile is the JSON Schema that the module publishes at its root.
const schemaFile = "plugin.schema.json"

// The module publishes the JSON Schema that Schema returns. Run the test
// with -update to write the file again after a change of the format.
func TestSchema(t *testing.T) {
	t.Parallel()

	t.Run("Schema", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the schema that the module publishes", func(t *testing.T) {
			t.Parallel()

			golden.MatchAt(t, schemaFile, declarative.Schema(), golden.ShouldUpdate())
		})

		t.Run("returns the same bytes on each call", func(t *testing.T) {
			t.Parallel()

			assert.Deterministic(t, func(struct{}) ([]byte, error) { return declarative.Schema(), nil }, struct{}{},
				"the schema does not depend on the order of a map")
		})

		t.Run("requires the keys that every manifest writes", func(t *testing.T) {
			t.Parallel()

			var schema struct {
				Required []string `json:"required"`
			}
			assert.NoError(t, json.Unmarshal(declarative.Schema(), &schema), "the schema is JSON")
			assert.Equal(t, schema.Required, []string{"format", "name", "version", "outputs", "rules"},
				"a manifest without one of these keys does not load")
		})
	})
}
