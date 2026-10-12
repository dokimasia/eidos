// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
	"errors"
	"io/fs"
	"maps"
	"math"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// These constants are the emitting plugin of the suite's fixture, the
// backends of the cases, and the words, names and bodies that the cases
// render.
const (
	// fixtureEmitter is the plugin that emits every unit of the fixture.
	fixtureEmitter plugin.ID = "gen"
	// printerName, memberName and halfName are the names of the fixture's
	// backends.
	printerName plugin.ID = "printer"
	memberName  plugin.ID = "member"
	halfName    plugin.ID = "half"
	// stubTarget is the target of every backend of the fixture.
	stubTarget plugin.Target = "stub"
	// lineComment is the token of a line comment in the fixture's
	// language.
	lineComment = "//"
	// fileExt is the extension of every file that the fixture's naming
	// returns.
	fileExt = ".txt"
	// runtimePkg is the import that the fixture's scaffold records.
	runtimePkg = "stub/runtime"
	// refTemplate is the name of the template of the reference form.
	refTemplate = "save.tpl"
	// These are the words of the units. The naming puts each word into a
	// file name.
	stubWord  = "stub"
	refWord   = "ref"
	rawWord   = "raw"
	hostsWord = "hosts"
	funcsWord = "funcs"
	sumsWord  = "sums"
	extraWord = "extra"
	richWord  = "rich"
	// These are the functions of the valid fixture and the calls in their
	// bodies.
	noopName  = "Noop"
	loadName  = "Load"
	saveName  = "Save"
	dumpName  = "Dump"
	guardCall = "guard"
	dumpBody  = "\tdump()\n"
	// fileA and fileB are the paths of the files that a scripted renderer
	// returns, and bodyX is the body of each.
	fileA = "a.txt"
	fileB = "b.txt"
	bodyX = "x\n"
	// abortMessage is the text of the error that ends a scripted render.
	abortMessage = "kaboom"
	// outsider is an origin that no render context has.
	outsider plugin.ID = "outsider"
	// intType is the spelling of the type of a parameter.
	intType = "int"
	// kindReason is the reason that a fixture backend gives for a kind
	// that it refuses.
	kindReason = "the fixture target declares no such construct"
	// companionSuffix is the suffix of the companion struct that the
	// duplicating lowering adds.
	companionSuffix = "Copy"
)

// These constants are the templates of the fixture's language for a
// struct and for a function that places its body.
const (
	structTpl   = "type {{.Name}} struct{}\n"
	functionTpl = "func {{.Name}}() {\n{{" + render.BuiltinBody + " .}}}\n"
	// refSource is the template of the reference form. It writes a call
	// and then the marker that places every pending slot.
	refSource = "\tsaving()\n{{" + render.BuiltinSlots + "}}"
)

// These constants are the names of the declarations of the member
// fixture. The fixture has one host of each kind with members, and the
// members of each host, so the member check reads a field, a method and
// both forms of a variant.
const (
	hostStruct    = "Row"
	structField   = "Cell"
	structMethod  = "Touch"
	hostInterface = "Store"
	ifaceField    = "Handle"
	ifaceMethod   = "Fetch"
	hostEnum      = "Phase"
	enumVariant   = "Open"
	hostSum       = "Shape"
	sumVariant    = "Circle"
	// prefixedField is a field name that the host name Row contains.
	prefixedField = "Ro"
	// sumComment and variantComment are the comments that the coverage
	// cases put on a sum and on its variant.
	sumComment     = "tagged"
	variantComment = "round"
)

// These constants are the contracts of the checks, as their records
// contain them. A case compares them with the records to find the
// contract that a check broke.
const (
	settleCompletes = "the settle completes"
	settlesClean    = "the suite's fixture settles clean"
	storeContained  = "the fixture contains a store"
	unitsKept       = "the settle adds and drops no unit"
	// memberRendered is the end of the contract of each member that the
	// member check reads. The contract starts with Host.Member.
	memberRendered = " renders beside its host, or a finding names it"
)

// fake is a backend whose render a failing case scripts. It embeds the
// kit backend of the valid fixture for every other method. The suite
// must catch every way in which such a render breaks the rules.
type fake struct {
	backend.Backend
	render func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error)
}

// Render renders through the scripted function.
func (f *fake) Render(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
	return f.render(ctx)
}

// collidingField is the name of the second field of the colliding
// fixture. A respell that lowers the leading letter gives it the name of
// the first field, so the settle reports both names and keeps them as
// emitted.
const collidingField = "cell"

// These constants are the member lists that a host template ranges
// over. A backend that ranges over them writes every member name. A backend that leaves them
// out drops the members without a finding, and
// [backendtest.AssertRenderedMembers] reports that defect.
const (
	fieldLines   = "{{range .Fields.Items}}\t{{.Name}}\n{{end}}"
	methodLines  = "{{range .Methods.Items}}\t{{.Name}}\n{{end}}"
	variantLines = "{{range .Variants.Items}}\t{{.Name}}\n{{end}}"
)

// These constants are the names of the declarations of the reference
// fixture. The settle rewrites each name, and another part of the
// fixture refers to it. The
// type of the field refers to its own host. The body of the method calls
// the function and guards on its parameter. The named slot contains a
// call of its own.
const (
	richFn     = "boot"
	richStruct = "row"
	richField  = "next"
	richMethod = "touch"
	richParam  = "item"
	richSlot   = "extra"
)

