// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Feature is one neutral capability of the declaration surface:
// something every corpus tree spells in its own language, or
// declares it cannot.
type Feature struct {
	// ID names the feature: lowercase with underscores, because it
	// doubles as a directory and package segment in every corpus
	// language.
	ID string

	// Doc states what the feature exercises.
	Doc string

	// Declares names what every loading spelling must put in the
	// graph, under the feature's package.
	Declares []Decl

	// Check optionally holds the whole feature to a graph-wide
	// expectation — a classification stamp, an attachment — with a
	// nil declaration in the context.
	Check func(tb assert.TB, c *Ctx)
}

// Decl is one expected declaration, language-neutral: the identity
// minus the language and the package, which the runner derives
// through the corpus convention.
type Decl struct {
	// Sub places the declaration in a subpackage of the feature,
	// empty for the feature's root.
	Sub string

	// Owner is the dotted chain of enclosing type names, empty at
	// the top level.
	Owner string

	// Name is the declared name every language's spelling shares.
	Name string

	// Kind is the declaration's kind.
	Kind symbol.Kind

	// Disc is a callable's discriminator: the parameter type
	// spellings, comma-joined, empty elsewhere.
	Disc string

	// Check optionally holds the loaded declaration's shape.
	Check func(tb assert.TB, c *Ctx)
}

// Ctx is what one expectation checks against.
type Ctx struct {
	// Lang is the language under test.
	Lang symbol.Lang

	// Decl is the looked-up declaration, nil for a feature-level
	// check.
	Decl symbol.Symbol

	// Graph is the sealed graph the corpus loaded into.
	Graph *store.Graph

	// Pkg derives a feature-relative package path through the
	// corpus's own convention: the feature's root for "", a
	// subpackage otherwise.
	Pkg func(sub string) string
}
