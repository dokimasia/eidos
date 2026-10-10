// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/cli"
	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The parts that the described composition adds to the fixture: an
// annotator that stamps a flag on each struct, the plan bindings with its
// generator, which depends on the plan of the mirror, and a check that reads
// the plan of the mirror.
const (
	flaggerName  plugin.ID    = "flagger"
	binderName   plugin.ID    = "binder"
	listerName   plugin.ID    = "lister"
	bindingsPlan              = "bindings"
	flagKey      meta.KeyName = "cli.flag"
)

// refinedConfig refines the sources and the layout of the plan of the
// mirror.
const refinedConfig = "version: 1\nplans:\n    mirrors:\n" +
	"        sources: {lang: fake, packages: [./svc/...], module: example.test/svc}\n" +
	"        layout: {policy: centralised, dir: gen, importBase: example.test/gen}\n"

// widthConfig selects the choice wide of the width policy, which the
// printer of the policed composition declares.
const widthConfig = "version: 1\npolicies:\n    text.width: wide\n"

// storeID is the struct Store of the package svc, which the lister looks up.
var storeID = symbol.Identity{Lang: frontendtest.ScriptedLang, Package: "svc", Name: "Store", Kind: symbol.KindStruct}

// widthPolicy is a lowering policy of the text target.
var widthPolicy = plugin.PolicySpec{
	Key: "text.width", Choices: []plugin.Choice{"narrow", "wide"}, Default: "narrow", Doc: "the width of a line",
}

// lister is a workspace check that reads the plan of the mirror. It looks
// the store up and ranges over the structs, so its record reads the
// declaration of the store and the structs as a kind.
type lister struct{}

// Name returns the name of the check.
func (lister) Name() plugin.ID { return listerName }

// Reads returns the plan of the mirror.
func (lister) Reads() []string { return []string{planName} }

// Check looks the store up, ranges over the structs and returns nil.
func (lister) Check(ctx *plugin.CheckContext) error {
	ctx.Reader.Lookup(storeID)
	for range ctx.Reader.ByKind(symbol.KindStruct) {
	}
	return nil
}

// memberEvent is an event of JSON output, with the member of a list that the
// event is about.
type memberEvent struct {
	Workspace string `json:"workspace"`
}

