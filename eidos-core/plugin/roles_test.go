// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/plugin"
)

// annotator is the smallest annotator: it records the one context
// its call received.
type annotator struct {
	named
	got *plugin.AnnotatorContext
}

func (a *annotator) Annotate(ctx *plugin.AnnotatorContext) error {
	a.got = ctx
	return nil
}

// generator mirrors it for the generate role.
type generator struct {
	named
	got *plugin.GeneratorContext
}

func (g *generator) Generate(ctx *plugin.GeneratorContext) error {
	g.got = ctx
	return nil
}

// check mirrors it for the check role, reading the plans it names.
type check struct {
	named
	reads []string
	got   *plugin.CheckContext
}

func (c *check) Reads() []string { return c.reads }

func (c *check) Check(ctx *plugin.CheckContext) error {
	c.got = ctx
	return nil
}

// A role is Plugin plus one method taking a context struct, and the
// context is the whole surface a phase call may touch, so the
// invocation shape is contract.
func TestRoles(t *testing.T) {
	t.Parallel()

	t.Run("Annotator", func(t *testing.T) {
		t.Parallel()

		t.Run("receives the context it is invoked with", func(t *testing.T) {
			t.Parallel()

			a := &annotator{name: "classify"}
			var p plugin.Annotator = a

			ctx := &plugin.AnnotatorContext{Plugin: "classify", Bucket: 2}
			assert.NoError(t, p.Annotate(ctx), "the fixture call passes")
			assert.Equal(t, a.got, ctx, "the phase call touches exactly what its context carries", assert.ByIdentity())
		})
	})

	t.Run("Generator", func(t *testing.T) {
		t.Parallel()

		t.Run("receives the context it is invoked with", func(t *testing.T) {
			t.Parallel()

			g := &generator{name: "stubgen"}
			var p plugin.Generator = g

			ctx := &plugin.GeneratorContext{
				Plugin: "stubgen", Bucket: 1, Emit: plugin.NewEmit(),
			}
			assert.NoError(t, p.Generate(ctx), "the fixture call passes")
			assert.Equal(t, g.got, ctx, "the phase call touches exactly what its context carries", assert.ByIdentity())
		})

		t.Run("reads the exports of the plans it depends on through its context", func(t *testing.T) {
			t.Parallel()

			g := &generator{name: "bindings"}
			exports := map[string]plugin.ExportDoc{"stubs": {Plan: "stubs"}}
			assert.NoError(t, g.Generate(&plugin.GeneratorContext{Plugin: "bindings", Exports: exports}),
				"the fixture call passes")
			assert.Equal(t, g.got.Exports["stubs"].Plan, "stubs", "the export the context hands over")
		})
	})

	t.Run("WorkspaceCheck", func(t *testing.T) {
		t.Parallel()

		t.Run("receives the context it is invoked with", func(t *testing.T) {
			t.Parallel()

			c := &check{name: "stubbed", reads: []string{"stubs"}}
			var p plugin.WorkspaceCheck = c

			ctx := &plugin.CheckContext{Plugin: "stubbed", Plans: []plugin.PlanRecord{{Name: "stubs"}}}
			assert.NoError(t, p.Check(ctx), "the fixture call passes")
			assert.Equal(t, c.got, ctx, "the check touches exactly what its context carries", assert.ByIdentity())
		})

		t.Run("names the plans it reads", func(t *testing.T) {
			t.Parallel()

			var p plugin.WorkspaceCheck = &check{name: "stubbed", reads: []string{"stubs"}}
			assert.Equal(t, p.Reads(), []string{"stubs"}, "the plans the check reads")
		})
	})
}
