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
	"go.dokimi.dev/eidos/declarative"
)

// outputManifest is a manifest with the primary family at line 5 and a
// rule that emits into it.
const outputManifest = `format: 1
name: acme-audit
version: 0.3.0
outputs:
    - per: source
      word: audit
rules:
    - on: struct
      emit: mirror
`

// A family of the manifest lowers to a family of the plugin, and the loader
// reports each declaration that the authoring surface refuses at the
// family's line.
func TestOutputs(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a generator with each family of the manifest", func(t *testing.T) {
			t.Parallel()

			provider, is := auditPlugin(t).(plugin.OutputProvider)
			assert.True(t, is, "the generator declares its families")
			assert.Equal(t, provider.Outputs(), []plugin.Output{
				{Per: plugin.PerSource, Word: "audit"},
				{Tag: contractTag, Per: plugin.PerPackage, Word: contractTag},
				{Tag: shapeTag, Per: plugin.PerPlan, Word: shapeTag},
				{Tag: testTag, Per: plugin.PerSource, Word: "audit", Template: testTemplate},
			}, "each family has its tag, its cardinality, its word and its file template")
		})

		tests := []struct {
			name string
			old  string
			new  string
			tree files.Tree
			want declarative.Error
		}{
			{
				name: "returns an Error at the family for a family without a cardinality",
				old:  "    - per: source\n      word: audit\n",
				new:  "    - word: audit\n",
				want: declarative.Error{
					File: manifestFile,
					Line: 5,
					Msg:  "the family has no per: use source, package or plan",
				},
			},
			{
				name: "returns an Error at the family for a family without a word",
				old:  "      word: audit\n",
				new:  "",
				want: declarative.Error{File: manifestFile, Line: 5, Msg: "the family has no word"},
			},
			{
				name: "returns an Error at the cardinality for a cardinality that the manifest cannot spell",
				old:  "per: source",
				new:  "per: file",
				want: declarative.Error{
					File: manifestFile, Line: 5, Msg: `"file" is not a cardinality: use source, package or plan`,
				},
			},
			{
				name: "returns an Error at the second family of a tag",
				old:  "rules:",
				new:  "    - per: package\n      word: contract\nrules:",
				want: declarative.Error{
					File: manifestFile,
					Line: 7,
					Msg:  `the family has the tag "" of the family at line 5`,
				},
			},
			{
				name: "returns an Error at the family for a file template that is not a path inside a tree",
				old:  "      word: audit\n",
				new:  "      word: audit\n      template: ../test.tmpl\n",
				tree: files.Tree{goTest: files.Text(testBody)},
				want: declarative.Error{
					File: manifestFile,
					Line: 5,
					Msg:  `the family's file template "../test.tmpl" is not a slash-separated path inside a tree`,
				},
			},
			{
				name: "returns an Error at the family for a file template without a tree",
				old:  "      word: audit\n",
				new:  "      word: audit\n      template: test.tmpl\n",
				want: declarative.Error{
					File: manifestFile,
					Line: 5,
					Msg:  `the plugin has no template tree for the family's file template "test.tmpl"`,
				},
			},
			{
				name: "returns an Error at the family for each tree without the file template",
				old:  "      word: audit\n",
				new:  "      word: audit\n      template: test.tmpl\n",
				tree: files.Tree{goTest: files.Text(testBody), "templates/typescript": files.Dir()},
				want: declarative.Error{
					File: manifestFile,
					Line: 5,
					Msg:  `the tree typescript does not contain the family's file template "test.tmpl"`,
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				manifest := strings.Replace(outputManifest, tt.old, tt.new, 1)
				assert.NotEqual(t, manifest, outputManifest, "the case changes the manifest")
				tree := files.Tree{manifestFile: files.Text(manifest)}
				maps.Copy(tree, tt.tree)
				assert.Equal(
					t,
					loadFaults(t, tree),
					[]declarative.Error{tt.want},
					"Load returns the fault of the family",
				)
			})
		}
	})
}
