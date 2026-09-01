// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/symbol"
)

// RefusedFact reports a stated fact the backend declares no idiom
// for: the declaration renders without it, and the finding is what
// keeps the narrowing loud. It is a warning, because one neutral
// store feeds several targets and a fact idiomatic in one target
// must not withhold the declaration from another.
var RefusedFact = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 34, Meaning: "a stated fact has no spelling in the target language",
})

// UndeclaredFact reports a stated fact the backend's coverage
// takes no stance on. It is an error naming a defect in the
// backend's own declaration: a fact the schema grows reaches every
// backend as this finding until each declares a verdict.
var UndeclaredFact = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 35, Meaning: "the backend declares no coverage verdict for a stated fact",
})

// Verdict says what a backend does with one stated fact.
//
// The zero value is [VerdictUndeclared], so a fact left out of a
// coverage declaration reports as a defect rather than passing as
// a stance.
type Verdict uint8

const (
	// VerdictUndeclared means the backend has taken no stance.
	VerdictUndeclared Verdict = iota
	// Renders means a layer spells the fact into the output: a
	// template, a helper, the respell, a lowering or a grouping.
	Renders
	// Holds means the target's semantics state the fact already,
	// and the spelling is empty.
	Holds
	// Refuses means the target declares no idiom: the render
	// reports the stated fact and the declaration renders without
	// it.
	Refuses
)

// Coverage is a backend's declared fact coverage: one verdict per
// fact, with per-kind exceptions where a fact's verdict differs by
// its carrier. It is the feature table as data: the conformance
// suite holds a declaration total over [symbol.Facts] and asserts
// the rendered findings against it, and the render's guard reads
// the same data, so the two cannot drift apart.
type Coverage struct {
	// Facts holds the base verdict per fact.
	Facts map[symbol.Fact]Verdict
	// Except holds the verdicts that differ by carrier kind,
	// consulted before Facts: a field's initializer may refuse
	// where a variable's renders.
	Except map[symbol.Kind]map[symbol.Fact]Verdict
}

// Of returns the verdict for one fact stated on one kind:
// the kind's exception where one is declared, the base verdict
// otherwise, and [VerdictUndeclared] where neither speaks.
func (c Coverage) Of(kind symbol.Kind, fact symbol.Fact) Verdict {
	if v, held := c.Except[kind][fact]; held {
		return v
	}
	return c.Facts[fact]
}

// Declared reports whether the backend declared any coverage: an
// empty declaration disables the guard, which is what a backend
// predating the coverage contract renders under.
func (c Coverage) Declared() bool { return len(c.Facts) > 0 }

// Coverer is implemented by a renderer declaring its fact
// coverage, which is how the conformance suite reads it back.
type Coverer interface {
	Coverage() Coverage
}
