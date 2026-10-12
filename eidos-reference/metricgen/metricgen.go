// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package metricgen

import (
	"go.dokimi.dev/eidos/reference/authgen"
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/reference/internal/callee"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "metricgen"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// Capability is the capability that metricgen provides. The next plugin of
// the middleware chain requires it, so its statement follows the one of
// metricgen.
const Capability plugin.Capability = "http.metrics"

// New returns metricgen. On each middleware function of a struct that
// handlergen serves, it appends h = <package>.<func>(h) into the named
// slot of the chain, after the statement of authgen. The options package
// and func set the function that measures, and a config sets them. Each
// call returns a new plugin, because a plugin instance belongs to one
// workspace.
func New() plugin.Generator {
	o := &callee.Options{}
	return sdk.NewPlugin(ID).
		Version(Version).
		Provides(Capability).
		Requires(authgen.Capability).
		Options(o).
		Handle(sdk.Where(sdk.HasKey(meta.Named[bool](handlergen.KeyHandler)),
			sdk.OnEmit(symbol.KindFunction, callee.Chain(o)))).
		Generator()
}
