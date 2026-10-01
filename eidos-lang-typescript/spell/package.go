// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"path"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Package names the package a TypeScript file at a routed path
// declares. Every file is a module of its own, so its package is its
// [typescript.ModulePath], named after the path's last element, the
// way the TypeScript frontend names a module it loads. Every path
// derives a package, so Package never returns an error.
func Package(p plugin.Placement) (symbol.Identity, error) {
	module := typescript.ModulePath(p.Path)
	return symbol.Identity{
		Lang: typescript.Lang, Package: module, Name: path.Base(module), Kind: symbol.KindPackage,
	}, nil
}
