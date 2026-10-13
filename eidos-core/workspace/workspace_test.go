// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// fixtureBrand is the brand of every fixture composition.
const fixtureBrand output.Brand = "fixture"

// The names that the workspace derives for the target fixture.
// printerSuffix follows the spelling of a target in the name of the
// fake backend of the target, and fixtureLowering is the name of the
// lowering entry of the target fixture.
const (
	printerSuffix             = "-printer"
	fixtureLowering plugin.ID = "fixture-lowering"
)

// fakeBackend is the smallest backend, with a name and the target that
// its plan resolves at composition. Its settle keeps every construct and
// every name, and it declares no policy. The methods of a rendering
// backend belong to the nil interface that it embeds, so a test that
// renders through it panics.
type fakeBackend struct {
	plugin.Backend
	name   plugin.ID
	target plugin.Target
}

// Name returns the backend's declared name.
func (b fakeBackend) Name() plugin.ID { return b.name }

// Target returns the target the backend renders.
func (b fakeBackend) Target() plugin.Target { return b.target }

// Policies returns nil.
func (fakeBackend) Policies() []plugin.PolicySpec { return nil }

// Lower keeps the declaration.
func (fakeBackend) Lower(symbol.Symbol) ([]symbol.Symbol, error) { return nil, nil }

// Respell returns the name unchanged.
func (fakeBackend) Respell(_, _ symbol.Kind, _ symbol.Visibility, name string) (string, error) {
	return name, nil
}

// TestWorkspace tests the frozen form of a composition. Its accessors
// return what the composition declared and the keys that Build
// registered.
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

	t.Run("Directives", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the sealed registry of the composition", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			assert.True(t, w.Directives().Sealed(), "the registry is sealed")
		})

		t.Run("returns the schemas of the kernel and of the targets", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			var names []directive.Name
			for s := range w.Directives().Schemas() {
				names = append(names, s.Canonical())
			}
			expect.That(t, names).
				Contains(directive.KernelOut, "the registry has the out directive of the kernel").
				Contains(directive.Name("fixture"), "the registry has the directive of the target")
		})
	})
}

// TestWorkspaceAllocs checks that the accessors allocate nothing,
// because the ordinary run runs no benchmark.
func TestWorkspaceAllocs(t *testing.T) {
	w, err := valid().Build()
	assert.NoError(t, err, "the fixture composition is valid")
	var brand output.Brand
	assert.MaxAllocs(t, func() { brand = w.Brand() }, 0, "Brand allocates nothing")
	assert.Equal(t, brand, fixtureBrand, "Brand returns the composition's brand")
	var zero bool
	assert.MaxAllocs(t, func() { zero = w.Kernel().IsZero() }, 0, "Kernel allocates nothing")
	assert.False(t, zero, "Kernel returns the registered keys")
	var registry *directive.Registry
	assert.MaxAllocs(t, func() { registry = w.Directives() }, 0, "Directives allocates nothing")
	assert.NotNil(t, registry, "Directives returns the registry")
}

// BenchmarkWorkspace measures the accessors of a built workspace.
func BenchmarkWorkspace(b *testing.B) {
	w, err := valid().Build()
	assert.NoError(b, err, "the fixture composition is valid")

	b.Run("Brand", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got output.Brand
		for c.Loop() {
			got = w.Brand()
		}
		assert.Equal(b, got, fixtureBrand, "Brand returns the declared brand")
	})

	b.Run("Kernel", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got meta.KernelKeys
		for c.Loop() {
			got = w.Kernel()
		}
		assert.False(b, got.IsZero(), "Kernel returns the registered keys")
	})

	b.Run("Directives", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got *directive.Registry
		for c.Loop() {
			got = w.Directives()
		}
		assert.NotNil(b, got, "Directives returns the registry")
	})
}

// quiet is an annotator handler that stamps nothing.
func quiet(*eidos.StructMatch, *eidos.Stamper) error { return nil }

// stamper returns an annotator of the facade that runs h once for each
// struct in scope.
func stamper(name plugin.ID, h func(*eidos.StructMatch, *eidos.Stamper) error) plugin.Annotator {
	return eidos.NewPlugin(name).Handle(eidos.OnStruct(h)).Annotator()
}

// generator returns a generator of the facade that runs h once for each
// struct in scope and emits into the family gen.
func generator(name plugin.ID, h func(*eidos.StructMatch, *eidos.Emitter) error) plugin.Generator {
	return eidos.NewPlugin(name).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(h)).Generator()
}

// mirrored appends one struct named after the subject. Every mirroring
// generator runs it.
func mirrored(m *eidos.StructMatch, e *eidos.Emitter) error {
	e.PackageFile().Append(&emit.Struct{
		Origin: m.Struct.Identity(),
		Name:   "For" + m.Struct.Name,
	})
	return nil
}

// mirror returns a generator that emits one struct for each subject.
func mirror(name plugin.ID) plugin.Generator {
	return generator(name, mirrored)
}

// caps returns its capabilities as a list.
func caps(cs ...plugin.Capability) []plugin.Capability { return cs }

// ordered returns an annotator that its priority and its capabilities
// place, and that records its runs into calls.
func ordered(
	name plugin.ID, pri int, provides, requires []plugin.Capability, calls *[]plugin.ID,
) plugin.Annotator {
	return stamperAt(name, pri, provides, requires,
		func(*eidos.StructMatch, *eidos.Stamper) error {
			*calls = append(*calls, name)
			return nil
		})
}

// stamperAt returns an annotator of the facade with its priority and
// its capabilities.
func stamperAt(
	name plugin.ID, pri int, provides, requires []plugin.Capability,
	h func(*eidos.StructMatch, *eidos.Stamper) error,
) plugin.Annotator {
	return eidos.NewPlugin(name).
		Priority(plugin.RoleAnnotator, pri).
		Provides(provides...).
		Requires(requires...).
		Handle(eidos.OnStruct(h)).Annotator()
}

// planTo returns a plan that runs gens and renders through the fake
// backend of target. The plans of one target take equal backend values,
// so the composition admits the backend once, as a target renders
// through one backend.
func planTo(name string, target plugin.Target, gens ...plugin.Generator) workspace.Plan {
	return workspace.Plan{
		Name:       name,
		Generators: gens,
		Backend:    fakeBackend{name: plugin.ID(target) + printerSuffix, target: target},
	}
}

// valid returns the smallest whole composition. It has the fixture
// brand, one annotator, one plan with one generator and one registered
// target.
func valid() *workspace.Builder {
	return workspace.New().
		Brand(fixtureBrand).
		Annotators(stamper("noter", quiet)).
		Targets("fixture").
		Plans(planTo("plan", "fixture", mirror("mirror")))
}

// alpha returns an unfrozen graph of one package with one positioned
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

// units returns the units of an emit store in their total order.
func units(e *plugin.Emit) []plugin.Unit {
	var out []plugin.Unit
	for u := range e.Units() {
		out = append(out, u)
	}
	return out
}
