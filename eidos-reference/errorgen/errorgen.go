// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package errorgen

import (
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/reference/internal/callee"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "errorgen"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// Capability is the capability that errorgen provides.
const Capability plugin.Capability = "http.error"

// Priority is the priority of the generator of errorgen. It runs after
// every generator of priority 0, and before auditgen, so its call is the
// first of the epilogue of ServeHTTP.
const Priority = 1

// New returns errorgen. On the method ServeHTTP of each handler that
// handlergen writes, it appends <package>.<func>(w, r) into the epilogue.
// The options package and func set the function that recovers the request,
// and a config sets them. Each call returns a new plugin, because a plugin
// instance belongs to one workspace.
func New() plugin.Generator {
	o := &callee.Options{}
	return sdk.NewPlugin(ID).
		Version(Version).
		Priority(plugin.RoleGenerator, Priority).
		Provides(Capability).
		Requires(handlergen.Capability).
		Options(o).
		Handle(sdk.Where(sdk.HasKey(meta.Named[bool](handlergen.KeyHandler)),
			sdk.OnEmit(symbol.KindMethod, callee.Epilogue(o)))).
		Generator()
}
