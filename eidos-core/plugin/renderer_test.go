// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/plugin"
)

// The fixture's names: the target the tree serves, one it does not,
// and the template the tree contains.
const (
	treedTarget   plugin.Target = "fixture"
	otherTarget   plugin.Target = "elsewhere"
	treedTemplate               = "method1.tpl"
)

// valueless is a fixture renderer returning files as values.
type valueless struct {
	named
}

// Render returns one file as a value.
func (valueless) Render(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
	return []plugin.RenderedFile{{Path: "svc/store_stub.go", Body: []byte("package store\n")}}, nil
}

// treed is a fixture plugin declaring a template tree for one
// target and no helpers.
type treed struct {
	named
}

// Templates returns the one tree for the fixture target.
func (treed) Templates(t plugin.Target) (fs.FS, bool) {
	if t != treedTarget {
		return nil, false
	}
	return fstest.MapFS{treedTemplate: &fstest.MapFile{Data: []byte("{{slots}}")}}, true
}

// TemplateTargets returns the fixture target alone.
func (treed) TemplateTargets() []plugin.Target { return []plugin.Target{treedTarget} }

// TemplateFuncs returns no helpers.
func (treed) TemplateFuncs(plugin.Target) template.FuncMap { return nil }

// Overrides returns no replaced names.
func (treed) Overrides(plugin.Target) []string { return nil }

// The render seam keeps rendering out of the composition's hands: a
// renderer returns values, and the provider surfaces declare the
// trees references resolve in.
func TestRenderer(t *testing.T) {
	t.Parallel()

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("returns files as values", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = valueless{name: "printer"}
			r, held := p.(plugin.Renderer)
			assert.True(t, held, "the render surface asserts")
			files, err := r.Render(&plugin.RenderContext{})
			assert.NoError(t, err, "the fixture renders")
			assert.Length(t, files, 1, "one file is returned")
			assert.Equal(t, files[0].Path, "svc/store_stub.go", "the file has its routed path")
		})

		t.Run("is absent from a plugin that renders nothing", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = named{name: "bare"}
			_, held := p.(plugin.Renderer)
			assert.False(t, held, "the render surface is opt-in")
		})
	})

	t.Run("RenderedFile", func(t *testing.T) {
		t.Parallel()

		derived := plugin.RenderedFile{
			Path:    "svc/store_stub.go",
			Plugins: []plugin.ID{"acme-audit", "stubgen"},
			Sources: []string{"svc/session.go", "svc/store.go"},
			Body:    []byte("package store\n"),
		}

		t.Run("lists every emitter that contributed", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, derived.Plugins, []plugin.ID{"acme-audit", "stubgen"},
				"the emitters are distinct and sorted")
		})

		t.Run("lists every routing key it derives from", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, derived.Sources, []string{"svc/session.go", "svc/store.go"},
				"units sharing a name assemble one file")
		})

		t.Run("lists no source for a plan file", func(t *testing.T) {
			t.Parallel()

			plan := plugin.RenderedFile{Path: "registry.go", Body: []byte("package p\n")}
			assert.Length(t, plan.Sources, 0, "a plan file derives from nothing")
		})
	})

	t.Run("Templates", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the tree for a declared target", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = treed{name: "shaper"}
			tp, held := p.(plugin.TemplateProvider)
			assert.True(t, held, "the template surface asserts")
			tree, declared := tp.Templates(treedTarget)
			assert.True(t, declared, "the declared target returns its tree")
			_, err := fs.Stat(tree, treedTemplate)
			assert.NoError(t, err, "the tree contains the referenced template")
		})

		t.Run("reports false for an undeclared target", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = treed{name: "shaper"}
			tp, held := p.(plugin.TemplateProvider)
			assert.True(t, held, "the template surface asserts")
			_, declared := tp.Templates(otherTarget)
			assert.False(t, declared, "no tree serves the other target")
		})
	})

	t.Run("TemplateTargets", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the targets with trees of their own", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = treed{name: "shaper"}
			tp, held := p.(plugin.TemplateProvider)
			assert.True(t, held, "the template surface asserts")
			assert.Equal(t, tp.TemplateTargets(), []plugin.Target{treedTarget},
				"the fixture target is the one declared")
		})
	})
}
