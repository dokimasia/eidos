// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-go/backend"
)

// The suite holds the declaration total and the rendered findings
// against it; this twin pins the cells that distinguish Go.
func TestCoverage(t *testing.T) {
	t.Parallel()

	c := backend.Coverage()
	assert.Equal(t, c.Of(symbol.KindStruct, symbol.FactFinal), render.Holds,
		"final holds: nothing subclasses")
	assert.Equal(t, c.Of(symbol.KindStruct, symbol.FactImplements), render.Holds,
		"implements holds: satisfaction is structural")
	assert.Equal(t, c.Of(symbol.KindField, symbol.FactTag), render.Renders,
		"the field tag is Go's own idiom")
	assert.Equal(t, c.Of(symbol.KindMethod, symbol.FactThrows), render.Refuses,
		"throws refuses until the error-return lowering lands")
	assert.Equal(t, c.Of(symbol.KindField, symbol.FactValue), render.Refuses,
		"a field's initializer refuses")
	assert.Equal(t, c.Of(symbol.KindVariable, symbol.FactValue), render.Renders,
		"where a variable's renders")
}
