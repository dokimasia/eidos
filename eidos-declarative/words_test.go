// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/declarative"
)

// A closed vocabulary of the manifest accepts its spellings alone, and an
// Error at the value lists them.
func TestWords(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			old  string
			new  string
			want declarative.Error
		}{
			{
				name: "returns an Error that lists the spellings for a sequence in place of a word",
				old:  "per: source",
				new:  "per: [source]",
				want: declarative.Error{
					File: manifestFile, Line: 5, Msg: "!!seq is not a cardinality: use source, package or plan",
				},
			},
			{
				name: "returns an Error that lists the spellings for a mapping in place of a word",
				old:  "emit: mirror",
				new:  "emit: {mirror: true}",
				want: declarative.Error{
					File: manifestFile, Line: 9, Msg: "!!map is not an emission: use mirror or implementation",
				},
			},
			{
				name: "returns an Error that quotes the spelling of a scalar that no word has",
				old:  "on: struct",
				new:  "on: Struct",
				want: declarative.Error{
					File: manifestFile,
					Line: 8,
					Msg:  `"Struct" is not a subject kind: use function, interface or struct`,
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				manifest := strings.Replace(ruleManifest, tt.old, tt.new, 1)
				assert.NotEqual(t, manifest, ruleManifest, "the case changes the manifest")
				got := loadFaults(t, files.Tree{manifestFile: files.Text(manifest)})
				assert.Equal(t, got, []declarative.Error{tt.want}, "Load returns the fault of the word")
			})
		}
	})
}
