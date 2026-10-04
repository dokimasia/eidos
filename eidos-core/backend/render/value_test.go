// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"errors"
	"fmt"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

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

// refuseValueAllocs is one refusal: the formatted message and the
// error that carries it.
const refuseValueAllocs = 2

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

// A refusal allocates its message and its error, and its text allocates
// the joined string, in the ordinary run, which runs no benchmark.
func TestValueAllocs(t *testing.T) {
	var err error
	assert.MaxAllocs(t, func() { err = render.RefuseValue(refusingLang, refusalFormat, refusedValue) },
		refuseValueAllocs, "RefuseValue allocates the formatted message and the error")
	refusal, is := errors.AsType[*render.ValueError](err)
	assert.True(t, is, "RefuseValue returns a ValueError")
	var text string
	assert.MaxAllocs(t, func() { text = refusal.Error() }, 1, "Error allocates the joined text")
	assert.HasPrefix(t, text, refusingLang+": ", "Error names the target first")
}

// BenchmarkValue measures a scaffold's refusal of a value and the
// refusal's text, which the render reports under UnspeltValue.
func BenchmarkValue(b *testing.B) {
	b.Run("RefuseValue", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(refuseValueAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			err = render.RefuseValue(refusingLang, refusalFormat, refusedValue)
		}
		_, is := errors.AsType[*render.ValueError](err)
		assert.True(b, is, "RefuseValue returns a ValueError")
	})

	b.Run("ValueError.Error", func(b *testing.B) {
		refusal := &render.ValueError{Lang: refusingLang, Msg: refusedValue}
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got string
		for c.Loop() {
			got = refusal.Error()
		}
		assert.Equal(b, got, refusingLang+": "+refusedValue, "Error joins the target and the message")
	})
}