// TestSuite tests the backend suite. The suite passes a valid backend,
// and fails each way in which a backend breaks the contract.
func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("RunBackendSuite", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the kit backend over the valid fixture", func(t *testing.T) {
			t.Parallel()

			backendtest.RunBackendSuite(t, wellRendered)
		})

		t.Run("passes a backend refusing a kind its fixture emits", func(t *testing.T) {
			t.Parallel()

			backendtest.RunBackendSuite(t, refusingRendered)
		})
	})

	t.Run("AssertRenderedMembers", func(t *testing.T) {
		t.Parallel()

		t.Run("passes host templates that place every member", func(t *testing.T) {
			t.Parallel()

			backendtest.AssertRenderedMembers(t, memberSpelt)
		})

		t.Run("fails a host template that drops its members", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a dropped member must fail the check",
				func(tb assert.TB) {
					backendtest.AssertRenderedMembers(tb, memberDropped)
				})
			assertMissing(t, failure, []string{
				hostStruct + "." + structField, hostStruct + "." + structMethod,
				hostInterface + "." + ifaceField, hostInterface + "." + ifaceMethod,
				hostEnum + "." + enumVariant, hostSum + "." + sumVariant,
			}, "the check reads the member lists of every host kind and reports each drop")
		})

		t.Run("fails a member that only another file contains", func(t *testing.T) {
			t.Parallel()

			// The struct's method shares its name with a function in a
			// second file, so the rendered bytes contain the name while
			// the host's own file drops the method.
			shadowed := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				tb.Helper()

				kinds := dropsMembers()
				kinds[symbol.KindFunction] = "func {{.Name}}() {}\n"
				r := memberBuilder(tb, kinds).Build()
				row := &emit.Struct{
					Origin: coretest.ID(coretest.StorePath, hostStruct, symbol.KindStruct),
					Name:   hostStruct,
				}
				row.Methods.Append(&emit.Method{
					Origin: coretest.ID(coretest.StorePath, structMethod, symbol.KindMethod),
					Name:   structMethod,
				})
				e := plugin.NewEmit()
				assert.NoError(tb, e.Add(unit(hostsWord, row)), "the host unit arrives")
				assert.NoError(tb, e.Add(unit(funcsWord, &emit.Function{
					Origin: coretest.ID(coretest.StorePath, structMethod, symbol.KindFunction),
					Name:   structMethod,
				})), "and the unit of the function sharing its method's name")
				return r, &backendtest.Fixture{Emit: e, Schedule: []plugin.ID{fixtureEmitter}}
			}
			failure := assert.Rejects(t, "a member that only another file contains must fail",
				func(tb assert.TB) {
					backendtest.AssertRenderedMembers(tb, shadowed)
				})
			assertMissing(t, failure, []string{hostStruct + "." + structMethod},
				"the failure names the member its host dropped")
		})

		t.Run("fails a member whose name occurs only inside a longer word", func(t *testing.T) {
			t.Parallel()

			// The host name Row contains the member name Ro, and no
			// whole word of the output is Ro.
			prefixed := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				tb.Helper()

				r := memberBuilder(tb, dropsMembers()).Build()
				row := &emit.Struct{
					Origin: coretest.ID(coretest.StorePath, hostStruct, symbol.KindStruct),
					Name:   hostStruct,
				}
				row.Fields.Append(&emit.Field{
					Origin: coretest.ID(coretest.StorePath, prefixedField, symbol.KindField),
					Name:   prefixedField,
				})
				e := plugin.NewEmit()
				assert.NoError(tb, e.Add(unit(hostsWord, row)), "the host unit arrives")
				return r, &backendtest.Fixture{Emit: e, Schedule: []plugin.ID{fixtureEmitter}}
			}
			failure := assert.Rejects(t, "a member that no whole word matches must fail",
				func(tb assert.TB) {
					backendtest.AssertRenderedMembers(tb, prefixed)
				})
			assertMissing(t, failure, []string{hostStruct + "." + prefixedField}, "the failure names the member")
		})

		t.Run("passes a member that a finding contains", func(t *testing.T) {
			t.Parallel()

			// A member the settle reported is excused from the bytes.
			backendtest.AssertRenderedMembers(t, memberExcused)
		})

		t.Run("fails the same member where no finding contains it", func(t *testing.T) {
			t.Parallel()

			silent := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				tb.Helper()

				return memberBuilder(tb, dropsMembers()).Build(), collidingFixture(tb)
			}
			failure := assert.Rejects(t, "a dropped member that no finding contains must fail",
				func(tb assert.TB) {
					backendtest.AssertRenderedMembers(tb, silent)
				})
			assertMissing(t, failure, []string{hostStruct + "." + structField, hostStruct + "." + collidingField},
				"the excusal is the settle's finding, not the fixture")
		})

		t.Run("passes the members of a host of a refused kind", func(t *testing.T) {
			t.Parallel()

			// A refused host renders nothing, so no member of it is missing.
			backendtest.AssertRenderedMembers(t, func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				tb.Helper()

				return sumRefusing(tb).Build(), memberFixture(tb)
			})
		})

		t.Run("fails a lowering that drops its input's origin", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a settle failing the plan must fail",
				func(tb assert.TB) {
					backendtest.AssertRenderedMembers(tb, driftingSetup)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{settleCompletes},
				"the failure names the step that failed")
		})
	})

	t.Run("AssertCoveredFacts", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a backend declaring no coverage", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "an undeclared coverage must fail",
				func(tb assert.TB) {
					backendtest.AssertCoveredFacts(tb, func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
						return wellBuilder(tb).Build(), wellFixture(tb)
					})
				})
			assert.Equal(t, coretest.Contracts(failure), []string{
				"the backend declares its fact coverage, because an undeclared coverage disarms the narrowing guard",
			}, "the failure names the missing declaration")
		})

		t.Run("passes a total declaration whose refusals report", func(t *testing.T) {
			t.Parallel()

			setup := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				tb.Helper()

				r := wellBuilder(tb).
					Coverage(total(map[symbol.Fact]render.Verdict{
						symbol.FactAbstract: render.Refuses,
					})).
					Build()
				return r, abstractFixture(tb)
			}
			backendtest.AssertCoveredFacts(t, setup)
		})

		t.Run("passes a total declaration whose rejections report", func(t *testing.T) {
			t.Parallel()

			setup := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				tb.Helper()

				r := wellBuilder(tb).
					Coverage(total(map[symbol.Fact]render.Verdict{
						symbol.FactAbstract: render.Rejects,
					})).
					Build()
				return r, abstractFixture(tb)
			}
			backendtest.AssertCoveredFacts(t, setup)
		})

		t.Run("counts a variant's refusal under the variant's kind", func(t *testing.T) {
			t.Parallel()

			// A Sum and its variant each have the refused comment. The
			// finding of the variant contains SumVariant, which contains Sum.
			setup := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				tb.Helper()

				r := memberBuilder(tb, spellsMembers()).
					Coverage(total(map[symbol.Fact]render.Verdict{symbol.FactComment: render.Refuses})).
					Build()
				e := plugin.NewEmit()
				assert.NoError(tb, e.Add(unit(hostsWord, shapeOf(sumComment, variantComment))),
					"the sum unit arrives")
				return r, &backendtest.Fixture{Emit: e, Schedule: []plugin.ID{fixtureEmitter}}
			}
			// Go randomises map iteration order per range, and the check
			// ranges over its expectations. Thirty runs visit the Sum key
			// before the SumVariant key at least once with high
			// probability, which is the order a substring match
			// misattributes under. Each refusal counts under the kind it
			// names.
			for range 30 {
				backendtest.AssertCoveredFacts(t, setup)
			}
		})

		t.Run("passes a refused fact stated on a declaration of a refused kind", func(t *testing.T) {
			t.Parallel()

			// A refused declaration renders nothing, so none of its facts
			// reports.
			backendtest.AssertCoveredFacts(t, func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				tb.Helper()

				r := sumRefusing(tb).
					Coverage(total(map[symbol.Fact]render.Verdict{symbol.FactComment: render.Refuses})).
					Build()
				e := plugin.NewEmit()
				assert.NoError(tb, e.Add(unit(hostsWord, shapeOf(sumComment, variantComment))),
					"the sum unit arrives")
				return r, &backendtest.Fixture{Emit: e, Schedule: []plugin.ID{fixtureEmitter}}
			})
		})

		t.Run("fails a declaration missing a fact", func(t *testing.T) {
			t.Parallel()

			partial := total(nil)
			delete(partial.Facts, symbol.FactAsync)
			failure := assert.Rejects(t, "a coverage hole must fail",
				func(tb assert.TB) {
					backendtest.AssertCoveredFacts(tb,
						func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
							tb.Helper()
							return wellBuilder(tb).Coverage(partial).Build(), wellFixture(tb)
						})
				})
			assert.Equal(t, coretest.Contracts(failure), []string{
				"the coverage takes a stance on " + symbol.FactAsync.String() +
					": a declaration is total over the fact set",
			}, "the failure names the missing fact")
		})

		t.Run("fails an exception on a kind that cannot state it", func(t *testing.T) {
			t.Parallel()

			astray := total(nil)
			astray.Except = map[symbol.Kind]map[symbol.Fact]render.Verdict{
				symbol.KindStruct: {symbol.FactTag: render.Refuses},
			}
			failure := assert.Rejects(t, "a stray exception must fail",
				func(tb assert.TB) {
					backendtest.AssertCoveredFacts(tb,
						func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
							tb.Helper()
							return wellBuilder(tb).Coverage(astray).Build(), wellFixture(tb)
						})
				})
			assert.Equal(t, coretest.Contracts(failure), []string{
				"the coverage excepts " + symbol.FactTag.String() + " on a " + symbol.KindStruct.String() +
					", which states it",
			}, "the failure names the stray fact on its kind")
		})

		t.Run("fails a setup returning no fixture", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a fixture without a store must fail",
				func(tb assert.TB) {
					backendtest.AssertCoveredFacts(tb, hollowSetup)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{storeContained},
				"the failure names what the setup did not return")
		})

		t.Run("fails a lowering that drops its input's origin", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a settle failing the plan must fail",
				func(tb assert.TB) {
					backendtest.AssertCoveredFacts(tb,
						func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
							tb.Helper()
							return wellBuilder(tb).Coverage(total(nil)).Lower(drifting).Build(), wellFixture(tb)
						})
				})
			assert.Equal(t, coretest.Contracts(failure), []string{settleCompletes},
				"the failure names the step that failed")
		})

		t.Run("fails a fixture the settle reports on", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a settle withholding a declaration must fail",
				func(tb assert.TB) {
					backendtest.AssertCoveredFacts(tb,
						func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
							tb.Helper()
							return wellBuilder(tb).Coverage(total(nil)).Respell(refusing).Build(), wellFixture(tb)
						})
				})
			assert.Equal(t, coretest.Contracts(failure), []string{settlesClean},
				"the failure names the fixture the coverage is read over")
		})

		t.Run("fails a render that aborts", func(t *testing.T) {
			t.Parallel()

			aborting := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				r := &fake{
					Backend: wellBackend(tb),
					render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
						return nil, errors.New(abortMessage)
					},
				}
				return r, &backendtest.Fixture{Emit: plugin.NewEmit()}
			}

			failure := assert.Rejects(t, "a fatal error must fail the check",
				func(tb assert.TB) {
					backendtest.AssertCoveredFacts(tb, aborting)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{"the render completes"},
				"the failure names the step that failed")
		})

		t.Run("fails a fact the declaration takes no stance on", func(t *testing.T) {
			t.Parallel()

			silent := total(nil)
			silent.Except = map[symbol.Kind]map[symbol.Fact]render.Verdict{
				symbol.KindStruct: {symbol.FactAbstract: render.VerdictUndeclared},
			}
			failure := assert.Rejects(t, "an undeclared verdict must fail",
				func(tb assert.TB) {
					backendtest.AssertCoveredFacts(tb,
						func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
							tb.Helper()
							return wellBuilder(tb).Coverage(silent).Build(), abstractFixture(tb)
						})
				})
			assert.Length(t, failure, 1, "the check fails once")
			assert.HasPrefix(t, failure[0].Contract, "every stated fact meets a verdict: ",
				"the failure names the fact that met no stance")
		})
	})

	t.Run("Rendered", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the findings of the settle and the render", func(t *testing.T) {
			t.Parallel()

			warning := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				return &fake{
					Backend: wellBuilder(tb).Respell(refusing).Build(),
					render: func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
						ctx.Sink.Warnf(render.UnformattedFile, position.Pos{File: fileA}, ctx.Plugin, "%s",
							abortMessage)
						return []plugin.RenderedFile{{Path: fileA, Body: []byte(bodyX)}}, nil
					},
				}, wellFixture(tb)
			}
			files, findings := backendtest.Rendered(t, warning)
			assert.Length(t, files, 1, "the render returns its file")
			codes := make([]diag.Code, len(findings))
			for i, d := range findings {
				codes[i] = d.Code
			}
			assert.Permutation(t, codes, []diag.Code{plugin.RefusedName, render.UnformattedFile},
				"Rendered returns the finding of the settle and the finding of the render")
		})

		t.Run("stops the test where the settle fails", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a settle failing the plan must fail",
				func(tb assert.TB) {
					backendtest.Rendered(tb, driftingSetup)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{settleCompletes},
				"the failure names the settle")
		})

		t.Run("stops the test where the render returns an error", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a render that returns an error must fail",
				func(tb assert.TB) {
					backendtest.Rendered(tb, scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
						return nil, errors.New(abortMessage)
					}))
				})
			assert.Equal(t, coretest.Contracts(failure), []string{
				"a file's problem attaches to the sink and the render continues",
			}, "the failure names the render")
		})
	})

	t.Run("RenderSettled", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the files of the settled fixture", func(t *testing.T) {
			t.Parallel()

			files := backendtest.RenderSettled(t, wellRendered)
			assert.NotEmpty(t, files, "the settled fixture renders whole")
		})

		t.Run("fails a run reporting an Error", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "an error finding must fail the check",
				func(tb assert.TB) {
					backendtest.RenderSettled(tb, reportsOnce(render.UnformattedFile,
						"the formatter refused "+fileA))
				})
			assert.Equal(t, coretest.Contracts(failure), []string{
				"the settled fixture renders clean: the formatter refused " + fileA,
			}, "the check names what a satellite's pins read, and the finding it read")
		})

		t.Run("passes a run reporting a declared kind refusal", func(t *testing.T) {
			t.Parallel()

			// A declared refusal is the fixture's, not a defect.
			files := backendtest.RenderSettled(t, refusingRendered)
			assert.NotEmpty(t, files, "the spelt kinds render")
		})

		t.Run("fails a lowering that drops its input's origin", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a settle failing the plan must fail",
				func(tb assert.TB) {
					backendtest.RenderSettled(tb, driftingSetup)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{settleCompletes},
				"the failure names the step that failed, not the render")
		})
	})

	t.Run("AssertSettledShape", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the kit's clean settle", func(t *testing.T) {
			t.Parallel()

			backendtest.AssertSettledShape(t, wellRendered)
		})

		t.Run("fails a setup that breaks its own isolation", func(t *testing.T) {
			t.Parallel()

			calls := 0
			setup := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				r, f := wellRendered(tb)
				calls++
				if calls > 1 {
					assert.NoError(tb,
						f.Emit.Add(unit(extraWord, fnOf(extraWord, emit.Body{}))),
						"the drifted unit arrives")
				}
				return r, f
			}
			failure := assert.Rejects(t, "a differing second build must fail",
				func(tb assert.TB) {
					backendtest.AssertSettledShape(tb, setup)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{unitsKept}, "the failure names the drift")
		})

		t.Run("passes a respell every reference follows", func(t *testing.T) {
			t.Parallel()

			backendtest.AssertSettledShape(t, richRespelt)
		})

		t.Run("passes a lowering that reshapes declarations", func(t *testing.T) {
			t.Parallel()

			// A declared lowering can reshape declarations, so the check
			// compares the declarations of a settle without the lowering.
			backendtest.AssertSettledShape(t, func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				return wellBuilder(tb).Lower(duplicating).Build(), wellFixture(tb)
			})
		})

		t.Run("fails a respelt build whose declarations differ beyond their names", func(t *testing.T) {
			t.Parallel()

			// The check's third build is the one that settles through the
			// respell alone, and this setup marks its struct abstract.
			calls := 0
			setup := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				r, f := richRespelt(tb)
				calls++
				if calls == 3 {
					for u := range f.Emit.Units() {
						for _, d := range u.Decls {
							if s, isStruct := d.(*emit.Struct); isStruct {
								s.Abstract = true
							}
						}
					}
				}
				return r, f
			}
			failure := assert.Rejects(t, "a declaration that differs beyond its names must fail",
				func(tb assert.TB) {
					backendtest.AssertSettledShape(tb, setup)
				})
			assert.Equal(t, coretest.Contracts(failure),
				[]string{"a respell changes name fields and reference spellings alone"},
				"the failure names the declarations that differ")
		})

		t.Run("fails a setup returning no store", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a fixture without a store must fail",
				func(tb assert.TB) {
					backendtest.AssertSettledShape(tb, hollowSetup)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{storeContained},
				"the failure names what the setup did not return")
		})

		t.Run("fails a lowering that drops its input's origin", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a settle failing the plan must fail",
				func(tb assert.TB) {
					backendtest.AssertSettledShape(tb, driftingSetup)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{settleCompletes},
				"the failure names the step that failed")
		})

		t.Run("stops at the end of the shorter build", func(t *testing.T) {
			t.Parallel()

			calls := 0
			setup := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				r, f := wellRendered(tb)
				calls++
				if calls == 1 {
					assert.NoError(tb,
						f.Emit.Add(unit(extraWord, fnOf(extraWord, emit.Body{}))),
						"the drifted unit arrives")
				}
				return r, f
			}
			failure := assert.Rejects(t, "a first build of one unit more must fail",
				func(tb assert.TB) {
					backendtest.AssertSettledShape(tb, setup)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{unitsKept},
				"the unit count is the one failure: the check stops where the shorter build ends")
		})
	})

	t.Run("AssertPopulatedFixture", func(t *testing.T) {
		t.Parallel()

		t.Run("fails an empty store", func(t *testing.T) {
			t.Parallel()

			hollow := scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
				return nil, nil
			})

			failure := assert.Rejects(t, "an empty store must fail the check",
				func(tb assert.TB) {
					backendtest.AssertPopulatedFixture(tb, hollow)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{"the fixture emits at least one unit"},
				"the check demands a populated fixture")
		})

		t.Run("fails a fixture without a store", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a fixture without a store must fail the check",
				func(tb assert.TB) {
					backendtest.AssertPopulatedFixture(tb, hollowSetup)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{storeContained},
				"the missing store is the one failure: the check stops and ranges over nothing")
		})
	})

	t.Run("AssertEmptyRender", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the kit backend", func(t *testing.T) {
			t.Parallel()

			backendtest.AssertEmptyRender(t, wellRendered)
		})

		tests := []struct {
			name     string
			give     func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error)
			contract string
		}{
			{
				name: "fails a render that returns a file for an empty store",
				give: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
					return []plugin.RenderedFile{{Path: fileA, Body: []byte(bodyX)}}, nil
				},
				contract: "the render of an empty store returns no file",
			},
			{
				name: "fails a render that returns an error for an empty store",
				give: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
					return nil, errors.New(abortMessage)
				},
				contract: "the render of an empty store completes",
			},
			{
				name: "fails a render that panics over an empty store",
				give: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
					panic(abortMessage)
				},
				contract: "the settle and the render of an empty store run without a panic",
			},
			{
				name: "fails an Error over an empty store",
				give: func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
					ctx.Sink.Errorf(render.UnformattedFile, position.Pos{File: fileA}, ctx.Plugin, "%s", abortMessage)
					return nil, nil
				},
				contract: "the render of an empty store reports no Error: " + abortMessage,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				failure := assert.Rejects(t, "a render that breaks the empty-store rule must fail the check",
					func(tb assert.TB) {
						backendtest.AssertEmptyRender(tb, scripted(tt.give))
					})
				assert.Equal(t, coretest.Contracts(failure), []string{tt.contract},
					"the check names the rule the render broke")
			})
		}
	})

	t.Run("AssertDeterministicRender", func(t *testing.T) {
		t.Parallel()

		t.Run("fails bytes that vary between runs", func(t *testing.T) {
			t.Parallel()

			var runs int
			varying := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				runs++
				stamp := strconv.Itoa(runs)
				return &fake{
					Backend: wellBackend(tb),
					render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
						return []plugin.RenderedFile{{Path: fileA, Body: []byte(stamp)}}, nil
					},
				}, &backendtest.Fixture{Emit: plugin.NewEmit()}
			}

			failure := assert.Rejects(t, "bytes that depend on run state must fail the check",
				func(tb assert.TB) {
					backendtest.AssertDeterministicRender(tb, varying)
				})
			assert.Length(t, failure, 1, "the check fails once")
			expect.Equal(t, failure[0].Assertion, "deterministic", "on the determinism of the render")
			expect.Equal(t, failure[0].Contract,
				"isolated renders return the same files and report the same findings",
				"under the byte-identity contract")
		})

		t.Run("passes one finding set reported in two orders", func(t *testing.T) {
			t.Parallel()

			var runs int
			swapping := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				runs++
				first := runs == 1
				r := &fake{
					Backend: wellBackend(tb),
					render: func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
						dropped := func() {
							ctx.Sink.Errorf(render.DroppedSlots,
								position.Pos{File: fileA, Line: 1, Col: 1}, ctx.Plugin,
								"gen's template placed no marker")
						}
						unformatted := func() {
							ctx.Sink.Errorf(render.UnformattedFile,
								position.Pos{File: fileB, Line: 2, Col: 1}, ctx.Plugin,
								"the formatter refused %s", fileB)
						}
						if first {
							dropped()
							unformatted()
						} else {
							unformatted()
							dropped()
						}
						return []plugin.RenderedFile{{Path: fileA, Body: []byte(bodyX)}}, nil
					},
				}
				return r, &backendtest.Fixture{Emit: plugin.NewEmit()}
			}

			backendtest.AssertDeterministicRender(t, swapping)
			assert.InRange(t, runs, 2, math.MaxInt32,
				"the check rendered more than once and compared the findings as a set")
		})
	})

	t.Run("AssertSpeltKinds", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a kind the language neither spells nor refuses", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "an unspelt kind must fail the check",
				func(tb assert.TB) {
					backendtest.AssertSpeltKinds(tb, reportsOnce(render.UnspeltKind,
						"printer declares no template for Enum"))
				})
			assert.Equal(t, coretest.Contracts(failure), []string{
				"every kind the fixture emits has a spelling or a declared refusal: " +
					"printer declares no template for Enum",
			}, "the check names the missing spelling")
		})

		t.Run("passes a kind the language declares refused", func(t *testing.T) {
			t.Parallel()

			// A declared refusal is a spelling decision.
			backendtest.AssertSpeltKinds(t, reportsOnce(render.RefusedKind,
				"printer refuses the Sum kind: "+kindReason))
		})
	})

	t.Run("AssertPlacedContent", func(t *testing.T) {
		t.Parallel()

		t.Run("fails content that went unplaced", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "dropped slot content must fail the check",
				func(tb assert.TB) {
					backendtest.AssertPlacedContent(tb, reportsOnce(render.DroppedSlots,
						"gen's template placed no marker and 2 contributions are pending"))
				})
			assert.Equal(t, coretest.Contracts(failure), []string{
				"every body arrives whole: gen's template placed no marker and 2 contributions are pending",
			}, "the check requires every body to arrive whole")
		})
	})

	t.Run("AssertContinuedRender", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a renderer that aborts", func(t *testing.T) {
			t.Parallel()

			aborting := scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
				return nil, errors.New(abortMessage)
			})

			failure := assert.Rejects(t, "a fatal error must fail the check",
				func(tb assert.TB) {
					backendtest.AssertContinuedRender(tb, aborting)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{
				"a file's problem attaches to the sink and the render continues",
			}, "the check names the continuation rule")
		})

		tests := []struct {
			name     string
			give     func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error)
			contract string
		}{
			{
				name: "fails a finding without a position",
				give: func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
					ctx.Sink.Errorf(render.UnformattedFile,
						position.Pos{}, ctx.Plugin, "somewhere, something broke")
					return nil, nil
				},
				contract: "every finding has a position: somewhere, something broke",
			},
			{
				name: "fails a finding under another plugin's origin",
				give: func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
					ctx.Sink.Errorf(render.UnformattedFile,
						position.Pos{File: fileA}, outsider, "not the context's")
					return nil, nil
				},
				contract: "every finding names the context's plugin as its origin: not the context's",
			},
			{
				name: "fails a returned file reported unformatted",
				give: func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
					ctx.Sink.Errorf(render.UnformattedFile,
						position.Pos{File: fileA}, ctx.Plugin, "the formatter refused %s", fileA)
					return []plugin.RenderedFile{{Path: fileA, Body: []byte(bodyX)}}, nil
				},
				contract: "a file the formatter refused remains withheld",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				failure := assert.Rejects(t, "a breach of the failure semantics must fail the check",
					func(tb assert.TB) {
						backendtest.AssertContinuedRender(tb, scripted(tt.give))
					})
				assert.Equal(t, coretest.Contracts(failure), []string{tt.contract},
					"the check names the rule the render broke")
			})
		}

		t.Run("passes a format failure the render continues past", func(t *testing.T) {
			t.Parallel()

			partial := func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
				f := wellFixture(tb)
				b := backend.New(halfName, stubTarget,
					plugin.CommentSyntax{Line: []string{lineComment}}).
					KindTemplates(map[symbol.Kind]string{
						symbol.KindStruct:   structTpl,
						symbol.KindFunction: "func {{.Name}}()\n",
					}).
					Naming(wordNaming).
					Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
						return []byte("\treturn\n"), nil
					}).
					Imports(func(*render.ImportSet) string { return "" }).
					Finalise(func(src []byte) ([]byte, error) {
						if strings.Contains(string(src), hostStruct) {
							return nil, errors.New("unparseable")
						}
						return src, nil
					}).
					Build()
				return b, f
			}

			backendtest.AssertContinuedRender(t, partial)
		})
	})
}

