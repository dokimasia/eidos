// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// goTree is the whole-contract fixture: a module root, two
// packages, a cross-package reference, a carrier, a test file, a
// signature root and a file that does not parse.
func goTree() fstest.MapFS {
	return fstest.MapFS{
		"go.mod": {Data: []byte("module example.test/fix\n")},
		"api/user.go": {Data: []byte(
			"package api\n\n// User is one account.\ntype User struct{ name string }\n",
		)},
		"store/row.go": {Data: []byte(
			"package store\n\nimport \"example.test/fix/api\"\n\n" +
				"// Row is one record.\n//+gen:table name=rows\n" +
				"type Row struct {\n\towner api.User\n\tcount int\n}\n\n" +
				"func (r *Row) Get(n int) int { return r.count + n }\n",
		)},
		"store/row_test.go": {Data: []byte(
			"package store\n\nfunc probe() {}\n",
		)},
		"dep/dep.go": {Data: []byte(
			"package dep\n\ntype Dep struct{}\n\nconst hidden = 1\n",
		)},
		"bad/oops.go": {Data: []byte("package bad\n\nfunc ({ nope\n")},
	}
}

// setup is the suite's entry over the whole-contract fixture.
func setup(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
	return frontend.New(nil), &frontendtest.Fixture{
		Sources:    goTree(),
		Signatures: []string{"dep"},
		Schemas:    frontendtest.ScriptedSchemas(),
		Keys:       frontend.Keys,
	}
}

// The frontend answers the same bar every frontend answers, over
// real Go source.
func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("passes the conformance suite", func(t *testing.T) {
		t.Parallel()
		frontendtest.RunFrontendSuite(t, setup)
	})

	t.Run("reports a file the parser refused, and continues", func(t *testing.T) {
		t.Parallel()

		f := frontend.New(nil)
		u := unitOf(t, f, goTree(), "bad/oops.go")
		assert.NoError(t, f.Parse(context.Background(), u),
			"a syntax error is the source's problem, not the load's")
	})
}

// unitOf partitions a tree and returns the unit holding one file.
func unitOf(
	tb assert.TB, f plugin.Frontend, tree fstest.MapFS, member string,
) *plugin.SourceUnit {
	tb.Helper()

	var claimed []plugin.SourceRef
	for path := range tree {
		if path != "go.mod" {
			claimed = append(claimed, plugin.SourceRef{Path: path})
		}
	}
	parts, err := f.Partition(context.Background(), claimed, treeReader{tree})
	assert.NoError(tb, err, "the fixture partitions")
	for _, part := range parts {
		for _, ref := range part {
			if ref.Path == member {
				return plugin.NewSourceUnit(part, tree, plugin.DepthFull,
					f.Syntax(), diag.NewSink(), f.Name())
			}
		}
	}
	tb.Fatalf("no unit holds %s", member)
	return nil
}

// treeReader is the partition's recorded door over a test tree.
type treeReader struct {
	tree fstest.MapFS
}

// Read returns one file's bytes.
func (r treeReader) Read(path string) ([]byte, error) { return r.tree.ReadFile(path) }
