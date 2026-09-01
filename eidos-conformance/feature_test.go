// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontendtest"
	"go.dokimi.dev/eidos/core/load"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// A language whose canonical paths differ from the corpus
// convention states its own derivation, and the expectations
// follow it — the seam a real language's package canon comes
// through.
func TestFeature(t *testing.T) {
	t.Parallel()

	tree := fstest.MapFS{
		"f/struct_fields/a.zz": {Data: []byte(
			"package own/struct_fields\ntype Point int string\n",
		)},
	}
	sink := diag.NewSink()
	g, _, err := load.Load(context.Background(), load.Config{
		FS:        tree,
		Frontends: []plugin.Frontend{frontendtest.NewScripted()},
		Sink:      sink,
	})
	assert.NoError(t, err, "the rehomed tree loads")

	c := conformance.Corpus{
		Frontend:  frontendtest.NewScripted(),
		Sources:   tree,
		PackageOf: func(id string) string { return "own/" + id },
	}
	checked := false
	f := conformance.Feature{
		ID: "struct_fields",
		Declares: []conformance.Decl{{
			Name: "Point", Kind: symbol.KindStruct,
			Check: func(tb assert.TB, ctx *conformance.Ctx) {
				checked = true
				assert.Equal(tb, ctx.Pkg(""), "own/struct_fields",
					"the derivation is the corpus's own")
				assert.Equal(tb, ctx.Pkg("dep"), "own/struct_fields/dep",
					"a subpackage extends it")
			},
		}},
	}
	conformance.AssertFeature(t, c, g, f)
	assert.True(t, checked, "the expectation ran against the rehomed package")
}