// assertMissing checks that the member check failed once for each
// member in want, and for no other member. Each entry of want has the
// form Host.Member, in the order of the store.
func assertMissing(tb assert.TB, failure []assert.Failure, want []string, msg string) {
	tb.Helper()

	contracts := make([]string, len(want))
	for i, member := range want {
		contracts[i] = member + memberRendered
	}
	assert.Equal(tb, coretest.Contracts(failure), contracts, msg)
}

// call returns a scaffold statement that calls n.
func call(n string) emit.Stmt {
	return emit.Stmt{Kind: emit.StmtExpr, Value: emit.Expr{Kind: emit.ExprName, Name: n}}
}

// unit returns a plan unit of the fixture's emitting plugin. The word is
// also the tag of the family, as distinct declared outputs arrive as
// distinct accumulators.
func unit(word string, decls ...symbol.Symbol) plugin.Unit {
	return plugin.Unit{
		Plugin: fixtureEmitter, Tag: word, Per: plugin.PerPlan, Word: word, Decls: decls,
	}
}

// fnOf returns a function declaration with the given body.
func fnOf(name string, body emit.Body) *emit.Function {
	f := &emit.Function{
		Origin: coretest.Struct(coretest.StorePath, name).ID,
		Name:   name,
	}
	f.Body = body
	return f
}

