// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The plugins of the probing composition, and the capability that puts
// the tagger in the bucket after the looker. The pointer is the generator
// whose directive resolves a reference.
const (
	lookerID  plugin.ID         = "looker"
	taggerID  plugin.ID         = "tagger"
	pointerID plugin.ID         = "pointer"
	looked    plugin.Capability = "looked"
)

// The row with one field, which the looker does not sample, and the
// pointer's directive on the row, which resolves Col.
const (
	narrowRow  = "type Row int\n"
	pointAtCol = "+pointer:at to=Col\n"
)

// The generators of the edge cases: the enumerator counts the structs of
// its plan's scope, and the packager takes the api package whole.
const (
	enumeratorID plugin.ID = "enumerator"
	packagerID   plugin.ID = "packager"
)

// The lines of the edge cases: the directives of the enumerator and the
// packager, which attach to the struct on the line before them, and a
// struct that an edit adds to the api package.
const (
	countLine   = "+enumerator:count\n"
	sumLine     = "+packager:sum\n"
	accountLine = "type Account int\n"
)

// pointerSchema is the pointer's directive. Its one param resolves a
// struct of the subject's package, so its validation reads the struct.
var pointerSchema = directive.Schema{
	Plugin: string(pointerID), Name: "at",
	Params: []directive.ParamSpec{{
		Key: "to", Type: directive.TypeReference, Resolution: directive.ResolveCallableInScope,
		Doc: "the struct the directive points at",
	}},
	Doc: "points at a struct of the same package",
}

// The directives of the enumerator and the packager. Each generator's one
// rule runs on each struct with its directive.
var (
	countSchema = directive.Schema{
		Plugin: string(enumeratorID), Name: "count", Doc: "counts the structs of the plan's scope",
	}
	sumSchema = directive.Schema{
		Plugin: string(packagerID), Name: "sum", Doc: "counts the declarations of the api package",
	}
)

// apiPackage is the api package of the rounds tree, which the packager
// takes whole.
var apiPackage = symbol.Identity{Lang: frontendtest.ScriptedLang, Package: "svc/api", Kind: symbol.KindPackage}

