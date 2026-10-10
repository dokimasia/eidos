// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The claimed file's fixture: the data that the emitter attaches to its
// record, and the import that a template records.
const (
	recordData   = "sampled"
	testingPkg   = "testing"
	recordsRange = "{{range .Records}}"
	rangeEnd     = "{{end}}"
)

// A family that claims its files renders each file through its file
// template, over the file's records, inside the frame of the target's
// skeleton.
func TestClaim(t *testing.T) {
	t.Parallel()

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("renders a claimed file through the family's template over its records", func(t *testing.T) {
			t.Parallel()

			body, sink := renderClaim(t, claimTree(recordsRange+"case {{.Decl.Name}} {{.Data}}\n"+rangeEnd),
				claimedUnit(handleName, recordData, emit.Body{}))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, body, "case "+handleName+" "+recordData+"\n",
				"the template writes the file from the record and its data")
		})

		t.Run("renders nil data for a record that the plugin attached none to", func(t *testing.T) {
			t.Parallel()

			body, sink := renderClaim(t, claimTree(recordsRange+"{{if .Data}}set{{else}}unset{{end}}"+rangeEnd),
				claimedUnit(handleName, nil, emit.Body{}))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, body, "unset", "the record has no data")
		})

		t.Run("frames the template's output with the imports that the template records", func(t *testing.T) {
			t.Parallel()

			body, sink := renderClaim(t, claimTree(action(render.BuiltinUse, `"`+testingPkg+`"`)+"run\n"),
				claimedUnit(handleName, nil, emit.Body{}))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, body, "import ("+testingPkg+")\nrun\n", "the skeleton writes the imports above the output")
		})

		t.Run("renders the body of a record through the body builtin", func(t *testing.T) {
			t.Parallel()

			body, sink := renderClaim(t, claimTree(recordsRange+action(render.BuiltinBody, ".Decl")+rangeEnd),
				claimedUnit(handleName, nil, emit.Body{Stmts: []emit.Stmt{call(contentCall)}}))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, body, stmt(contentCall), "the kind machinery spells the record's statements")
		})

		t.Run("reports DroppedSlots for a record body whose slots the template does not place", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			b.Prologue.Append(call(proCall))
			tree := claimTree(recordsRange + "{{.Decl.Name}}" + rangeEnd)
			_, sink := renderClaim(t, tree, claimedUnit(handleName, nil, b))
			assert.Contains(t, reported(t, sink, render.DroppedSlots), "1 pending statements",
				"the finding counts the content that the file does not contain")
		})

		t.Run("reports nothing for a record body whose slot content the template places", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			b.Prologue.Append(call(proCall))
			_, sink := renderClaim(t, claimTree(recordsRange+action(render.BuiltinBody, ".Decl")+rangeEnd),
				claimedUnit(handleName, nil, b))
			coretest.AssertCodes(t, sink)
		})

		unresolved := []struct {
			name  string
			trees map[plugin.ID]fs.FS
			want  string
		}{
			{
				name:  "reports UnresolvedRef for a claim whose plugin declares no tree",
				trees: nil,
				want:  "no tree is declared",
			},
			{
				name: "reports UnresolvedRef for a claim whose tree does not contain the template",
				trees: map[plugin.ID]fs.FS{
					emitter: fstest.MapFS{otherTpl: &fstest.MapFile{Data: []byte("run\n")}},
				},
				want: "its tree does not contain",
			},
			{
				name:  "reports UnresolvedRef for a file template that does not parse",
				trees: claimTree("{{"),
				want:  "does not parse",
			},
		}
		for _, tt := range unresolved {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, sink := renderClaim(t, tt.trees, claimedUnit(handleName, nil, emit.Body{}))
				assert.Contains(t, reported(t, sink, render.UnresolvedRef), tt.want, "the finding names the reason")
			})
		}

		t.Run("reports RefusedTemplate for a file template that reads a missing key of a record", func(t *testing.T) {
			t.Parallel()

			_, sink := renderClaim(t, claimTree(recordsRange+"{{.Data."+ghostKey+"}}"+rangeEnd),
				claimedUnit(handleName, map[string]any{markKey: markData}, emit.Body{}))
			refused := reported(t, sink, render.RefusedTemplate)
			assert.Contains(t, refused, ghostKey, "the finding names the missing key")
		})

		t.Run("withholds a claimed file whose template fails", func(t *testing.T) {
			t.Parallel()

			body, _ := renderClaim(t, claimTree("{{"), claimedUnit(handleName, nil, emit.Body{}))
			assert.Equal(t, body, "", "the pass writes no file that the template did not write")
		})

		t.Run("reports MixedClaim for a file of a claimed unit and another unit", func(t *testing.T) {
			t.Parallel()

			beside := unitOf(weaverPlugin, storeKey, alphaName)
			body, sink := renderClaim(t, claimTree("run\n"), claimedUnit(handleName, nil, emit.Body{}), beside)
			coretest.AssertReports(t, sink, render.MixedClaim)
			assert.Equal(t, body, "", "the pass withholds the file")
		})
	})
}

// claimedUnit returns a per-source unit of the emitter's family that
// claims its files through [fileTpl], with one function record of the
// name and the body. A non-nil data is the data of the record's origin.
func claimedUnit(name string, data any, body emit.Body) plugin.Unit {
	u := fn(storeKey, name, body)
	u.Template = fileTpl
	if data != nil {
		u.Data = map[symbol.Identity]any{coretest.Struct(coretest.StorePath, name).ID: data}
	}
	return u
}

// claimTree returns the emitter's tree with the file template src under
// [fileTpl].
func claimTree(src string) map[plugin.ID]fs.FS {
	return map[plugin.ID]fs.FS{emitter: fstest.MapFS{fileTpl: &fstest.MapFile{Data: []byte(src)}}}
}

// renderClaim renders the units through the fixture language, with
// trees as the template trees of the plugins, and returns the first
// file's bytes beside the run's findings. An empty result means that the
// pass withheld the file.
func renderClaim(tb assert.TB, trees map[plugin.ID]fs.FS, units ...plugin.Unit) (string, *diag.Sink) {
	tb.Helper()

	p, err := render.New(passName, language())
	assert.NoError(tb, err, "the language composes")
	sink := diag.NewSink()
	e := seeded(tb, units...)
	files, err := p.Render(&plugin.RenderContext{
		Emit: e, Files: backendtest.Files(e, p), Trees: trees, Sink: sink, Plugin: passName,
	})
	assert.NoError(tb, err, "the pass renders every file")
	if len(files) == 0 {
		return "", sink
	}
	return string(files[0].Body), sink
}
