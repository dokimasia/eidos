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
	"go.dokimi.dev/eidos/core/internal/coretest"
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

// unitCode is a code for the source unit fixtures' findings.
var unitCode = diag.Code{Prefix: "tst", Number: 5}

// frontendOrigin is the origin the fixture unit reports under.
const frontendOrigin = diag.Origin("golang")

// unitFile is the one member the fixture unit declares, with the
// module file as its shared input.
func unitFile() plugin.SourceRef {
	return plugin.SourceRef{Path: "svc/store/row.go", Shared: []string{"go.mod"}}
}

// unitOf builds a source unit over the given tree, one file with
// one shared input, full depth.
func unitOf(tb assert.TB, tree fstest.MapFS) *plugin.SourceUnit {
	tb.Helper()

	u, _ := reporting(tb, tree)
	return u
}

// reporting builds the fixture unit together with the sink its
// findings arrive in, so a case reads back what the frontend
// reported rather than only that it returned.
func reporting(tb assert.TB, tree fstest.MapFS) (*plugin.SourceUnit, *diag.Sink) {
	tb.Helper()

	sink := diag.NewSink()
	return plugin.NewSourceUnit(
		[]plugin.SourceRef{unitFile()}, tree, plugin.DepthFull, goSyntax(),
		sink, frontendOrigin,
	), sink
}

