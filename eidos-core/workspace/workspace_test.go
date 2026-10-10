// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
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

// A Workspace is the composition's frozen form. Its accessors return
// what the composition declared and the keys Build registered.
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
}

// The accessors allocate nothing in the ordinary run, which runs no
// benchmark.
func TestWorkspaceAllocs(t *testing.T) {
	w, err := valid().Build()
	assert.NoError(t, err, "the fixture composition is valid")
	var brand output.Brand
	assert.MaxAllocs(t, func() { brand = w.Brand() }, 0, "Brand allocates nothing")
	assert.Equal(t, brand, fixtureBrand, "Brand returns the composition's brand")
	var zero bool
	assert.MaxAllocs(t, func() { zero = w.Kernel().IsZero() }, 0, "Kernel allocates nothing")
	assert.False(t, zero, "Kernel returns the registered keys")
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
}

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
