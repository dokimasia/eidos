// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/declarative"
)

// optionManifest is a manifest with the option redact at line 8.
const optionManifest = `format: 1
name: acme-audit
version: 0.3.0
outputs:
    - per: source
      word: audit
options:
    - key: redact
      type: bool
      default: true
      doc: When true, a record leaves out the arguments of the call.
rules:
    - on: struct
      emit: mirror
`

// typedOptions are options of each type, with and without a default.
const typedOptions = `options:
    - key: level
      type: string
      default: info
      doc: The level of each record.
    - key: limit
      type: int
      default: 3
      doc: The number of records of one call.
    - key: tags
      type: list
      default: [audit, ledger]
      doc: The tags of each record.
    - key: strict
      type: bool
      doc: When true, a record that fails stops the call.
    - key: sink
      type: string
      doc: The function that receives each record.
    - key: skip
      type: list
      doc: The methods without a record.
rules:`

// The options of the manifest lower to one options struct, which the
// workspace validates, populates and folds into the fingerprint as it does
// the options of a typed plugin.
func TestOptions(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("returns options with the default of each option", func(t *testing.T) {
			t.Parallel()

			manifest := strings.Replace(optionManifest, "options:\n    - key: redact\n      type: bool\n"+
				"      default: true\n      doc: When true, a record leaves out the arguments of the call.\nrules:",
				typedOptions, 1)
			encoded, err := plugin.EncodeOptions(loadedPlugin(t, manifest))
			assert.NoError(t, err, "the options encode")
			assert.Equal(t, string(encoded),
				`{"level":"info","limit":3,"tags":["audit","ledger"],"strict":false,"sink":"","skip":null}`,
				"each option has its default, or the zero value of its type")
		})

		t.Run("returns options that meet the tag contract", func(t *testing.T) {
			t.Parallel()

			assert.Empty(
				t,
				plugin.ValidateOptions(auditPlugin(t)),
				"the options struct has an opt and a doc tag on each field",
			)
		})

		t.Run("returns options that the config of a workspace populates", func(t *testing.T) {
			t.Parallel()

			g := auditPlugin(t)
			configured(t, g)
			encoded, err := plugin.EncodeOptions(g)
			assert.NoError(t, err, "the options encode")
			assert.Equal(t, string(encoded), `{"redact":false}`, "the option has the value of the config")
		})

		t.Run("passes the options that the config populated to a template", func(t *testing.T) {
			t.Parallel()

			g := auditPlugin(t)
			configured(t, g)
			impl := only[*emit.Struct](t, generated(t, g, ledgerFixture(t)), "")
			post := impl.Methods.Items()[0]
			assert.Equal(t, executed(t, "{{.Data.Options.redact}}", post, post.Body.Ref.Data), "false",
				"the template reads the value of the config")
		})

		t.Run("passes no options to a template of a plugin without options", func(t *testing.T) {
			t.Parallel()

			units := recordUnits(t)
			assert.Length(t, units, 1, "the family has one unit for the one source file")
			assert.Equal(
				t,
				executed(t, "{{if .Data.Options}}set{{else}}unset{{end}}", nil, recordData(units[0], ledgerID)),
				"unset",
				"the data of a plugin without options has no options",
			)
		})

		tests := []struct {
			name string
			old  string
			new  string
			want declarative.Error
		}{
			{
				name: "returns an Error at the option for a key that starts with an upper-case letter",
				old:  "key: redact",
				new:  "key: Redact",
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  `"Redact" is not an option key: use a lower-case letter followed by letters and digits`,
				},
			},
			{
				name: "returns an Error at the option for a key with a hyphen",
				old:  "key: redact",
				new:  "key: re-dact",
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  `"re-dact" is not an option key: use a lower-case letter followed by letters and digits`,
				},
			},
			{
				name: "returns an Error at the second option of a key",
				old:  "rules:",
				new:  "    - key: redact\n      type: string\n      doc: Another meaning.\nrules:",
				want: declarative.Error{
					File: manifestFile,
					Line: 12,
					Msg:  "the option redact is declared at line 8 too",
				},
			},
			{
				name: "returns an Error at the option for an option without a type",
				old:  "      type: bool\n      default: true\n",
				new:  "",
				want: declarative.Error{
					File: manifestFile, Line: 8, Msg: "the option redact has no type: use string, int, bool or list",
				},
			},
			{
				name: "returns an Error at the option for an option without a doc",
				old:  "      doc: When true, a record leaves out the arguments of the call.\n",
				new:  "",
				want: declarative.Error{File: manifestFile, Line: 8, Msg: "the option redact has no doc"},
			},
			{
				name: "returns an Error at the option for a boolean default that YAML reads as a string",
				old:  "default: true",
				new:  "default: yes",
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  "the default of the option redact is not a value of the type bool",
				},
			},
			{
				name: "returns an Error at the option for a string default that YAML reads as an integer",
				old:  "type: bool\n      default: true",
				new:  "type: string\n      default: 3",
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  "the default of the option redact is not a value of the type string",
				},
			},
			{
				name: "returns an Error at the option for an integer default that int64 does not hold",
				old:  "type: bool\n      default: true",
				new:  "type: int\n      default: 99999999999999999999",
				want: declarative.Error{
					File: manifestFile, Line: 8, Msg: "the default of the option redact is not a value of the type int",
				},
			},
			{
				name: "returns an Error at the option for a list default with an element that is not a string",
				old:  "type: bool\n      default: true",
				new:  "type: list\n      default: [audit, 3]",
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  "the default of the option redact is not a value of the type list",
				},
			},
			{
				name: "returns an Error at the type for a type that the manifest cannot spell",
				old:  "type: bool",
				new:  "type: float",
				want: declarative.Error{
					File: manifestFile, Line: 9, Msg: `"float" is not an option type: use string, int, bool or list`,
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				manifest := strings.Replace(optionManifest, tt.old, tt.new, 1)
				assert.NotEqual(t, manifest, optionManifest, "the case changes the manifest")
				got := loadFaults(t, files.Tree{manifestFile: files.Text(manifest)})
				assert.Equal(t, got, []declarative.Error{tt.want}, "Load returns the fault of the option")
			})
		}
	})
}

// configured builds a composition of the generator g with the config that
// sets the option redact to false, so the workspace populates the options
// of g, and fails the test where Build returns an error.
func configured(tb testing.TB, g plugin.Generator) {
	tb.Helper()

	_, err := workspace.New().
		Brand(auditBrand).
		Targets(goTarget).
		Plans(workspace.Plan{
			Name:       auditPlan,
			Generators: []plugin.Generator{g},
			Backend:    printer{},
			Layout:     layout.Config{Dir: auditDir},
		}).
		Config(workspace.Config{Options: map[string]map[string]any{pluginName: {"redact": false}}}).
		Build()
	assert.NoError(tb, err, "the composition builds")
}

// loadedPlugin loads a plugin directory with the manifest alone, and fails
// the test where Load returns an error.
func loadedPlugin(tb testing.TB, manifest string) plugin.Generator {
	tb.Helper()

	g, err := declarative.Load(files.Workspace(tb, files.Tree{manifestFile: files.Text(manifest)}))
	assert.NoError(tb, err, "the plugin directory loads")
	return g
}
