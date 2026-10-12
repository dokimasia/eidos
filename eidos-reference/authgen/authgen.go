// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package authgen

import (
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/reference/internal/callee"
	"go.dokimi.dev/eidos/reference/middlewaregen"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "authgen"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// Capability is the capability that authgen provides. The next plugin of
// the middleware chain requires it, so its statement follows the one of
// authgen.
const Capability plugin.Capability = "http.auth"

// New returns authgen. On each middleware function of a struct that
// handlergen serves, it appends h = <package>.<func>(h) into the named
// slot of the chain. The options package and func set the function that
// authenticates, and a config sets them. Each call returns a new plugin,
// because a plugin instance belongs to one workspace.
func New() plugin.Generator {
	o := &callee.Options{}
	return sdk.NewPlugin(ID).
		Version(Version).
		Provides(Capability).
		Requires(middlewaregen.Capability).
		Options(o).
		Handle(sdk.Where(sdk.HasKey(meta.Named[bool](handlergen.KeyHandler)),
			sdk.OnEmit(symbol.KindFunction, callee.Chain(o)))).
		Generator()
}
