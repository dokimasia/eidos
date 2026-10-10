// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"errors"

	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Package names the package a Java file at a routed path declares: the
// package its declarations derive from. A Java file states its package
// in its clause, and one package may be written under several source
// roots, so the path does not decide it. A plan file derives from no
// package, and Package returns an error for it. It allocates nothing
// for a file with an origin.
func Package(p plugin.Placement) (symbol.Identity, error) {
	if p.Origin.IsZero() {
		return symbol.Identity{}, errors.New(
			"spell: a plan file derives from no package, and a Java file declares its declarations' package")
	}
	return p.Origin, nil
}