// A warm run reads the records that each dirty edge routes, and probes
// each candidate for its own annotator invocations, so it executes again
// what an edit changed and withdraws what the edit removed. A membership
// edge routes a plan's reader only where the plan's sources admit a
// package in which the membership changed, and a package edge routes the
// reader that took the package whole.
func TestDirty(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		before := warmTree(rowLine, colLine, readerLine)

		t.Run("validates again a subject whose validation read an edge that the edit made dirty", func(t *testing.T) {
			t.Parallel()

			w := built(t, pointing(t, ledger.NewMem()))
			report, err := warmAfter(t, w, warmTree(rowLine, pointAtCol, readerLine, markedLine, colLine),
				warmTree(rowLine, pointAtCol, readerLine, markedLine, widerCol))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Stats.Validated, 1, "the run validates the row alone")
		})

		t.Run("validates again a subject whose validation read a removed declaration", func(t *testing.T) {
			t.Parallel()

			w := built(t, pointing(t, ledger.NewMem()))
			report, err := warmAfter(t, w, warmTree(rowLine, pointAtCol, readerLine, markedLine, colLine),
				warmTree(rowLine, pointAtCol, readerLine, markedLine))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the reference to the removed struct fails the run")
			assert.Length(t, findings(report.Sink, directive.UnresolvedReference), 1,
				"the row's directive no longer resolves")
		})

		t.Run("runs again an invocation that looked up a declaration that the edit changed", func(t *testing.T) {
			t.Parallel()

			var tag meta.Key[bool]
			w := built(t, probing(t, ledger.NewMem(), &tag))
			report, err := warmAfter(t, w, before, warmTree(widerRow, colLine, readerLine))
			assert.NoError(t, err, "the run is clean")
			sample, held := meta.Get(report.Facts, readerID, w.Kernel().Sample)
			assert.True(t, held, "the looker samples the reader")
			assert.Equal(t, sample, "3", "the sample is the new field count of the row")
		})

		t.Run("withdraws the claims of an invocation whose fact gate no longer admits its subject", func(t *testing.T) {
			t.Parallel()

			var tag meta.Key[bool]
			w := built(t, probing(t, ledger.NewMem(), &tag))
			report, err := warmAfter(t, w, before, warmTree(narrowRow, colLine, readerLine))
			assert.NoError(t, err, "the run is clean")
			_, held := meta.Get(report.Facts, readerID, tag)
			assert.False(t, held, "the reader loses its tag with its sample")
		})

		t.Run("runs a fact-gated rule on a subject whose fact appeared", func(t *testing.T) {
			t.Parallel()

			var tag meta.Key[bool]
			w := built(t, probing(t, ledger.NewMem(), &tag))
			report, err := warmAfter(t, w, warmTree(narrowRow, colLine, readerLine), before)
			assert.NoError(t, err, "the run is clean")
			_, held := meta.Get(report.Facts, readerID, tag)
			assert.True(t, held, "the reader gains a tag with its sample")
		})

		t.Run("runs no enumeration of a plan whose sources exclude the package of a new struct", func(t *testing.T) {
			t.Parallel()

			w := built(t, roundsBuilder(t, ledger.NewMem(), workspace.Sources{Packages: []string{"svc/store"}},
				enumerating(t)))
			report, err := warmAfter(t, w, roundsTree(rowLine+readerLine+countLine, userLine),
				roundsTree(rowLine+readerLine+countLine, userLine+accountLine))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, generatedBy(report, enumeratorID), 0, "the enumerator keeps its count of the store package")
		})

		t.Run("runs no enumeration of a plan whose sources exclude a package that an edit removes", func(t *testing.T) {
			t.Parallel()

			w := built(t, roundsBuilder(t, ledger.NewMem(), workspace.Sources{Packages: []string{"svc/store"}},
				enumerating(t)))
			after := roundsTree(rowLine+readerLine+countLine, userLine)
			delete(after, apiSource)
			report, err := warmAfter(t, w, roundsTree(rowLine+readerLine+countLine, userLine), after)
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, generatedBy(report, enumeratorID), 0, "the enumerator keeps its count of the store package")
		})

		t.Run("runs again an enumeration of a plan whose sources admit the package of a new struct",
			func(t *testing.T) {
				t.Parallel()

				w := built(t, roundsBuilder(t, ledger.NewMem(), workspace.Sources{Packages: []string{"svc/store"}},
					enumerating(t)))
				report, err := warmAfter(t, w, roundsTree(rowLine+readerLine+countLine, userLine),
					roundsTree(rowLine+readerLine+countLine+colLine, userLine))
				assert.NoError(t, err, "the run is clean")
				assert.Equal(t, generatedBy(report, enumeratorID), 1, "the enumerator counts the store package again")
			})

		t.Run("runs again a reader that took a package whole after an edit to one of its members", func(t *testing.T) {
			t.Parallel()

			w := built(t, roundsBuilder(t, ledger.NewMem(), workspace.Sources{}, packaging(t)))
			report, err := warmAfter(t, w, roundsTree(rowLine+readerLine+sumLine, userLine),
				roundsTree(rowLine+readerLine+sumLine, widerUser))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, generatedBy(report, packagerID), 1, "the packager runs on the reader once")
		})
	})
}

// pointing returns the builder of a composition over the scripted
// frontend that records into a ledger, with the scripted rules. Its plan
// runs the pointer, whose directive resolves a struct, and the marker.
func pointing(tb assert.TB, l ledger.Ledger) *workspace.Builder {
	tb.Helper()

	pointer, held := eidos.NewPlugin(pointerID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.Directive(pointerSchema, eidos.OnStruct(
			func(*eidos.StructMatch, *eidos.Emitter) error { return nil },
		))).Build().(plugin.Generator)
	assert.True(tb, held, "the pointer lowers to the generator role")
	marker, held := eidos.NewPlugin(markerID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.Directive(markSchema, eidos.OnStruct(mirrored))).Build().(plugin.Generator)
	assert.True(tb, held, "the marker lowers to the generator role")
	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Rules(rulestest.Scripted()).
		Targets("fixture").
		Plans(workspace.Plan{
			Name: "plan", Generators: []plugin.Generator{pointer, marker}, Backend: printer(tb, "fixture"),
		}).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return l, nil })
}