// reported returns a sink's findings in report order, which is what
// a case comparing severities and origins reads.
func reported(s *diag.Sink) []diag.Diag {
	var out []diag.Diag
	for d := range s.All() {
		out = append(out, d)
	}
	return out
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

	t.Run("Files", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the members in partition order", func(t *testing.T) {
			t.Parallel()

			members := []plugin.SourceRef{
				{Path: "svc/store/row.go", Shared: []string{"go.mod"}},
				{Path: "svc/store/col.go"},
			}
			u := plugin.NewSourceUnit(
				members, tree, plugin.DepthFull, goSyntax(),
				diag.NewSink(), frontendOrigin,
			)
			assert.Equal(t, u.Files(), members,
				"the partition fixed the order, and the parse reads it back unchanged")
		})
	})

	t.Run("Depth", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the depth the unit loads at", func(t *testing.T) {
			t.Parallel()

			u := plugin.NewSourceUnit(
				[]plugin.SourceRef{unitFile()}, tree, plugin.DepthSignatures,
				goSyntax(), diag.NewSink(), frontendOrigin,
			)
			assert.Equal(t, u.Depth(), plugin.DepthSignatures,
				"signature-only loading and the full one share one code path, "+
					"and this is what tells them apart")
			assert.Equal(t, unitOf(t, tree).Depth(), plugin.DepthFull,
				"a unit loading everything says so too")
		})
	})

	t.Run("reports a declared file the tree does not hold", func(t *testing.T) {
		t.Parallel()

		u := plugin.NewSourceUnit(
			[]plugin.SourceRef{{Path: "svc/store/absent.go"}}, tree, plugin.DepthFull,
			goSyntax(), diag.NewSink(), frontendOrigin,
		)
		before := u.ReadSum()
		_, err := u.Read("svc/store/absent.go")
		assert.HasError(t, err, "the jail admits the path and the tree does not hold it")
		assert.Contains(t, err.Error(), "svc/store/absent.go", "naming the path")
		assert.True(t, bytes.Equal(u.ReadSum(), before),
			"a read that returned nothing folds nothing, so the key names "+
				"only the bytes the parse saw")
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

	t.Run("seeds the fold with the roster itself", func(t *testing.T) {
		t.Parallel()

		one := plugin.NewSourceUnit(
			[]plugin.SourceRef{{Path: "svc/store/row.go"}}, tree, plugin.DepthFull,
			goSyntax(), diag.NewSink(), diag.Origin("golang"),
		)
		wider := plugin.NewSourceUnit(
			[]plugin.SourceRef{{Path: "svc/store/row.go"}, {Path: "svc/other/x.go"}},
			tree, plugin.DepthFull, goSyntax(), diag.NewSink(), diag.Origin("golang"),
		)
		assert.False(t, bytes.Equal(one.ReadSum(), wider.ReadSum()),
			"a member no parse read still shapes the key")

		shared := plugin.NewSourceUnit(
			[]plugin.SourceRef{{Path: "svc/store/row.go", Shared: []string{"go.mod"}}},
			tree, plugin.DepthFull, goSyntax(), diag.NewSink(), diag.Origin("golang"),
		)
		assert.False(t, bytes.Equal(one.ReadSum(), shared.ReadSum()),
			"a declared shared input shapes it the same way")
	})

	t.Run("takes a comment apart into docs, carriers and annotations", func(t *testing.T) {
		t.Parallel()

		u := unitOf(t, tree)
		parts := u.Comment(
			"/**\n * Row is one record.\n * +gen:table name=rows\n * go:embed schema.sql\n */",
			position.Pos{File: "svc/store/row.go", Line: 3},
		)
		assert.Equal(t, parts.Docs, []string{"Row is one record.", "go:embed schema.sql"},
			"a block comment carries no directives, the way the toolchains read them")
		assert.Length(t, parts.Carriers, 1, "the carrier splits out")
		assert.Equal(t, parts.Carriers[0].Payload, "gen:table name=rows", "marker stripped")
		assert.Equal(t, parts.Carriers[0].Pos.Line, 5,
			"positioned at its own line inside the block")
		assert.Length(t, parts.Annotations, 0, "nothing lowers off a block")
	})

	t.Run("reads directives off adjacent line markers alone", func(t *testing.T) {
		t.Parallel()

		u := unitOf(t, tree)
		parts := u.Comment(
			"//go:embed schema.sql\n// go:generate reruns this\n//export CFunc\n// export is prose here",
			position.Pos{File: "svc/store/row.go", Line: 1},
		)
		assert.Equal(t, parts.Annotations, symbol.Annotations{
			{Name: "go:embed", Args: []string{"schema.sql"}},
			{Name: "export", Args: []string{"CFunc"}},
		}, "the colon form and the legacy space form both need the marker adjacent")
		assert.Equal(t, parts.Docs, []string{
			"go:generate reruns this", "export is prose here",
		}, "a spaced marker keeps prose as prose")
	})

	t.Run("opens a carrier only where a letter follows the mark", func(t *testing.T) {
		t.Parallel()

		u := unitOf(t, tree)
		parts := u.Comment(
			"// Options:\n// + item one\n// +1 point\n// +gen:table name=rows",
			position.Pos{File: "svc/store/row.go", Line: 1},
		)
		assert.Length(t, parts.Carriers, 1, "a bullet and a bare mark never carry intent")
		assert.Equal(t, parts.Carriers[0].Payload, "gen:table name=rows", "the real one does")
		assert.Equal(t, parts.Docs, []string{
			"Options:", "+ item one", "+1 point",
		}, "everything else stays documentation")
	})

	t.Run("folds a continued carrier into one payload", func(t *testing.T) {
		t.Parallel()

		u := unitOf(t, tree)
		parts := u.Comment(
			"// +gen:out user.go \\\n// plugin=buildergen \\\n// pkg=usersx\n// Trailing doc.",
			position.Pos{File: "svc/store/row.go", Line: 4},
		)
		assert.Length(t, parts.Carriers, 1, "the continuation folds")
		assert.Equal(t, parts.Carriers[0].Payload, "gen:out user.go plugin=buildergen pkg=usersx",
			"joined by single spaces, backslashes gone")
		assert.Equal(t, parts.Carriers[0].Pos.Line, 4, "at the opening line")
		assert.Equal(t, parts.Docs, []string{"Trailing doc."},
			"the continuation lines are spent")
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

	t.Run("keeps prose whose head is not a tool name", func(t *testing.T) {
		t.Parallel()

		u := unitOf(t, tree)
		parts := u.Comment(
			"// Row is one record.\n// Note:keyed by name.\n// go_embed:schema.sql",
			position.Pos{File: "svc/store/row.go", Line: 1},
		)
		assert.Equal(t, parts.Docs, []string{
			"Row is one record.", "Note:keyed by name.", "go_embed:schema.sql",
		}, "a tool name is lowercase alphanumeric throughout, so a capital "+
			"and an underscore each keep the line prose")
		assert.Length(t, parts.Annotations, 0, "nothing lowers as a directive")
	})

	t.Run("Errorf", func(t *testing.T) {
		t.Parallel()

		t.Run("files each severity under the frontend's origin", func(t *testing.T) {
			t.Parallel()

			u, sink := reporting(t, tree)
			at := position.Pos{File: "svc/store/row.go", Line: 4, Col: 2}
			u.Errorf(unitCode, at, "the declaration names %s twice", "Row")
			u.Warnf(unitCode, at, "the declaration shadows an import")
			u.Infof(unitCode, at, "the declaration loaded")

			coretest.AssertCodes(t, sink, unitCode, unitCode, unitCode)
			coretest.AssertPositioned(t, sink)
			got := reported(sink)
			assert.Equal(t,
				[]diag.Severity{got[0].Severity, got[1].Severity, got[2].Severity},
				[]diag.Severity{
					diag.SeverityError, diag.SeverityWarning, diag.SeverityInfo,
				},
				"each door reports at its own severity, and only the first fails a run")
			assert.True(t, sink.Failed(), "which the Error one does")
			for _, d := range got {
				assert.Equal(t, d.Origin, frontendOrigin,
					"every finding is filed under the frontend that reported it")
				assert.Equal(t, d.Pos, at, "at the position it was given")
			}
			assert.Equal(t, got[0].Msg, "the declaration names Row twice",
				"the format arguments reach the message")
		})
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

	t.Run("rehomes records onto the subject that stands", func(t *testing.T) {
		t.Parallel()

		gb := unitOf(t, tree).Graph()
		old := &node.Alias{Name: "Color"}
		standing := &node.Enum{Name: "Color"}
		other := &node.Struct{Name: "Row"}
		gb.Attach(old, directive.Raw{Name: "gen:stringer"})
		gb.Attach(other, directive.Raw{Name: "gen:table"})
		gb.Stamp(old, meta.RawStamp{Key: "fake.underlying", Value: "basic"})

		gb.Rehome(old, standing)
		assert.True(t, gb.Attachments()[0].Subject == symbol.Symbol(standing),
			"the attachment follows the replacement")
		assert.True(t, gb.Attachments()[1].Subject == symbol.Symbol(other),
			"an unrelated subject stays put")
		assert.True(t, gb.StampRecords()[0].Subject == symbol.Symbol(standing),
			"the stamp follows the same way")
		assert.Panics(t, func() { gb.Rehome(nil, standing) }, "a nil source is a defect")
		assert.Panics(t, func() { gb.Rehome(old, nil) }, "a nil destination is a defect")
	})
}
