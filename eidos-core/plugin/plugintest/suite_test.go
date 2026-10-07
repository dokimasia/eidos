// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugintest_test

import (
	"fmt"
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
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
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

// auditedField is the field the auditing weaver and its SPI twins
// append into the seeded struct.
const auditedField = "audited"

// spare is a hand-rolled generator: it declares its family and
// nothing else, so the suite meets a plugin the facade never gave
// a template surface to.
type spare struct{ handRolled }

// Outputs returns the one family the generator declares.
func (spare) Outputs() []plugin.Output {
	return []plugin.Output{{Per: plugin.PerPackage, Word: "audit"}}
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

// counted is a hand-rolled generator that names its one struct after
// the worker count its phase call runs on: output the parallel
// dispatch check must expose.
type counted struct{ handRolled }

// Generate flushes one plan unit whose struct names the worker count.
func (counted) Generate(ctx *plugin.GeneratorContext) error {
	return ctx.Emit.Add(plugin.Unit{
		Plugin: "twin", Per: plugin.PerPlan, Word: "audit",
		Decls: []symbol.Symbol{&emit.Struct{Name: "On" + strconv.Itoa(ctx.Workers)}},
	})
}

// workerStamper is a hand-rolled annotator that stamps its flag on
// every struct when its phase call runs on more than one worker: facts
// the parallel dispatch check must expose.
type workerStamper struct{ key meta.Key[bool] }

// Name returns the annotator's name.
func (workerStamper) Name() plugin.ID { return "stamper" }

// Annotate stamps the flag on every struct, and on more than one
// worker alone.
func (w workerStamper) Annotate(ctx *plugin.AnnotatorContext) error {
	if ctx.Workers <= 1 {
		return nil
	}
	for s := range ctx.Reader.ByKind(symbol.KindStruct) {
		decl, ok := s.(*node.Struct)
		if !ok {
			continue
		}
		err := meta.Stamp(ctx.Facts, w.key, true, meta.Claim{
			Subject: decl.Identity(), Authority: meta.AuthorityPlugin, Bucket: ctx.Bucket, Plugin: ctx.Plugin,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// workerReporter is a hand-rolled generator that warns at a position
// when its phase call runs on more than one worker: findings the
// parallel dispatch check must expose.
type workerReporter struct{ at position.Pos }

// Name returns the generator's name.
func (workerReporter) Name() plugin.ID { return "reporter" }

// Generate warns, and on more than one worker alone.
func (w workerReporter) Generate(ctx *plugin.GeneratorContext) error {
	if ctx.Workers > 1 {
		ctx.Sink.Warnf(suiteCode, w.at, ctx.Plugin, "runs on %d workers", ctx.Workers)
	}
	return nil
}

// journaledKey is the match a journaling hand-rolled generator records.
var journaledKey = plugin.MatchKey{Plugin: "journaling"}

// doubled is a hand-rolled generator that journals one match twice:
// the repeat the selective dispatch check must expose.
type doubled struct{}

// Name returns the generator's name.
func (doubled) Name() plugin.ID { return "journaling" }

// Generate journals one match twice where the call has a journal.
func (doubled) Generate(ctx *plugin.GeneratorContext) error {
	if ctx.Journal != nil {
		ctx.Journal.Invoked(plugin.Invocation{Match: journaledKey})
		ctx.Journal.Invoked(plugin.Invocation{Match: journaledKey})
	}
	return nil
}

// forgetful is a hand-rolled generator that journals its match only in
// a call without a selection: the selected call the selective dispatch
// check must expose.
type forgetful struct{}

// Name returns the generator's name.
func (forgetful) Name() plugin.ID { return "journaling" }

// Generate journals one match where the call has a journal and no
// selection.
func (forgetful) Generate(ctx *plugin.GeneratorContext) error {
	if ctx.Journal != nil && ctx.Select == nil {
		ctx.Journal.Invoked(plugin.Invocation{Match: journaledKey})
	}
	return nil
}

// unattributed is an SPI spelling of the auditing weaver that appends
// into the seeded struct's field slot and names itself among no unit's
// contributors: the twin the twins check must expose.
type unattributed struct{}

// Name returns the weaver's name, which the facade spelling shares.
func (unattributed) Name() plugin.ID { return "weaver" }

// Generate appends the audited field into every struct of the store.
func (unattributed) Generate(ctx *plugin.GeneratorContext) error {
	for s := range ctx.Emit.ByKind(symbol.KindStruct) {
		if host, held := s.(*emit.Struct); held {
			host.Fields.Append(&emit.Field{Name: auditedField})
		}
	}
	return nil
}

// attributed is the SPI spelling of the auditing weaver: it appends
// into the seeded struct's field slot and names itself among the
// contributors of the unit that contains the struct.
type attributed struct{}

// Name returns the weaver's name, which the facade spelling shares.
func (attributed) Name() plugin.ID { return "weaver" }

// Generate appends the audited field into every struct of the store,
// and records the contribution.
func (attributed) Generate(ctx *plugin.GeneratorContext) error {
	for s := range ctx.Emit.ByKind(symbol.KindStruct) {
		if host, held := s.(*emit.Struct); held {
			host.Fields.Append(&emit.Field{Name: auditedField})
			ctx.Emit.Contribute(host, ctx.Plugin)
		}
	}
	return nil
}

// woven is how many instances of the weaver's repeatable directive the
// weaving fixture places on one subject.
const woven = 16

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

		t.Run("passes a weaver whose handlers append into one slot", func(t *testing.T) {
			t.Parallel()

			plugintest.RunPluginSuite(t, weaving)
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
			assert.Equal(t, coretest.Contracts(failure), []string{
				"the fixture graph's files declare something: an empty run passes vacuously and proves nothing",
			}, "the check names what the fixture lacks")
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

		t.Run("fails a second pass that changes a fact's value", func(t *testing.T) {
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
			assert.Equal(t, coretest.Contracts(failure),
				[]string{"a repeated pass leaves every fact's value as the first pass left it"},
				"the check names what changed")
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
			assert.Equal(t, coretest.Contracts(failure), []string{"isolated runs emit the same bytes"},
				"the check names the byte-identity contract")
		})
	})

	t.Run("AssertParallelDispatch", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a weaver whose handlers append into one slot", func(t *testing.T) {
			t.Parallel()

			plugintest.AssertParallelDispatch(t, weaving)
		})

		t.Run("fails emit that depends on the worker count", func(t *testing.T) {
			t.Parallel()

			setup := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f, _, _ := twoStructs(tb)
				return counted{}, f
			}
			failure := assert.Rejects(t, "emit that varies with the worker count must fail the check",
				func(tb assert.TB) {
					plugintest.AssertParallelDispatch(tb, setup)
				})
			assert.Contains(t, coretest.Contracts(failure), "a parallel phase call emits what a sequential one does",
				"the check names the worker-count contract")
		})

		t.Run("fails facts that depend on the worker count", func(t *testing.T) {
			t.Parallel()

			setup := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f, _, _ := twoStructs(tb)
				return workerStamper{key: plugintest.Key[bool](tb, f, "t.flag", "marks a fixture subject")}, f
			}
			failure := assert.Rejects(t, "facts that vary with the worker count must fail the check",
				func(tb assert.TB) {
					plugintest.AssertParallelDispatch(tb, setup)
				})
			assert.Contains(t, coretest.Contracts(failure),
				"and it ends with the fact values a sequential call ends with", "the check names the fact contract")
		})

		t.Run("fails findings that depend on the worker count", func(t *testing.T) {
			t.Parallel()

			setup := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f, alpha, _ := twoStructs(tb)
				return workerReporter{at: alpha.Pos}, f
			}
			failure := assert.Rejects(t, "findings that vary with the worker count must fail the check",
				func(tb assert.TB) {
					plugintest.AssertParallelDispatch(tb, setup)
				})
			assert.Contains(t, coretest.Contracts(failure), "and it reports the same findings in the same order",
				"the check names the finding contract")
		})
	})

	t.Run("AssertSelective", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a dual-role plugin", func(t *testing.T) {
			t.Parallel()

			plugintest.AssertSelective(t, wellBehaved)
		})

		t.Run("passes a weaver whose handlers append into one slot", func(t *testing.T) {
			t.Parallel()

			plugintest.AssertSelective(t, weaving)
		})

		t.Run("passes a hand-rolled plugin that journals nothing", func(t *testing.T) {
			t.Parallel()

			plugintest.AssertSelective(t, bare)
		})

		t.Run("passes findings reported in the order the index enumerates", func(t *testing.T) {
			t.Parallel()

			plugintest.AssertSelective(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				p := eidos.NewPlugin("reporter").
					Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
						m.Warnf(suiteCode, "visited %s", m.Struct.Name)
						return nil
					})).
					Build()
				return p, reversed(tb)
			})
		})

		t.Run("fails emit that depends on the order of invocations", func(t *testing.T) {
			t.Parallel()

			setup := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				order := 0
				p := eidos.NewPlugin("cheat").
					Output(plugin.Output{Per: plugin.PerPackage, Word: "out"}).
					Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
						order++
						e.PackageFile().Append(&emit.Struct{
							Origin: m.Struct.Identity(), Name: "Order" + strconv.Itoa(order),
						})
						return nil
					})).
					Build()
				return p, reversed(tb)
			}
			failure := assert.Rejects(t, "emit that varies with the order of invocations must fail the check",
				func(tb assert.TB) {
					plugintest.AssertSelective(tb, setup)
				})
			assert.Contains(t, coretest.Contracts(failure), "a selection of every match emits what the whole call does",
				"the check names the selection contract")
		})

		t.Run("fails facts that depend on the order of invocations", func(t *testing.T) {
			t.Parallel()

			setup := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f := reversed(tb)
				key := plugintest.Key[string](tb, f, "t.order", "names a subject's place in the dispatch")
				order := 0
				p := eidos.NewPlugin("cheat").
					Handle(eidos.OnStruct(func(_ *eidos.StructMatch, st *eidos.Stamper) error {
						order++
						eidos.Stamp(st, key, strconv.Itoa(order))
						return nil
					})).
					Build()
				return p, f
			}
			failure := assert.Rejects(t, "facts that vary with the order of invocations must fail the check",
				func(tb assert.TB) {
					plugintest.AssertSelective(tb, setup)
				})
			assert.Contains(t, coretest.Contracts(failure), "and it ends with the fact values the whole call ends with",
				"the check names the fact contract")
		})

		t.Run("fails findings that depend on the order of invocations", func(t *testing.T) {
			t.Parallel()

			setup := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				order := 0
				p := eidos.NewPlugin("cheat").
					Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
						order++
						m.Warnf(suiteCode, "visited %d", order)
						return nil
					})).
					Build()
				return p, reversed(tb)
			}
			failure := assert.Rejects(t, "findings that vary with the order of invocations must fail the check",
				func(tb assert.TB) {
					plugintest.AssertSelective(tb, setup)
				})
			assert.Contains(t, coretest.Contracts(failure), "and it reports the same findings",
				"the check names the finding contract")
		})

		t.Run("fails a journal that lists a match twice", func(t *testing.T) {
			t.Parallel()

			setup := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f, _, _ := twoStructs(tb)
				return doubled{}, f
			}
			failure := assert.Rejects(t, "a repeated record must fail the check",
				func(tb assert.TB) {
					plugintest.AssertSelective(tb, setup)
				})
			assert.Contains(t, coretest.Contracts(failure),
				"the journal lists every match once, in canonical match order",
				"the check names the journal contract")
		})

		t.Run("fails a selected call that journals other matches", func(t *testing.T) {
			t.Parallel()

			setup := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				f, _, _ := twoStructs(tb)
				return forgetful{}, f
			}
			failure := assert.Rejects(t, "a selected call that drops a record must fail the check",
				func(tb assert.TB) {
					plugintest.AssertSelective(tb, setup)
				})
			assert.Contains(t, coretest.Contracts(failure),
				"the selected run journals the matches the whole run journaled",
				"the check names the selection contract")
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
			assert.Equal(t, coretest.Contracts(failure), []string{"the options struct meets the tag contract"},
				"the check names the tag contract")
			fault, _ := failure[0].Got()
			assert.Contains(t, fmt.Sprint(fault), "doc", "and the fault names the missing tag")
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
			assert.Equal(t, coretest.Contracts(failure), []string{"the tree meets the template rules"},
				"the check names the rule")
			fault, _ := failure[0].Got()
			assert.Contains(t, fmt.Sprint(fault), "marker", "and the fault names the dropped marker")
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
			assert.Equal(t, coretest.Contracts(failure),
				[]string{"the plugin declares templates, so the check proves something"},
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
			assert.Equal(t, coretest.Contracts(failure),
				[]string{"a fixture language meets a declared tree, so the check proves something"},
				"the facade gives every plugin the provider's shape, so the check does not pass on the shape alone")
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
			assert.Equal(t, coretest.Contracts(failure),
				[]string{"every finding names the position it is about: nowhere in particular"},
				"the check names what the finding is missing")
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
			assert.Contains(t, coretest.Contracts(failure), "every unit names the plugin that emitted it",
				"the check names the attribution rule")
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
								e.Slot(&s.Methods).Append(&emit.Method{Origin: m.Origin(), Name: "Audit"})
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
			assert.Equal(t, coretest.Contracts(failure), []string{"the gate records are stable across builds"},
				"the check names the stability rule")
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

		t.Run("passes a weaver and its SPI twin that names itself among the contributors", func(t *testing.T) {
			t.Parallel()

			plugintest.AssertTwins(t, auditing, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				return attributed{}, seeded(tb)
			})
		})

		t.Run("fails a weaver's SPI twin that names itself among no contributors", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a twin that drops the weaver's attribution must fail the check",
				func(tb assert.TB) {
					plugintest.AssertTwins(tb, auditing, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
						return unattributed{}, seeded(tb)
					})
				})
			assert.Equal(t, coretest.Contracts(failure),
				[]string{"both spellings of the plugin emit the same bytes"},
				"the check names the byte-identity contract")
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
			assert.Equal(t, coretest.Contracts(failure),
				[]string{"the graph is input truth, and no phase call rewrites it"},
				"the check names the rule the mutation broke")
		})
	})
}

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

