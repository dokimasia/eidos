// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
)

// The pattern of a name, which its schema publishes.
const (
	namePattern    = `^[a-z][a-z0-9]*(-[a-z][a-z0-9]*)*$`
	keywordPattern = "pattern"
)

// A name is lowercase words that open with a letter, joined by single
// hyphens, so the generator can join its words into a Go identifier.
func TestName(t *testing.T) {
	t.Parallel()

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("UnmarshalYAML", func(t *testing.T) {
			t.Parallel()

			for _, give := range []string{"a", "probe", "batch-writer", "s3-store", "x2"} {
				t.Run("decodes "+give, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, decodeAs[specfront.Name](t, give), specfront.Name(give), "the name decodes")
				})
			}

			for _, give := range []string{"Probe", "2fa", "batch--writer", "batch-", "batch_writer", "batch-2x"} {
				t.Run("returns an error for "+give, func(t *testing.T) {
					t.Parallel()

					var got specfront.Name
					typed := assert.ErrorAs[*yaml.TypeError](t, yaml.Unmarshal([]byte(give), &got),
						"the decoder reports the fault as a type error")
					assert.Equal(t, typed.Errors, []string{
						`line 1: "` + give + `" is no name: write lowercase words that open with a letter, joined by hyphens`,
					}, "the entry states the pattern in words")
				})
			}

			t.Run("returns the error of the decoder for a value that is no text", func(t *testing.T) {
				t.Parallel()

				var got specfront.Name
				typed := assert.ErrorAs[*yaml.TypeError](t, yaml.Unmarshal([]byte("[probe]"), &got),
					"the decoder reports the fault as a type error")
				assert.Equal(t, typed.Errors, []string{"line 1: cannot unmarshal !!seq into string"},
					"the entry is the decoder's own")
			})
		})

		t.Run("JSONSchema", func(t *testing.T) {
			t.Parallel()

			t.Run("returns a string of the pattern", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, specfront.Name("").JSONSchema(), map[string]any{
					keywordType: typeString, keywordPattern: namePattern,
				}, "the schema publishes the pattern")
			})
		})
	})
}
