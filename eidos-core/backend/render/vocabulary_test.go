// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The merged template vocabulary decides which helper a template
// calls, so the order the plugins' helpers fold in and what the merge
// refuses are contract.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	shouting := func() render.Language {
		l := language()
		l.Funcs = template.FuncMap{"shout": strings.ToUpper}
		l.Kinds[symbol.KindStruct] = "type {{shout .Name}} struct{}\n"
		return l
	}
	runMerged := func(
		t *testing.T, l render.Language, ctx *plugin.RenderContext,
	) (string, *diag.Sink) {
		t.Helper()
		pass, err := render.New("printer", l)
		assert.NoError(t, err, "the language composes")
		sink := diag.NewSink()
		ctx.Sink = sink
		ctx.Plugin = "printer"
		files, err := pass.Render(ctx)
		assert.NoError(t, err, "the pass runs whole")
		if len(files) == 0 {
			return "", sink
		}
		return string(files[0].Body), sink
	}

	t.Run("the shared vocabulary serves the kind templates", func(t *testing.T) {
		t.Parallel()

		body, sink := runMerged(t, shouting(), &plugin.RenderContext{
			Emit: seeded(t, unitOf("gen", "store.go", "Alpha")),
		})
		assert.False(t, sink.Failed(), "the shared helper is valid")
		assert.Contains(t, body, "type ALPHA struct{}",
			"registered once, called anywhere")
	})

	t.Run("a declared override replaces the helper everywhere", func(t *testing.T) {
		t.Parallel()

		body, sink := runMerged(t, shouting(), &plugin.RenderContext{
			Emit:      seeded(t, unitOf("gen", "store.go", "Alpha")),
			Schedule:  []plugin.ID{"gen", "styler"},
			Funcs:     map[plugin.ID]template.FuncMap{"styler": {"shout": strings.ToLower}},
			Overrides: map[plugin.ID][]string{"styler": {"shout"}},
		})
		assert.False(t, sink.Failed(), "the replace verb is declared and valid")
		assert.Contains(t, body, "type alpha struct{}",
			"the backend's own templates render through the override")
	})

	t.Run("the latest schedule position wins", func(t *testing.T) {
		t.Parallel()

		ctx := func(schedule ...plugin.ID) *plugin.RenderContext {
			return &plugin.RenderContext{
				Emit:     seeded(t, unitOf("gen", "store.go", "Alpha")),
				Schedule: schedule,
				Funcs: map[plugin.ID]template.FuncMap{
					"early": {"shout": func(s string) string { return "early_" + s }},
					"late":  {"shout": func(s string) string { return "late_" + s }},
				},
				Overrides: map[plugin.ID][]string{
					"early": {"shout"}, "late": {"shout"},
				},
			}
		}
		body, _ := runMerged(t, shouting(), ctx("early", "late"))
		assert.Contains(t, body, "late_Alpha",
			"the plugin that runs last changes how the construct renders")
		body, _ = runMerged(t, shouting(), ctx("late", "early"))
		assert.Contains(t, body, "early_Alpha",
			"and the order is the schedule's, not the map's")
	})

	t.Run("an undeclared shadow is refused and reported", func(t *testing.T) {
		t.Parallel()

		body, sink := runMerged(t, shouting(), &plugin.RenderContext{
			Emit:     seeded(t, unitOf("gen", "store.go", "Alpha")),
			Schedule: []plugin.ID{"gen"},
			Funcs:    map[plugin.ID]template.FuncMap{"gen": {"shout": strings.ToLower}},
		})
		assert.True(t, sink.Failed(), "a shadow without a declaration reports")
		assert.Contains(t, body, "type ALPHA struct{}",
			"and the shared helper stands")
	})

	t.Run("a plugin's own helper serves its reference", func(t *testing.T) {
		t.Parallel()

		var b emit.Body
		b.Ref = &emit.TemplateRef{Name: "method1.tpl", Data: map[string]any{"x": "go"}}
		body, sink := runMerged(t, shouting(), &plugin.RenderContext{
			Emit:     seeded(t, fn("store.go", "Handle", b)),
			Schedule: []plugin.ID{"gen"},
			Funcs: map[plugin.ID]template.FuncMap{"gen": {"mark": func(s string) string {
				return strings.ToUpper(s[:1]) + s[1:]
			}}},
			Trees: map[plugin.ID]fs.FS{
				"gen": fstest.MapFS{"method1.tpl": &fstest.MapFile{
					Data: []byte("\t{{mark .Data.x}}()\n{{slots}}"),
				}},
			},
		})
		assert.False(t, sink.Failed(), "a private helper is the plugin's own")
		assert.Contains(t, body, "\tGo()\n", "and its templates call it")
	})

	t.Run("a reserved name in the shared vocabulary is a fault", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Funcs = template.FuncMap{"body": strings.ToUpper}
		_, err := render.New("printer", l)
		assert.HasError(t, err, "the builtins' names are the pass's own")
		assert.Contains(t, err.Error(), "body", "naming the collision")
	})

	t.Run("a plugin the schedule does not hold still merges", func(t *testing.T) {
		t.Parallel()

		b := refBody()
		b.Ref.Data = map[string]any{"x": "go"}
		body, sink := runMerged(t, shouting(), &plugin.RenderContext{
			Emit:  seeded(t, fn("store.go", "Handle", b)),
			Funcs: map[plugin.ID]template.FuncMap{"gen": {"mark": strings.ToUpper}},
			Trees: refTree("\t{{mark .Data.x}}()\n{{slots}}"),
		})
		assert.False(t, sink.Failed(), "a helper needs no schedule position to merge")
		assert.Contains(t, body, "\tGO()\n",
			"and a fixture without a schedule still renders through it")
	})

	t.Run("a plugin claiming a builtin is refused", func(t *testing.T) {
		t.Parallel()

		body, sink := runMerged(t, shouting(), &plugin.RenderContext{
			Emit: seeded(t, fn("store.go", "Handle",
				emit.Body{Stmts: []emit.Stmt{call("content")}})),
			Schedule: []plugin.ID{"gen"},
			Funcs: map[plugin.ID]template.FuncMap{
				"gen": {render.BuiltinBody: strings.ToUpper},
			},
		})
		coretest.AssertCodes(t, sink, render.UndeclaredOverride)
		assert.Contains(t, body, "\tcontent()\n",
			"and the builtin stands, so the body still places its content")
	})

	t.Run("two plugins registering one helper collide", func(t *testing.T) {
		t.Parallel()

		b := refBody()
		b.Ref.Data = map[string]any{"x": "go"}
		body, sink := runMerged(t, shouting(), &plugin.RenderContext{
			Emit: seeded(t, fn("store.go", "Handle", b)),
			Funcs: map[plugin.ID]template.FuncMap{
				"alpha": {"mark": strings.ToUpper},
				"beta":  {"mark": strings.ToLower},
			},
			Trees: refTree("\t{{mark .Data.x}}()\n{{slots}}"),
		})
		coretest.AssertCodes(t, sink, render.HelperCollision)
		assert.Contains(t, body, "\tGO()\n",
			"the first registration in composition order stands")
	})
}