// wordNaming returns the word of a unit followed by the fixture's
// extension.
func wordNaming(u plugin.Unit) string { return u.Word + fileExt }

// passThrough is the fixture's formatter. It returns its input.
func passThrough(src []byte) ([]byte, error) { return src, nil }

// wellBackend builds the fixture's backend through the kit. The backend
// has two kind templates, a scaffold for names and returns, and a
// formatter that returns its input.
func wellBackend(tb assert.TB) backend.Backend {
	tb.Helper()

	return wellBuilder(tb).Coverage(total(nil)).Build()
}

// wellBuilder returns the declaration of the fixture's backend, so a
// case can add parts before Build.
func wellBuilder(tb assert.TB) *backend.Builder {
	tb.Helper()

	return backend.New(printerName, stubTarget,
		plugin.CommentSyntax{Line: []string{lineComment}}).
		KindTemplates(map[symbol.Kind]string{
			symbol.KindStruct:   structTpl,
			symbol.KindFunction: functionTpl,
		}).
		Naming(wordNaming).
		Scaffold(func(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
			switch s.Kind {
			case emit.StmtExpr:
				set.Add(runtimePkg)
				return []byte("\t" + s.Value.Name + "()\n"), nil
			case emit.StmtReturn:
				return []byte("\treturn\n"), nil
			default:
				return nil, errors.New("the fixture spells names and returns only")
			}
		}).
		Imports(func(set *render.ImportSet) string {
			if set.Len() == 0 {
				return ""
			}
			return "import (" + strings.Join(set.Paths(), " ") + ")\n"
		}).
		Finalise(passThrough)
}

