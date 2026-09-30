// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugintest_test

import (
	"io/fs"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"text/template"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/plugin/plugintest"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The template fixture's names: the two languages a setup lints
// against, the helper one of them shares, the template a tree
// contains, and the marker that template places.
const (
	fixtureTarget plugin.Target = "fixture"
	vocalTarget   plugin.Target = "vocal"
	wordHelper                  = "word"
	templateName                = "method1.tpl"
	slotsMarker                 = "{{slots}}"
)

// suiteCode is a code for the fixture plugins' findings.
var suiteCode = diag.Code{Prefix: "tst", Number: 2}

// twoStructs returns a fixture with two positioned structs.
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
// weave the suite passes.
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

// spare is a hand-rolled generator: it declares its family and
// nothing else, so the suite meets a plugin the facade never gave
// a template surface to.
type spare struct{ handRolled }

// Outputs returns the one family the generator declares.
func (spare) Outputs() []plugin.Output {
	return []plugin.Output{{Per: plugin.PerPackage, Word: "audit"}}
}

// bare returns the hand-rolled generator over the fixture.
func bare(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
	tb.Helper()

	f, _, _ := twoStructs(tb)
	return spare{}, f
}

// quietRule returns a graph rule whose handler emits nothing.
func quietRule() eidos.Rule {
	return eidos.OnGraph(func(*eidos.GraphMatch, *eidos.Emitter) error { return nil })
}

// treeOf returns a one-template tree whose template reads text.
func treeOf(text string) fs.FS {
	return fstest.MapFS{templateName: &fstest.MapFile{Data: []byte(text)}}
}

// fixtureLanguage returns the smallest language the template check
// can lint against: no shared vocabulary.
func fixtureLanguage() render.Language {
	return render.Language{
		Kinds:    map[symbol.Kind]string{symbol.KindStruct: "type {{.Name}} struct{}\n"},
		Naming:   func(u plugin.Unit) string { return u.Key },
		Scaffold: func(emit.Stmt, *render.ImportSet) ([]byte, error) { return nil, nil },
		Imports:  func(*render.ImportSet) string { return "" },
		Finalise: func(src []byte) ([]byte, error) { return src, nil },
	}
}

// vocalLanguage returns the fixture language with one shared helper,
// the name an override can replace.
func vocalLanguage() render.Language {
	l := fixtureLanguage()
	l.Funcs = func(*render.ImportSet) template.FuncMap {
		return template.FuncMap{wordHelper: strings.ToLower}
	}
	return l
}

// treed returns a setup whose plugin declares a tree for the
// fixture target alone, which is what makes the suite run its
// template lint.
func treed(tree fs.FS) plugintest.Setup {
	return func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
		tb.Helper()

		f, _, _ := twoStructs(tb)
		f.Languages = map[plugin.Target]render.Language{fixtureTarget: fixtureLanguage()}
		p := eidos.NewPlugin("treed").
			Output(plugin.Output{Per: plugin.PerPlan, Word: "out"}).
			For(fixtureTarget, eidos.Templates(tree)).
			Handle(quietRule()).
			Build()
		return p, f
	}
}

// rogue is a hand-rolled generator flushing a unit under another
// plugin's name.
type rogue struct{}

// Name returns the name the generator declares.
func (rogue) Name() plugin.ID { return "honest" }

// Generate flushes one unit attributed to another plugin.
func (rogue) Generate(ctx *plugin.GeneratorContext) error {
	return ctx.Emit.Add(plugin.Unit{
		Plugin: "impostor", Per: plugin.PerPlan, Word: "out",
	})
}

// handRolled is the SPI spelling of the facade twin in the twins
// case: it reads the structs through the tracked reader and flushes
// one per-package unit in canonical order.
type handRolled struct{}

// Name returns the twin's name, which the facade spelling shares.
func (handRolled) Name() plugin.ID { return "twin" }

// Generate emits one struct per fixture struct into one
// per-package unit.
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

