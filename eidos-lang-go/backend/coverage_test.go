// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
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
	assert.Equal(t, c.Of(symbol.KindMethod, symbol.FactThrows), render.Renders,
		"an announced failure lowers into the error return")
	assert.Equal(t, c.Of(symbol.KindInterface, symbol.FactSealed), render.Refuses,
		"nothing seals")
	assert.Equal(t, c.Of(symbol.KindField, symbol.FactValue), render.Refuses,
		"a field's initializer refuses")
	assert.Equal(t, c.Of(symbol.KindVariable, symbol.FactValue), render.Renders,
		"where a variable's renders")
}
