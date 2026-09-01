// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/load"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// stdTree is the happy-path fixture: two packages, one cross-package
// reference, a directive, a constant, and a dependency package
// loaded signature-only.
func stdTree() fstest.MapFS {
	return fstest.MapFS{
		"mod.zz": {Data: []byte("mod v1\n")},
		"svc/api/user.zz": {Data: []byte(
			"package svc/api\ntype User string\n",
		)},
		"svc/store/row.zz": {Data: []byte(
			"package svc/store\nimport api svc/api\ntype Row api.User int\n+gen:table name=users\nconst rowmax\n",
		)},
		"svc/dep/dep.zz": {Data: []byte(
			"package svc/dep\ntype Dep string\nconst hidden\n",
		)},
	}
}

// loadTree drives one load over a tree with the fake frontend and
// the standard configuration, mutated per test.
func loadTree(
	tb assert.TB, tree fs.FS, mutate ...func(*load.Config),
) (*store.Graph, *load.Report, *diag.Sink) {
	tb.Helper()

	sink := diag.NewSink()
	cfg := load.Config{
		FS:         tree,
		Frontends:  []plugin.Frontend{newFake()},
		Sink:       sink,
		PluginSet:  []byte("set-1"),
		Signatures: []string{"svc/dep"},
	}
	for _, m := range mutate {
		m(&cfg)
	}
	g, report, err := load.Load(context.Background(), cfg)
	assert.NoError(tb, err, "the load finishes")
	return g, report, sink
}

// carries reports whether the sink holds a finding under the code.
func carries(sink *diag.Sink, c diag.Code) bool {
	for d := range sink.All() {
		if d.Code == c {
			return true
		}
	}
	return false
}

// rowID is the standard tree's one struct in svc/store.
func rowID() symbol.Identity {
	return symbol.Identity{
		Lang: fakeLang, Package: "svc/store", Name: "Row", Kind: symbol.KindStruct,
	}
}

// The driver is the read side's one pipeline, so its phases are
// pinned end to end over the scripted language.
func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("seals a graph the store serves", func(t *testing.T) {
		t.Parallel()

		g, _, _ := loadTree(t, stdTree())
		assert.True(t, g.Frozen(), "the load ends at the seal")

		row, held := g.Lookup(rowID())
		assert.True(t, held, "a loaded declaration is indexed")
		assert.Equal(t, row.(*node.Struct).Name, "Row", "as its own kind")

		pkg, held := g.PackageOf(rowID())
		assert.True(t, held, "under its package")
		assert.Equal(t, pkg.ID, symbol.Identity{
			Lang: fakeLang, Package: "svc/store", Kind: symbol.KindPackage,
		}, "whose identity is canonical")

		_, held = g.Lookup(symbol.Identity{
			Lang: fakeLang, Package: "svc/store", Name: "svc/store/row.zz", Kind: symbol.KindFile,
		})
		assert.True(t, held, "the file is a declaration of its package")
	})

	t.Run("loads signature roots shallow", func(t *testing.T) {
		t.Parallel()

		g, report, _ := loadTree(t, stdTree())
		_, held := g.Lookup(symbol.Identity{
			Lang: fakeLang, Package: "svc/store", Name: "rowmax", Kind: symbol.KindConstant,
		})
		assert.True(t, held, "a full unit keeps its constants")

		_, held = g.Lookup(symbol.Identity{
			Lang: fakeLang, Package: "svc/dep", Name: "hidden", Kind: symbol.KindConstant,
		})
		assert.False(t, held, "a signature unit drops what its parse skipped")

		for _, u := range report.Units {
			want := plugin.DepthFull
			if u.Files[0] == "svc/dep/dep.zz" {
				want = plugin.DepthSignatures
			}
			assert.Equal(t, u.Depth, want, "the report records each unit's depth")
		}
	})

	t.Run("attaches directives on assigned identities", func(t *testing.T) {
		t.Parallel()

		g, _, _ := loadTree(t, stdTree())
		raws := g.DirectivesOf(rowID())
		assert.Length(t, raws, 1, "the carrier's instance is attached")
		assert.Equal(t, raws[0].Name, directive.Name("gen:table"), "under its spelling")
		assert.Equal(t, raws[0].Args[0].Key, "name", "arguments parsed")
	})

	t.Run("keeps the first of two declarations spelling one identity", func(t *testing.T) {
		t.Parallel()

		tree := fstest.MapFS{
			"a/one.zz": {Data: []byte("package shared\ntype Twin left\n")},
			"b/two.zz": {Data: []byte("package shared\ntype Twin right\n")},
		}
		g, _, sink := loadTree(t, tree)
		assert.True(t, carries(sink, load.DuplicateDeclaration), "the second reports")

		twin, held := g.Lookup(symbol.Identity{
			Lang: fakeLang, Package: "shared", Name: "Twin", Kind: symbol.KindStruct,
		})
		assert.True(t, held, "one Twin stands")
		assert.Equal(t, twin.(*node.Struct).Fields[0].Type.Spelling, "left",
			"the first in unit order")
	})

	t.Run("replays unit findings under the frontend's origin", func(t *testing.T) {
		t.Parallel()

		tree := fstest.MapFS{
			"bad/oops.zz": {Data: []byte("type Lost string\n")},
		}
		_, _, sink := loadTree(t, tree)
		found := false
		for d := range sink.All() {
			if d.Code == fakeBadFile {
				found = true
				assert.Equal(t, d.Origin, diag.Origin("fakefront"), "origin-bound")
				assert.Equal(t, d.Pos.File, "bad/oops.zz", "positioned")
			}
		}
		assert.True(t, found, "a unit's finding reaches the run sink")
	})

	t.Run("refuses two claims on one file", func(t *testing.T) {
		t.Parallel()

		rival := newFake()
		rival.name = "rival"
		sink := diag.NewSink()
		_, _, err := load.Load(context.Background(), load.Config{
			FS:        stdTree(),
			Frontends: []plugin.Frontend{newFake(), rival},
			Sink:      sink,
		})
		assert.HasError(t, err, "an overlapping claim is a composition defect")
		assert.Contains(t, err.Error(), "fakefront", "naming the first claimant")
		assert.Contains(t, err.Error(), "rival", "and the second")
	})

	t.Run("refuses a versionless frontend", func(t *testing.T) {
		t.Parallel()

		sink := diag.NewSink()
		_, _, err := load.Load(context.Background(), load.Config{
			FS:        stdTree(),
			Frontends: []plugin.Frontend{versionless{newFake()}},
			Sink:      sink,
		})
		assert.HasError(t, err, "every unit key folds the version")
		assert.Contains(t, err.Error(), "version", "saying why")
	})
}

// versionless hides the fake's version, which the driver must
// refuse.
type versionless struct {
	f *fake
}

func (v versionless) Name() plugin.ID              { return v.f.Name() }
func (v versionless) Lang() symbol.Lang            { return v.f.Lang() }
func (v versionless) Syntax() plugin.CommentSyntax { return v.f.Syntax() }
func (v versionless) Selection() []string          { return v.f.Selection() }
func (v versionless) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	return v.f.Parse(ctx, u)
}

func (v versionless) Partition(
	ctx context.Context, files []plugin.SourceRef, r plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	return v.f.Partition(ctx, files, r)
}

func (v versionless) Resolve(scope plugin.ImportScope, spelling string) []symbol.Identity {
	return v.f.Resolve(scope, spelling)
}