// Plan writes the composition of each workspace and runs nothing.
func TestPlan(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("writes each frontend, annotator, plan and check as text", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(version)})
			status, stdout, stderr := invoke(t, described, cmdPlan, root)
			expect.Equal(t, status, cli.StatusOK, "plan succeeds")
			expect.Equal(t, stdout,
				"frontend fakefront 1\n"+
					"annotator 1 flagger 2\n"+
					"plan mirrors: target text, backend printer, commit order 1\n"+
					"  sources: every package\n"+
					"  generator 2 mirror\n"+
					"  layout: policy inherit\n"+
					"plan bindings: target text, backend printer, commit order 2\n"+
					"  sources: every package\n"+
					"  depends on: mirrors\n"+
					"  generator 1 binder 3\n"+
					"  layout: policy inherit\n"+
					"check lister: reads mirrors\n",
				"the output describes the composition")
			expect.Empty(t, stderr, "plan reports nothing")
		})

		t.Run("writes a component event for each frontend, annotator and check", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdPlan, workspaceDir(t, files.Tree{}), jsonFlag)
			expect.Contains(t, stdout, `{"event":"component","role":"frontend","name":"fakefront","version":"1"}`+"\n",
				"the frontend has its version")
			expect.Contains(t, stdout,
				`{"event":"component","role":"annotator","name":"flagger","version":"2","bucket":1}`+"\n",
				"the annotator has its bucket and its version")
			expect.Contains(t, stdout, `{"event":"component","role":"check","name":"lister","reads":["mirrors"]}`+"\n",
				"the check has the plans that it reads")
		})

		t.Run("writes a plan event for each plan", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdPlan, workspaceDir(t, files.Tree{}), jsonFlag)
			expect.Contains(t, stdout,
				`{"event":"plan","name":"mirrors","order":1,"sources":{},"generate":[{"name":"mirror","bucket":2}],`+
					`"backend":{"name":"printer"},"target":"text","layout":{"policy":"inherit"}}`+"\n",
				"the plan of the mirror commits first")
			expect.Contains(t, stdout,
				`{"event":"plan","name":"bindings","order":2,"sources":{},"dependsOn":["mirrors"],`+
					`"generate":[{"name":"binder","version":"3","bucket":1}],`+
					`"backend":{"name":"printer"},"target":"text","layout":{"policy":"inherit"}}`+"\n",
				"the plan bindings depends on the plan of the mirror")
		})

		t.Run("writes the sources and the layout that the config file refines", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(refinedConfig)})
			_, stdout, _ := invoke(t, compose, cmdPlan, root)
			expect.Contains(t, stdout, "  sources: language fake, packages ./svc/..., module example.test/svc\n",
				"the sources are the refined ones")
			expect.Contains(t, stdout, "  layout: policy centralised, dir gen, import base example.test/gen\n",
				"the layout is the refined one")
		})

		t.Run("writes the refined sources and layout into the plan event", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(refinedConfig)})
			_, stdout, _ := invoke(t, compose, cmdPlan, root, jsonFlag)
			assert.Contains(t, stdout,
				`"sources":{"lang":"fake","packages":["./svc/..."],"module":"example.test/svc"},`+
					`"generate":[{"name":"mirror","bucket":1}],"backend":{"name":"printer"},"target":"text",`+
					`"layout":{"policy":"centralised","dir":"gen","importBase":"example.test/gen"}}`+"\n",
				"the event has the refined fields")
		})

		t.Run("writes the choice of each policy of a plan's backend", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(widthConfig)})
			_, stdout, _ := invoke(t, policed, cmdPlan, root)
			assert.Contains(t, stdout, "  policies: text.width wide\n", "the plan has the choice of the config")
		})

		t.Run("writes the choice of each policy into the plan event", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(widthConfig)})
			_, stdout, _ := invoke(t, policed, cmdPlan, root, jsonFlag)
			assert.Contains(t, stdout, `"layout":{"policy":"inherit"},"policies":{"text.width":"wide"}}`+"\n",
				"the event has the choice of the config")
		})

		t.Run("writes the name of the member into each event of a list", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, compose, cmdPlan, listed(t), jsonFlag)
			assert.Equal(t, decoded[memberEvent](t, stdout, eventPlan),
				[]memberEvent{{Workspace: memberSvc}, {Workspace: memberTool}},
				"each member has one plan, in the order of the list")
		})

		t.Run("returns StatusUsage for a config error", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(faultyConfig)})
			status, _, stderr := invoke(t, compose, cmdPlan, root)
			expect.Equal(t, status, cli.StatusUsage, "the config error is a usage error")
			expect.Contains(t, stderr, "field workerz not found", "plan writes the fault")
		})
	})
}

// described returns the fixture composition over the scripted frontend with
// the annotator flagger, the plan bindings and the check lister. flagger
// registers the key cli.flag, reads it on each struct and then stamps it
// true, so the claim derives from the read. bindings mirrors each struct
// into the file binding.txt of its package. Both plans share one printer.
func described() *workspace.Builder {
	var flag meta.Key[bool]
	flagger := eidos.NewPlugin(flaggerName).
		Version("2").
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace(flagKey.Namespace()); err != nil {
				return err
			}
			k, err := meta.Register[bool](r, meta.KeySpec{Name: flagKey, Doc: "flags each struct"})
			flag = k
			return err
		}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
			eidos.Fact(m, flag)
			eidos.Stamp(st, flag, true)
			return nil
		})).
		Build().(plugin.Annotator)
	binder := eidos.NewPlugin(binderName).
		Version("3").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "binding"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			e.PackageFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: "Bound" + m.Struct.Name})
			return nil
		})).
		Build().(plugin.Generator)
	shared := printer()
	return workspace.New().
		Brand(brand).
		Frontends(frontendtest.NewScripted()).
		Annotators(flagger).
		Targets(target).
		Plans(
			workspace.Plan{Name: planName, Generators: []plugin.Generator{mirror()}, Backend: shared},
			workspace.Plan{
				Name: bindingsPlan, DependsOn: []string{planName},
				Generators: []plugin.Generator{binder}, Backend: shared,
			},
		).
		Checks(lister{})
}

// policed returns the fixture composition with a printer that declares the
// width policy.
func policed() *workspace.Builder {
	return workspace.New().
		Brand(brand).
		Frontends(frontendtest.NewScripted()).
		Targets(target).
		Plans(workspace.Plan{Name: planName, Generators: []plugin.Generator{mirror()}, Backend: printer(widthPolicy)})
}
