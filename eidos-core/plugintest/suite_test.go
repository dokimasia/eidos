// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugintest_test

import (
	"io/fs"
	"strconv"
	"testing"
	"testing/fstest"
	"text/template"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/plugintest"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
)

// suiteCode is a code for the fixture plugins' findings.
var suiteCode = diag.Code{Prefix: "tst", Number: 2}

// twoStructs returns a fixture holding two positioned structs.
func twoStructs(tb assert.TB) (*plugintest.Fixture, *node.Struct, *node.Struct) {
	tb.Helper()

	alpha := coretest.Struct(coretest.StorePath, "Alpha")
	alpha.Pos = position.Pos{File: "alpha.go", Line: 3, Col: 1}
	beta := coretest.Struct(coretest.StorePath, "Beta")
	beta.Pos = position.Pos{File: "beta.go", Line: 7, Col: 1}
	f := plugintest.New(tb)
	f.Load(tb, coretest.Package(coretest.StorePath, alpha, beta))
	return f, alpha, beta
}

// wellBehaved is a dual-role plugin: its annotator half classifies
// and its generator half emits for the classified subjects, the
// weave the suite should wave through.
func wellBehaved(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
	tb.Helper()

	f, _, _ := twoStructs(tb)
	key := plugintest.Key[bool](tb, f, "t.flag", "marks a fixture subject")
	p := eidos.NewPlugin("suite").
		Options(&struct {
			Header string `opt:"header" doc:"the banner every audit opens with"`
		}{}).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "audit"}).
		Handle(
			eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
				eidos.Stamp(st, key, true)
				return nil
			}),
			eidos.Where(eidos.HasKey(key),
				eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.PackageFile().Append(&emit.Struct{
						Origin: m.Struct.Identity(),
						Name:   "For" + m.Struct.Name,
					})
					return nil
				})),
		).
		Build()
	return p, f
}

// The suite is the contract a plugin author tests against, so it
// has to accept a valid plugin through and reject each way of
// cheating: that second half is what justifies it.
func TestRunPluginSuite(t *testing.T) {
	t.Parallel()

	plugintest.RunPluginSuite(t, wellBehaved)
}

func TestAssertDeterministicEmit(t *testing.T) {
	t.Parallel()

	t.Run("rejects emit that varies between runs", func(t *testing.T) {
		t.Parallel()

		var runs int
		nondeterministic := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
			f, _, _ := twoStructs(tb)
			runs++
			stamp := strconv.Itoa(runs)
			p := eidos.NewPlugin("cheat").
				Output(plugin.Output{Per: plugin.PerPlan, Word: "out"}).
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					e.PlanFile().Append(&emit.Struct{
						Origin: coretest.Struct(coretest.StorePath, "Alpha").ID,
						Name:   "Run" + stamp,
					})
					return nil
				})).
				Build()
			return p, f
		}

		failure := assert.Rejects(t, "emit carrying run state must fail the check",
			func(tb assert.TB) {
				plugintest.AssertDeterministicEmit(tb, nondeterministic)
			})
		assert.Contains(t, failure, "same bytes",
			"the check names the byte-identity contract")
	})
}

func TestAssertOptionsSchema(t *testing.T) {
	t.Parallel()

	t.Run("rejects a struct outside the tag contract", func(t *testing.T) {
		t.Parallel()

		undocumented := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
			f, _, _ := twoStructs(tb)
			p := eidos.NewPlugin("cheat").
				Options(&struct {
					Depth int `opt:"depth"`
				}{}).
				Output(plugin.Output{Per: plugin.PerPlan, Word: "out"}).
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					return nil
				})).
				Build()
			return p, f
		}

		failure := assert.Rejects(t, "an undocumented option must fail the check",
			func(tb assert.TB) {
				plugintest.AssertOptionsSchema(tb, undocumented)
			})
		assert.Contains(t, failure, "doc", "the check names the missing tag")
	})
}

// templated wraps a plugin with a declared template tree for the
// fixture target.
type templated struct {
	plugin.Plugin
	tree fs.FS
}

func (p templated) Templates(t plugin.Target) (fs.FS, bool) {
	if t != "fixture" {
		return nil, false
	}
	return p.tree, true
}

func (templated) TemplateFuncs(plugin.Target) template.FuncMap { return nil }
func (templated) Overrides() []string                          { return nil }

// fixtureLanguage returns the smallest language the template check
// can lint against.
func fixtureLanguage() render.Language {
	return render.Language{
		Kinds:    map[symbol.Kind]string{symbol.KindStruct: "type {{.Name}} struct{}\n"},
		Naming:   func(u plugin.Unit) string { return u.Key },
		Scaffold: func(emit.Stmt, *render.ImportSet) ([]byte, error) { return nil, nil },
		Imports:  func(*render.ImportSet) string { return "" },
		Finalise: func(src []byte) ([]byte, error) { return src, nil },
	}
}