// wellFixture returns the valid fixture. Its store has every kind that
// the backend spells and a body in each of the four content forms. A
// tree with the marker places the slot content of the reference.
func wellFixture(tb assert.TB) *backendtest.Fixture {
	tb.Helper()

	refBody := emit.Body{Ref: &emit.TemplateRef{Name: refTemplate}}
	refBody.Prologue.Append(call(guardCall))
	e := plugin.NewEmit()
	assert.Total(tb, e.Add, []plugin.Unit{
		unit(stubWord,
			&emit.Struct{
				Origin: coretest.Struct(coretest.StorePath, hostStruct).ID,
				Name:   hostStruct,
			},
			fnOf(noopName, emit.Body{}),
			fnOf(loadName, emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}}),
		),
		unit(refWord, fnOf(saveName, refBody)),
		unit(rawWord, fnOf(dumpName, emit.Body{Verbatim: dumpBody})),
	}, "the fixture unit arrives")
	return &backendtest.Fixture{
		Emit:     e,
		Schedule: []plugin.ID{fixtureEmitter},
		Trees: map[plugin.ID]fs.FS{
			fixtureEmitter: fstest.MapFS{
				refTemplate: &fstest.MapFile{Data: []byte(refSource)},
			},
		},
	}
}

// wellRendered returns the kit backend with the valid fixture.
func wellRendered(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
	tb.Helper()

	return wellBackend(tb), wellFixture(tb)
}

