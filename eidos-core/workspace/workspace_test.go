// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"sync"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/workspace"
)

// fixtureBrand is the brand every fixture composition declares.
const fixtureBrand output.Brand = "fixture"

// fakeBackend is the smallest backend: a name and the target its
// plan resolves at composition. Nothing invokes it, which is the
// scope's own rule.
type fakeBackend struct {
	name   plugin.ID
	target plugin.Target
}

// Name returns the backend's declared name.
func (b fakeBackend) Name() plugin.ID { return b.name }

// Target returns the target the backend renders.
func (b fakeBackend) Target() plugin.Target { return b.target }

// quiet is an annotator handler stamping nothing.
func quiet(*eidos.StructMatch, *eidos.Stamper) error { return nil }

// stamper returns a facade-built annotator running h once per
// struct in scope.
func stamper(name plugin.ID, h func(*eidos.StructMatch, *eidos.Stamper) error) plugin.Annotator {
	p, held := eidos.NewPlugin(name).Handle(eidos.OnStruct(h)).Build().(plugin.Annotator)
	if !held {
		panic("workspace_test: a stamper rule lowers to the annotator role")
	}
	return p
}

// generator returns a facade-built generator running h once per
// struct in scope, emitting into the "gen" family.
func generator(name plugin.ID, h func(*eidos.StructMatch, *eidos.Emitter) error) plugin.Generator {
	p, held := eidos.NewPlugin(name).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(h)).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// mirrored appends one struct named after the subject: the handler
// every mirroring generator runs.
func mirrored(m *eidos.StructMatch, e *eidos.Emitter) error {
	e.PackageFile().Append(&emit.Struct{
		Origin: m.Struct.Identity(),
		Name:   "For" + m.Struct.Name,
	})
	return nil
}

// mirror returns a generator emitting one struct per subject.
func mirror(name plugin.ID) plugin.Generator {
	return generator(name, mirrored)
}

// caps spells a capability list inline.
func caps(cs ...plugin.Capability) []plugin.Capability { return cs }

// ordered returns an annotator placed by priority and capabilities,
// recording its runs into calls.
func ordered(
	name plugin.ID, pri int, provides, requires []plugin.Capability, calls *[]plugin.ID,
) plugin.Annotator {
	return stamperAt(name, pri, provides, requires,
		func(*eidos.StructMatch, *eidos.Stamper) error {
			*calls = append(*calls, name)
			return nil
		})
}

// stamperAt returns a facade-built annotator with its ordering
// inputs declared.
func stamperAt(
	name plugin.ID, pri int, provides, requires []plugin.Capability,
	h func(*eidos.StructMatch, *eidos.Stamper) error,
) plugin.Annotator {
	p, held := eidos.NewPlugin(name).
		Priority(plugin.RoleAnnotator, pri).
		Provides(provides...).
		Requires(requires...).
		Handle(eidos.OnStruct(h)).Build().(plugin.Annotator)
	if !held {
		panic("workspace_test: a stamper rule lowers to the annotator role")
	}
	return p
}

// planTo returns a plan running gens toward a backend naming target.
func planTo(name string, target plugin.Target, gens ...plugin.Generator) workspace.Plan {
	return workspace.Plan{
		Name:       name,
		Generators: gens,
		Backend:    fakeBackend{name: plugin.ID(name) + "-printer", target: target},
	}
}

// valid returns the smallest whole composition: the fixture brand,
// one annotator, one plan with one generator, one registered target.
func valid() *workspace.Builder {
	return workspace.New().
		Brand(fixtureBrand).
		Annotators(stamper("noter", quiet)).
		Targets("fixture").
		Plans(planTo("plan", "fixture", mirror("mirror")))
}

// alpha returns an unfrozen one-package graph with one positioned
// struct.
func alpha(tb assert.TB) (*store.Graph, *node.Struct) {
	tb.Helper()

	s := coretest.Struct(coretest.StorePath, "Alpha")
	s.Pos = position.Pos{File: "alpha.go", Line: 3, Col: 1}
	g := store.New()
	assert.NoError(tb, g.AddPackage(coretest.Package(coretest.StorePath, s)),
		"the fixture package is admitted")
	return g, s
}

// units collects an emit store's units in their total order.
func units(e *plugin.Emit) []plugin.Unit {
	var out []plugin.Unit
	for u := range e.Units() {
		out = append(out, u)
	}
	return out
}

// The workspace is the composition's frozen form: what Build
// returns survives any number of runs, and every run leaves its own
// report behind.
func TestWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("Brand", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the composition's brand", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			assert.Equal(t, w.Brand(), fixtureBrand, "the brand is the declared one")
		})
	})

	t.Run("Kernel", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the kernel's registered keys", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			assert.False(t, w.Kernel().IsZero(), "the handles name the kernel's keys")
		})
	})

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a report with the findings, the facts and one store per plan", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), g)
			assert.NoError(t, err, "the fixture run is clean")
			assert.NotNil(t, report.Sink, "the report has the findings")
			assert.NotNil(t, report.Facts, "the report has the arbitrated facts")
			assert.Length(t, report.Emits, 1, "the report has one store per plan")
			assert.Length(t, units(report.Emits["plan"]), 1, "the store has the mirrored unit")
		})

		t.Run("returns a store of its own to each concurrent run", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			reports := make([]*workspace.Report, 2)
			errs := make([]error, 2)
			var wg sync.WaitGroup
			for i := range reports {
				g, _ := alpha(t)
				wg.Go(func() {
					reports[i], errs[i] = w.Run(t.Context(), g)
				})
			}
			wg.Wait()
			for i := range reports {
				assert.NoError(t, errs[i], "each run is clean")
				assert.Length(t, units(reports[i].Emits["plan"]), 1, "each run has its own unit")
			}
		})
	})
}
