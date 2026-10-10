// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative_test

import (
	"maps"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/declarative"
)

// ruleManifest is a manifest with a rule at line 8 that mirrors each
// struct into the primary family.
const ruleManifest = `format: 1
name: acme-audit
version: 0.3.0
outputs:
    - per: source
      word: audit
rules:
    - on: struct
      emit: mirror
`

// A rule of the manifest lowers to a rule of the authoring surface, and the
// loader reports each rule that it cannot lower at the rule's line.
func TestRules(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a generator with a gate record for each rule", func(t *testing.T) {
			t.Parallel()

			subscribed, is := auditPlugin(t).(plugin.Subscribed)
			assert.True(t, is, "the generator declares its gates")
			assert.Equal(t, subscribed.Subscriptions(), []plugin.Subscription{
				{Rule: 0, Kind: symbol.KindInterface, Phase: plugin.PhaseGenerate},
				{Rule: 1, Kind: symbol.KindFunction, Phase: plugin.PhaseGenerate},
				{Rule: 2, Kind: symbol.KindStruct, Phase: plugin.PhaseGenerate},
				{Rule: 3, Kind: symbol.KindInterface, Directive: auditDirective, Phase: plugin.PhaseGenerate},
				{Rule: 4, Kind: symbol.KindFunction, Directive: auditDirective, Phase: plugin.PhaseGenerate},
				{Rule: 5, Kind: symbol.KindFunction, Directive: auditDirective, Phase: plugin.PhaseGenerate},
			}, "the bare rules come first, then the rules of the directive")
		})

		tests := []struct {
			name string
			old  string
			new  string
			tree files.Tree
			want declarative.Error
		}{
			{
				name: "returns an Error at the rule for a rule without a kind",
				old:  "    - on: struct\n      emit: mirror\n",
				new:  "    - emit: mirror\n",
				want: declarative.Error{
					File: manifestFile, Line: 8, Msg: "the rule has no on: use function, interface or struct",
				},
			},
			{
				name: "returns an Error at the rule for a rule without an emission",
				old:  "      emit: mirror\n",
				new:  "",
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  "the rule has no emit: use mirror or implementation",
				},
			},
			{
				name: "returns an Error at the rule for an implementation of a struct",
				old:  "emit: mirror",
				new:  "emit: implementation",
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  "a rule on struct cannot emit implementation",
				},
			},
			{
				name: "returns an Error at the rule for a directive that the manifest does not declare",
				old:  "      emit: mirror\n",
				new:  "      directive: trace\n      emit: mirror\n",
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  "the rule has the directive trace, and the manifest does not declare it",
				},
			},
			{
				name: "returns an Error at the rule for a family that the manifest does not declare",
				old:  "      emit: mirror\n",
				new:  "      output: test\n      emit: mirror\n",
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  `the rule emits into the family "test", which the manifest does not declare`,
				},
			},
			{
				name: "returns an Error at the second rule of a kind and a family",
				old:  "      emit: mirror\n",
				new:  "      emit: mirror\n    - on: struct\n      emit: mirror\n",
				want: declarative.Error{
					File: manifestFile,
					Line: 10,
					Msg:  `the rule emits into the family "" for each struct, as the rule at line 8 does`,
				},
			},
			{
				name: "returns an Error at the rule for a template of a declaration without a body",
				old:  "      emit: mirror\n",
				new:  "      emit: mirror\n      template: function.tmpl\n",
				want: declarative.Error{
					File: manifestFile, Line: 8, Msg: "the rule has a template, and a mirror of a struct has no body",
				},
			},
			{
				name: "returns an Error at the rule for no template of a declaration with a body",
				old:  "on: struct",
				new:  "on: function",
				want: declarative.Error{
					File: manifestFile, Line: 8, Msg: "the rule emits bodies, and it has no template to render them",
				},
			},
			{
				name: "returns an Error at the rule for a template that is not a path inside a tree",
				old:  "    - on: struct\n      emit: mirror\n",
				new:  "    - on: function\n      emit: mirror\n      template: ../function.tmpl\n",
				tree: files.Tree{goFunction: files.Text(functionBody)},
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  `the rule's template "../function.tmpl" is not a slash-separated path inside a tree`,
				},
			},
			{
				name: "returns an Error at the rule for a template without a tree",
				old:  "    - on: struct\n      emit: mirror\n",
				new:  "    - on: function\n      emit: mirror\n      template: function.tmpl\n",
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  `the plugin has no template tree for the rule's template "function.tmpl"`,
				},
			},
			{
				name: "returns an Error at the rule for each tree without the template",
				old:  "    - on: struct\n      emit: mirror\n",
				new:  "    - on: function\n      emit: mirror\n      template: function.tmpl\n",
				tree: files.Tree{goFunction: files.Text(functionBody), "templates/typescript": files.Dir()},
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  `the tree typescript does not contain the rule's template "function.tmpl"`,
				},
			},
			{
				name: "returns an Error at the kind for a kind that the manifest cannot spell",
				old:  "on: struct",
				new:  "on: method",
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  `"method" is not a subject kind: use function, interface or struct`,
				},
			},
			{
				name: "returns an Error at the emission for an emission that the manifest cannot spell",
				old:  "emit: mirror",
				new:  "emit: double",
				want: declarative.Error{
					File: manifestFile, Line: 9, Msg: `"double" is not an emission: use mirror or implementation`,
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				manifest := strings.Replace(ruleManifest, tt.old, tt.new, 1)
				assert.NotEqual(t, manifest, ruleManifest, "the case changes the manifest")
				tree := files.Tree{manifestFile: files.Text(manifest)}
				maps.Copy(tree, tt.tree)
				assert.Equal(t, loadFaults(t, tree), []declarative.Error{tt.want}, "Load returns the fault of the rule")
			})
		}

		t.Run(
			"returns no Error for a rule with bodies and no template in a family that claims its files",
			func(t *testing.T) {
				t.Parallel()

				manifest := strings.Replace(
					ruleManifest,
					"      word: audit\n",
					"      word: audit\n      template: test.tmpl\n",
					1,
				)
				manifest = strings.Replace(manifest, "on: struct", "on: function", 1)
				assert.NotEqual(t, manifest, ruleManifest, "the case changes the manifest")
				_, err := declarative.Load(files.Workspace(t, files.Tree{
					manifestFile: files.Text(manifest), goTest: files.Text(testBody),
				}))
				assert.NoError(t, err, "the file template writes each record whole")
			},
		)
	})
}
