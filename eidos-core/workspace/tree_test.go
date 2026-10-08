// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"errors"
	"fmt"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The files the tree cases add: a file in the store package's directory
// that declares another package, and a file of the store package in a
// directory that sorts before the store's.
const (
	strangerSource = "svc/store/stranger.zz"
	earlierSource  = "svc/aaa/early.zz"
)

// The store package's per-package file after a file of an earlier
// directory joined the package.
const earlierGenerated = "svc/aaa/gen.txt"

// widthModulerID names the annotator that states the store package's
// module after the width of the row.
const widthModulerID plugin.ID = "width-moduler"

// widthModuler is an annotator that implements its role directly. It
// states that the store package is a module whose path names the number
// of the row's fields.
type widthModuler struct{}

// Name returns the annotator's name.
func (widthModuler) Name() plugin.ID { return widthModulerID }

// Annotate looks the row up and stamps the store package's module facts.
func (widthModuler) Annotate(ctx *plugin.AnnotatorContext) error {
	width := 0
	if row, held := ctx.Reader.Lookup(rowID); held {
		if s, is := row.(*node.Struct); is {
			width = len(s.Fields)
		}
	}
	claim := meta.Claim{Subject: storePackage, Bucket: ctx.Bucket, Plugin: widthModulerID}
	if err := meta.Stamp(ctx.Facts, ctx.Kernel.Module, fmt.Sprintf("example.test/w%d", width), claim); err != nil {
		return err
	}
	return meta.Stamp(ctx.Facts, ctx.Kernel.ModuleRoot, ".", claim)
}

// A warm run renders again each file whose placement an edit changed:
// the residents of its directory, the files of its package, or the
// modules.
func TestTree(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		before := roundsTree(rowLine, userLine)

		t.Run("renders the file of a directory whose residents an edit changed", func(t *testing.T) {
			t.Parallel()

			after := roundsTree(rowLine, userLine)
			after[strangerSource] = &fstest.MapFile{Data: []byte("package svc/stranger\n")}
			report, err := warmAfter(t, built(t, residentsNaming(t, ledger.NewMem())), before, after)
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Stats.Rendered, 1, "the store directory's file renders under its new residents")
			cold := sealedRun(t, built(t, residentsNaming(t, ledger.NewMem())), workspace.Input{Tree: after})
			assert.Equal(t, report.Manifest.Files, cold.Manifest.Files,
				"the warm run records the same entries as the cold run")
		})

		t.Run("moves the file of a package that a file of an earlier directory joined", func(t *testing.T) {
			t.Parallel()

			after := roundsTree(rowLine, userLine)
			after[earlierSource] = &fstest.MapFile{Data: []byte("package svc/store\n")}
			report, err := warmAfter(t, sealing(t, ledger.NewMem(), "plan"), before, after)
			assert.NoError(t, err, "the run is clean")
			assert.Contains(t, paths(report.Manifest), earlierGenerated,
				"the package's file moves to the earlier directory")
			cold := sealedRun(t, sealing(t, ledger.NewMem(), "plan"), workspace.Input{Tree: after})
			assert.Equal(t, report.Manifest.Files, cold.Manifest.Files,
				"the warm run records the same entries as the cold run")
		})

		t.Run("renders every placed file of a plan whose modules an edit changed", func(t *testing.T) {
			t.Parallel()

			w := built(t, sealingBuilder(t, ledger.NewMem(), "plan").Annotators(widthModuler{}))
			report, err := warmAfter(t, w, before, roundsTree(widerRow, userLine))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Stats.Rendered, 2, "the api package's file renders against the new modules too")
		})
	})
}

// residentsNaming returns the builder of a composition over the scripted
// frontend that records into a ledger. Its one plan mirrors every struct
// into a file whose package the residents of the file's directory name,
// and whose first line states that package.
func residentsNaming(tb assert.TB, l ledger.Ledger) *workspace.Builder {
	tb.Helper()

	printer := backend.New("residents-printer", "fixture", plugin.CommentSyntax{Line: []string{"//"}}).
		FileTemplate("package {{.Pkg.Name}}\n{{" + render.BuiltinDecls + "}}").
		KindTemplates(map[symbol.Kind]string{symbol.KindStruct: "type {{.Name}} struct{}\n"}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Packages(func(p plugin.Placement) (symbol.Identity, error) {
			pkg := p.Origin
			pkg.Name = fmt.Sprintf("r%d", len(p.Residents))
			return pkg, nil
		}).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return nil, errors.New("the fixture spells no statements")
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Coverage(render.Coverage{Facts: totalCoverage()}).
		Build()
	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Targets("fixture").
		Plans(workspace.Plan{Name: "plan", Generators: []plugin.Generator{mirror(mirrorID)}, Backend: printer}).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return l, nil })
}
