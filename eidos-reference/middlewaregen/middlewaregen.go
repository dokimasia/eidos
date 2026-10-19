// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package middlewaregen

import (
	"embed"
	"fmt"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "middlewaregen"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// Capability is the capability that middlewaregen provides. A plugin that
// appends into the chain of a middleware function requires it.
const Capability plugin.Capability = "http.middleware"

// ChainSlot is the named slot of the body of a middleware function. Each
// statement of the slot wraps the handler in one middleware.
const ChainSlot = "chain"

// Word is the word of the family of the middleware functions.
const Word = "middleware"

// The template of the body of a middleware function, the parameter and the
// result type of the function, and the format of its documentation. The
// format takes the names of the function and of the struct.
const (
	bodyTemplate = "middleware.tmpl"
	handlerParam = "h"
	handlerType  = "http.Handler"
	httpPackage  = "net/http"
	functionDoc  = "%s wraps h in the middleware of %s."
)

// templates are the templates of middlewaregen, below the directory of each
// target.
//
//go:embed templates
var templates embed.FS

// New returns middlewaregen. For each struct that handlergen serves, it
// writes the function <T>Middleware(h http.Handler) http.Handler into the
// family [Word], beside the source of the struct. The template
// middleware.tmpl writes the body of the function: the prologue, the named
// slot [ChainSlot] and the epilogue, and then the return of h. A plugin of
// the chain appends a statement h = <package>.<func>(h) into the slot. The
// plugin serves the target golang alone. Each call returns a new plugin,
// because a plugin instance belongs to one workspace.
func New() plugin.Generator {
	return sdk.NewPlugin(ID).
		Version(Version).
		Provides(Capability).
		Requires(handlergen.Capability).
		Output(plugin.Output{Per: plugin.PerSource, Word: Word}).
		For(golang.Target, sdk.TemplatesOf(templates, golang.Target)).
		Handle(sdk.Where(sdk.HasKey(meta.Named[bool](handlergen.KeyHandler)), sdk.OnStruct(generate))).
		Generator()
}

// generate writes the middleware function of one struct. It declares the
// named slot [ChainSlot] on the body of the function, and the template
// places the slot.
func generate(m *sdk.StructMatch, e *sdk.Emitter) error {
	s := m.Struct
	name := e.JoinName(Word, s.Name)
	handler := &emit.TypeRef{Spelling: handlerType, Package: httpPackage}
	fn := &emit.Function{
		Origin:     s.ID,
		Doc:        []string{fmt.Sprintf(functionDoc, name, s.Name)},
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Params:     emit.SlotOf(&emit.Param{Name: handlerParam, Type: handler}),
		Returns:    emit.SlotOf(&emit.Return{Type: handler}),
	}
	fn.Body.Declare(ChainSlot)
	fn.Body.Ref = e.Ref(bodyTemplate, nil)
	e.File().Append(fn)
	return nil
}
