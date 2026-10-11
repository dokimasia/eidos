// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package tracegen

import (
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/reference/internal/callee"
	"go.dokimi.dev/eidos/reference/metricgen"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "tracegen"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// Capability is the capability that tracegen provides.
const Capability plugin.Capability = "http.trace"

// New returns tracegen. On each middleware function of a struct that
// handlergen serves, it appends h = <package>.<func>(h) into the named
// slot of the chain, after the statement of metricgen. The options package
// and func set the function that traces, and a config sets them. Each call
// returns a new plugin, because a plugin instance belongs to one
// workspace.
func New() plugin.Generator {
	o := &callee.Options{}
	g, _ := sdk.NewPlugin(ID).
		Version(Version).
		Provides(Capability).
		Requires(metricgen.Capability).
		Options(o).
		Handle(sdk.Where(sdk.HasKey(meta.Named[bool](handlergen.KeyHandler)),
			sdk.OnEmit(symbol.KindFunction, callee.Chain(o)))).
		Build().(plugin.Generator)
	return g
}
