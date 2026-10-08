// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/golden"

	"go.dokimi.dev/eidos/cli/internal/config"
)

// schemaFile is the JSON Schema that the module publishes at its root.
var schemaFile = filepath.Join("..", "..", "config.schema.json")

// The module publishes the JSON Schema that Schema returns. Run the test
// with -update to write the file again after a change of the format.
func TestSchema(t *testing.T) {
	t.Parallel()

	t.Run("Schema", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the schema that the module publishes", func(t *testing.T) {
			t.Parallel()

			golden.MatchAt(t, schemaFile, config.Schema(), golden.ShouldUpdate())
		})

		t.Run("returns the same bytes on each call", func(t *testing.T) {
			t.Parallel()

			assert.Deterministic(t, func(struct{}) ([]byte, error) { return config.Schema(), nil }, struct{}{},
				"the schema does not depend on the order of a map")
		})
	})
}
