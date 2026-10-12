// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package auditweaver

import (
	"go.dokimi.dev/eidos/reference/debugweaver"
	"go.dokimi.dev/eidos/reference/internal/callee"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "auditweaver"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// Capability is the capability that auditweaver provides.
const Capability plugin.Capability = "audit"

// AuditDirective is the directive that puts a struct or an interface
// under auditweaver.
const AuditDirective directive.Name = "audit"

// The defaults of the options: the function that logs, and the format of
// the entry that it logs.
const (
	DefaultPackage = "log"
	DefaultFunc    = "Printf"
	DefaultFormat  = "audit: %s"
)

// schema is the directive of auditweaver. Its rule runs on the methods that
// the plan emits, so the schema lists the kinds of their origins.
var schema = directive.Schema{
	Plugin: string(ID),
	Name:   AuditDirective,
	Kinds:  []symbol.Kind{symbol.KindStruct, symbol.KindInterface},
	Doc:    "logs the entry of each method that the plan emits from the declaration",
}

// New returns auditweaver. On each method with a receiver that the plan
// emits from a declaration under [AuditDirective], it appends
// <package>.<func>(<format>, "<T>.<M>") into the prologue, after the
// statement of debugweaver. The options package, func and format set the
// call, and default to log.Printf with the format "audit: %s". Each call
// returns a new plugin, because a plugin instance belongs to one
// workspace.
func New() plugin.Generator {
	o := &callee.Log{Package: DefaultPackage, Func: DefaultFunc, Format: DefaultFormat}
	return sdk.NewPlugin(ID).
		Version(Version).
		Provides(Capability).
		Requires(debugweaver.Capability).
		Options(o).
		Handle(sdk.Directive(schema, sdk.OnEmit(symbol.KindMethod, callee.Weave(o)))).
		Generator()
}
