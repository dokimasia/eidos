// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The scripted language stands in for five real ones, so its own
// lowering is pinned: what it declares, binds, stamps and refuses.
func TestFrontend(t *testing.T) {
	t.Parallel()

	tree := fstest.MapFS{
		"svc/a.zz": {Data: []byte(
			"package svc\nimport api dep\ntype A api.B int\nmethod Get int\nconst low\nstamp fake.testFile yes\n+gen:table name=t\n",
		)},
	}

	t.Run("partitions by directory and parses the statements", func(t *testing.T) {
		t.Parallel()

		f := frontendtest.NewScripted()
		parts, err := f.Partition(context.Background(),
			[]plugin.SourceRef{{Path: "svc/a.zz"}}, reader{tree})
		assert.NoError(t, err, "the partition groups")
		assert.Length(t, parts, 1, "one directory, one unit")

		u := plugin.NewSourceUnit(parts[0], tree, plugin.DepthFull,
			f.Syntax(), diag.NewSink(), f.Name())
		assert.NoError(t, f.Parse(context.Background(), u), "the unit parses")

		gb := u.Graph()
		assert.Length(t, gb.Packages(), 1, "one package declared")
		file := gb.Packages()[0].Files[0]
		assert.Length(t, file.Decls, 2, "a type and a constant")
		assert.Length(t, gb.Scopes(), 1, "the bindings recorded")
		assert.Length(t, gb.Attachments(), 1, "the directive recorded")
		assert.Length(t, gb.StampRecords(), 1, "the stamp recorded")
	})

	t.Run("resolves through bindings and leaves builtins", func(t *testing.T) {
		t.Parallel()

		f := frontendtest.NewScripted()
		scope := plugin.ImportScope{
			File:     symbol.Identity{Lang: frontendtest.ScriptedLang, Package: "svc"},
			Bindings: map[string][]string{"api": {"dep"}},
		}
		got := f.Resolve(scope, "api.B")
		assert.Length(t, got, 1, "a bound alias probes its package")
		assert.Equal(t, got[0].Package, "dep", "at the bound path")
		assert.Length(t, f.Resolve(scope, "Own"), 1, "a bare capital probes its own package")
		assert.Empty(t, f.Resolve(scope, "int"), "a builtin is nobody's")
	})
}

// reader is the recorded partition door over a test tree.
type reader struct {
	tree fstest.MapFS
}

// Read returns one file's bytes.
func (r reader) Read(path string) ([]byte, error) {
	return r.tree.ReadFile(path)
}
