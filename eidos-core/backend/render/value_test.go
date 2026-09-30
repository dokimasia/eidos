// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"errors"
	"fmt"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
)

// The refusal's fixture: the target that refuses a value, and the
// reason it states.
const (
	refusingLang  = "java"
	refusalFormat = "no spelling for %s"
	refusedValue  = "a nil map"
)

// valueUnit returns a unit of one struct and one function whose body
// states s, so a skipped function leaves the struct to render.
func valueUnit(s emit.Stmt) plugin.Unit {
	return besideAlpha(fn(storeKey, handleName, emit.Body{Stmts: []emit.Stmt{s}}))
}

// A value refusal is classified by its type, so the render reports
// it under its own code and every other refusal as a template
// refusal.
func TestValue(t *testing.T) {
	t.Parallel()

	t.Run("RefuseValue", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error naming the target before the reason", func(t *testing.T) {
			t.Parallel()

			err := render.RefuseValue(refusingLang, refusalFormat, refusedValue)
			assert.Equal(t, err.Error(), refusingLang+": "+fmt.Sprintf(refusalFormat, refusedValue),
				"the target, then the reason")
		})

		t.Run("returns a ValueError naming the target", func(t *testing.T) {
			t.Parallel()

			err := render.RefuseValue(refusingLang, refusalFormat, refusedValue)
			refused, is := errors.AsType[*render.ValueError](err)
			assert.True(t, is, "a caller reads the classification through errors.As")
			assert.Equal(t, refused.Lang, refusingLang, "naming the target that refused")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("reports UnspeltValue for a value the language cannot spell", func(t *testing.T) {
			t.Parallel()

			_, sink := runPass(t, language(), seeded(t, valueUnit(unspellable())))
			coretest.AssertCodes(t, sink, render.UnspeltValue)
		})

		t.Run("reports the language's reason for a value it cannot spell", func(t *testing.T) {
			t.Parallel()

			_, sink := runPass(t, language(), seeded(t, valueUnit(unspellable())))
			assert.Contains(t, reported(t, sink, render.UnspeltValue), "spells no value",
				"the finding states the language's own reason")
		})

		t.Run("skips a declaration whose value the language cannot spell", func(t *testing.T) {
			t.Parallel()

			files, _ := runPass(t, language(), seeded(t, valueUnit(unspellable())))
			assert.Equal(t, string(files[0].Body), "type "+alphaName+" struct{}\n",
				"the file renders without it, the way any refused spelling does")
		})

		t.Run("reports RefusedTemplate for a refusal that is not a value", func(t *testing.T) {
			t.Parallel()

			_, sink := runPass(t, language(), seeded(t, valueUnit(refused())))
			coretest.AssertCodes(t, sink, render.RefusedTemplate)
		})
	})
}
