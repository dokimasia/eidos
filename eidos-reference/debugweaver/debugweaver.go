// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package debugweaver

import (
	"go.dokimi.dev/eidos/reference/internal/callee"
	"go.dokimi.dev/eidos/reference/mockgen"
	"go.dokimi.dev/eidos/reference/repogen"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// ID is the name of the plugin.
const ID plugin.ID = "debugweaver"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// Capability is the capability that debugweaver provides. auditweaver
// requires it, so the trace of a method precedes its audit.
const Capability plugin.Capability = "trace"

// The defaults of the options: the function that logs, and the format of
// the entry that it logs.
const (
	DefaultPackage = "log"
	DefaultFunc    = "Printf"
	DefaultFormat  = "debug: %s entered"
)

// New returns debugweaver. On each method of each struct that the plan
// emits from a struct that repogen stamps, it appends
// <package>.<func>(<format>, "<T>.<M>") into the prologue. Those methods
// are the methods of <T>Repo, and the methods of the mock of <T>Repository
// where mockgen writes one. debugweaver runs after repogen and mockgen
// where the composition contains them, and requires neither. The options
// package, func and format set the call, and default to log.Printf with
// the format "debug: %s entered". Each call returns a new plugin, because
// a plugin instance belongs to one workspace.
func New() plugin.Generator {
	o := &callee.Log{Package: DefaultPackage, Func: DefaultFunc, Format: DefaultFormat}
	return sdk.NewPlugin(ID).
		Version(Version).
		Provides(Capability).
		After(repogen.Capability, mockgen.Capability).
		Options(o).
		Handle(sdk.Where(sdk.HasKey(meta.Named[bool](repogen.KeyRepository)), callee.Weave(o))).
		Generator()
}