// seeded returns the two-struct fixture with an earlier bucket's unit
// that contains one struct for alpha.
func seeded(tb assert.TB) *plugintest.Fixture {
	tb.Helper()

	f, alpha, _ := twoStructs(tb)
	f.Seed(tb, plugin.Unit{
		Plugin: "earlier", Per: plugin.PerSource, Word: "impl", Key: "alpha.go",
		Decls: []symbol.Symbol{&emit.Struct{Origin: alpha.ID, Name: "ForAlpha"}},
	})
	return f
}

// auditing is a weaver over the seeded struct that appends the audited
// field into its slot through the Emitter.
func auditing(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
	tb.Helper()

	p := eidos.NewPlugin("weaver").
		Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
			if s, held := m.Value.(*emit.Struct); held {
				e.Slot(&s.Fields).Append(&emit.Field{Name: auditedField})
			}
			return nil
		})).
		Build()
	return p, seeded(tb)
}

// weaving is a weaver over an earlier bucket's struct whose origin has
// many instances of one repeatable directive: every instance appends a
// field into the one struct's slot through the Emitter.
func weaving(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
	tb.Helper()

	f, alpha, _ := twoStructs(tb)
	schema := directive.Schema{Plugin: "weaver", Name: "audit", Repeatable: true, Doc: "audits the subject"}
	instances := make([]directive.Directive, woven)
	for i := range instances {
		instances[i] = directive.Directive{Name: schema.Canonical(), Instance: i}
	}
	f.Validated(tb, alpha.ID, instances...)
	f.Seed(tb, plugin.Unit{
		Plugin: "earlier", Per: plugin.PerSource, Word: "impl", Key: "alpha.go",
		Decls: []symbol.Symbol{&emit.Struct{Origin: alpha.ID, Name: "ForAlpha"}},
	})
	p := eidos.NewPlugin("weaver").
		Handle(eidos.Directive(schema,
			eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
				s, held := m.Value.(*emit.Struct)
				if !held {
					return nil
				}
				e.Slot(&s.Fields).Append(&emit.Field{Name: "audit" + strconv.Itoa(m.Directive().Instance)})
				return nil
			}))).
		Build()
	return p, f
}

// reversed returns a fixture with two positioned structs declared in
// the reverse of their identity order, which is the order the index
// enumerates them in.
func reversed(tb assert.TB) *plugintest.Fixture {
	tb.Helper()

	alpha := coretest.Struct(coretest.StorePath, "Alpha")
	alpha.Pos = position.Pos{File: "alpha.go", Line: 3, Col: 1}
	beta := coretest.Struct(coretest.StorePath, "Beta")
	beta.Pos = position.Pos{File: "beta.go", Line: 7, Col: 1}
	f := plugintest.New(tb)
	f.Load(tb, coretest.Package(coretest.StorePath, beta, alpha))
	return f
}
