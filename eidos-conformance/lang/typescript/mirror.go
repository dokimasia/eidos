// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	tsfrontend "go.dokimi.dev/eidos/lang/typescript/frontend"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The Go plan of the mirror fixture over TypeScript source, its generator,
// the family word of the generator, and the import path of the root of the
// tree. The Go backend derives the package of each file from that import
// path, because a TypeScript tree has no Go module.
const (
	mirrorPlan           = "go-mirror"
	mirrorID   plugin.ID = "gomirror"
	mirrorWord           = "mirror"
	mirrorBase           = "example.com/acme"
)

// ComposeMirror returns the composition of the mirror fixture over root. It
// has the TypeScript frontend and rules, one plan whose generator mirrors
// each TypeScript interface as a Go struct toward the Go backend, a disk
// sink at root and the ledger of the state directory under root. The
// generator translates the type of each property through the Emitter, so a
// type without a Go spelling reports RefusedType at its property.
//
// ComposeMirror returns the error of Build, and the composition of the
// fixture builds without one.
func ComposeMirror(root string) (*workspace.Workspace, error) {
	return workspace.New().
		Brand(Brand).
		Frontends(tsfrontend.New()).
		Rules(tsrules.New()).
		Targets(golang.Target).
		Plans(workspace.Plan{
			Name:       mirrorPlan,
			Generators: []plugin.Generator{mirror()},
			Backend:    gobackend.New(),
			Layout:     layout.Config{ImportBase: mirrorBase},
		}).
		Output(func() (output.Sink, error) { return output.NewDisk(root, Brand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, Brand) }).
		Build()
}

// mirror returns the generator of the mirror fixture. It writes a Go struct
// with a field for each property of each TypeScript interface.
func mirror() plugin.Generator {
	return eidos.NewPlugin(mirrorID).
		Output(plugin.Output{Per: plugin.PerSource, Word: mirrorWord}).
		Handle(eidos.OnInterface(mirrorInterface)).
		Generator()
}

// mirrorInterface writes the Go struct of one TypeScript interface, with
// a field for each property under the name of the property and with its
// type in Go. It withholds the struct where Go has no spelling of the type
// of a property, after the Emitter reports the refusal.
func mirrorInterface(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	iface := m.Interface
	fields, spelled := e.Fields(iface.Fields, nil)
	if !spelled {
		return nil
	}
	e.File().Append(&emit.Struct{
		Origin:     iface.ID,
		Doc:        iface.Doc,
		Name:       iface.Name,
		Visibility: iface.Visibility,
		Fields:     emit.SlotOf(fields...),
	})
	return nil
}