// shapeOf returns the sum host of the member fixture with one variant.
// comment is the comment of the sum, and variant is the comment of its
// variant.
func shapeOf(comment, variant string) *emit.Sum {
	shape := &emit.Sum{
		Origin:  coretest.ID(coretest.StorePath, hostSum, symbol.KindSum),
		Name:    hostSum,
		Comment: comment,
	}
	shape.Variants.Append(&emit.SumVariant{
		Origin:  coretest.ID(coretest.StorePath, sumVariant, symbol.KindSumVariant),
		Name:    sumVariant,
		Comment: variant,
	})
	return shape
}

// refusingRendered is the valid setup with a sum beside the kinds that
// the backend spells. The backend refuses the sum kind, so the suite
// meets a declared refusal as it does over the canonical fixture.
func refusingRendered(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
	tb.Helper()

	r := wellBuilder(tb).
		Coverage(total(nil)).
		RefusedKinds(map[symbol.Kind]string{symbol.KindSum: kindReason}).
		Build()
	f := wellFixture(tb)
	assert.NoError(tb, f.Emit.Add(unit(sumsWord, shapeOf("", ""))), "the refused unit arrives")
	return r, f
}

// scripted returns a setup with the fake backend and a fixture without
// units.
func scripted(r func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error)) backendtest.Setup {
	return func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
		return &fake{Backend: wellBackend(tb), render: r}, &backendtest.Fixture{Emit: plugin.NewEmit()}
	}
}

