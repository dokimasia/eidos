// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugintest_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/plugintest"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// suiteCode is a code for the fixture plugins' findings.
var suiteCode = diag.Code{Prefix: "tst", Number: 2}

// twoStructs answers a fixture holding two positioned structs.
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
// has to wave a lawful plugin through and reject each way of
// cheating: that second half is what earns it.
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

		failure := assert.Rejects(t, "emit carrying run state must fail the rung",
			func(tb assert.TB) {
				plugintest.AssertDeterministicEmit(tb, nondeterministic)
			})
		assert.Contains(t, failure, "same bytes",
			"the rung names the byte-identity contract")
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

		failure := assert.Rejects(t, "a positionless finding must fail the rung",
			func(tb assert.TB) {
				plugintest.AssertPositionedDiagnostics(tb, unpositioned)
			})
		assert.Contains(t, failure, "position",
			"the rung names what the finding is missing")
	})
}

// rogue is a hand-rolled generator flushing a unit under a name it
// does not own.
type rogue struct{}

func (rogue) Name() string { return "honest" }

func (rogue) Generate(ctx *plugin.GeneratorContext) error {
	return ctx.Emit.Add(plugin.Unit{
		Plugin: "impostor", Per: plugin.PerPlan, Word: "out",
	})
}

func TestAssertAttributedEmit(t *testing.T) {
	t.Parallel()

	t.Run("rejects a unit under a stranger's name", func(t *testing.T) {
		t.Parallel()

		misattributed := func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
			f, _, _ := twoStructs(tb)
			return rogue{}, f
		}

		failure := assert.Rejects(t, "a misattributed unit must fail the rung",
			func(tb assert.TB) {
				plugintest.AssertAttributedEmit(tb, misattributed)
			})
		assert.Contains(t, failure, "names the plugin",
			"the rung names the attribution law")
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

		failure := assert.Rejects(t, "a varying declaration must fail the rung",
			func(tb assert.TB) {
				plugintest.AssertStableDeclaration(tb, unstable)
			})
		assert.Contains(t, failure, "stable",
			"the rung names the stability law")
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

func (handRolled) Name() string { return "twin" }

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

		failure := assert.Rejects(t, "a handler rewriting its subject must fail the rung",
			func(tb assert.TB) {
				plugintest.AssertNoStructuralWrites(tb, mutating)
			})
		assert.Contains(t, failure, "input truth",
			"the rung names the law the mutation broke")
	})
}
