// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The merge's fixture: the plugins whose helpers fold in, the helper
// a reference template calls, and the data it spells.
const (
	// stylerPlugin declares an override of the shared helper.
	stylerPlugin plugin.ID = "styler"
	// earlyPlugin and latePlugin both override the shared helper.
	earlyPlugin plugin.ID = "early"
	latePlugin  plugin.ID = "late"
	// alphaPlugin and betaPlugin both register one private helper.
	alphaPlugin plugin.ID = "alpha"
	betaPlugin  plugin.ID = "beta"
	// markHelper is a helper of a plugin's own.
	markHelper = "mark"
	// markKey is the key of the reference data a template marks, and
	// markData the value under it.
	markKey  = "x"
	markData = "go"
	// contentCall is the statement a body places.
	contentCall = "content"
)

// The merged template vocabulary decides which helper a template
// calls, so the order the plugins' helpers fold in and what the merge
// refuses are contract.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	shouting := func() render.Language {
		l := language()
		l.Funcs = helpers(template.FuncMap{shoutHelper: strings.ToUpper})
		l.Kinds[symbol.KindStruct] = "type " + action(shoutHelper, ".Name") + " struct{}\n"
		return l
	}
	runMerged := func(
		t *testing.T, l render.Language, ctx *plugin.RenderContext,
	) (string, *diag.Sink) {
		t.Helper()

		pass, err := render.New(passName, l)
		assert.NoError(t, err, "the language composes")
		sink := diag.NewSink()
		ctx.Sink = sink
		ctx.Plugin = passName
		files, err := pass.Render(ctx)
		assert.NoError(t, err, "the pass renders every file")
		if len(files) == 0 {
			return "", sink
		}
		return string(files[0].Body), sink
	}
	// marked is a reference body whose template marks the data.
	marked := func() emit.Body {
		b := refBody()
		b.Ref.Data = map[string]any{markKey: markData}
		return b
	}
	markTree := refTree("\t" + action(markHelper, ".Data."+markKey) + "()\n" +
		action(render.BuiltinSlots))
	shouted := "type " + strings.ToUpper(alphaName) + " struct{}"

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a shared helper named after a builtin", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Funcs = helpers(template.FuncMap{render.BuiltinBody: strings.ToUpper})
			_, err := render.New(passName, l)
			assert.HasError(t, err, "the builtin names are the pass's own")
			assert.Contains(t, err.Error(), render.BuiltinBody, "naming the collision")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("renders a kind template through the shared vocabulary", func(t *testing.T) {
			t.Parallel()

			body, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit: seeded(t, unitOf(emitter, storeKey, alphaName)),
			})
			coretest.AssertCodes(t, sink)
			assert.Contains(t, body, shouted, "the shared helper spells the name")
		})

		t.Run("renders the language's templates through a declared override", func(t *testing.T) {
			t.Parallel()

			body, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit:      seeded(t, unitOf(emitter, storeKey, alphaName)),
				Schedule:  []plugin.ID{emitter, stylerPlugin},
				Funcs:     map[plugin.ID]template.FuncMap{stylerPlugin: {shoutHelper: strings.ToLower}},
				Overrides: map[plugin.ID][]string{stylerPlugin: {shoutHelper}},
			})
			coretest.AssertCodes(t, sink)
			assert.Contains(t, body, "type "+strings.ToLower(alphaName)+" struct{}",
				"the override replaces the shared helper in the kind template")
		})

		t.Run("renders through the override of the plugin scheduled last", func(t *testing.T) {
			t.Parallel()

			ctx := func(schedule ...plugin.ID) *plugin.RenderContext {
				return &plugin.RenderContext{
					Emit:     seeded(t, unitOf(emitter, storeKey, alphaName)),
					Schedule: schedule,
					Funcs: map[plugin.ID]template.FuncMap{
						earlyPlugin: {shoutHelper: func(s string) string { return string(earlyPlugin) + "_" + s }},
						latePlugin:  {shoutHelper: func(s string) string { return string(latePlugin) + "_" + s }},
					},
					Overrides: map[plugin.ID][]string{
						earlyPlugin: {shoutHelper}, latePlugin: {shoutHelper},
					},
				}
			}
			body, _ := runMerged(t, shouting(), ctx(earlyPlugin, latePlugin))
			assert.Contains(t, body, string(latePlugin)+"_"+alphaName,
				"the plugin scheduled last spells the name")
			body, _ = runMerged(t, shouting(), ctx(latePlugin, earlyPlugin))
			assert.Contains(t, body, string(earlyPlugin)+"_"+alphaName,
				"the schedule decides the order, not the map")
		})

		t.Run("reports UndeclaredOverride for a helper that shadows a shared name undeclared", func(t *testing.T) {
			t.Parallel()

			_, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit:     seeded(t, unitOf(emitter, storeKey, alphaName)),
				Schedule: []plugin.ID{emitter},
				Funcs:    map[plugin.ID]template.FuncMap{emitter: {shoutHelper: strings.ToLower}},
			})
			coretest.AssertCodes(t, sink, render.UndeclaredOverride)
		})

		t.Run("renders through the shared helper a shadow declares no override of", func(t *testing.T) {
			t.Parallel()

			body, _ := runMerged(t, shouting(), &plugin.RenderContext{
				Emit:     seeded(t, unitOf(emitter, storeKey, alphaName)),
				Schedule: []plugin.ID{emitter},
				Funcs:    map[plugin.ID]template.FuncMap{emitter: {shoutHelper: strings.ToLower}},
			})
			assert.Contains(t, body, shouted, "the shared helper spells the name")
		})

		t.Run("renders a reference template through the emitting plugin's helper", func(t *testing.T) {
			t.Parallel()

			capitalized := func(s string) string { return strings.ToUpper(s[:1]) + s[1:] }
			body, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit:     seeded(t, fn(storeKey, handleName, marked())),
				Schedule: []plugin.ID{emitter},
				Funcs:    map[plugin.ID]template.FuncMap{emitter: {markHelper: capitalized}},
				Trees:    markTree,
			})
			coretest.AssertCodes(t, sink)
			assert.Contains(t, body, "\t"+capitalized(markData)+"()\n",
				"the plugin's helper spells the call")
		})

		t.Run("merges the helpers of a plugin the schedule does not name", func(t *testing.T) {
			t.Parallel()

			body, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit:  seeded(t, fn(storeKey, handleName, marked())),
				Funcs: map[plugin.ID]template.FuncMap{emitter: {markHelper: strings.ToUpper}},
				Trees: markTree,
			})
			coretest.AssertCodes(t, sink)
			assert.Contains(t, body, "\t"+strings.ToUpper(markData)+"()\n",
				"a helper needs no schedule position to merge")
		})

		t.Run("reports UndeclaredOverride for a plugin helper named after a builtin", func(t *testing.T) {
			t.Parallel()

			_, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit: seeded(t, fn(storeKey, handleName,
					emit.Body{Stmts: []emit.Stmt{call(contentCall)}})),
				Schedule: []plugin.ID{emitter},
				Funcs: map[plugin.ID]template.FuncMap{
					emitter: {render.BuiltinBody: strings.ToUpper},
				},
			})
			coretest.AssertCodes(t, sink, render.UndeclaredOverride)
		})

		t.Run("renders through the builtin a plugin helper is named after", func(t *testing.T) {
			t.Parallel()

			body, _ := runMerged(t, shouting(), &plugin.RenderContext{
				Emit: seeded(t, fn(storeKey, handleName,
					emit.Body{Stmts: []emit.Stmt{call(contentCall)}})),
				Schedule: []plugin.ID{emitter},
				Funcs: map[plugin.ID]template.FuncMap{
					emitter: {render.BuiltinBody: strings.ToUpper},
				},
			})
			assert.Contains(t, body, "\t"+contentCall+"()\n", "the body builtin places the content")
		})

		t.Run("reports HelperCollision for a helper two plugins register", func(t *testing.T) {
			t.Parallel()

			_, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit: seeded(t, fn(storeKey, handleName, marked())),
				Funcs: map[plugin.ID]template.FuncMap{
					alphaPlugin: {markHelper: strings.ToUpper},
					betaPlugin:  {markHelper: strings.ToLower},
				},
				Trees: markTree,
			})
			coretest.AssertCodes(t, sink, render.HelperCollision)
		})

		t.Run("renders through the helper registered first in composition order", func(t *testing.T) {
			t.Parallel()

			body, _ := runMerged(t, shouting(), &plugin.RenderContext{
				Emit: seeded(t, fn(storeKey, handleName, marked())),
				Funcs: map[plugin.ID]template.FuncMap{
					alphaPlugin: {markHelper: strings.ToUpper},
					betaPlugin:  {markHelper: strings.ToLower},
				},
				Trees: markTree,
			})
			assert.Contains(t, body, "\t"+strings.ToUpper(markData)+"()\n",
				"the first plugin in name order keeps the helper")
		})
	})
}
