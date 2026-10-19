// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/graphql/frontend"
)

// The files that the claim cases select beside aSchema and bSchema, and the
// configuration of gqlgen of the refusal cases.
const (
	hiddenSchema  = "a/.hidden.graphqls"
	nestedSchema  = "a/deep/z.graphqls"
	skippedSchema = "a/skip.graphqls"
	rootDocument  = "schema.graphql"
	gqlgenFile    = "gqlgen.yml"
)

// A schema claims a file as graphql-config or gqlgen decides it, so a
// workspace loads the units that its tools load.
func TestSchema(t *testing.T) {
	t.Parallel()

	t.Run("matches", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			giveFile string
			giveText string
			claimed  []string
			want     [][]string
		}{
			{
				name:     "claims a file that an entry of the key documents matches",
				giveFile: rootRC,
				giveText: "schema: a/x.graphqls\ndocuments: b/*.graphqls\n",
				claimed:  []string{aSchema, bSchema},
				want:     [][]string{{aSchema, bSchema}},
			},
			{
				name:     "claims a file that include matches",
				giveFile: rootRC,
				giveText: "schema: a/x.graphqls\ninclude: b/*.graphqls\n",
				claimed:  []string{aSchema, bSchema},
				want:     [][]string{{aSchema, bSchema}},
			},
			{
				name:     "does not claim a file that include and exclude match",
				giveFile: rootRC,
				giveText: "schema: a/x.graphqls\ninclude: \"*/*.graphqls\"\nexclude: a/skip.graphqls\n",
				claimed:  []string{aSchema, skippedSchema},
				want:     [][]string{{skippedSchema}, {aSchema}},
			},
			{
				name:     "claims a file that an entry matches although exclude matches it",
				giveFile: rootRC,
				giveText: "schema: \"a/*.graphqls\"\nexclude: a/skip.graphqls\n",
				claimed:  []string{aSchema, skippedSchema},
				want:     [][]string{{skippedSchema, aSchema}},
			},
			{
				name:     "does not claim a file that a negated entry matches",
				giveFile: rootRC,
				giveText: "schema:\n  - \"a/*.graphqls\"\n  - \"!a/skip.graphqls\"\n",
				claimed:  []string{aSchema, skippedSchema},
				want:     [][]string{{skippedSchema}, {aSchema}},
			},
			{
				name:     "matches the alternatives of braces",
				giveFile: rootRC,
				giveText: "schema: \"{a,b}/*.graphqls\"\n",
				claimed:  []string{aSchema, bSchema},
				want:     [][]string{{aSchema, bSchema}},
			},
			{
				name:     "matches a file whose name opens with a dot",
				giveFile: rootRC,
				giveText: "schema: \"a/*.graphqls\"\n",
				claimed:  []string{aSchema, hiddenSchema},
				want:     [][]string{{hiddenSchema, aSchema}},
			},
			{
				name:     "matches any number of directories for **",
				giveFile: rootRC,
				giveText: "schema: \"a/**/*.graphqls\"\n",
				claimed:  []string{aSchema, nestedSchema},
				want:     [][]string{{nestedSchema, aSchema}},
			},
			{
				name:     "cleans the path of an entry",
				giveFile: rootRC,
				giveText: "schema:\n  - ./a/x.graphqls\n  - b/../b/y.graphqls\n",
				claimed:  []string{aSchema, bSchema},
				want:     [][]string{{aSchema, bSchema}},
			},
			{
				name:     "matches a file above the directory of the configuration",
				giveFile: "a/deep/" + rootRC,
				giveText: "schema:\n  - \"*.graphqls\"\n  - ../x.graphqls\n",
				claimed:  []string{aSchema, nestedSchema},
				want:     [][]string{{nestedSchema, aSchema}},
			},
			{
				name:     "matches no file for an entry of graphql-config that does not parse",
				giveFile: rootRC,
				giveText: "schema: \"a/[.graphqls\"\n",
				claimed:  []string{aSchema},
				want:     [][]string{{aSchema}},
			},
			{
				name:     "defines a schema for each project",
				giveFile: rootRC,
				giveText: "projects:\n  one:\n    schema: a/x.graphqls\n  two:\n    schema: b/y.graphqls\n",
				claimed:  []string{aSchema, bSchema},
				want:     [][]string{{aSchema}, {bSchema}},
			},
			{
				name:     "reads the legacy keys",
				giveFile: legacyRC,
				giveText: "schemaPath: a/x.graphqls\nincludes:\n  - \"*/*.graphqls\"\nexcludes:\n  - a/skip.graphqls\n",
				claimed:  []string{aSchema, bSchema, skippedSchema},
				want:     [][]string{{skippedSchema}, {aSchema, bSchema}},
			},
			{
				name:     "matches a glob of Go for gqlgen",
				giveFile: gqlgenFile,
				giveText: "schema:\n  - \"a/*.graphqls\"\n",
				claimed:  []string{aSchema, nestedSchema},
				want:     [][]string{{nestedSchema}, {aSchema}},
			},
			{
				name:     "walks the directory before ** for gqlgen",
				giveFile: gqlgenFile,
				giveText: "schema:\n  - \"a/**/*.graphqls\"\n",
				claimed:  []string{aSchema, nestedSchema, bSchema},
				want:     [][]string{{nestedSchema, aSchema}, {bSchema}},
			},
			{
				name:     "walks the directory of the configuration for ./** of gqlgen",
				giveFile: gqlgenFile,
				giveText: "schema:\n  - \"./**/*.graphqls\"\n",
				claimed:  []string{aSchema, bSchema},
				want:     [][]string{{aSchema, bSchema}},
			},
			{
				name:     "reads schema.graphql for a configuration of gqlgen without the key schema",
				giveFile: gqlgenFile,
				giveText: "model:\n  filename: models.go\n",
				claimed:  []string{aSchema, rootDocument},
				want:     [][]string{{aSchema}, {rootDocument}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tree := fstest.MapFS{tt.giveFile: {Data: []byte(tt.giveText)}}
				assert.Equal(t, membersOf(partitioned(t, tree, tt.claimed...)), tt.want,
					"the schema of the configuration claims the files that its tool claims")
			})
		}
	})

	t.Run("gqlgenPattern", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			giveText string
			want     string
		}{
			{
				name:     "reports UnreadConfig for a glob of gqlgen that does not parse",
				giveText: "schema:\n  - \"a/[.graphqls\"\n",
				want:     `frontend: gqlgen refuses the glob "a/[.graphqls"`,
			},
			{
				name:     "reports UnreadConfig for a walk of gqlgen without a directory before **",
				giveText: "schema:\n  - \"**/*.graphqls\"\n",
				want:     `frontend: gqlgen refuses the walk "**/*.graphqls", which names no directory before **`,
			},
			{
				name:     "reports UnreadConfig for a walk of gqlgen whose expression does not compile",
				giveText: "schema:\n  - \"a/**/(.graphqls\"\n",
				want:     `frontend: gqlgen refuses the walk "a/**/(.graphqls"`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tree := fstest.MapFS{
					gqlgenFile: {Data: []byte(tt.giveText)},
					aSchema:    {Data: []byte(aSource)},
					bSchema:    {Data: []byte(bSource)},
				}
				assert.Equal(t, membersOf(partitioned(t, tree, aSchema, bSchema)), [][]string{{aSchema}, {bSchema}},
					"gqlgen does not load the configuration, so it claims no file")
				_, sink := parsedTree(t, tree)
				found := findingsOf(sink, frontend.UnreadConfig)
				assert.Length(t, found, 1, "the unit of the first file reports the configuration")
				expect.Contains(t, found[0].Msg, tt.want, "the finding names the entry that gqlgen refuses")
			})
		}
	})

	t.Run("packageOf", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			giveFile string
			giveText string
			want     string
		}{
			{
				name:     "gives the schema of a configuration at the root the package of the root",
				giveFile: rootRC,
				giveText: "schema: a/x.graphqls\n",
				want:     "",
			},
			{
				name:     "gives the schema of a configuration the package of its directory",
				giveFile: "a/" + rootRC,
				giveText: "schema: x.graphqls\n",
				want:     "a",
			},
			{
				name:     "gives a project the directory of its configuration joined with its name",
				giveFile: "a/" + rootRC,
				giveText: "projects:\n  shop:\n    schema: x.graphqls\n",
				want:     "a/shop",
			},
			{
				name:     "gives the schema of gqlgen the package of its directory",
				giveFile: "a/" + gqlgenFile,
				giveText: "schema: x.graphqls\n",
				want:     "a",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gbs, sink := parsedTree(t, fstest.MapFS{
					tt.giveFile: {Data: []byte(tt.giveText)},
					aSchema:     {Data: []byte(aSource)},
				})
				assert.Empty(t, codesOf(sink), "the configuration and the schema load clean")
				assert.Equal(t, gbs[0].Packages()[0].ID.Package, tt.want, "the package of the unit is its schema")
			})
		}
	})

	t.Run("graphqlConfigSchemas", func(t *testing.T) {
		t.Parallel()

		t.Run("reads a key projects that is not a map as no projects", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{rootRC: {Data: []byte("projects:\n  - one\nschema: \"*/*.graphqls\"\n")}}
			assert.Equal(t, membersOf(partitioned(t, tree, aSchema, bSchema)), [][]string{{aSchema, bSchema}},
				"the key schema of the configuration claims both files")
		})
	})
}
