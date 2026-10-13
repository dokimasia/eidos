// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/golden"

	"go.dokimi.dev/eidos/cli/internal/config"
)

// schemaFile is the JSON Schema that the module publishes at its root.
var schemaFile = filepath.Join("..", "..", "config.schema.json")

// optionsKey is the key of the options of a document.
const optionsKey = "options"

// documentOf is the part of a schema that the options cases read: the
// properties of the properties of a document, the first schema of oneOf.
type documentOf struct {
	OneOf []struct {
		Properties map[string]struct {
			Properties map[string]any `json:"properties"`
		} `json:"properties"`
	} `json:"oneOf"`
}

// The module publishes the JSON Schema that Schema returns. Run the test
// with -update to write the file again after a change of the format.
func TestSchema(t *testing.T) {
	t.Parallel()

	t.Run("Schema", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the schema that the module publishes", func(t *testing.T) {
			t.Parallel()

			golden.MatchAt(t, schemaFile, config.Schema(nil), golden.ShouldUpdate())
		})

		t.Run("returns the schema of the options of each plugin under the options of a document", func(t *testing.T) {
			t.Parallel()

			give := map[string]any{"stubgen": map[string]any{"type": "object"}}
			var got documentOf
			assert.NoError(t, json.Unmarshal(config.Schema(give), &got), "the schema is JSON")
			assert.NotEmpty(t, got.OneOf, "the schema has the schema of a document")
			assert.Equal(t, got.OneOf[0].Properties[optionsKey].Properties, give,
				"the options of a document have the schema of the options of each plugin")
		})

		t.Run("returns the same bytes on each call", func(t *testing.T) {
			t.Parallel()

			assert.Deterministic(t, func(struct{}) ([]byte, error) { return config.Schema(nil), nil }, struct{}{},
				"the schema does not depend on the order of a map")
		})
	})
}
