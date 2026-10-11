// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative_test

import (
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/declarative"
)

// directiveManifest is a manifest with the directive audit of one param at
// line 5, a struct rule with the directive, and the param level at line 8.
const directiveManifest = `format: 1
name: acme-audit
version: 0.3.0
directives:
    - name: audit
      doc: Records each call.
      params:
          - key: level
            type: string
            doc: The level of each record.
outputs:
    - per: source
      word: audit
rules:
    - on: struct
      directive: audit
      emit: mirror
`

// A directive of the manifest lowers to a schema of the plugin, and the
// loader reports each schema that the directive registry refuses at the
// directive's line.
func TestDirectives(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a generator with the schema of each directive", func(t *testing.T) {
			t.Parallel()

			provider, is := auditPlugin(t).(plugin.DirectiveProvider)
			assert.True(t, is, "the generator declares its directives")
			assert.Equal(t, provider.Directives(), []directive.Schema{{
				Plugin: pluginName,
				Name:   "audit",
				Params: []directive.ParamSpec{
					{
						Key: levelKey, Type: directive.TypeString, Required: true,
						Choices: []string{"debug", infoLevel, warnLevel}, Doc: "The level of each record.",
					},
					{
						Key: sinkKey, Type: directive.TypeReference, Resolution: directive.ResolveCallableInScope,
						Required: true, Doc: "The function that receives each record.",
					},
					{
						Key: fieldsKey, Type: directive.TypeList, ListOf: directive.TypeInt,
						Doc: "The positions of the arguments that a record contains.",
					},
					{Key: strictKey, Type: directive.TypeBool, Doc: "When true, a record that fails stops the call."},
				},
				Kinds: []symbol.Kind{symbol.KindFunction, symbol.KindInterface},
				Doc:   "Generates code that records each call of the declaration.",
			}}, "the schema has the fields that the manifest sets and the kinds of the rules of the directive")
		})

		t.Run(
			"returns a schema whose instance on a kind that no rule handles reports MisplacedDirective",
			func(t *testing.T) {
				t.Parallel()

				provider, is := auditPlugin(t).(plugin.DirectiveProvider)
				assert.True(t, is, "the generator declares its directives")
				r := directive.NewRegistry()
				assert.Total(t, r.Register, directive.Kernel(), "the kernel schemas register first")
				assert.Total(t, r.Register, provider.Directives(), "the schema of the manifest registers")
				assert.Empty(t, r.Seal(), "the registry seals")
				raw, err := directive.Parse(string(auditDirective) + " level=info sink=" + recordSink)
				assert.NoError(t, err, "the instance parses")
				raw.Pos = position.Pos{File: ledgerFile, Line: 14, Col: 1}
				sink := diag.NewSink()
				got := directive.Validate(entryID, []directive.Raw{raw}, r, meta.NewRegistry(), nil, sink)
				assert.Empty(t, got, "the instance on the struct Entry is not returned")
				findings := slices.Collect(sink.All())
				assert.Length(t, findings, 1, "the instance reports one finding")
				assert.Equal(t, findings[0].Code, directive.MisplacedDirective,
					"the finding is under MisplacedDirective")
			},
		)

		tests := []struct {
			name string
			old  string
			new  string
			want declarative.Error
		}{
			{
				name: "returns an Error at the directive for a param without a doc",
				old:  "            doc: The level of each record.\n",
				new:  "",
				want: declarative.Error{
					File: manifestFile,
					Line: 5,
					Msg:  `directive: audit param "level" states no semantics`,
				},
			},
			{
				name: "returns an Error at the directive for a reference without a resolution",
				old:  "type: string",
				new:  "type: reference",
				want: declarative.Error{
					File: manifestFile,
					Line: 5,
					Msg:  `directive: audit param "level" references and states no resolution kind`,
				},
			},
			{
				name: "returns an Error at the param for an element type of a param that is not a list",
				old:  "type: string\n",
				new:  "type: string\n            of: string\n",
				want: declarative.Error{
					File: manifestFile, Line: 8, Msg: "the param level has an element type, and only a list has one",
				},
			},
			{
				name: "returns an Error at the param for a resolution of a param that is not a reference",
				old:  "type: string\n",
				new:  "type: string\n            resolve: package-var\n",
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  "the param level resolves, and only a reference or a list of references resolves",
				},
			},
			{
				name: "returns an Error at the type for a type that the manifest cannot spell",
				old:  "type: string",
				new:  "type: number",
				want: declarative.Error{
					File: manifestFile,
					Line: 9,
					Msg:  `"number" is not a param type: use string, int, bool, list or reference`,
				},
			},
			{
				name: "returns an Error at the element type for a list of lists",
				old:  "type: string\n",
				new:  "type: list\n            of: list\n",
				want: declarative.Error{
					File: manifestFile,
					Line: 10,
					Msg:  `"list" is not an element type: use string, int, bool or reference`,
				},
			},
			{
				name: "returns an Error at the resolution for a resolution of metadata keys",
				old:  "type: string\n",
				new:  "type: reference\n            resolve: metadata-key\n",
				want: declarative.Error{
					File: manifestFile,
					Line: 10,
					Msg: `"metadata-key" is not a resolution: use callable-in-scope, package-var, value-field, ` +
						"host-param, member-on-handle or type-in-scope",
				},
			},
			{
				name: "returns an Error at the directive for a directive that no rule has",
				old:  "      directive: audit\n",
				new:  "",
				want: declarative.Error{File: manifestFile, Line: 5, Msg: "no rule has the directive audit"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				manifest := strings.Replace(directiveManifest, tt.old, tt.new, 1)
				assert.NotEqual(t, manifest, directiveManifest, "the case changes the manifest")
				got := loadFaults(t, files.Tree{manifestFile: files.Text(manifest)})
				assert.Equal(t, got, []declarative.Error{tt.want}, "Load returns the fault of the directive")
			})
		}

		t.Run("returns an Error at the directive for the name of a kernel directive", func(t *testing.T) {
			t.Parallel()

			manifest := strings.ReplaceAll(directiveManifest, ": audit\n", ": skip\n")
			assert.Equal(t, loadFaults(t, files.Tree{manifestFile: files.Text(manifest)}), []declarative.Error{{
				File: manifestFile, Line: 5, Msg: "directive: skip is a kernel name, which no plugin claims",
			}}, "the registry refuses the schema")
		})

		t.Run("returns an Error at the second directive of a name", func(t *testing.T) {
			t.Parallel()

			manifest := strings.Replace(directiveManifest, "outputs:",
				"    - name: audit\n      doc: Records each call again.\noutputs:", 1)
			assert.Equal(t, loadFaults(t, files.Tree{manifestFile: files.Text(manifest)}), []declarative.Error{
				{
					File: manifestFile,
					Line: 11,
					Msg:  `directive: acme-audit:audit is registered twice: "Records each call." and "Records each call again."`,
				},
			}, "the registry refuses the second schema")
		})

		t.Run("returns an Error at the param for each directive whose params an alias repeats", func(t *testing.T) {
			t.Parallel()

			manifest := "format: 1\nname: acme-audit\nversion: 0.3.0\ndirectives:\n" +
				"    - name: audit\n      doc: Records each call.\n      params: &params\n" +
				"          - key: level\n            type: string\n            of: string\n            doc: The level of each record.\n" +
				"    - name: trace\n      doc: Traces each call.\n      params: *params\n" +
				"outputs:\n    - per: source\n      word: audit\n" +
				"rules:\n    - on: struct\n      directive: audit\n      emit: mirror\n" +
				"    - on: interface\n      directive: trace\n      emit: mirror\n"
			fault := declarative.Error{
				File: manifestFile,
				Line: 8,
				Msg:  "the param level has an element type, and only a list has one",
			}
			assert.Equal(
				t,
				loadFaults(t, files.Tree{manifestFile: files.Text(manifest)}),
				[]declarative.Error{fault, fault},
				"the fault of the param is at the anchor for both directives",
			)
		})

		t.Run("returns an Error at the param of a directive that an alias repeats", func(t *testing.T) {
			t.Parallel()

			manifest := "format: 1\nname: acme-audit\nversion: 0.3.0\ndirectives:\n" +
				"    - &audit\n      name: audit\n      doc: Records each call.\n      params:\n" +
				"          - key: level\n            type: string\n            of: string\n            doc: The level of each record.\n" +
				"    - *audit\n" +
				"outputs:\n    - per: source\n      word: audit\n" +
				"rules:\n    - on: struct\n      directive: audit\n      emit: mirror\n"
			fault := declarative.Error{
				File: manifestFile,
				Line: 9,
				Msg:  "the param level has an element type, and only a list has one",
			}
			assert.Equal(t, loadFaults(t, files.Tree{manifestFile: files.Text(manifest)}), []declarative.Error{
				fault,
				fault,
				{
					File: manifestFile,
					Line: 13,
					Msg:  `directive: acme-audit:audit is registered twice: "Records each call." and "Records each call."`,
				},
			}, "the fault of the param is at the anchor for the alias")
		})

		t.Run("returns no Error of the registry under a name that is not an identifier", func(t *testing.T) {
			t.Parallel()

			manifest := strings.Replace(directiveManifest, "name: acme-audit", "name: acme audit", 1)
			assert.Equal(t, loadFaults(t, files.Tree{manifestFile: files.Text(manifest)}), []declarative.Error{{
				File: manifestFile,
				Line: 2,
				Msg:  `"acme audit" is not a name: use a letter followed by letters, digits, hyphens and underscores`,
			}}, "the check of the name reports the fault once")
		})
	})
}
