// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/graphql/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The files and the configurations that the partition cases claim and read.
const (
	rootRC      = ".graphqlrc.yml"
	rootPlainRC = ".graphqlrc"
	legacyRC    = ".graphqlconfig"
	packageFile = "package.json"
	subGqlgen   = "sub/gqlgen.yml"
	aSchema     = "a/x.graphqls"
	bSchema     = "b/y.graphqls"
	aDocument   = "a/q.graphql"
	bDocument   = "b/r.graphql"
	subSchema   = "sub/s.graphqls"
	brokenRC    = "b/.graphqlrc.json"
)

// partitionAllocs is the budget of one partition of the canonical corpus of
// 10,000 files in 1000 directories. One run counted 64,200 allocations:
// 58,058 for the probes of the 29 configuration names in each of the 1001
// directories, each with its joined path and the error of a read of a name
// that the directory does not contain, and the units and their members the
// rest. The budget is about 5% above the count.
const partitionAllocs = 67_500

// The configurations of the partition cases.
const (
	// bothSchemas claims the schema files of a and b.
	bothSchemas = "schema:\n  - a/*.graphqls\n  - b/*.graphqls\n"
	// everySchema claims every schema file of the workspace.
	everySchema = "schema: \"**/*.graphqls\"\n"
	// ownSchemas claims the schema files of the directory of a configuration
	// of gqlgen.
	ownSchemas = "schema:\n  - \"*.graphqls\"\n"
)

