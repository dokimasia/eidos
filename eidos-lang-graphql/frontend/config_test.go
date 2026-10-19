// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/graphql/frontend"
	"go.dokimi.dev/eidos/sdk/position"
)

// The sources of the two schema files that the configuration cases claim,
// and the file at the root that two projects claim.
const (
	aSource    = "type A {\n  a: Int\n}\n"
	bSource    = "type B {\n  b: Int\n}\n"
	rootSchema = "x.graphqls"
)

// The configuration files of the format cases.
const (
	jsonRC = ".graphqlrc.json"
	tomlRC = "graphql.config.toml"
)

// The projects of the order cases, in the order in which a document writes
// them, and in the order of their names.
const (
	secondProject = "second"
	firstProject  = "first"
)

// A configuration decides the units, so every format of graphql-config and
// of gqlgen decodes into the same schemas, and a configuration that the
// frontend does not read reports at the file.
func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("decodeConfig", func(t *testing.T) {
		t.Parallel()

		formats := []struct {
			name     string
			giveFile string
			giveText string
		}{
			{
				name:     "reads a configuration of YAML",
				giveFile: ".graphqlrc.yaml",
				giveText: bothSchemas,
			},
			{
				name:     "reads a configuration without an extension as YAML",
				giveFile: rootPlainRC,
				giveText: bothSchemas,
			},
			{
				name:     "reads a configuration of JSON",
				giveFile: jsonRC,
				giveText: `{"schema": ["a/*.graphqls", "b/*.graphqls"]}`,
			},
			{
				name:     "reads a configuration of TOML",
				giveFile: tomlRC,
				giveText: "schema = [\"a/*.graphqls\", \"b/*.graphqls\"]\n",
			},
			{
				name:     "reads the key graphql of package.json",
				giveFile: packageFile,
				giveText: `{"name": "shop", "graphql": {"schema": "*/*.graphqls"}}`,
			},
			{
				name:     "reads a legacy configuration of JSON",
				giveFile: ".graphqlconfig.json",
				giveText: `{"schemaPath": "*/*.graphqls"}`,
			},
			{
				name:     "reads a configuration of gqlgen",
				giveFile: "gqlgen.yaml",
				giveText: "schema:\n  - \"*/*.graphqls\"\n",
			},
			{
				name:     "reads a hidden configuration of gqlgen",
				giveFile: ".gqlgen.yml",
				giveText: "schema: \"*/*.graphqls\"\n",
			},
		}
		for _, tt := range formats {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tree := fstest.MapFS{tt.giveFile: {Data: []byte(tt.giveText)}}
				assert.Equal(t, membersOf(partitioned(t, tree, aSchema, bSchema)), [][]string{{aSchema, bSchema}},
					"the schema of the configuration claims both files")
			})
		}

		orders := []struct {
			name     string
			giveFile string
			giveText string
			want     string
		}{
			{
				name:     "keeps the projects of JSON in the order of the document",
				giveFile: jsonRC,
				giveText: `{"projects": {"second": {"schema": "x.graphqls"}, "first": {"schema": "x.graphqls"}}}`,
				want:     secondProject,
			},
			{
				name:     "keeps the projects of YAML in the order of the document",
				giveFile: rootRC,
				giveText: "projects:\n  second:\n    schema: x.graphqls\n  first:\n    schema: x.graphqls\n",
				want:     secondProject,
			},
			{
				name:     "orders the projects of TOML by their names",
				giveFile: tomlRC,
				giveText: "[projects.second]\nschema = \"x.graphqls\"\n\n[projects.first]\nschema = \"x.graphqls\"\n",
				want:     firstProject,
			},
			{
				name:     "follows an alias of YAML to its anchor",
				giveFile: rootRC,
				giveText: "base: &base\n  schema: x.graphqls\nprojects:\n  second: *base\n",
				want:     secondProject,
			},
		}
		for _, tt := range orders {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gbs, sink := parsedTree(t, fstest.MapFS{
					tt.giveFile: {Data: []byte(tt.giveText)},
					rootSchema:  {Data: []byte(aSource)},
				})
				assert.Empty(t, codesOf(sink), "the configuration and the schema load clean")
				assert.Length(t, gbs, 1, "the file is one unit")
				assert.Equal(t, gbs[0].Packages()[0].ID.Package, tt.want, "the first project claims the file")
			})
		}

		entries := []struct {
			name     string
			giveText string
		}{
			{
				name:     "reads a string entry among the scalars of a list",
				giveText: `{"schema": [null, 1, true, "a/*.graphqls", "b/*.graphqls"]}`,
			},
			{
				name:     "reads the first key of a map as an entry",
				giveText: `{"schema": [{"a/x.graphqls": {"loader": "./loader.js"}}, "b/*.graphqls"]}`,
			},
			{
				name:     "reads no entry from a map without keys",
				giveText: `{"schema": [{}, "a/*.graphqls", "b/*.graphqls"]}`,
			},
		}
		for _, tt := range entries {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tree := fstest.MapFS{jsonRC: {Data: []byte(tt.giveText)}}
				assert.Equal(t, membersOf(partitioned(t, tree, aSchema, bSchema)), [][]string{{aSchema, bSchema}},
					"the entries claim both files")
			})
		}

		empties := []struct {
			name     string
			giveText string
		}{
			{name: "defines no schema in an empty document", giveText: ""},
			{name: "defines no schema in a document that is a scalar", giveText: "shop\n"},
			{name: "defines no schema without the key schema", giveText: "documents: \"*/*.graphqls\"\n"},
		}
		for _, tt := range empties {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tree := fstest.MapFS{rootRC: {Data: []byte(tt.giveText)}}
				assert.Equal(t, membersOf(partitioned(t, tree, aSchema, bSchema)), [][]string{{aSchema}, {bSchema}},
					"each file is a unit of its directory")
			})
		}

		t.Run("reads a table of TOML without a schema", func(t *testing.T) {
			t.Parallel()

			gbs, sink := parsedTree(t, fstest.MapFS{
				tomlRC:  {Data: []byte("[extensions]\nlevel = 3\n")},
				aSchema: {Data: []byte(aSource)},
			})
			assert.Empty(t, codesOf(sink), "the configuration decodes")
			assert.Equal(t, gbs[0].Packages()[0].ID.Package, "a", "the file is in the package of its directory")
		})

		unread := []struct {
			name     string
			giveFile string
			giveText string
			want     string
		}{
			{
				name:     "reports UnreadConfig for a configuration in TypeScript",
				giveFile: "graphql.config.ts",
				giveText: "export default { schema: 'a/*.graphqls' };\n",
				want:     "frontend: the configuration is code, and the frontend does not run it",
			},
			{
				name:     "reports UnreadConfig for an empty configuration of JSON",
				giveFile: jsonRC,
				want:     "frontend: decode JSON: EOF",
			},
			{
				name:     "reports UnreadConfig for a bad item of a list of JSON",
				giveFile: jsonRC,
				giveText: "[x]",
				want:     "frontend: decode JSON",
			},
			{
				name:     "reports UnreadConfig for a key of JSON that is not a string",
				giveFile: jsonRC,
				giveText: "{1: 2}",
				want:     "frontend: decode JSON",
			},
			{
				name:     "reports UnreadConfig for a key of JSON without a value",
				giveFile: jsonRC,
				giveText: `{"schema": }`,
				want:     "frontend: decode JSON",
			},
			{
				name:     "reports UnreadConfig for a list of JSON that does not close",
				giveFile: jsonRC,
				giveText: `{"schema": [1`,
				want:     "frontend: decode JSON",
			},
			{
				name:     "reports UnreadConfig for an object of JSON that does not close",
				giveFile: jsonRC,
				giveText: `{"schema": 1`,
				want:     "frontend: decode JSON",
			},
			{
				name:     "reports UnreadConfig for a list of JSON that a brace closes",
				giveFile: jsonRC,
				giveText: `{"schema": [1}`,
				want:     "frontend: decode JSON",
			},
			{
				name:     "reports UnreadConfig for an object of JSON that a bracket closes",
				giveFile: jsonRC,
				giveText: `{"schema": 1]`,
				want:     "frontend: decode JSON",
			},
			{
				name:     "reports UnreadConfig for a document of JSON that continues after its value",
				giveFile: jsonRC,
				giveText: "{} {}",
				want:     "frontend: decode JSON: the document continues after its value",
			},
			{
				name:     "reports UnreadConfig for a package.json that does not decode",
				giveFile: packageFile,
				giveText: "{",
				want:     "frontend: decode JSON",
			},
			{
				name:     "reports UnreadConfig for a configuration of YAML that does not decode",
				giveFile: rootRC,
				giveText: "schema: [",
				want:     "frontend: decode YAML",
			},
			{
				name:     "reports UnreadConfig for a configuration of TOML that does not decode",
				giveFile: tomlRC,
				giveText: "schema = ",
				want:     "frontend: decode TOML",
			},
			{
				name:     "reports UnreadConfig for a configuration of gqlgen that does not decode",
				giveFile: gqlgenFile,
				giveText: "schema: [",
				want:     "frontend: decode YAML",
			},
		}
		for _, tt := range unread {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, sink := parsedTree(t, fstest.MapFS{
					tt.giveFile: {Data: []byte(tt.giveText)},
					aSchema:     {Data: []byte(aSource)},
					bSchema:     {Data: []byte(bSource)},
				})
				found := findingsOf(sink, frontend.UnreadConfig)
				assert.Length(t, found, 1, "the unit of the first file reports the configuration once")
				expect.Contains(t, found[0].Msg, tt.want, "the finding states why the frontend does not read it")
				expect.Equal(t, found[0].Pos, position.Pos{File: tt.giveFile, Line: 1, Col: 1},
					"the finding is at the start of the configuration")
			})
		}
	})
}
