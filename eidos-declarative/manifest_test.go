// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/declarative"
)

// noFormat is the message of an Error for a manifest without a format.
const noFormat = "the manifest has no format key: add format: 1"

// structRule is the tail of a manifest with a primary family and one rule
// that mirrors each struct: the smallest family and rule that pass their
// checks.
const structRule = "outputs:\n    - per: source\n      word: audit\nrules:\n    - on: struct\n      emit: mirror\n"

// The decoder reads the format first and the manifest with known fields,
// and the checks of the manifest as a whole report at a key or at the
// manifest's mapping.
func TestManifest(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want []declarative.Error
		}{
			{
				name: "returns an Error at line 1 for an empty manifest",
				give: "",
				want: []declarative.Error{{File: manifestFile, Line: 1, Msg: noFormat}},
			},
			{
				name: "returns an Error at the mapping for a manifest without a format",
				give: "# a comment\nname: acme-audit\n",
				want: []declarative.Error{{File: manifestFile, Line: 2, Msg: noFormat}},
			},
			{
				name: "returns an Error for a manifest that is not a mapping",
				give: "- format\n",
				want: []declarative.Error{
					{File: manifestFile, Line: 1, Msg: "the manifest is not a mapping of keys to values"},
				},
			},
			{
				name: "returns an Error at the format for a format other than 1",
				give: "format: 2\nunknown: key\n",
				want: []declarative.Error{{
					File: manifestFile, Line: 1, Msg: "the manifest has format 2, and this binary reads only format 1",
				}},
			},
			{
				name: "returns an Error at the format for a format that is not a number",
				give: "name: acme-audit\nformat: one\n",
				want: []declarative.Error{
					{
						File: manifestFile,
						Line: 2,
						Msg:  "the manifest has format one, and this binary reads only format 1",
					},
				},
			},
			{
				name: "returns an Error at the line of a syntax error",
				give: "format: 1\n\tname: acme-audit\n",
				want: []declarative.Error{
					{File: manifestFile, Line: 2, Msg: "found a tab character that violates indentation"},
				},
			},
			{
				name: "returns an Error for each unknown key",
				give: "format: 1\ntitle: acme-audit\nrelease: 0.3.0\n",
				want: []declarative.Error{
					{File: manifestFile, Line: 2, Msg: "field title not found in type declarative.manifest"},
					{File: manifestFile, Line: 3, Msg: "field release not found in type declarative.manifest"},
				},
			},
			{
				name: "returns an Error at the line of a value of the wrong type",
				give: "format: 1\nname: [acme-audit]\n",
				want: []declarative.Error{{File: manifestFile, Line: 2, Msg: "cannot unmarshal !!seq into string"}},
			},
			{
				name: "returns an Error at the line of a key written twice",
				give: "format: 1\nname: acme-audit\nname: acme\n",
				want: []declarative.Error{
					{File: manifestFile, Line: 3, Msg: `mapping key "name" already defined at line 2`},
				},
			},
			{
				name: "returns an Error at the contract for a contract version that the kernel does not have",
				give: "format: 1\ncontract: 2\nname: acme-audit\nversion: 0.3.0\n" + structRule,
				want: []declarative.Error{{
					File: manifestFile, Line: 2,
					Msg: "the plugin acme-audit is written against contract version 2, " +
						"and the kernel has contract version 1",
				}},
			},
			{
				name: "returns an Error at the mapping for a manifest without a name",
				give: "format: 1\nversion: 0.3.0\n" + structRule,
				want: []declarative.Error{{File: manifestFile, Line: 1, Msg: "the manifest has no name"}},
			},
			{
				name: "returns an Error at the name for a name that is not an identifier",
				give: "format: 1\nname: acme audit\nversion: 0.3.0\n" + structRule,
				want: []declarative.Error{
					{
						File: manifestFile,
						Line: 2,
						Msg:  `"acme audit" is not a name: use a letter followed by letters, digits, hyphens and underscores`,
					},
				},
			},
			{
				name: "returns an Error at the mapping for a manifest without a version",
				give: "format: 1\nname: acme-audit\n" + structRule,
				want: []declarative.Error{{File: manifestFile, Line: 1, Msg: "the manifest has no version"}},
			},
			{
				name: "returns an Error at the version for an empty version",
				give: "format: 1\nname: acme-audit\nversion: \"\"\n" + structRule,
				want: []declarative.Error{{File: manifestFile, Line: 3, Msg: "the manifest has no version"}},
			},
			{
				name: "returns an Error at the list for an empty label among the provided capabilities",
				give: "format: 1\nname: acme-audit\nversion: 0.3.0\nprovides: [audit, \"\"]\n" + structRule,
				want: []declarative.Error{
					{File: manifestFile, Line: 4, Msg: "the list provides has an empty capability label"},
				},
			},
			{
				name: "returns an Error at the list for an empty label among the required capabilities",
				give: "format: 1\nname: acme-audit\nversion: 0.3.0\nrequires: [\"\"]\n" + structRule,
				want: []declarative.Error{
					{File: manifestFile, Line: 4, Msg: "the list requires has an empty capability label"},
				},
			},
			{
				name: "returns an Error at the mapping for a manifest without a family",
				give: "format: 1\nname: acme-audit\nversion: 0.3.0\nrules:\n    - on: struct\n      emit: mirror\n",
				want: []declarative.Error{
					{File: manifestFile, Line: 1, Msg: "the manifest declares no output family"},
					{
						File: manifestFile,
						Line: 5,
						Msg:  `the rule emits into the family "", which the manifest does not declare`,
					},
				},
			},
			{
				name: "returns an Error at the mapping for a manifest without a rule",
				give: "format: 1\nname: acme-audit\nversion: 0.3.0\noutputs:\n    - per: source\n      word: audit\n",
				want: []declarative.Error{{File: manifestFile, Line: 1, Msg: "the manifest declares no rule"}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := loadFaults(t, files.Tree{manifestFile: files.Text(tt.give)})
				assert.Equal(t, got, tt.want, "Load returns the faults of the manifest")
			})
		}

		accepted := []struct {
			name string
			give string
		}{
			{
				name: "returns no error for a manifest that states the contract version of the kernel",
				give: "format: 1\ncontract: 1\nname: acme-audit\nversion: 0.3.0\n" + structRule,
			},
			{
				name: "returns no error for a manifest without a contract version",
				give: "format: 1\nname: acme-audit\nversion: 0.3.0\n" + structRule,
			},
		}
		for _, tt := range accepted {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := declarative.Load(files.Workspace(t, files.Tree{manifestFile: files.Text(tt.give)}))
				assert.NoError(t, err, "Load reads the manifest")
			})
		}
	})
}
