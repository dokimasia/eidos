// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin_test

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/plugin"
)

// valueless is a fixture renderer returning files as values.
type valueless struct {
	named
}

func (valueless) Render(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
	return []plugin.RenderedFile{{Name: "store_stub.go", Body: []byte("package store\n")}}, nil
}

// treed is a fixture plugin declaring a template tree for one
// target.
type treed struct {
	named
}

func (treed) Templates(t plugin.Target) (fs.FS, bool) {
	if t != "fixture" {
		return nil, false
	}
	return fstest.MapFS{"method1.tpl": &fstest.MapFile{Data: []byte("{{slots}}")}}, true
}

func (treed) TemplateFuncs(plugin.Target) template.FuncMap { return nil }
func (treed) Overrides() []string                          { return nil }

// The render seam is the contract that keeps rendering out of the
// composition's hands: a renderer returns values, and the provider
// surfaces declare the trees references resolve in.
func TestRenderer(t *testing.T) {
	t.Parallel()

	t.Run("a renderer returns files as values", func(t *testing.T) {
		t.Parallel()

		var p plugin.Plugin = valueless{name: "printer"}
		r, held := p.(plugin.Renderer)
		assert.True(t, held, "the render surface asserts")
		files, err := r.Render(&plugin.RenderContext{})
		assert.NoError(t, err, "the fixture renders")
		assert.Length(t, files, 1, "returning its files")
		assert.Equal(t, files[0].Name, "store_stub.go",
			"each carrying its target-spelled name")
	})

	t.Run("a bare plugin renders nothing", func(t *testing.T) {
		t.Parallel()

		var p plugin.Plugin = named{name: "bare"}
		_, held := p.(plugin.Renderer)
		assert.False(t, held, "the render surface is opt-in")
	})

	t.Run("a template provider declares per target", func(t *testing.T) {
		t.Parallel()

		var p plugin.Plugin = treed{name: "shaper"}
		tp, held := p.(plugin.TemplateProvider)
		assert.True(t, held, "the template surface asserts")
		tree, declared := tp.Templates("fixture")
		assert.True(t, declared, "the declared target returns its tree")
		_, err := fs.Stat(tree, "method1.tpl")
		assert.NoError(t, err, "holding the referenced template")
		_, declared = tp.Templates("elsewhere")
		assert.False(t, declared, "an undeclared target returns nothing")
	})
}
