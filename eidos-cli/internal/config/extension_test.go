// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/cli/internal/config"
)

// referenceName is the name of the extension of the fixture.
const referenceName = "reference"

// extendedFile has the extension of the fixture, beside another extension
// and the keys of the kernel.
const extendedFile = `version: 1
options:
    stubgen: {suffix: _stub}
extensions:
    other: {anything: [1, 2]}
    reference:
        name: canary
        canary: {rounds: 4}
`

// canary is a section of the settings of the fixture.
type canary struct {
	Rounds int `yaml:"rounds"`
}

// settings are the settings of a consumer under the extension of the
// fixture.
type settings struct {
	Name   string `yaml:"name"`
	Canary canary `yaml:"canary"`
}

// Extension decodes one extension of a config file strictly, and reports
// every fault with the line of the fault in the file.
func TestExtension(t *testing.T) {
	t.Parallel()

	t.Run("Extension", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a file with the extension", func(t *testing.T) {
			t.Parallel()

			var got settings
			found, err := config.Extension(fileName, []byte(extendedFile), referenceName, &got)
			assert.NoError(t, err, "the extension decodes")
			assert.True(t, found, "the file has the extension")
		})

		t.Run("decodes the value of the extension into v", func(t *testing.T) {
			t.Parallel()

			var got settings
			_, err := config.Extension(fileName, []byte(extendedFile), referenceName, &got)
			assert.NoError(t, err, "the extension decodes")
			want := settings{Name: "canary", Canary: canary{Rounds: 4}}
			assert.Equal(t, got, want, "v has the value of the extension")
		})

		t.Run("keeps a value of v that the extension does not set", func(t *testing.T) {
			t.Parallel()

			got := settings{Canary: canary{Rounds: 9}}
			data := "version: 1\nextensions:\n    reference: {name: canary}\n"
			_, err := config.Extension(fileName, []byte(data), referenceName, &got)
			assert.NoError(t, err, "the extension decodes")
			assert.Equal(t, got, settings{Name: "canary", Canary: canary{Rounds: 9}}, "v keeps its rounds")
		})

		t.Run("keeps v for an extension of null", func(t *testing.T) {
			t.Parallel()

			got := settings{Name: "kept"}
			data := "version: 1\nextensions:\n    reference: null\n"
			found, err := config.Extension(fileName, []byte(data), referenceName, &got)
			assert.NoError(t, err, "the extension decodes")
			assert.True(t, found, "the file has the extension")
			assert.Equal(t, got, settings{Name: "kept"}, "v keeps its value")
		})

		tests := []struct {
			name string
			give string
		}{
			{name: "reports false for a file without extensions", give: "version: 1\nworkspace: platform\n"},
			{name: "reports false for a file without the extension", give: "version: 1\nextensions: {other: 1}\n"},
			{name: "reports false for extensions of null", give: "version: 1\nextensions: null\n"},
			{name: "reports false for an empty file", give: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := settings{Name: "kept"}
				found, err := config.Extension(fileName, []byte(tt.give), referenceName, &got)
				assert.NoError(t, err, "a file without the extension is no fault")
				assert.False(t, found, "the file has no such extension")
				assert.Equal(t, got, settings{Name: "kept"}, "v keeps its value")
			})
		}

		faulty := []struct {
			name string
			give string
			want []config.Error
		}{
			{
				name: "returns an Error at the line of a key that v does not declare",
				give: "version: 1\nextensions:\n    reference:\n        canary: {roundz: 4}\n",
				want: []config.Error{
					{File: fileName, Line: 4, Msg: "field roundz not found in type config_test.canary"},
				},
			},
			{
				name: "returns an Error at the line of a value of the wrong type",
				give: "version: 1\nextensions:\n    reference:\n        canary: {rounds: four}\n",
				want: []config.Error{{File: fileName, Line: 4, Msg: "cannot unmarshal !!str `four` into int"}},
			},
			{
				name: "returns an Error at the line of a syntax error",
				give: "version: 1\n\textensions: {}\n",
				want: []config.Error{{File: fileName, Line: 2, Msg: "found a tab character that violates indentation"}},
			},
		}
		for _, tt := range faulty {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var got settings
				_, err := config.Extension(fileName, []byte(tt.give), referenceName, &got)
				assert.Equal(t, faultsOf(t, err), tt.want, "Extension returns the faults of the extension")
			})
		}

		names := []struct {
			name string
			give string
		}{
			{name: "panics for an empty name", give: ""},
			{name: "panics for the name -", give: "-"},
			{name: "panics for a name with a comma", give: "reference,inline"},
		}
		for _, tt := range names {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var got settings
				assert.Panics(t, func() {
					_, _ = config.Extension(fileName, []byte(extendedFile), tt.give, &got)
				}, "the name is a defect of the binary")
			})
		}

		t.Run("panics for a v that is not a pointer", func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() {
				_, _ = config.Extension(fileName, []byte(extendedFile), referenceName, settings{})
			}, "v is a defect of the binary")
		})

		t.Run("panics for a nil pointer", func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() {
				_, _ = config.Extension(fileName, []byte(extendedFile), referenceName, (*settings)(nil))
			}, "v is a defect of the binary")
		})
	})
}