// The suite is the contract a plugin author tests against, so it
// passes a valid plugin and fails each way of cheating, and the
// failing half is what justifies it.
func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("RunPluginSuite", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a dual-role plugin", func(t *testing.T) {
			t.Parallel()

			plugintest.RunPluginSuite(t, wellBehaved)
		})

		t.Run("lints the tree a plugin declares", func(t *testing.T) {
			t.Parallel()

			plugintest.RunPluginSuite(t, treed(treeOf(slotsMarker)))
		})

		t.Run("skips the lint for a plugin that declares no tree", func(t *testing.T) {
			t.Parallel()

			plugintest.RunPluginSuite(t, bare)
		})
	})

	t.Run("AssertPopulatedFixture", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a fixture whose files declare nothing", func(t *testing.T) {
			t.Parallel()

			empty := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f := plugintest.New(tb)
				f.Load(tb, coretest.Package(coretest.StorePath))
				return spare{}, f
			}
			failure := assert.Rejects(t, "a fixture of empty files must fail the check",
				func(tb assert.TB) {
					plugintest.AssertPopulatedFixture(tb, empty)
				})
			assert.Contains(t, failure, "declare nothing", "the check names what the fixture lacks")
		})

		t.Run("passes a fixture that declares one function", func(t *testing.T) {
			t.Parallel()

			plugintest.AssertPopulatedFixture(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f := plugintest.New(tb)
				f.Load(tb, coretest.Package(coretest.StorePath,
					coretest.Function(coretest.StorePath, coretest.FunctionName)))
				return spare{}, f
			})
		})
	})

	t.Run("AssertIdempotentAnnotate", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a second pass that moves a winning value", func(t *testing.T) {
			t.Parallel()

			stateful := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f, _, _ := twoStructs(tb)
				key := plugintest.Key[bool](tb, f, "t.flag", "marks a fixture subject")
				calls := 0
				p := eidos.NewPlugin("cheat").
					Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
						// Two subjects make one pass, so the stamp arrives on
						// the second pass alone.
						calls++
						if calls > 2 {
							eidos.Stamp(st, key, true)
						}
						return nil
					})).
					Build()
				return p, f
			}
			failure := assert.Rejects(t, "a fact the second pass adds must fail the check",
				func(tb assert.TB) {
					plugintest.AssertIdempotentAnnotate(tb, stateful)
				})
			assert.Contains(t, failure, "winning value", "the check names what moved")
		})
	})

	t.Run("AssertDeterministicEmit", func(t *testing.T) {
		t.Parallel()

		t.Run("fails emit that varies between runs", func(t *testing.T) {
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

			failure := assert.Rejects(t, "emit that varies with run state must fail the check",
				func(tb assert.TB) {
					plugintest.AssertDeterministicEmit(tb, nondeterministic)
				})
			assert.Contains(t, failure, "same bytes", "the check names the byte-identity contract")
		})
	})

	t.Run("AssertOptionsSchema", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a struct outside the tag contract", func(t *testing.T) {
			t.Parallel()

			undocumented := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f, _, _ := twoStructs(tb)
				p := eidos.NewPlugin("cheat").
					Options(&struct {
						Depth int `opt:"depth"`
					}{}).
					Output(plugin.Output{Per: plugin.PerPlan, Word: "out"}).
					Handle(quietRule()).
					Build()
				return p, f
			}

			failure := assert.Rejects(t, "an undocumented option must fail the check",
				func(tb assert.TB) {
					plugintest.AssertOptionsSchema(tb, undocumented)
				})
			assert.Contains(t, failure, "doc", "the check names the missing tag")
		})
	})

	t.Run("AssertTemplates", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a valid tree", func(t *testing.T) {
			t.Parallel()

			plugintest.AssertTemplates(t, treed(treeOf(slotsMarker)))
		})

		t.Run("fails a tree that drops the marker", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a dropped marker must fail the check",
				func(tb assert.TB) {
					plugintest.AssertTemplates(tb, treed(treeOf("bare\n")))
				})
			assert.Contains(t, failure, "marker", "the check names the rule")
		})

		t.Run("passes an override declared for one of two languages", func(t *testing.T) {
			t.Parallel()

			plugintest.AssertTemplates(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				tb.Helper()

				f, _, _ := twoStructs(tb)
				f.Languages = map[plugin.Target]render.Language{
					fixtureTarget: fixtureLanguage(),
					vocalTarget:   vocalLanguage(),
				}
				p := eidos.NewPlugin("overriding").
					Output(plugin.Output{Per: plugin.PerPlan, Word: "out"}).
					Templates(treeOf(slotsMarker)).
					For(vocalTarget, eidos.Overrides(template.FuncMap{wordHelper: strings.ToUpper})).
					Handle(quietRule()).
					Build()
				return p, f
			})
		})

		t.Run("fails a plugin without the template surface", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a check over a plugin with no trees must fail",
				func(tb assert.TB) {
					plugintest.AssertTemplates(tb, bare)
				})
			assert.Contains(t, failure, "declares no templates",
				"a lint the plugin cannot even be asked for proves nothing")
		})

		t.Run("fails a setup that declares no tree", func(t *testing.T) {
			t.Parallel()

			treeless := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f, _, _ := twoStructs(tb)
				f.Languages = map[plugin.Target]render.Language{fixtureTarget: fixtureLanguage()}
				p := eidos.NewPlugin("bare").
					Output(plugin.Output{Per: plugin.PerPlan, Word: "out"}).
					Handle(quietRule()).
					Build()
				return p, f
			}
			failure := assert.Rejects(t, "a lint over nothing must fail the check",
				func(tb assert.TB) {
					plugintest.AssertTemplates(tb, treeless)
				})
			assert.Contains(t, failure, "proves nothing",
				"the facade gives every plugin the provider's shape, so the "+
					"check does not pass on the shape alone")
		})
	})

	t.Run("AssertPositionedDiagnostics", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a finding without a position", func(t *testing.T) {
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
			assert.Contains(t, failure, "position", "the check names what the finding is missing")
		})
	})

	t.Run("AssertAttributedEmit", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a unit under another plugin's name", func(t *testing.T) {
			t.Parallel()

			misattributed := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f, _, _ := twoStructs(tb)
				return rogue{}, f
			}

			failure := assert.Rejects(t, "a misattributed unit must fail the check",
				func(tb assert.TB) {
					plugintest.AssertAttributedEmit(tb, misattributed)
				})
			assert.Contains(t, failure, "names the plugin", "the check names the attribution rule")
		})

		t.Run("passes a weaver over another plugin's seeded unit", func(t *testing.T) {
			t.Parallel()

			plugintest.AssertAttributedEmit(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f, alpha, _ := twoStructs(tb)
				f.Seed(tb, plugin.Unit{
					Plugin: "earlier", Per: plugin.PerSource, Word: "impl", Key: "a.go",
					Decls: []symbol.Symbol{&emit.Struct{Origin: alpha.ID, Name: "Gen"}},
				})
				p := eidos.NewPlugin("weaver").
					Handle(eidos.OnEmit(symbol.KindStruct,
						func(m *eidos.EmitMatch, e *eidos.Emitter) error {
							s, ok := m.Value.(*emit.Struct)
							if ok {
								s.Methods.Append(&emit.Method{Origin: m.Origin(), Name: "Audit"})
							}
							return nil
						})).
					Build()
				return p, f
			})
		})
	})

	t.Run("AssertStableDeclaration", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a declaration that varies between builds", func(t *testing.T) {
			t.Parallel()

			var builds int
			unstable := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f, _, _ := twoStructs(tb)
				builds++
				b := eidos.NewPlugin("cheat").Handle(quietRule())
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
			assert.Contains(t, failure, "stable", "the check names the stability rule")
		})
	})

	t.Run("AssertTwins", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a facade plugin and its SPI twin that emit one output", func(t *testing.T) {
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
	})

	t.Run("AssertNoStructuralWrites", func(t *testing.T) {
		t.Parallel()

		t.Run("fails an in-place mutation of a declaration", func(t *testing.T) {
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
			assert.Contains(t, failure, "input truth", "the check names the rule the mutation broke")
		})
	})
}
