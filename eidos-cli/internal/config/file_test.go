// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/cli/internal/config"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/layout"
)

// fileName is the file name that each case passes to Decode.
const fileName = ".acme.yaml"

// noVersion is the message of an Error for a file without a version.
const noVersion = "the file has no version key: add version: 1"

// fullDocument is a document that sets every key.
const fullDocument = `version: 1
workspace: platform
workers: 4
ignore: ["legacy:"]
memo:
    limit: 512MiB
    dir: /var/cache/acme/memo
plans:
    go-services:
        sources: {lang: golang, packages: ["./svc/..."], module: example.com/platform}
        layout: {policy: centralised, dir: gen, importBase: example.com/platform/gen}
    go-mocks:
        enabled: false
options:
    stubgen: {suffix: _stub}
`

// fullList is a list with two workspaces.
const fullList = `version: 1
workspaces:
    - {root: ./platform, config: ./platform/.acme.yaml}
    - {root: ./tools/gen}
`

// Decode decodes a config file strictly, and reports every fault with the
// line of the fault.
func TestFile(t *testing.T) {
	t.Parallel()

	t.Run("Decode", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a Document with every key of the file", func(t *testing.T) {
			t.Parallel()

			f, err := config.Decode(fileName, []byte(fullDocument))
			assert.NoError(t, err, "the document decodes")
			assert.Nil(t, f.List, "a document is not a list")
			assert.Equal(t, f.Document, &config.Document{
				Version:   config.Current,
				Workspace: "platform",
				Workers:   new(config.Count(4)),
				Ignore:    []directive.Name{"legacy:"},
				Memo:      &config.Memo{Limit: 512 * 1048576, Dir: "/var/cache/acme/memo"},
				Plans: map[string]config.Plan{
					"go-services": {
						Sources: &config.Sources{
							Lang: "golang", Packages: []string{"./svc/..."}, Module: "example.com/platform",
						},
						Layout: &config.Layout{
							Policy:     config.Policy(layout.PolicyCentralised),
							Dir:        "gen",
							ImportBase: "example.com/platform/gen",
						},
					},
					"go-mocks": {Enabled: new(false)},
				},
				Options: map[string]map[string]any{"stubgen": {"suffix": "_stub"}},
			}, "the document has the value of each key")
		})

		t.Run("returns a Document with only a version for a file of one key", func(t *testing.T) {
			t.Parallel()

			f, err := config.Decode(fileName, []byte("version: 1\n"))
			assert.NoError(t, err, "the document decodes")
			assert.Equal(t, f.Document, &config.Document{Version: config.Current}, "every other key is unset")
		})

		t.Run("returns the string yes for a string field", func(t *testing.T) {
			t.Parallel()

			f, err := config.Decode(fileName, []byte("version: 1\nworkspace: yes\n"))
			assert.NoError(t, err, "the document decodes")
			assert.Equal(t, f.Document.Workspace, "yes", "the decoder keeps yes as a string")
		})

		t.Run("returns the boolean false for no in a boolean field", func(t *testing.T) {
			t.Parallel()

			f, err := config.Decode(fileName, []byte("version: 1\nplans:\n    go-mocks: {enabled: no}\n"))
			assert.NoError(t, err, "the document decodes")
			assert.Equal(t, f.Document.Plans["go-mocks"].Enabled, new(false), "the decoder reads no as false")
		})

		t.Run("returns a List for a file with the key workspaces", func(t *testing.T) {
			t.Parallel()

			f, err := config.Decode(fileName, []byte(fullList))
			assert.NoError(t, err, "the list decodes")
			assert.Nil(t, f.Document, "a list is not a document")
			assert.Equal(t, f.List, &config.List{
				Version: config.Current,
				Workspaces: []config.Entry{
					{Root: "./platform", Config: "./platform/.acme.yaml"},
					{Root: "./tools/gen"},
				},
			}, "the list has each entry in order")
		})

		tests := []struct {
			name string
			give string
			want []config.Error
		}{
			{
				name: "returns an Error at line 1 for an empty file",
				give: "",
				want: []config.Error{{File: fileName, Line: 1, Msg: noVersion}},
			},
			{
				name: "returns an Error at the mapping for a file without a version",
				give: "# a comment\nworkspace: platform\n",
				want: []config.Error{{File: fileName, Line: 2, Msg: noVersion}},
			},
			{
				name: "returns an Error for a file that is not a mapping",
				give: "- version\n",
				want: []config.Error{{File: fileName, Line: 1, Msg: "the file is not a mapping of keys to values"}},
			},
			{
				name: "returns an Error at the version for a version other than 1",
				give: "version: 2\n",
				want: []config.Error{{
					File: fileName, Line: 1, Msg: "the file has version 2, and this binary reads only version 1",
				}},
			},
			{
				name: "returns an Error at the version for a version that is not a number",
				give: "workspace: platform\nversion: one\n",
				want: []config.Error{{
					File: fileName, Line: 2, Msg: "the file has version one, and this binary reads only version 1",
				}},
			},
			{
				name: "returns an Error at the line of a syntax error",
				give: "version: 1\n\tworkspace: platform\n",
				want: []config.Error{{File: fileName, Line: 2, Msg: "found a tab character that violates indentation"}},
			},
			{
				name: "returns an Error for each unknown key of a document",
				give: "version: 1\nsourcse: svc\nworkerz: 2\n",
				want: []config.Error{
					{File: fileName, Line: 2, Msg: "field sourcse not found in type config.Document"},
					{File: fileName, Line: 3, Msg: "field workerz not found in type config.Document"},
				},
			},
			{
				name: "returns an Error at the line of a value of the wrong type",
				give: "version: 1\nworkspace: [platform]\n",
				want: []config.Error{{File: fileName, Line: 2, Msg: "cannot unmarshal !!seq into string"}},
			},
			{
				name: "returns an Error for a key of a document in a list",
				give: "version: 1\nworkers: 2\nworkspaces: [{root: platform}]\n",
				want: []config.Error{{File: fileName, Line: 2, Msg: "field workers not found in type config.List"}},
			},
			{
				name: "returns an Error for an unknown key of an entry",
				give: "version: 1\nworkspaces: [{root: platform, size: 2}]\n",
				want: []config.Error{{File: fileName, Line: 2, Msg: "field size not found in type config.Entry"}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := config.Decode(fileName, []byte(tt.give))
				assert.Equal(t, faultsOf(t, err), tt.want, "Decode returns the faults of the file")
			})
		}
	})

	t.Run("Error", func(t *testing.T) {
		t.Parallel()

		t.Run("Error", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the file, the line and the message", func(t *testing.T) {
				t.Parallel()

				err := &config.Error{File: fileName, Line: 3, Msg: "the list has no workspaces"}
				assert.Equal(t, err.Error(), ".acme.yaml:3: the list has no workspaces", "the text has the line")
			})

			t.Run("returns the file and the message for an Error without a line", func(t *testing.T) {
				t.Parallel()

				err := &config.Error{File: fileName, Msg: "control characters are not allowed"}
				assert.Equal(t, err.Error(), ".acme.yaml: control characters are not allowed",
					"the text has no line")
			})
		})
	})
}

// faultsOf returns the Errors that err contains, in order. err is one
// *config.Error, or an error of [errors.Join] that wraps *config.Error
// values alone.
func faultsOf(tb testing.TB, err error) []config.Error {
	tb.Helper()

	errs := []error{err}
	if joined, ok := errors.AsType[interface {
		error
		Unwrap() []error
	}](err); ok {
		errs = joined.Unwrap()
	}
	out := make([]config.Error, 0, len(errs))
	for _, e := range errs {
		out = append(out, *assert.ErrorAs[*config.Error](tb, e, "each fault is a *config.Error"))
	}
	return out
}
