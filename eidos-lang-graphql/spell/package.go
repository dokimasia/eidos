// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Package names the package of a GraphQL file at a routed path. A GraphQL
// schema has one namespace, so every file of a plan has one package: the
// package of the language without a path and a name. The layout then
// qualifies no reference, and a file has no import. Package does not read
// its placement, returns no error and does not allocate.
func Package(plugin.Placement) (symbol.Identity, error) {
	return symbol.Identity{Lang: graphql.Lang, Kind: symbol.KindPackage}, nil
}