// probing returns the builder of a composition over the scripted
// frontend that records into a ledger. The looker looks the row up from
// the reader. While the row has two fields or more, it stamps the field
// count of the row as the kernel's sample on the reader. The tagger runs
// in the next bucket on each struct that has a sample, and stamps the tag
// that it registers into tag. The tagger's fact gate takes the kernel's
// handles from a fresh registry, which registers the kernel's keys first
// as the composition does, so both name the same keys.
func probing(tb assert.TB, l ledger.Ledger, tag *meta.Key[bool]) *workspace.Builder {
	tb.Helper()

	kernel, err := meta.Kernel(meta.NewRegistry())
	assert.NoError(tb, err, "the kernel's keys register")
	looker, held := eidos.NewPlugin(lookerID).
		Provides(looked).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
			if m.Struct.Name != readerID.Name {
				return nil
			}
			row, _ := m.Reader().Lookup(rowID)
			if n := fieldCount(row); n >= 2 {
				eidos.Stamp(st, m.Kernel().Sample, strconv.Itoa(n))
			}
			return nil
		})).Build().(plugin.Annotator)
	assert.True(tb, held, "the looker lowers to the annotator role")
	tagger, held := eidos.NewPlugin(taggerID).
		Requires(looked).
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace("tag"); err != nil {
				return err
			}
			k, err := meta.Register[bool](r, meta.KeySpec{Name: "tag.sampled", Doc: "marks a sampled struct"})
			*tag = k
			return err
		}).
		Handle(eidos.Where(eidos.HasKey(kernel.Sample), eidos.OnStruct(
			func(_ *eidos.StructMatch, st *eidos.Stamper) error {
				eidos.Stamp(st, *tag, true)
				return nil
			},
		))).Build().(plugin.Annotator)
	assert.True(tb, held, "the tagger lowers to the annotator role")
	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Annotators(looker, tagger).
		Targets("fixture").
		Plans(workspace.Plan{Name: "plan", Generators: []plugin.Generator{mirror(mirrorID)}, Backend: printer(tb, "fixture")}).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return l, nil })
}

// enumerating returns the enumerator. On each struct with the count
// directive, it counts the structs of its plan's scope, and mirrors the
// struct with the count as its documentation.
func enumerating(tb assert.TB) plugin.Generator {
	tb.Helper()

	p, held := eidos.NewPlugin(enumeratorID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "count"}).
		Handle(eidos.Directive(countSchema, eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			n := 0
			for range m.Reader().ByKind(symbol.KindStruct) {
				n++
			}
			e.PackageFile().Append(&emit.Struct{
				Origin: m.Struct.Identity(), Name: "Count" + m.Struct.Name, Doc: []string{strconv.Itoa(n)},
			})
			return nil
		}))).Build().(plugin.Generator)
	assert.True(tb, held, "the enumerator lowers to the generator role")
	return p
}

// packaging returns the packager. On each struct with the sum directive,
// it counts the declarations of the api package, which it takes whole. It
// mirrors the struct with the count as its documentation.
func packaging(tb assert.TB) plugin.Generator {
	tb.Helper()

	p, held := eidos.NewPlugin(packagerID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "sum"}).
		Handle(eidos.Directive(sumSchema, eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			n := 0
			if pkg, found := m.Reader().PackageOf(apiPackage); found {
				for _, f := range pkg.Files {
					n += len(f.Decls)
				}
			}
			e.PackageFile().Append(&emit.Struct{
				Origin: m.Struct.Identity(), Name: "Sum" + m.Struct.Name, Doc: []string{strconv.Itoa(n)},
			})
			return nil
		}))).Build().(plugin.Generator)
	assert.True(tb, held, "the packager lowers to the generator role")
	return p
}
