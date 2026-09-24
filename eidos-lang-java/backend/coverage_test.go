// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The suite checks the declaration total and the rendered findings
// against it, and this twin pins the cells that distinguish Java.
func TestCoverage(t *testing.T) {
	t.Parallel()

	c := backend.Coverage()
	assert.Equal(t, c.Of(symbol.KindMethod, symbol.FactThrows), render.Renders,
		"the throws clause is Java's own")
	assert.Equal(t, c.Of(symbol.KindMethod, symbol.FactMultiReturn), render.Refuses,
		"a callable returns one value")
	assert.Equal(t, c.Of(symbol.KindEnumVariant, symbol.FactValue), render.Refuses,
		"a valued constant takes the constructor form")
	assert.Equal(t, c.Of(symbol.KindEnum, symbol.FactMethods), render.Renders,
		"an enum declares behaviour")
	assert.Equal(t, c.Of(symbol.KindSum, symbol.FactMethods), render.Refuses,
		"where a sum's variant classes would owe bodies")
	assert.Equal(t, c.Of(symbol.KindStruct, symbol.FactTypes), render.Renders,
		"nested types render at member depth")
	assert.Equal(t, c.Of(symbol.KindInterface, symbol.FactSealed), render.Renders,
		"sealing is Java's own")
	assert.Equal(t, c.Of(symbol.KindStruct, symbol.FactLevel), render.Renders,
		"a member class spells static, and the lowering refuses it at file scope")
	assert.Equal(t, c.Of(symbol.KindInterface, symbol.FactProperties), render.Renders,
		"an interface's properties render as its constants")
}
