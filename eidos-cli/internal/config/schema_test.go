// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/golden"

	"go.dokimi.dev/eidos/cli/internal/config"
)

// schemaFile is the JSON Schema that the module publishes at its root.
var schemaFile = filepath.Join("..", "..", "config.schema.json")

// The keys of the format that the cases read: the options of a document,
// the version of a file and the workspaces of a list. propertiesKey is the
// keyword of the properties of an object.
const (
	optionsKey    = "options"
	versionKey    = "version"
	workspacesKey = "workspaces"
	propertiesKey = "properties"
)

// topOf is the part of a schema that the cases read: the properties of a
// document and of a list, the two schemas of oneOf, and what each admits
// beside its properties.
type topOf struct {
	OneOf []struct {
		Properties map[string]any `json:"properties"`
		Additional any            `json:"additionalProperties"`
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
			var got topOf
			assert.NoError(t, json.Unmarshal(config.Schema(give), &got), "the schema is JSON")
			assert.NotEmpty(t, got.OneOf, "the schema has the schema of a document")
			options, _ := got.OneOf[0].Properties[optionsKey].(map[string]any)
			assert.Equal(t, options[propertiesKey], any(give),
				"the options of a document have the schema of the options of each plugin")
		})

		t.Run("returns the same bytes on each call", func(t *testing.T) {
			t.Parallel()

			assert.Deterministic(t, func(struct{}) ([]byte, error) { return config.Schema(nil), nil }, struct{}{},
				"the schema does not depend on the order of a map")
		})

		var top topOf
		assert.NoError(t, json.Unmarshal(config.Schema(nil), &top), "the schema is JSON")
		assert.Length(t, top.OneOf, 2, "the schema has the schema of a document and of a list")
		document, list := top.OneOf[0], top.OneOf[1]

		t.Run("returns a document and a list that admit any other top-level key", func(t *testing.T) {
			t.Parallel()

			expect.Equal(t, document.Additional, any(map[string]any{}), "a document admits the key of an option")
			expect.Equal(t, list.Additional, any(map[string]any{}), "a list admits the key of an option")
		})

		t.Run("returns a document without the key workspaces", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, document.Properties[workspacesKey], any(false), "the key workspaces makes a file a list")
		})

		t.Run("returns a list without the other keys of a document", func(t *testing.T) {
			t.Parallel()

			for _, key := range config.Keys() {
				if key != versionKey && key != workspacesKey {
					expect.Equal(t, list.Properties[key], any(false), "a list does not have the key "+key)
				}
			}
		})
	})
}
