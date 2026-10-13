// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/cli"
	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// publishedSchema is the JSON Schema of the config file format that the
// module publishes at its root.
const publishedSchema = "config.schema.json"

// tunedName is the name of the generator with options of the Schema cases.
const tunedName plugin.ID = "tuned"

// tunedOptions are the options of the tuned generator. The suffix is
// required.
type tunedOptions struct {
	Suffix string `opt:"suffix" doc:"the suffix of a tuned name" schema:"required"`
}

// untaggedOptions break the tag contract, because the field has no opt tag.
type untaggedOptions struct {
	Suffix string `doc:"the suffix of a tuned name"`
}

// schemaEvent is the schema event of JSON output.
type schemaEvent struct {
	Schema json.RawMessage `json:"schema"`
}

// optionsOf is the part of a schema that the cases read: the schema of the
// options of each plugin under the options of a document, the first schema
// of oneOf.
type optionsOf struct {
	OneOf []struct {
		Properties struct {
			Options struct {
				Properties map[string]any `json:"properties"`
			} `json:"options"`
		} `json:"properties"`
	} `json:"oneOf"`
}

// Schema returns the JSON Schema of the config file of a binary, with the
// options of each plugin of its composition, and the command schema prints
// it.
func TestSchema(t *testing.T) {
	t.Parallel()

	t.Run("Schema", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the published schema for a composition without options", func(t *testing.T) {
			t.Parallel()

			files.HasContent(t, publishedSchema, string(cli.Schema(compose)), "the composition adds no options")
		})

		t.Run("returns the schema of the options of each plugin under the options of a document", func(t *testing.T) {
			t.Parallel()

			gen := tuned(&tunedOptions{Suffix: "_tuned"})
			encoded, err := json.Marshal(plugin.OptionsSchema(gen))
			assert.NoError(t, err, "the schema of the options encodes")
			var want any
			assert.NoError(t, json.Unmarshal(encoded, &want), "the schema of the options decodes")
			got := optionProperties(t, cli.Schema(tunedCompose(gen)))
			assert.Equal(t, got, map[string]any{string(tunedName): want},
				"the options of a document have the schema of the options of the plugin")
		})

		t.Run("returns the options of a composition that does not build without its config file", func(t *testing.T) {
			t.Parallel()

			gen := tuned(&tunedOptions{})
			_, err := tunedCompose(gen)().Build()
			assert.HasError(t, err, "the composition requires the suffix")
			assert.Contains(t, optionProperties(t, cli.Schema(tunedCompose(gen))), string(tunedName),
				"the schema has the options of the plugin")
		})

		t.Run("leaves out a plugin whose options break the tag contract", func(t *testing.T) {
			t.Parallel()

			got := optionProperties(t, cli.Schema(tunedCompose(tuned(&untaggedOptions{}))))
			assert.Empty(t, got, "the schema has no options of the plugin")
		})
	})

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the schema to standard output", func(t *testing.T) {
			t.Parallel()

			status, stdout, stderr := invoke(t, compose, cmdSchema, t.TempDir())
			expect.Equal(t, status, cli.StatusOK, "schema succeeds")
			expect.Equal(t, stdout, string(cli.Schema(compose)), "schema writes the schema of the binary")
			expect.Empty(t, stderr, "schema reports nothing")
		})

		t.Run("writes the schema in a schema event in JSON output", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, compose, cmdSchema, t.TempDir(), jsonFlag)
			expect.Equal(t, decoded[header](t, stdout, ""),
				[]header{{Event: eventStart}, {Event: eventSchema}, {Event: eventSummary}},
				"the output has the start event, the schema event and the summary event")
			events := decoded[schemaEvent](t, stdout, eventSchema)
			assert.Length(t, events, 1, "the output has one schema event")
			var got, want any
			assert.NoError(t, json.Unmarshal(events[0].Schema, &got), "the schema of the event decodes")
			assert.NoError(t, json.Unmarshal(cli.Schema(compose), &want), "the schema of the binary decodes")
			assert.Equal(t, got, want, "the event has the schema of the binary")
		})

		t.Run("reads no config file", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(faultyConfig)})
			status, _, stderr := invoke(t, compose, cmdSchema, root)
			expect.Equal(t, status, cli.StatusOK, "the config file does not fail schema")
			expect.Empty(t, stderr, "schema reports nothing about the config file")
		})
	})
}

// tuned returns a generator named tuned with opts as its options.
func tuned(opts any) plugin.Generator {
	return eidos.NewPlugin(tunedName).
		Options(opts).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "tuned"}).
		Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error { return nil })).
		Generator()
}

// tunedCompose returns the fixture composition with gen beside the mirror
// in its plan.
func tunedCompose(gen plugin.Generator) cli.Compose {
	return func() *workspace.Builder {
		return workspace.New().
			Brand(brand).
			Frontends(frontendtest.NewScripted()).
			Targets(target).
			Plans(workspace.Plan{Name: planName, Generators: []plugin.Generator{mirror(), gen}, Backend: printer()})
	}
}

// optionProperties returns the schema of the options of each plugin under
// the options of a document in the schema s, by the name of the plugin.
func optionProperties(t *testing.T, s []byte) map[string]any {
	t.Helper()

	var got optionsOf
	assert.NoError(t, json.Unmarshal(s, &got), "the schema is JSON")
	assert.NotEmpty(t, got.OneOf, "the schema has the schema of a document")
	return got.OneOf[0].Properties.Options.Properties
}
