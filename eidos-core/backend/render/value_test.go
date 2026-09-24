// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
)

// A value refusal is classified by its type, so the render reports
// it under its own code and every other refusal as a template
// refusal.
func TestValueError(t *testing.T) {
	t.Parallel()

	t.Run("RefuseValue returns a classifiable refusal naming the target", func(t *testing.T) {
		t.Parallel()

		err := render.RefuseValue("java", "no spelling for %s", "a nil map")
		assert.Equal(t, err.Error(), "java: no spelling for a nil map",
			"the target, then the reason")
		refused, is := errors.AsType[*render.ValueError](err)
		assert.True(t, is, "a caller reaches the classification through errors.As")
		assert.Equal(t, refused.Lang, "java", "naming the target that refused")
	})

	t.Run("a value the language cannot spell reports under its own code", func(t *testing.T) {
		t.Parallel()

		var b emit.Body
		b.Stmts = []emit.Stmt{unspellable()}
		files, sink := runPass(t, language(), seeded(t, fn("store.go", "Handle", b)))
		coretest.AssertCodes(t, sink, render.UnspeltValue)
		assert.Contains(t, reported(t, sink, render.UnspeltValue), "spells no value",
			"the refusal carries the language's own reason")
		assert.Equal(t, string(files[0].Body), "",
			"and the declaration is skipped, the way any refused spelling is")
	})

	t.Run("a refusal that is not a value stays a template refusal", func(t *testing.T) {
		t.Parallel()

		var b emit.Body
		b.Stmts = []emit.Stmt{refused()}
		_, sink := runPass(t, language(), seeded(t, fn("store.go", "Handle", b)))
		coretest.AssertCodes(t, sink, render.RefusedTemplate)
	})
}