// The partition groups the claimed files into the units of their schemas,
// so a schema loads whole and its documents validate against it.
func TestPartition(t *testing.T) {
	t.Parallel()

	t.Run("partition", func(t *testing.T) {
		t.Parallel()

		t.Run("makes the files of one schema one unit", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{rootRC: {Data: []byte(bothSchemas)}}
			assert.Equal(t, membersOf(partitioned(t, tree, bSchema, aSchema)), [][]string{{aSchema, bSchema}},
				"a schema across two directories loads as one unit, its files in path order")
		})

		t.Run("makes the files of a directory that no schema claims one unit", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, membersOf(partitioned(t, fstest.MapFS{}, bDocument, aSchema, aDocument)),
				[][]string{{aDocument, aSchema}, {bDocument}}, "each directory is a unit of its own")
		})

		t.Run("returns the units in the order of their first files", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{rootRC: {Data: []byte("schema: b/*.graphqls\n")}}
			assert.Equal(t, membersOf(partitioned(t, tree, bSchema, aDocument)), [][]string{{aDocument}, {bSchema}},
				"the unit of a directory and the unit of a schema sort together")
		})

		t.Run("joins a file to the schema of its own directory first", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{rootRC: {Data: []byte(everySchema)}, subGqlgen: {Data: []byte(ownSchemas)}}
			assert.Equal(t, membersOf(partitioned(t, tree, aSchema, subSchema)), [][]string{{aSchema}, {subSchema}},
				"the schema of gqlgen in sub claims its file before the schema of the root")
		})

		t.Run("shares a file with every other schema that claims it", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{rootRC: {Data: []byte(everySchema)}, subGqlgen: {Data: []byte(ownSchemas)}}
			units := partitioned(t, tree, aSchema, subSchema)
			assert.Length(t, units, 2, "the schema of the root and the schema of sub")
			expect.Equal(t, units[0][0].Shared, []string{rootRC, subGqlgen, subSchema},
				"the schema of the root reads the file of sub after the configurations")
			expect.Equal(t, units[1][0].Shared, []string{rootRC, subGqlgen},
				"the schema of sub reads the configurations alone")
		})

		t.Run("shares every configuration that the frontend reads with every unit", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{"b/" + rootRC: {Data: []byte("schema: \"*.graphqls\"\n")}}
			units := partitioned(t, tree, aDocument, bSchema)
			assert.Length(t, units, 2, "the unit of a and the schema of b")
			expect.Equal(t, units[0][0].Shared, []string{"b/" + rootRC},
				"a unit outside the directory of a configuration reads it, because a pattern can claim any file")
		})

		t.Run("shares a configuration that does not decode with the unit of its first file alone", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{brokenRC: {Data: []byte("{")}}
			units := partitioned(t, tree, aDocument, bDocument)
			assert.Length(t, units, 2, "the files of a and of b are units of their directories")
			expect.Empty(t, units[0][0].Shared, "the unit of a does not report the configuration of b")
			expect.Equal(t, units[1][0].Shared, []string{brokenRC}, "the unit of b reports it")
		})

		t.Run("reads the first configuration of graphql-config in the order of its names", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				rootPlainRC: {Data: []byte(bothSchemas)},
				rootRC:      {Data: []byte("schema: nothing.graphqls\n")},
			}
			units := partitioned(t, tree, aSchema, bSchema)
			expect.Equal(t, membersOf(units), [][]string{{aSchema, bSchema}}, "the schema of .graphqlrc claims both")
			expect.Equal(t, units[0][0].Shared, []string{rootPlainRC}, "the unit reads .graphqlrc alone")
		})

		t.Run("skips a package.json without the key graphql", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				packageFile: {Data: []byte(`{"name": "shop"}`)},
				legacyRC:    {Data: []byte("schemaPath: \"*/*.graphqls\"\n")},
			}
			units := partitioned(t, tree, aSchema, bSchema)
			expect.Equal(t, membersOf(units), [][]string{{aSchema, bSchema}}, "the legacy configuration claims both")
			expect.Equal(t, units[0][0].Shared, []string{legacyRC}, "the unit does not read the package.json")
		})

		t.Run("reads a configuration of graphql-config and one of gqlgen in one directory", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				"sub/" + rootRC: {Data: []byte("schema: nothing.graphqls\n")},
				subGqlgen:       {Data: []byte(ownSchemas)},
			}
			units := partitioned(t, tree, subSchema)
			assert.Equal(t, units[0][0].Shared, []string{"sub/" + rootRC, subGqlgen}, "the unit reads both")
		})

		t.Run("claims a file outside the directory of its configuration", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{"a/" + rootRC: {Data: []byte("schema:\n  - \"*.graphqls\"\n  - ../b/*.graphqls\n")}}
			assert.Equal(t, membersOf(partitioned(t, tree, aSchema, bSchema)), [][]string{{aSchema, bSchema}},
				"the schema of a claims the file of b")
		})

		t.Run("makes no unit of a schema that claims no file", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{rootRC: {Data: []byte("schema: nothing.graphqls\n")}}
			assert.Equal(t, membersOf(partitioned(t, tree, aSchema)), [][]string{{aSchema}},
				"the file of a is a unit of its directory")
		})

		t.Run("gives every member of a unit the one list of shared inputs", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{rootRC: {Data: []byte(bothSchemas)}}
			units := partitioned(t, tree, aSchema, bSchema)
			assert.Equal(t, units[0][1].Shared, units[0][0].Shared, "the members share the list",
				assert.ByIdentity())
		})

		t.Run("probes a directory above two siblings once", func(t *testing.T) {
			t.Parallel()

			r := countingReader{tree: fstest.MapFS{}, reads: map[string]int{}}
			refs := []plugin.SourceRef{{Path: aSchema}, {Path: bSchema}}
			_, err := frontend.New().Partition(t.Context(), refs, r)
			assert.NoError(t, err, "the tree partitions")
			assert.Equal(t, r.reads[rootRC], 1, "the second chain stops at the directory that the first probed")
		})

		t.Run("leaves the claimed files in their order", func(t *testing.T) {
			t.Parallel()

			refs := []plugin.SourceRef{{Path: bSchema}, {Path: aSchema}}
			_, err := frontend.New().Partition(t.Context(), refs, treeReader{fstest.MapFS{}})
			assert.NoError(t, err, "the tree partitions")
			assert.Equal(t, refs[0].Path, bSchema, "the partition sorts a copy")
		})

		t.Run("returns nothing for no files", func(t *testing.T) {
			t.Parallel()

			units, err := frontend.New().Partition(t.Context(), nil, treeReader{fstest.MapFS{}})
			assert.NoError(t, err, "an empty claim partitions")
			assert.Empty(t, units, "there is no unit")
		})
	})

	t.Run("readShared", func(t *testing.T) {
		t.Parallel()

		t.Run("orders the configurations of one directory as the partition reads them", func(t *testing.T) {
			t.Parallel()

			gbs, sink := parsedTree(t, fstest.MapFS{
				"graphql.config.json": {Data: []byte(`{"projects": {"shop": {"schema": "x.graphqls"}}}`)},
				gqlgenFile:            {Data: []byte("schema: x.graphqls\n")},
				rootSchema:            {Data: []byte(aSource)},
			})
			assert.Empty(t, codesOf(sink), "both configurations load clean")
			assert.Equal(t, gbs[0].Packages()[0].ID.Package, "shop",
				"the configuration of graphql-config claims the file before the one of gqlgen, which sorts first")
		})

		t.Run("reads nothing from a package.json without the key graphql", func(t *testing.T) {
			t.Parallel()

			f := frontend.New()
			tree := fstest.MapFS{packageFile: {Data: []byte(`{"name": "shop"}`)}, aSchema: {Data: []byte(aSource)}}
			sink := diag.NewSink()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: aSchema, Shared: []string{packageFile}}}, tree,
				plugin.DepthFull, f.Syntax(), brand, sink, f.Name())
			assert.NoError(t, f.Parse(t.Context(), u), "the unit parses")
			expect.Empty(t, codesOf(sink), "the package.json is no configuration and reports nothing")
			expect.Equal(t, u.Graph().Packages()[0].ID.Package, "a", "the file is in the package of its directory")
		})

		t.Run("returns the error of a configuration that does not read", func(t *testing.T) {
			t.Parallel()

			f := frontend.New()
			tree := fstest.MapFS{aSchema: {Data: []byte(aSource)}}
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: aSchema, Shared: []string{rootRC}}}, tree,
				plugin.DepthFull, f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.HasError(t, f.Parse(t.Context(), u),
				"a configuration that the partition shared and the tree does not contain is a fault of the load")
		})
	})
}

// BenchmarkPartition measures the partition of the canonical corpus, which
// probes the configuration names of every directory once, and fails above
// partitionAllocs. Only -bench checks the budget.
func BenchmarkPartition(b *testing.B) {
	tree := scaledSchema()
	refs := claimedIn(tree)
	f := frontend.New()
	var (
		units [][]plugin.SourceRef
		err   error
	)
	c := bench.Start(b).MaxAllocs(partitionAllocs)
	defer c.End()
	for c.Loop() {
		units, err = f.Partition(b.Context(), refs, treeReader{tree})
	}
	assert.NoError(b, err, "the corpus partitions")
	assert.Length(b, units, benchPackages, "each directory is a unit")
}

// countingReader reads a tree and counts the reads of each path.
type countingReader struct {
	tree  fstest.MapFS
	reads map[string]int
}

// Read counts the read of p and returns the bytes of the file.
func (r countingReader) Read(p string) ([]byte, error) {
	r.reads[p]++
	return r.tree.ReadFile(p)
}
