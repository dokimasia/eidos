// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package auditgen

import (
	"go.dokimi.dev/eidos/reference/errorgen"
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/reference/internal/callee"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// ID is the name of the plugin.
const ID plugin.ID = "auditgen"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// New returns auditgen. On the method ServeHTTP of each handler that
// handlergen writes, it appends <package>.<func>(w, r) into the epilogue,
// after the call of errorgen. The options package and func set the
// function that records the request, and a config sets them. Each call
// returns a new plugin, because a plugin instance belongs to one
// workspace.
func New() plugin.Generator {
	o := &callee.Options{}
	return sdk.NewPlugin(ID).
		Version(Version).
		Requires(handlergen.Capability).
		After(errorgen.Capability).
		Options(o).
		Handle(sdk.Where(sdk.HasKey(meta.Named[bool](handlergen.KeyHandler)), callee.Epilogue(o))).
		Generator()
}