// hollowSetup returns a backend with a fixture without a store. Every
// check fails on it before it reads anything. The backend returns a
// whole file, so a check fails on the missing store alone.
func hollowSetup(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
	return &fake{Backend: wellBackend(tb), render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
		return []plugin.RenderedFile{{Path: fileA, Body: []byte(bodyX)}}, nil
	}}, &backendtest.Fixture{}
}

// hollowBacked returns a kit backend with a fixture without a store, so
// a check that settles through the backend has nothing to settle.
func hollowBacked(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
	tb.Helper()

	return wellBackend(tb), &backendtest.Fixture{}
}

// drifting is a lowering that drops the provenance of its input.
// [plugin.Settle] fails the plan on this defect, without a finding on a
// declaration.
func drifting(s symbol.Symbol) ([]symbol.Symbol, error) {
	st, isStruct := s.(*emit.Struct)
	if !isStruct {
		return nil, nil
	}
	return []symbol.Symbol{&emit.Struct{Name: st.Name}}, nil
}

// driftingSetup returns the kit backend with the drifting lowering and
// the valid fixture, so a check meets a settle that fails the plan.
func driftingSetup(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
	tb.Helper()

	return wellBuilder(tb).Lower(drifting).Build(), wellFixture(tb)
}

// refusing is a respell that returns an error for every struct name.
// The settle withholds the struct under a finding and the plan
// continues, so a check meets a settle that reports and does not fail.
func refusing(
	_, kind symbol.Kind, _ symbol.Visibility, name string,
) (string, error) {
	if kind == symbol.KindStruct {
		return "", errors.New("the fixture target spells no struct name")
	}
	return name, nil
}

// refusedSetup returns the kit backend with the refusing respell and
// the valid fixture. A check then meets a settle that reports and
// withholds a declaration while the plan continues.
func refusedSetup(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
	tb.Helper()

	return wellBuilder(tb).Respell(refusing).Build(), wellFixture(tb)
}

// lowerFirst returns a name with its leading letter lowered. It gives
// the two fields of the colliding fixture one name.
func lowerFirst(
	_, _ symbol.Kind, _ symbol.Visibility, name string,
) (string, error) {
	if name == "" {
		return name, nil
	}
	return strings.ToLower(name[:1]) + name[1:], nil
}

// memberFixture returns a store with one host of each kind with
// members. Each host has the members that a rendered file must contain.
func memberFixture(tb assert.TB) *backendtest.Fixture {
	tb.Helper()

	row := &emit.Struct{
		Origin: coretest.ID(coretest.StorePath, hostStruct, symbol.KindStruct),
		Name:   hostStruct,
	}
	row.Fields.Append(&emit.Field{
		Origin: coretest.ID(coretest.StorePath, structField, symbol.KindField),
		Name:   structField,
	})
	row.Methods.Append(&emit.Method{
		Origin: coretest.ID(coretest.StorePath, structMethod, symbol.KindMethod),
		Name:   structMethod,
	})
	store := &emit.Interface{
		Origin: coretest.ID(coretest.StorePath, hostInterface, symbol.KindInterface),
		Name:   hostInterface,
	}
	store.Fields.Append(&emit.Field{
		Origin: coretest.ID(coretest.StorePath, ifaceField, symbol.KindField),
		Name:   ifaceField,
	})
	store.Methods.Append(&emit.Method{
		Origin: coretest.ID(coretest.StorePath, ifaceMethod, symbol.KindMethod),
		Name:   ifaceMethod,
	})
	phase := &emit.Enum{
		Origin: coretest.ID(coretest.StorePath, hostEnum, symbol.KindEnum),
		Name:   hostEnum,
	}
	phase.Variants.Append(&emit.EnumVariant{
		Origin: coretest.ID(coretest.StorePath, enumVariant, symbol.KindEnumVariant),
		Name:   enumVariant,
	})

	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(unit(hostsWord, row, store, phase, shapeOf("", ""))),
		"the memberful unit arrives")
	return &backendtest.Fixture{Emit: e, Schedule: []plugin.ID{fixtureEmitter}}
}

// collidingFixture returns a store with a struct of two fields, to
// which lowerFirst gives one name.
func collidingFixture(tb assert.TB) *backendtest.Fixture {
	tb.Helper()

	row := &emit.Struct{
		Origin: coretest.ID(coretest.StorePath, hostStruct, symbol.KindStruct),
		Name:   hostStruct,
	}
	row.Fields.Append(
		&emit.Field{
			Origin: coretest.ID(coretest.StorePath, structField, symbol.KindField),
			Name:   structField,
		},
		&emit.Field{
			Origin: coretest.ID(coretest.StorePath, collidingField, symbol.KindField),
			Name:   collidingField,
		},
	)

	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(unit(hostsWord, row)), "the colliding unit arrives")
	return &backendtest.Fixture{Emit: e, Schedule: []plugin.ID{fixtureEmitter}}
}

// spellsMembers returns the kind templates that place every member
// inside its host.
func spellsMembers() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    "type {{.Name}} struct {\n" + fieldLines + methodLines + "}\n",
		symbol.KindInterface: "type {{.Name}} interface {\n" + fieldLines + methodLines + "}\n",
		symbol.KindEnum:      "type {{.Name}} enum {\n" + variantLines + "}\n",
		symbol.KindSum:       "type {{.Name}} sum {\n" + variantLines + "}\n",
	}
}

