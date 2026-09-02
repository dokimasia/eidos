// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"bytes"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// goSyntax is the fixture's comment forms: Go's line form and the
// C-family block with a star gutter, the tool-directive convention
// declared.
func goSyntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line:       []string{"//"},
		Blocks:     []plugin.CommentBlock{{Open: "/**", Close: "*/", Gutter: "*"}},
		Directives: true,
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

	t.Run("takes a comment apart into docs, carriers and annotations", func(t *testing.T) {
		t.Parallel()

		u := unitOf(t, tree)
		parts := u.Comment(
			"/**\n * Row is one record.\n * +gen:table name=rows\n * go:embed schema.sql\n */",
			position.Pos{File: "svc/store/row.go", Line: 3},
		)
		assert.Equal(t, parts.Docs, []string{"Row is one record."},
			"neither a carrier nor a directive is documentation")
		assert.Length(t, parts.Carriers, 1, "the carrier splits out")
		assert.Equal(t, parts.Carriers[0].Payload, "gen:table name=rows", "marker stripped")
		assert.Equal(t, parts.Carriers[0].Pos.Line, 5,
			"positioned at its own line inside the block")
		assert.Length(t, parts.Annotations, 1, "the directive lowers as an annotation")
		assert.Equal(t, parts.Annotations[0], symbol.Annotation{
			Name: "go:embed", Args: []string{"schema.sql"},
		}, "named without its marker, arguments split")
	})

	t.Run("mines no annotations for a language without the convention", func(t *testing.T) {
		t.Parallel()

		u := plugin.NewSourceUnit(
			[]plugin.SourceRef{{Path: "svc/store/row.go"}}, tree, plugin.DepthFull,
			plugin.CommentSyntax{Line: []string{"//"}},
			diag.NewSink(), diag.Origin("protobuf"),
		)
		parts := u.Comment(
			"// Row is one record.\n// go:embed schema.sql",
			position.Pos{File: "svc/store/row.go", Line: 1},
		)
		assert.Equal(t, parts.Docs, []string{"Row is one record.", "go:embed schema.sql"},
			"an undeclared convention keeps prose as prose")
		assert.Length(t, parts.Annotations, 0, "no annotation is invented")
		assert.Equal(t, u.DocLines([]string{"go:generate x"}), []string{"go:generate x"},
			"the doc filter holds to the same declaration")
	})

	t.Run("keeps a bare URL in the documentation", func(t *testing.T) {
		t.Parallel()

		u := unitOf(t, tree)
		parts := u.Comment(
			"// Row is one record.\n// https://example.test/spec\n// note: keyed by name.",
			position.Pos{File: "svc/store/row.go", Line: 1},
		)
		assert.Equal(t, parts.Docs, []string{
			"Row is one record.", "https://example.test/spec", "note: keyed by name.",
		}, "a URL's slash and a prose colon's space both fail the directive rule")
		assert.Length(t, parts.Annotations, 0, "nothing lowers as a directive")
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
	})

	t.Run("records bindings and attachments in record order", func(t *testing.T) {
		t.Parallel()

		gb := unitOf(t, tree).Graph()
		file := &node.File{Path: "svc/store/row.go"}
		gb.Scope(file, map[string]string{"emit": "core/emit"})
		scopes := gb.Scopes()
		assert.Length(t, scopes, 1, "the binding record is kept")
		assert.True(t, scopes[0].File == file, "under its file node")

		row := &node.Struct{Name: "Row"}
		gb.Attach(row, directive.Raw{Name: "gen:table"})
		attached := gb.Attachments()
		assert.Length(t, attached, 1, "the attachment is kept")
		assert.True(t, attached[0].Subject == symbol.Symbol(row), "on its subject")
		assert.Equal(t, attached[0].Raw.Name, directive.Name("gen:table"), "carrying the instance")

		gb.Stamp(file, meta.RawStamp{Key: "fake.testFile", Value: true})
		stamps := gb.StampRecords()
		assert.Length(t, stamps, 1, "the stamp is kept")
		assert.True(t, stamps[0].Subject == symbol.Symbol(file), "on its subject")

		assert.Panics(t, func() { gb.Scope(nil, nil) }, "a nil file is a defect")
		assert.Panics(t, func() { gb.Attach(nil, directive.Raw{}) }, "a nil subject is a defect")
		assert.Panics(t, func() { gb.Stamp(nil, meta.RawStamp{}) }, "a nil stamp subject is a defect")
	})
}
