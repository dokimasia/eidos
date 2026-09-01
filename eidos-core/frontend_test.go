// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontendtest"
	"go.dokimi.dev/eidos/core/internal/fakelang"
	"go.dokimi.dev/eidos/core/load"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// kitFake lowers the scripted language through the kit, its
// test-file stamp moved from a parse statement into a classifier,
// which is the seam under test.
func kitFake() plugin.Frontend {
	inner := fakelang.New()
	return eidos.NewFrontend("kitfake", fakelang.Lang, inner.Syntax()).
		Version(inner.Ver).
		Match(inner.Sel...).
		Units(inner.Partition).
		Parse(inner.Parse).
		Classify(markTests).
		Resolve(inner.Resolve).
		Options(inner.Opts).
		Build()
}

// markTests stamps the test-file key on every parsed file whose
// path says so.
func markTests(u *plugin.SourceUnit) error {
	gb := u.Graph()
	for _, pkg := range gb.Packages() {
		for _, f := range pkg.Files {
			if strings.HasSuffix(f.Path, "_test.zz") {
				gb.Stamp(f, meta.RawStamp{
					Key: fakelang.TestFileKey, Value: "classified", Pos: f.Pos,
				})
			}
		}
	}
	return nil
}

// kitTree is the fixture the kit-built frontend loads: two
// packages, a cross-package reference, a carrier, a test file for
// the classifier, and a signature root.
func kitTree() fstest.MapFS {
	return fstest.MapFS{
		"mod.zz": {Data: []byte("mod v1\n")},
		"svc/api/user.zz": {Data: []byte(
			"package svc/api\ntype User string\n",
		)},
		"svc/store/row.zz": {Data: []byte(
			"package svc/store\nimport api svc/api\ntype Row api.User int\n+gen:table name=users\n",
		)},
		"svc/store/row_test.zz": {Data: []byte(
			"package svc/store\ntype RowTest string\n",
		)},
		"svc/dep/dep.zz": {Data: []byte(
			"package svc/dep\ntype Dep string\nconst hidden\n",
		)},
	}
}

// The kit lowers a declaration to the frontend role, so the built
// frontend meets the same conformance bar a hand-rolled one does,
// and the lowering's own seams — classifier order, fatality, the
// declaration defects — are pinned beside it.
func TestNewFrontend(t *testing.T) {
	t.Parallel()

	t.Run("passes the conformance suite", func(t *testing.T) {
		t.Parallel()

		frontendtest.RunFrontendSuite(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
			return kitFake(), &frontendtest.Fixture{
				Sources:    kitTree(),
				Signatures: []string{"svc/dep"},
				Schemas:    fakelang.Schemas(),
				Keys:       fakelang.Keys,
			}
		})
	})

	t.Run("classifies after the parse, onto the store", func(t *testing.T) {
		t.Parallel()

		sink := diag.NewSink()
		g, _, err := load.Load(context.Background(), load.Config{
			FS:        kitTree(),
			Frontends: []plugin.Frontend{kitFake()},
			Sink:      sink,
		})
		assert.NoError(t, err, "the fixture loads")
		file := symbol.Identity{
			Lang: fakelang.Lang, Package: "svc/store",
			Name: "svc/store/row_test.zz", Kind: symbol.KindFile,
		}
		stamps := g.StampsOf(file)
		assert.Length(t, stamps, 1, "the classifier's stamp reached the store")
		assert.Equal(t, stamps[0].Key, fakelang.TestFileKey, "under its key")
		assert.Equal(t, stamps[0].Value.(string), "classified", "with the classifier's value")
	})

	t.Run("a classifier's error is fatal like a parse error", func(t *testing.T) {
		t.Parallel()

		broken := errors.New("kitfake: the classifier refuses")
		inner := fakelang.New()
		f := eidos.NewFrontend("kitfake", fakelang.Lang, inner.Syntax()).
			Version("1").
			Match(inner.Sel...).
			Units(inner.Partition).
			Parse(inner.Parse).
			Classify(func(*plugin.SourceUnit) error { return broken }).
			Resolve(inner.Resolve).
			Build()
		_, _, err := load.Load(context.Background(), load.Config{
			FS:        kitTree(),
			Frontends: []plugin.Frontend{f},
			Sink:      diag.NewSink(),
		})
		assert.ErrorIs(t, err, broken, "the load stops rather than sealing half-made stamps")
	})

	t.Run("declares options only when declared", func(t *testing.T) {
		t.Parallel()

		inner := fakelang.New()
		bare := eidos.NewFrontend("bare", fakelang.Lang, inner.Syntax()).
			Version("1").Match("**/*.zz").
			Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).
			Build()
		_, is := bare.(plugin.OptionsProvider)
		assert.False(t, is, "no declaration, no provider")

		_, is = kitFake().(plugin.OptionsProvider)
		assert.True(t, is, "a declared configuration folds into the keys")
	})

	t.Run("panics on a declaration defect", func(t *testing.T) {
		t.Parallel()

		inner := fakelang.New()
		whole := func() *eidos.FrontendBuilder {
			return eidos.NewFrontend("kitfake", fakelang.Lang, inner.Syntax()).
				Version("1").Match("**/*.zz").
				Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve)
		}
		assert.NotPanics(t, func() { whole().Build() }, "the whole declaration builds")
		assert.Panics(t, func() {
			eidos.NewFrontend("", fakelang.Lang, inner.Syntax()).Build()
		}, "an empty name is a defect")
		assert.Panics(t, func() {
			eidos.NewFrontend("kitfake", "", inner.Syntax()).
				Version("1").Match("**/*.zz").
				Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).Build()
		}, "an empty language is a defect")
		assert.Panics(t, func() {
			eidos.NewFrontend("kitfake", fakelang.Lang, inner.Syntax()).
				Match("**/*.zz").
				Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).Build()
		}, "a missing version is a defect, because every unit key folds it")
		assert.Panics(t, func() {
			eidos.NewFrontend("kitfake", fakelang.Lang, inner.Syntax()).
				Version("1").
				Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).Build()
		}, "an empty claim is a defect")
		assert.Panics(t, func() {
			eidos.NewFrontend("kitfake", fakelang.Lang, inner.Syntax()).
				Version("1").Match("**/*.zz").
				Parse(inner.Parse).Resolve(inner.Resolve).Build()
		}, "a missing partition is a defect")
		assert.Panics(t, func() {
			eidos.NewFrontend("kitfake", fakelang.Lang, inner.Syntax()).
				Version("1").Match("**/*.zz").
				Units(inner.Partition).Resolve(inner.Resolve).Build()
		}, "a missing parse is a defect")
		assert.Panics(t, func() {
			eidos.NewFrontend("kitfake", fakelang.Lang, inner.Syntax()).
				Version("1").Match("**/*.zz").
				Units(inner.Partition).Parse(inner.Parse).Build()
		}, "a missing resolve is a defect: silence is written, never defaulted")
	})
}
