// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql_test

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/conformance/lang/graphql"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	graphqllang "go.dokimi.dev/eidos/lang/graphql"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
)

// The tree of the catalog fixture and the goldens of the files that its
// plans write.
const (
	catalogTree = "testdata/catalog/tree"
	catalogWant = "testdata/catalog/want"
)

// frameEnd ends the lines of the frame that the output contract writes
// before the body of a file.
const frameEnd = "\n\n"

// The files of the tree whose types no target spells: a schema that names
// the undefined type Missing in each position that a generator translates,
// and a document with a variable of the type. The schema does not load, so
// the document loads without types.
const (
	undefinedSchema   = "broken/schema.graphql"
	undefinedDocument = "broken/keyed.graphql"
	undefinedSource   = "type Query {\n  lost: Missing\n}\n\n" +
		"type Mutation {\n  find(key: Missing): String\n}\n\n" +
		"type Subscription {\n  fetch(key: ID!): Missing\n}\n\n" +
		"type Holder {\n  lost: Missing\n}\n\n" +
		"type Keeper {\n  id: ID!\n  fetch(key: ID!): Missing\n}\n\n" +
		"interface Shape implements Missing {\n  id: ID!\n}\n\n" +
		"interface Edge {\n  lost: Missing\n}\n\n" +
		"interface Port {\n  open(key: Missing): String\n}\n\n" +
		"union Pick = Holder | Missing\n"
	keyedSource = "query Keyed($key: Missing) {\n  lost\n}\n"
)

// catalogFixture returns the workspace suite's fixture of the catalog
// composition: the tree, the composition, its three plans, the goldens of
// the files that they write, and the edit that makes a field of Review
// nullable.
func catalogFixture(t *testing.T) workspacetest.Fixture {
	t.Helper()

	return workspacetest.Fixture{
		Tree:    os.DirFS(filepath.FromSlash(catalogTree)),
		Compose: func(root string) (*workspace.Builder, error) { return graphql.ComposeCatalog(root), nil },
		Plans:   graphql.CatalogPlans,
		Want:    workspacetest.Want(t, catalogWant),
		Edit:    graphql.EditCatalog,
	}
}

// catalogRoot copies the tree of the catalog fixture into a new directory,
// and returns the directory.
func catalogRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	assert.NoError(t, os.CopyFS(root, os.DirFS(filepath.FromSlash(catalogTree))), "the catalog tree copies")
	return root
}

// cleanRun runs the catalog composition with the plans over root, and fails
// the test where the run is not clean. Each Error of the run fails the test
// on a line of its own, before the run's error does.
func cleanRun(t *testing.T, root string, plans ...workspace.Plan) {
	t.Helper()

	w, err := graphql.ComposeCatalog(root).Plans(plans...).Build()
	assert.NoError(t, err, "the composition builds")
	report, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
	for d := range report.Sink.All() {
		expect.NotEqual(t, d.Severity, diag.SeverityError,
			fmt.Sprintf("%s at %s is not an Error of the run: %s", d.Code, d.Pos, d.Msg))
	}
	assert.NoError(t, err, "the run is clean")
}

// refusedRun runs the catalog composition with every plan over root, whose
// tree has types that a target does not spell, and returns the report. It
// fails the test where the run does not fail.
func refusedRun(t *testing.T, root string) *workspace.Report {
	t.Helper()

	w, err := graphql.ComposeCatalog(root).Plans(graphql.CatalogPlans()...).Build()
	assert.NoError(t, err, "the composition builds")
	report, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
	assert.ErrorIs(t, err, workspace.ErrRunFailed, "the refusals fail the run")
	return report
}

// refusedAt returns the file and the line of each RefusedType finding that
// the plugin by reported in the report, in the order of the sink.
func refusedAt(report *workspace.Report, by diag.Origin) []string {
	var out []string
	for d := range report.Sink.All() {
		if d.Code == eidos.RefusedType && d.Origin == by {
			out = append(out, fmt.Sprintf("%s:%d", d.Pos.File, d.Pos.Line))
		}
	}
	return out
}

// graphqlPaths returns the slash-separated path of every GraphQL file of the
// tree at dir, in lexical order.
func graphqlPaths(t *testing.T, dir string) []string {
	t.Helper()

	var paths []string
	err := fs.WalkDir(os.DirFS(dir), ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && path.Ext(p) == graphqllang.Extension {
			paths = append(paths, p)
		}
		return err
	})
	assert.NoError(t, err, "the tree walks")
	return paths
}

// bodyOf returns the text of a written file inside the frame that the
// output contract adds: the lines up to the first blank line, and the
// blank line and the trailer at the end.
func bodyOf(t *testing.T, stamped string) string {
	t.Helper()

	_, rest, framed := strings.Cut(stamped, frameEnd)
	assert.True(t, framed, "the written file has a frame")
	end := strings.LastIndex(strings.TrimSuffix(rest, "\n"), frameEnd)
	assert.NotEqual(t, end, -1, "the written file has a blank line before its trailer")
	return rest[:end+1]
}
