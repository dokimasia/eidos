// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-rust/backend"
)

// The suite holds the declaration total and the rendered findings
// against it; this twin pins the cells that distinguish Rust.
func TestCoverage(t *testing.T) {
	t.Parallel()

	c := backend.Coverage()
	assert.Equal(t, c.Of(symbol.KindInterface, symbol.FactExtends), render.Renders,
		"a trait widens through supertraits")
	assert.Equal(t, c.Of(symbol.KindStruct, symbol.FactExtends), render.Refuses,
		"where a struct inherits nothing")
	assert.Equal(t, c.Of(symbol.KindTypeParam, symbol.FactConstParam), render.Renders,
		"const parameters are Rust's own")
	assert.Equal(t, c.Of(symbol.KindParam, symbol.FactVariadic), render.Refuses,
		"a function takes a fixed arity")
	assert.Equal(t, c.Of(symbol.KindEnumVariant, symbol.FactValue), render.Renders,
		"a variant's value is its discriminant")
	assert.Equal(t, c.Of(symbol.KindField, symbol.FactLevel), render.Refuses,
		"no statics inside types")
	assert.Equal(t, c.Of(symbol.KindInterface, symbol.FactTypes), render.Renders,
		"a trait's nested types are its associated types")
	assert.Equal(t, c.Of(symbol.KindStruct, symbol.FactTypes), render.Refuses,
		"and Rust nests nothing else")
}
