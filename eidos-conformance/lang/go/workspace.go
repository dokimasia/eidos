// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	gobackend "go.dokimi.dev/eidos/lang/go/backend"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// StubsPlan is the workspace fixture's producing plan: scoped to svc,
// it doubles every interface under the stub directive and exports the
// doubles.
const StubsPlan = "stubs"

// The workspace fixture's dependent plan, the sources of each plan, the
// registry's generator and family word, and the check's name.
const (
	registryPlan           = "registry"
	svcSources             = "./svc/..."
	adminSources           = "./admin/..."
	registrarID  plugin.ID = "registrar"
	registryWord           = "registry"
	stubbedID    plugin.ID = "stubbed"
)

// The prefix, number and meaning that [Unstubbed] registers.
const (
	unstubbedPrefix  diag.Prefix = "ACME"
	unstubbedNumber  int         = 1
	unstubbedMeaning string      = "an interface under the stub directive has no double that the stubs plan exports"
)

// Unstubbed is the code [Stubbed] reports under: an interface under the
// stub directive without a double that [StubsPlan] exports.
var Unstubbed = diag.MustRegister(unstubbedPrefix, diag.CodeSpec{
	Number:  unstubbedNumber,
	Meaning: unstubbedMeaning,
})

// Stubbed is the workspace fixture's check: it reads the record of
// [StubsPlan] and reports an Error at each interface under the stub
// directive from which the plan exports no double. It builds each key
// from the doubles generator's naming convention, the way a dependent
// knows a declaration before the producing plan runs. The zero value is
// the check.
type Stubbed struct{}

// Name returns the check's name, stubbed.
func (Stubbed) Name() plugin.ID { return stubbedID }

// Reads returns [StubsPlan], the one plan whose record the check reads.
func (Stubbed) Reads() []string { return []string{StubsPlan} }

// Check reports an Error under [Unstubbed] at each interface under the
// stub directive, under either spelling the directive's schema
// recognises, from which the plan it reads exports no double. It
// returns nil. The Error it reports fails the run.
func (Stubbed) Check(ctx *plugin.CheckContext) error {
	export := ctx.Plans[0].Export
	for _, spelled := range []directive.Name{stubSchema.Name, stubSchema.Canonical()} {
		for s := range ctx.Index.ByDirective(spelled) {
			iface, is := s.(*node.Interface)
			if !is {
				continue
			}
			key := plugin.ExportKey{Origin: iface.ID, Plugin: stubgenID, Name: doubleName(iface.Name)}
			if len(export.Find(key)) == 0 {
				ctx.Sink.Errorf(Unstubbed, iface.Position(), ctx.Plugin,
					"interface %s is under the stub directive, and plan %q exports no double of it",
					iface.Name, StubsPlan)
			}
		}
	}
	return nil
}

// ComposeWorkspace returns the workspace fixture's composition over
// root without its plans: the Go frontend and rules, a disk sink at
// root and the state directory's ledger under root. [WorkspacePlans]
// returns the plans it runs.
func ComposeWorkspace(root string) *workspace.Builder {
	return workspace.New().
		Brand(Brand).
		Frontends(gofrontend.New(nil)).
		Rules(gorules.New()).
		Targets(golang.Target).
		Output(func() (output.Sink, error) { return output.NewDisk(root, Brand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, Brand) })
}

// WorkspacePlans returns the workspace fixture's plans: [StubsPlan],
// scoped to svc, which doubles every interface under the stub
// directive, and the registry plan, scoped to admin, which depends on
// [StubsPlan] and aliases every double its export lists. extra joins
// the registry plan's generators. Both plans render through one Go
// backend, because a composition admits a plugin name twice only for
// the same provider.
func WorkspacePlans(extra ...plugin.Generator) []workspace.Plan {
	backend := gobackend.New()
	return []workspace.Plan{
		{
			Name:       StubsPlan,
			Sources:    workspace.Sources{Packages: []string{svcSources}},
			Generators: []plugin.Generator{doubles()},
			Backend:    backend,
		},
		{
			Name:       registryPlan,
			Sources:    workspace.Sources{Packages: []string{adminSources}},
			DependsOn:  []string{StubsPlan},
			Generators: append([]plugin.Generator{registrar()}, extra...),
			Backend:    backend,
		},
	}
}

// doubles returns the workspace fixture's stub generator: per interface
// under the stub directive, a double in the primary family, under the
// name doubleName gives it.
func doubles() plugin.Generator {
	return generator(eidos.NewPlugin(stubgenID).
		Output(plugin.Output{Per: plugin.PerSource, Word: stubWord}).
		Handle(eidos.Directive(stubSchema, eidos.OnInterface(func(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
			e.File().Append(double(doubleName(m.Interface.Name), m))
			return nil
		}))).
		Build())
}

// doubleName returns the name the doubles generator emits for the
// double of an interface: the stub word, then the interface's name, in
// the neutral convention. The Go settle respells it for a public
// declaration, so stubStore declares StubStore.
func doubleName(iface string) string { return stubWord + iface }

// registrar returns the registry plan's generator: for each struct of
// its scope, a type alias in the struct's package for each double
// [StubsPlan] exports, under the export's spelling and qualified with
// the export's package. The registry plan's scope declares one struct,
// and the rule takes no directive, so a composition without the plan
// still validates every directive of the tree.
func registrar() plugin.Generator {
	return generator(eidos.NewPlugin(registrarID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: registryWord}).
		Handle(eidos.OnStruct(register)).
		Build())
}

// register aliases each double of the stubs plan's export, reading its
// spelling and its import path from the export instead of applying the
// doubles generator's naming and the Go settle a second time.
func register(m *eidos.StructMatch, e *eidos.Emitter) error {
	export, _ := m.Export(StubsPlan)
	out := e.PackageFile()
	for _, s := range export.Symbols {
		if s.Plugin != stubgenID || s.Kind != symbol.KindStruct || s.Host != "" {
			continue
		}
		out.Append(&emit.Alias{
			Origin: m.Struct.ID,
			Name:   s.Spelling,
			Target: &emit.TypeRef{Spelling: s.Spelling, Package: s.Package.Package},
		})
	}
	return nil
}
