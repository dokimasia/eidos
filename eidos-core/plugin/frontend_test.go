// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"bytes"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// goSyntax is the fixture's comment forms: Go's line form and the
// C-family block with a star gutter.
func goSyntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line:   []string{"//"},
		Blocks: []plugin.CommentBlock{{Open: "/**", Close: "*/", Gutter: "*"}},
	}
}

// unitOf builds a source unit over the given tree, one file with
// one shared input, full depth.
func unitOf(tb assert.TB, tree fstest.MapFS) *plugin.SourceUnit {
	tb.Helper()

	return plugin.NewSourceUnit(
		[]plugin.SourceRef{{Path: "svc/store/row.go", Shared: []string{"go.mod"}}},
		tree, plugin.DepthFull, goSyntax(),
		diag.NewSink(), diag.Origin("golang"),
	)
}

// The source unit is the one door bytes enter a frontend through,
// so the jail, the fold and the comment pipeline are pinned here.
func TestSourceUnit(t *testing.T) {
	t.Parallel()

	tree := fstest.MapFS{
		"svc/store/row.go": {Data: []byte("package store\n")},
		"go.mod":           {Data: []byte("module svc\n")},
		"svc/other/x.go":   {Data: []byte("package other\n")},
	}

	t.Run("reads the unit's files and shared inputs alone", func(t *testing.T) {
		t.Parallel()

		u := unitOf(t, tree)
		b, err := u.Read("svc/store/row.go")
		assert.NoError(t, err, "a member file reads")
		assert.Equal(t, string(b), "package store\n", "whole")
		_, err = u.Read("go.mod")
		assert.NoError(t, err, "a declared shared input reads")
		_, err = u.Read("svc/other/x.go")
		assert.HasError(t, err, "a path outside the unit refuses")
		assert.Contains(t, err.Error(), "svc/other/x.go", "naming the path")
	})

	t.Run("folds every accepted read into the unit key", func(t *testing.T) {
		t.Parallel()

		before := unitOf(t, tree).ReadSum()
		u := unitOf(t, tree)
		_, err := u.Read("svc/store/row.go")
		assert.NoError(t, err, "the read succeeds")
		assert.False(t, bytes.Equal(before, u.ReadSum()),
			"a read changes the fold, so the cache knows what was seen")

		twin := unitOf(t, tree)
		_, err = twin.Read("svc/store/row.go")
		assert.NoError(t, err, "the twin reads the same bytes")
		assert.True(t, bytes.Equal(u.ReadSum(), twin.ReadSum()),
			"the same reads fold to the same key")
	})

	t.Run("strips comments through the syntax", func(t *testing.T) {
		t.Parallel()

		u := unitOf(t, tree)
		assert.Equal(t, u.Doc("// Row is one record.\n// It keys by name."),
			[]string{"Row is one record.", "It keys by name."},
			"line markers drop, one leading space tolerated")
		assert.Equal(t, u.Doc("/**\n * Row is one record.\n */"),
			[]string{"Row is one record."},
			"block delimiters and the star gutter drop, blank edges trimmed")
		assert.Equal(t, u.Doc("// Row is one record.\n//go:embed schema.sql"),
			[]string{"Row is one record."},
			"a directive line is not documentation")
		assert.Equal(t, u.DocLines([]string{"Clean.", "go:generate x"}),
			[]string{"Clean."},
			"already-clean lines pass the same directive rule")
	})

	t.Run("builds packages once per path, in first-touch order", func(t *testing.T) {
		t.Parallel()

		gb := unitOf(t, tree).Graph()
		a := gb.Package("svc/store")
		b := gb.Package("svc/api")
		assert.True(t, a == gb.Package("svc/store"), "one package per path")
		assert.Equal(t, len(gb.Packages()), 2, "both created")
		assert.True(t, gb.Packages()[0] == a && gb.Packages()[1] == b,
			"in first-touch order, which partition order fixed")

		file := symbol.Identity{Package: "svc/store", Name: "row.go"}
		gb.Scope(file, plugin.ImportScope{File: file, Bindings: map[string]string{"emit": "core/emit"}})
		scopes := gb.Scopes()
		assert.Length(t, scopes, 1, "the scope is recorded")
		assert.Equal(t, scopes[0].File, file, "under its file")
	})
}
