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

// The mirror fixture: its Go plan over TypeScript source, its generator,
// the generator's family word, and the import path of the tree's root,
// from which the Go backend derives the package of each file, because a
// TypeScript tree has no Go module.
const (
	mirrorPlan           = "go-mirror"
	mirrorID   plugin.ID = "gomirror"
	mirrorWord           = "mirror"
	mirrorBase           = "example.com/acme"
)

// ComposeMirror returns the mirror fixture's composition over root: the
// TypeScript frontend and rules, and one plan whose generator mirrors
// each TypeScript interface as a Go struct toward the Go backend, with a
// disk sink at root and the state directory's ledger under root. The
// generator translates the type of each property through the Emitter,
// so a type without a Go spelling reports RefusedType at its property.
//
// It returns the error of Build. The fixture's composition builds
// without one.
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

// mirror returns the mirror fixture's generator: for each TypeScript
// interface, a Go struct with a field for each property.
func mirror() plugin.Generator {
	return generator(eidos.NewPlugin(mirrorID).
		Output(plugin.Output{Per: plugin.PerSource, Word: mirrorWord}).
		Handle(eidos.OnInterface(mirrorInterface)).
		Build())
}

// mirrorInterface writes the Go struct of one TypeScript interface, with
// a field for each property under the property's name and with its type
// in Go. It withholds the struct where Go has no spelling of a
// property's type, after the Emitter reports the refusal.
func mirrorInterface(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	iface := m.Interface
	s := &emit.Struct{Origin: iface.ID, Doc: iface.Doc, Name: iface.Name, Visibility: iface.Visibility}
	for _, f := range iface.Fields {
		t, spelled := e.Type(f.ID, f.Type)
		if !spelled {
			return nil
		}
		s.Fields.Append(&emit.Field{Origin: f.ID, Doc: f.Doc, Name: f.Name, Visibility: f.Visibility, Type: t})
	}
	e.File().Append(s)
	return nil
}
