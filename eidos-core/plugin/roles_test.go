// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

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

// generator mirrors it for the generate seat.
type generator struct {
	named
	got *plugin.GeneratorContext
}

func (g *generator) Generate(ctx *plugin.GeneratorContext) error {
	g.got = ctx
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
			assert.True(t, a.got == ctx,
				"the phase call touches exactly what its context carries")
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
			assert.True(t, g.got == ctx,
				"the phase call touches exactly what its context carries")
		})
	})
}
