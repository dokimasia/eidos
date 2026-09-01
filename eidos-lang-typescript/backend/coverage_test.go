// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-typescript/backend"
)

// The suite holds the declaration total and the rendered findings
// against it; this twin pins the cells that distinguish
// TypeScript.
func TestCoverage(t *testing.T) {
	t.Parallel()

	c := backend.Coverage()
	assert.Equal(t, c.Of(symbol.KindField, symbol.FactTag), render.Refuses,
		"the field tag stays a Go idiom")
	assert.Equal(t, c.Of(symbol.KindTypeParam, symbol.FactVariance), render.Renders,
		"variance spells in and out")
	assert.Equal(t, c.Of(symbol.KindStruct, symbol.FactAnnotations), render.Renders,
		"decorators apply to classes")
	assert.Equal(t, c.Of(symbol.KindInterface, symbol.FactAnnotations), render.Refuses,
		"and to nothing else")
	assert.Equal(t, c.Of(symbol.KindParam, symbol.FactParamDefault), render.Refuses,
		"a parameter default refuses until its spelling lands")
}