func TestAssertTemplates(t *testing.T) {
	t.Parallel()

	setupWith := func(tree fs.FS) plugintest.Setup {
		return func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
			f, _, _ := twoStructs(tb)
			f.Languages = map[plugin.Target]render.Language{"fixture": fixtureLanguage()}
			p := eidos.NewPlugin("treed").
				Output(plugin.Output{Per: plugin.PerPlan, Word: "out"}).
				Handle(eidos.OnGraph(func(*eidos.GraphMatch, *eidos.Emitter) error {
					return nil
				})).
				Build()
			return templated{Plugin: p, tree: tree}, f
		}
	}

	t.Run("waves a valid tree through", func(t *testing.T) {
		t.Parallel()

		plugintest.AssertTemplates(t, setupWith(fstest.MapFS{
			"method1.tpl": &fstest.MapFile{Data: []byte("{{slots}}")},
		}))
	})

	t.Run("rejects a tree dropping the marker", func(t *testing.T) {
		t.Parallel()

		failure := assert.Rejects(t, "a dropped marker must fail the check",
			func(tb assert.TB) {
				plugintest.AssertTemplates(tb, setupWith(fstest.MapFS{
					"method1.tpl": &fstest.MapFile{Data: []byte("bare\n")},
				}))
			})
		assert.Contains(t, failure, "marker", "the check names the rule")
	})
}

func TestAssertPositionedDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("rejects a finding without a position", func(t *testing.T) {
		t.Parallel()

		unpositioned := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
			f, _, _ := twoStructs(tb)
			p := eidos.NewPlugin("cheat").
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					m.Warnf(suiteCode, position.Pos{}, "nowhere in particular")
					return nil
				})).
				Build()
			return p, f
		}

		failure := assert.Rejects(t, "a positionless finding must fail the check",
			func(tb assert.TB) {
				plugintest.AssertPositionedDiagnostics(tb, unpositioned)
			})
		assert.Contains(t, failure, "position",
			"the check names what the finding is missing")
	})
}

// rogue is a hand-rolled generator flushing a unit under a name it
// does not own.
type rogue struct{}

func (rogue) Name() plugin.ID { return "honest" }

func (rogue) Generate(ctx *plugin.GeneratorContext) error {
	return ctx.Emit.Add(plugin.Unit{
		Plugin: "impostor", Per: plugin.PerPlan, Word: "out",
	})
}

func TestAssertAttributedEmit(t *testing.T) {
	t.Parallel()

	t.Run("rejects a unit under another plugin's name", func(t *testing.T) {
		t.Parallel()

		misattributed := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
			f, _, _ := twoStructs(tb)
			return rogue{}, f
		}

		failure := assert.Rejects(t, "a misattributed unit must fail the check",
			func(tb assert.TB) {
				plugintest.AssertAttributedEmit(tb, misattributed)
			})
		assert.Contains(t, failure, "names the plugin",
			"the check names the attribution rule")
	})
}

func TestAssertStableDeclaration(t *testing.T) {
	t.Parallel()

	t.Run("rejects a declaration that varies between builds", func(t *testing.T) {
		t.Parallel()

		var builds int
		unstable := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
			f, _, _ := twoStructs(tb)
			builds++
			b := eidos.NewPlugin("cheat").
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					return nil
				}))
			if builds > 1 {
				b.Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error { return nil }))
			}
			return b.Build(), f
		}

		failure := assert.Rejects(t, "a varying declaration must fail the check",
			func(tb assert.TB) {
				plugintest.AssertStableDeclaration(tb, unstable)
			})
		assert.Contains(t, failure, "stable",
			"the check names the stability rule")
	})
}

func TestAssertTwins(t *testing.T) {
	t.Parallel()

	t.Run("holds a facade plugin and its SPI twin to one output", func(t *testing.T) {
		t.Parallel()

		facade := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
			f, _, _ := twoStructs(tb)
			p := eidos.NewPlugin("twin").
				Output(plugin.Output{Per: plugin.PerPackage, Word: "audit"}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.PackageFile().Append(&emit.Struct{
						Origin: m.Struct.Identity(),
						Name:   "For" + m.Struct.Name,
					})
					return nil
				})).
				Build()
			return p, f
		}
		plugintest.AssertTwins(t, facade, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
			f, _, _ := twoStructs(tb)
			return handRolled{}, f
		})
	})
}

// handRolled is the SPI spelling of the facade twin above: it reads
// the structs through the tracked reader and flushes one
// per-package unit in canonical order.
type handRolled struct{}

func (handRolled) Name() plugin.ID { return "twin" }

func (handRolled) Generate(ctx *plugin.GeneratorContext) error {
	unit := plugin.Unit{
		Plugin: "twin", Per: plugin.PerPackage, Word: "audit",
		Key: coretest.StorePath,
	}
	for s := range ctx.Reader.ByKind(symbol.KindStruct) {
		decl, ok := s.(*node.Struct)
		if !ok {
			continue
		}
		unit.Decls = append(unit.Decls, &emit.Struct{
			Origin: decl.Identity(),
			Name:   "For" + decl.Name,
		})
		unit.Origins = append(unit.Origins, decl.Identity())
	}
	if len(unit.Origins) > 0 {
		if pkg, held := ctx.Reader.PackageOf(unit.Origins[0]); held {
			unit.Pkg = pkg.ID
		}
	}
	return ctx.Emit.Add(unit)
}

func TestAssertNoStructuralWrites(t *testing.T) {
	t.Parallel()

	t.Run("rejects an in-place mutation of a declaration", func(t *testing.T) {
		t.Parallel()

		mutating := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
			f, _, _ := twoStructs(tb)
			p := eidos.NewPlugin("cheat").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					m.Struct.Name = "Rewritten"
					return nil
				})).
				Build()
			return p, f
		}

		failure := assert.Rejects(t, "a handler rewriting its subject must fail the check",
			func(tb assert.TB) {
				plugintest.AssertNoStructuralWrites(tb, mutating)
			})
		assert.Contains(t, failure, "input truth",
			"the check names the rule the mutation broke")
	})
}