// dropsMembers returns kind templates that do not range over a member
// list.
func dropsMembers() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    "type {{.Name}} struct {}\n",
		symbol.KindInterface: "type {{.Name}} interface {}\n",
		symbol.KindEnum:      "type {{.Name}} enum {}\n",
		symbol.KindSum:       "type {{.Name}} sum {}\n",
	}
}

// memberBuilder returns the declaration of a backend with the kind
// templates kinds, so one case renders members and another drops them.
func memberBuilder(tb assert.TB, kinds map[symbol.Kind]string) *backend.Builder {
	tb.Helper()

	return backend.New(memberName, stubTarget,
		plugin.CommentSyntax{Line: []string{lineComment}}).
		KindTemplates(kinds).
		Naming(wordNaming).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return nil, errors.New("the member fixture states no scaffolding")
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(passThrough)
}

// sumRefusing returns the member backend that refuses the sum kind. Its
// other host templates place every member.
func sumRefusing(tb assert.TB) *backend.Builder {
	tb.Helper()

	kinds := spellsMembers()
	delete(kinds, symbol.KindSum)
	return memberBuilder(tb, kinds).
		RefusedKinds(map[symbol.Kind]string{symbol.KindSum: kindReason})
}

// memberSpelt returns the setup whose host templates place every
// member.
func memberSpelt(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
	tb.Helper()

	return memberBuilder(tb, spellsMembers()).Build(), memberFixture(tb)
}

// memberDropped returns the setup whose host templates place the host
// name alone, so no member renders.
func memberDropped(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
	tb.Helper()

	return memberBuilder(tb, dropsMembers()).Build(), memberFixture(tb)
}

// memberExcused returns the setup that drops its members after the
// settle reported them. The two fields collide under the respell, so a
// finding contains both, and the check reports no drop.
func memberExcused(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
	tb.Helper()

	return memberBuilder(tb, dropsMembers()).Respell(lowerFirst).Build(), collidingFixture(tb)
}

// richFixture returns a store with every kind of reference that the
// settle follows. The store has a type reference to a declared type, the
// body of a method, a named slot, a guard on a parameter and a call
// without a callee.
func richFixture(tb assert.TB) *backendtest.Fixture {
	tb.Helper()

	body := emit.Body{Stmts: []emit.Stmt{
		{
			Kind: emit.StmtGuard,
			Name: richParam,
			Then: []emit.Stmt{{Kind: emit.StmtReturn}},
		},
		{Kind: emit.StmtExpr, Value: emit.Expr{Kind: emit.ExprCall}},
	}}
	body.Prologue.Append(emit.Stmt{
		Kind: emit.StmtExpr,
		Value: emit.Expr{
			Kind: emit.ExprCall,
			Fn:   &emit.Expr{Kind: emit.ExprName, Name: richFn},
			Args: []emit.Expr{{Kind: emit.ExprName, Name: richParam}},
		},
	})
	body.Declare(richSlot).Append(call(richFn))

	row := &emit.Struct{
		Origin: coretest.ID(coretest.StorePath, richStruct, symbol.KindStruct),
		Name:   richStruct,
	}
	row.Fields.Append(&emit.Field{
		Origin: coretest.ID(coretest.StorePath, richField, symbol.KindField),
		Name:   richField,
		Type:   &emit.TypeRef{Spelling: richStruct},
	})
	row.Methods.Append(&emit.Method{
		Origin: coretest.ID(coretest.StorePath, richMethod, symbol.KindMethod),
		Name:   richMethod,
		Params: emit.SlotOf(&emit.Param{Name: richParam, Type: &emit.TypeRef{Spelling: intType}}),
		Body:   body,
	})
	boot := &emit.Function{
		Origin: coretest.ID(coretest.StorePath, richFn, symbol.KindFunction),
		Name:   richFn,
	}

	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(unit(richWord, row, boot)),
		"the reference unit arrives")
	return &backendtest.Fixture{Emit: e, Schedule: []plugin.ID{fixtureEmitter}}
}

// richRespelt returns the setup whose backend respells every declared
// name and declares no lowering. The check compares its settled store
// with a fresh build after it normalizes every name.
func richRespelt(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
	tb.Helper()

	return wellBuilder(tb).Respell(upperFirst).Build(), richFixture(tb)
}

// upperFirst returns a name with its leading letter raised. The settle
// applies it to every reference to a declared name.
func upperFirst(
	_, _ symbol.Kind, _ symbol.Visibility, name string,
) (string, error) {
	if name == "" {
		return name, nil
	}
	return strings.ToUpper(name[:1]) + name[1:], nil
}

// duplicating is a lowering that adds a companion struct under the
// origin of each struct. The companion reshapes the store, so the check
// compares the declarations of a settle without the lowering.
func duplicating(s symbol.Symbol) ([]symbol.Symbol, error) {
	st, isStruct := s.(*emit.Struct)
	if !isStruct {
		return nil, nil
	}
	return []symbol.Symbol{st, &emit.Struct{Origin: st.Origin, Name: st.Name + companionSuffix}}, nil
}

// total returns a coverage with a verdict for every fact. A verdict in
// over replaces the default verdict of its fact.
func total(over map[symbol.Fact]render.Verdict) render.Coverage {
	facts := map[symbol.Fact]render.Verdict{}
	for _, f := range symbol.Facts() {
		facts[f] = render.Renders
	}
	maps.Copy(facts, over)
	return render.Coverage{Facts: facts}
}

// abstractFixture returns the valid fixture with the abstract fact on
// its struct, so the coverage guard of a render meets a fact.
func abstractFixture(tb assert.TB) *backendtest.Fixture {
	tb.Helper()

	f := wellFixture(tb)
	for u := range f.Emit.Units() {
		for _, d := range u.Decls {
			if s, isStruct := d.(*emit.Struct); isStruct {
				s.Abstract = true
			}
		}
	}
	return f
}

// reportsOnce returns a setup whose renderer reports one Error under
// code at fileA and returns no file.
func reportsOnce(code diag.Code, msg string) backendtest.Setup {
	return scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
		ctx.Sink.Errorf(code, position.Pos{File: fileA}, ctx.Plugin, "%s", msg)
		return nil, nil
	})
}
