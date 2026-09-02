// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
	"errors"
	"io/fs"
	"maps"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// call returns the one-line scaffold statement naming n.
func call(n string) emit.Stmt {
	return emit.Stmt{Kind: emit.StmtExpr, Value: emit.Expr{Kind: emit.ExprName, Name: n}}
}

// unit returns one flushed plan unit under the emitting fixture
// plugin, its word doubling as the family tag the way distinct
// declared outputs arrive as distinct accumulators.
func unit(word string, decls ...symbol.Symbol) plugin.Unit {
	return plugin.Unit{
		Plugin: "gen", Tag: word, Per: plugin.PerPlan, Word: word, Decls: decls,
	}
}

// fnOf returns a function declaration carrying body.
func fnOf(name string, body emit.Body) *emit.Function {
	f := &emit.Function{
		Origin: coretest.Struct(coretest.StorePath, name).ID,
		Name:   name,
	}
	f.Body = body
	return f
}

// wellBackend builds the fixture backend through the kit: two
// kinds, a scaffold for names and returns, and a pass-through
// formatter.
func wellBackend(tb assert.TB) plugin.Renderer {
	tb.Helper()

	r, held := wellBuilder(tb).
		Coverage(total(nil)).
		Build().(plugin.Renderer)
	assert.True(tb, held, "the kit backend renders")
	return r
}

// wellBuilder accumulates the fixture backend's declaration, so a
// test can widen it before Build.
func wellBuilder(tb assert.TB) *backend.Builder {
	tb.Helper()

	return backend.New("printer", "stub",
		plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{
			symbol.KindStruct:   "type {{.Name}} struct{}\n",
			symbol.KindFunction: "func {{.Name}}() {\n{{body .}}}\n",
		}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
			switch s.Kind {
			case emit.StmtExpr:
				set.Add("stub/runtime")
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
		Finalise(func(src []byte) ([]byte, error) { return src, nil })
}

// wellFixture returns the valid fixture: a store carrying every
// kind the backend spells and a body in each of the four content
// forms, with the reference's slot content spliced through a
// marker-placing tree.
func wellFixture(tb assert.TB) *backendtest.Fixture {
	tb.Helper()

	refBody := emit.Body{Ref: &emit.TemplateRef{Name: "save.tpl"}}
	refBody.Prologue.Append(call("guard"))
	e := plugin.NewEmit()
	for _, u := range []plugin.Unit{
		unit("stub",
			&emit.Struct{
				Origin: coretest.Struct(coretest.StorePath, "Row").ID,
				Name:   "Row",
			},
			fnOf("Noop", emit.Body{}),
			fnOf("Load", emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}}),
		),
		unit("ref", fnOf("Save", refBody)),
		unit("raw", fnOf("Dump", emit.Body{Verbatim: "\tdump()\n"})),
	} {
		assert.NoError(tb, e.Add(u), "the fixture unit arrives")
	}
	return &backendtest.Fixture{
		Emit:     e,
		Schedule: []plugin.ID{"gen"},
		Trees: map[plugin.ID]fs.FS{
			"gen": fstest.MapFS{
				"save.tpl": &fstest.MapFile{Data: []byte("\tsaving()\n{{slots}}")},
			},
		},
	}
}

// wellRendered returns the valid setup: the kit backend over the
// valid fixture.
func wellRendered(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	return wellBackend(tb), wellFixture(tb)
}

// fake is a renderer the rejection tests script: the suite has to
// catch every way a renderer can cheat the rules.
type fake struct {
	render func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error)
}

func (f *fake) Render(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
	return f.render(ctx)
}

// covering is a renderer declaring fact coverage without holding
// the backend roles, so a check reads a declaration off a renderer
// that never settles.
type covering struct {
	fake
	coverage render.Coverage
}

// Coverage implements [render.Coverer] through the declared value.
func (c *covering) Coverage() render.Coverage { return c.coverage }

// scripted returns a setup handing the fake over a bare fixture.
func scripted(r func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error)) backendtest.Setup {
	return func(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
		return &fake{render: r}, &backendtest.Fixture{Emit: plugin.NewEmit()}
	}
}

// hollowSetup hands a renderer over a fixture carrying no store,
// which every check refuses before it reads anything. The renderer
// answers a whole file, so what a check refuses is the missing
// store and nothing further along.
func hollowSetup(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	return &fake{render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
		return []plugin.RenderedFile{{Name: "a.txt", Body: []byte("x\n")}}, nil
	}}, &backendtest.Fixture{}
}

// hollowBacked hands a kit backend over a fixture carrying no
// store, so a check that takes the backend's seams still has
// nothing to settle.
func hollowBacked(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	return wellBackend(tb), &backendtest.Fixture{}
}

// drifting is a lowering that drops its input's provenance: the
// defect [plugin.Settle] refuses as a plan failure rather than as
// one declaration's finding.
func drifting(s symbol.Symbol) ([]symbol.Symbol, error) {
	st, held := s.(*emit.Struct)
	if !held {
		return nil, nil
	}
	return []symbol.Symbol{&emit.Struct{Name: st.Name}}, nil
}

// driftingSetup hands the kit backend whose lowering drifts over
// the valid fixture, so a check meets a settle that fails the plan.
func driftingSetup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := wellBuilder(tb).Lower(drifting).Build().(plugin.Renderer)
	assert.True(tb, held, "the lowering backend renders")
	return r, wellFixture(tb)
}

// refusing is a respell whose target spells no struct name: the
// settle withholds the declaration under a finding and the plan
// stands, so a check meets a settle that reports without failing.
func refusing(
	_, kind symbol.Kind, _ symbol.Visibility, name string,
) (string, error) {
	if kind == symbol.KindStruct {
		return "", errors.New("the fixture target spells no struct name")
	}
	return name, nil
}

// refusedSetup hands the kit backend whose respell refuses a struct
// name over the valid fixture, so a check meets a settle that
// reports and withholds a declaration while the plan stands.
func refusedSetup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := wellBuilder(tb).Respell(refusing).Build().(plugin.Renderer)
	assert.True(tb, held, "the respelling backend renders")
	return r, wellFixture(tb)
}

// lowerFirst spells a name with its leading letter lowered, which
// is what makes the colliding fixture's two fields meet.
func lowerFirst(
	_, _ symbol.Kind, _ symbol.Visibility, name string,
) (string, error) {
	if name == "" {
		return name, nil
	}
	return strings.ToLower(name[:1]) + name[1:], nil
}

// The declarations the member fixture carries: one host per
// member-holding kind and the members hanging on each, so a check
// reading member names meets a field, a method and both variant
// forms.
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
)

// memberFixture returns a store carrying one host per
// member-holding kind, each holding the members a rendered file has
// to carry.
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
	shape := &emit.Sum{
		Origin: coretest.ID(coretest.StorePath, hostSum, symbol.KindSum),
		Name:   hostSum,
	}
	shape.Variants.Append(&emit.SumVariant{
		Origin: coretest.ID(coretest.StorePath, sumVariant, symbol.KindSumVariant),
		Name:   sumVariant,
	})

	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(unit("hosts", row, store, phase, shape)),
		"the memberful unit arrives")
	return &backendtest.Fixture{Emit: e, Schedule: []plugin.ID{"gen"}}
}

// collidingField is the second field name the colliding fixture
// carries. It settles to the first one's spelling under a respell
// that lowers the leading letter, so the settle reports both names
// and keeps them as emitted.
const collidingField = "cell"

// collidingFixture returns a store whose struct carries two fields
// one respell cannot tell apart.
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
	assert.NoError(tb, e.Add(unit("hosts", row)), "the colliding unit arrives")
	return &backendtest.Fixture{Emit: e, Schedule: []plugin.ID{"gen"}}
}

// The member lists a host template ranges. A backend spelling them
// puts every member name in the bytes; one leaving them out drops
// the members with no finding, which is the defect
// [backendtest.AssertRenderedMembers] exists to catch.
const (
	fieldLines   = "{{range .Fields.Items}}\t{{.Name}}\n{{end}}"
	methodLines  = "{{range .Methods.Items}}\t{{.Name}}\n{{end}}"
	variantLines = "{{range .Variants.Items}}\t{{.Name}}\n{{end}}"
)

// spellsMembers is the kind spelling that places every member name
// inside its host.
func spellsMembers() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    "type {{.Name}} struct {\n" + fieldLines + methodLines + "}\n",
		symbol.KindInterface: "type {{.Name}} interface {\n" + fieldLines + methodLines + "}\n",
		symbol.KindEnum:      "type {{.Name}} enum {\n" + variantLines + "}\n",
		symbol.KindSum:       "type {{.Name}} sum {\n" + variantLines + "}\n",
	}
}

// dropsMembers is the kind spelling that ranges no member list.
func dropsMembers() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    "type {{.Name}} struct {}\n",
		symbol.KindInterface: "type {{.Name}} interface {}\n",
		symbol.KindEnum:      "type {{.Name}} enum {}\n",
		symbol.KindSum:       "type {{.Name}} sum {}\n",
	}
}

// memberBuilder accumulates a backend over the member-holding kinds
// with the given spellings, so one test renders members and another
// drops them.
func memberBuilder(tb assert.TB, kinds map[symbol.Kind]string) *backend.Builder {
	tb.Helper()

	return backend.New("member", "stub",
		plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(kinds).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return nil, errors.New("the member fixture carries no scaffolding")
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil })
}

// memberSpelt is the setup whose host templates place every member.
func memberSpelt(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := memberBuilder(tb, spellsMembers()).Build().(plugin.Renderer)
	assert.True(tb, held, "the member backend renders")
	return r, memberFixture(tb)
}

// memberDropped is the setup whose host templates place the host
// name alone, so every member arrives nowhere.
func memberDropped(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := memberBuilder(tb, dropsMembers()).Build().(plugin.Renderer)
	assert.True(tb, held, "the member backend renders")
	return r, memberFixture(tb)
}

// memberExcused is the setup that drops its members after a settle
// already reported them: the two fields collide under the respell,
// so a finding names both and the check has no drop to report.
func memberExcused(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := memberBuilder(tb, dropsMembers()).
		Respell(lowerFirst).Build().(plugin.Renderer)
	assert.True(tb, held, "the respelling member backend renders")
	return r, collidingFixture(tb)
}

// The declarations the reference fixture carries. Each name is one
// the settle rewrites and something else follows: the field's type
// names its own host, the method's body calls the function and
// guards on its parameter, and the named slot holds a call of its
// own.
const (
	richFn     = "boot"
	richStruct = "row"
	richField  = "next"
	richMethod = "touch"
	richParam  = "item"
	richSlot   = "extra"
)

// richFixture returns a store carrying every reference the settle
// follows: a type reference naming a declared type, a member
// method's body, a named slot, a guard on a parameter, and a call
// whose callee is absent.
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
		Params: []*emit.Param{{Name: richParam, Type: &emit.TypeRef{Spelling: "int"}}},
		Body:   body,
	})
	boot := &emit.Function{
		Origin: coretest.ID(coretest.StorePath, richFn, symbol.KindFunction),
		Name:   richFn,
	}

	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(unit("rich", row, boot)),
		"the reference unit arrives")
	return &backendtest.Fixture{Emit: e, Schedule: []plugin.ID{"gen"}}
}

// richRespelt is the setup whose backend respells every declared
// name and declares no lowering, so the settled store is compared
// against a fresh build once every name normalizes.
func richRespelt(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := wellBuilder(tb).Respell(upperFirst).Build().(plugin.Renderer)
	assert.True(tb, held, "the respelling backend renders")
	return r, richFixture(tb)
}

// upperFirst spells a name with its leading letter raised, which is
// a convention the settle carries through every reference that
// follows a declared name.
func upperFirst(
	_, _ symbol.Kind, _ symbol.Visibility, name string,
) (string, error) {
	if name == "" {
		return name, nil
	}
	return strings.ToUpper(name[:1]) + name[1:], nil
}

// The suite is the contract a backend author tests against, so it
// has to accept a valid backend through and reject each way of
// cheating: that second half is what justifies it.
func TestAssertSettledShape(t *testing.T) {
	t.Parallel()

	t.Run("accepts the kit's clean settle", func(t *testing.T) {
		t.Parallel()

		backendtest.AssertSettledShape(t, wellRendered)
	})

	t.Run("rejects a setup that breaks its own isolation", func(t *testing.T) {
		t.Parallel()

		calls := 0
		setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			r, f := wellRendered(tb)
			calls++
			if calls > 1 {
				assert.NoError(tb,
					f.Emit.Add(unit("extra", fnOf("extra", emit.Body{}))),
					"the drifted unit arrives")
			}
			return r, f
		}
		failure := assert.Rejects(t, "a differing second build must fail",
			func(tb assert.TB) {
				backendtest.AssertSettledShape(tb, setup)
			})
		assert.Contains(t, failure, "unit", "the refusal names the drift")
	})

	t.Run("accepts a respell every reference follows", func(t *testing.T) {
		t.Parallel()

		backendtest.AssertSettledShape(t, richRespelt)
	})

	t.Run("passes a renderer declaring no seams", func(t *testing.T) {
		t.Parallel()

		// The store grows per call, so a check reading it at all
		// reports; a renderer declaring no seams is never read.
		calls := 0
		seamless := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			calls++
			e := plugin.NewEmit()
			for i := range calls {
				word := "u" + strconv.Itoa(i)
				assert.NoError(tb, e.Add(unit(word, fnOf(word, emit.Body{}))),
					"the unit arrives")
			}
			return &fake{render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
				return nil, nil
			}}, &backendtest.Fixture{Emit: e}
		}
		rec := assert.NewRecorder()
		backendtest.AssertSettledShape(rec, seamless)
		assert.False(t, rec.Failed(),
			"a hand-rolled renderer declares no settle to hold")
	})

	t.Run("passes a lowering that reshapes declarations", func(t *testing.T) {
		t.Parallel()

		setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			r, held := wellBuilder(tb).Lower(duplicating).Build().(plugin.Renderer)
			assert.True(tb, held, "the lowering backend renders")
			return r, wellFixture(tb)
		}
		rec := assert.NewRecorder()
		backendtest.AssertSettledShape(rec, setup)
		assert.False(t, rec.Failed(),
			"a declared lowering may reshape declarations the check "+
				"would otherwise compare")
	})

	t.Run("rejects a setup carrying no store", func(t *testing.T) {
		t.Parallel()

		failure := assert.Rejects(t, "a fixture without a store must fail",
			func(tb assert.TB) {
				backendtest.AssertSettledShape(tb, hollowSetup)
			})
		assert.Contains(t, failure, "fixture",
			"the refusal names what the setup did not carry")
	})

	t.Run("rejects a lowering that drops its input's origin", func(t *testing.T) {
		t.Parallel()

		failure := assert.Rejects(t, "a settle failing the plan must fail",
			func(tb assert.TB) {
				backendtest.AssertSettledShape(tb, driftingSetup)
			})
		assert.Contains(t, failure, "settle",
			"the refusal names the step that failed")
	})

	t.Run("stops at the shorter build rather than reading past it", func(t *testing.T) {
		t.Parallel()

		calls := 0
		setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			r, f := wellRendered(tb)
			calls++
			if calls == 1 {
				assert.NoError(tb,
					f.Emit.Add(unit("extra", fnOf("extra", emit.Body{}))),
					"the drifted unit arrives")
			}
			return r, f
		}
		rec := assert.NewRecorder()
		backendtest.AssertSettledShape(rec, setup)
		assert.Equal(t, len(rec.Failures()), 1,
			"the count is the one failure: the comparison stops where the "+
				"shorter build ends")
		assert.Contains(t, rec.Message(), "unit",
			"the refusal names the unit contract")
	})
}

// duplicating is a lowering that emits a second declaration under
// its input's origin: a reshaping the declaration comparison would
// refuse, which is why a lowering backend is held to its unit keys
// and origins alone.
func duplicating(s symbol.Symbol) ([]symbol.Symbol, error) {
	st, held := s.(*emit.Struct)
	if !held {
		return nil, nil
	}
	return []symbol.Symbol{st, &emit.Struct{Origin: st.Origin, Name: st.Name}}, nil
}

func TestRenderSettled(t *testing.T) {
	t.Parallel()

	t.Run("settles and renders the fixture once", func(t *testing.T) {
		t.Parallel()

		files := backendtest.RenderSettled(t, wellRendered)
		assert.True(t, len(files) > 0, "the settled fixture renders whole")
	})

	t.Run("rejects a run reporting an error finding", func(t *testing.T) {
		t.Parallel()

		reporting := scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
			ctx.Sink.Errorf(render.UnformattedFile,
				position.Pos{File: "a.txt"}, ctx.Plugin,
				"the formatter refused a.txt")
			return nil, nil
		})

		failure := assert.Rejects(t, "an error finding must fail the check",
			func(tb assert.TB) {
				backendtest.RenderSettled(tb, reporting)
			})
		assert.Contains(t, failure, "renders clean",
			"the check names what a satellite's pins read")
	})

	t.Run("rejects a lowering that drops its input's origin", func(t *testing.T) {
		t.Parallel()

		failure := assert.Rejects(t, "a settle failing the plan must fail",
			func(tb assert.TB) {
				backendtest.RenderSettled(tb, driftingSetup)
			})
		assert.Contains(t, failure, "settle",
			"the refusal names the step that failed, not the render")
	})
}

func TestRunBackendSuite(t *testing.T) {
	t.Parallel()

	backendtest.RunBackendSuite(t, wellRendered)
}

func TestAssertPopulatedFixture(t *testing.T) {
	t.Parallel()

	t.Run("rejects an empty store", func(t *testing.T) {
		t.Parallel()

		hollow := scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
			return nil, nil
		})

		failure := assert.Rejects(t, "an empty store must fail the check",
			func(tb assert.TB) {
				backendtest.AssertPopulatedFixture(tb, hollow)
			})
		assert.Contains(t, failure, "unit",
			"the check demands a populated fixture")
	})

	t.Run("rejects a fixture carrying no store", func(t *testing.T) {
		t.Parallel()

		rec := assert.NewRecorder()
		backendtest.AssertPopulatedFixture(rec, hollowSetup)
		assert.Equal(t, len(rec.Failures()), 1,
			"the missing store is the one failure: the check stops rather "+
				"than ranging over what it does not hold")
		assert.Contains(t, rec.Message(), "store",
			"the refusal names what the fixture lacks")
	})
}

func TestAssertDeterministicRender(t *testing.T) {
	t.Parallel()

	t.Run("rejects bytes that vary between runs", func(t *testing.T) {
		t.Parallel()

		var runs int
		varying := func(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			runs++
			stamp := strconv.Itoa(runs)
			return &fake{render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
				return []plugin.RenderedFile{{Name: "a.txt", Body: []byte(stamp)}}, nil
			}}, &backendtest.Fixture{Emit: plugin.NewEmit()}
		}

		failure := assert.Rejects(t, "bytes carrying run state must fail the check",
			func(tb assert.TB) {
				backendtest.AssertDeterministicRender(tb, varying)
			})
		assert.Contains(t, failure, "same bytes",
			"the check names the byte-identity contract")
	})

	t.Run("accepts one finding set reported in two orders", func(t *testing.T) {
		t.Parallel()

		var runs int
		swapping := func(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			runs++
			first := runs == 1
			r := &fake{render: func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
				dropped := func() {
					ctx.Sink.Errorf(render.DroppedSlots,
						position.Pos{File: "a.txt", Line: 1, Col: 1}, ctx.Plugin,
						"gen's template placed no marker")
				}
				unformatted := func() {
					ctx.Sink.Errorf(render.UnformattedFile,
						position.Pos{File: "b.txt", Line: 2, Col: 1}, ctx.Plugin,
						"the formatter refused b.txt")
				}
				if first {
					dropped()
					unformatted()
				} else {
					unformatted()
					dropped()
				}
				return []plugin.RenderedFile{{Name: "a.txt", Body: []byte("x\n")}}, nil
			}}
			return r, &backendtest.Fixture{Emit: plugin.NewEmit()}
		}

		backendtest.AssertDeterministicRender(t, swapping)
		assert.Equal(t, runs, 2,
			"the findings are the run's order and the check compares them as a set")
	})
}

// total returns a coverage declaring one verdict for every fact,
// with the given overrides.
func total(over map[symbol.Fact]render.Verdict) render.Coverage {
	facts := map[symbol.Fact]render.Verdict{}
	for _, f := range symbol.Facts() {
		facts[f] = render.Renders
	}
	maps.Copy(facts, over)
	return render.Coverage{Facts: facts}
}

func TestAssertCoveredFacts(t *testing.T) {
	t.Parallel()

	t.Run("rejects a renderer declaring no coverage", func(t *testing.T) {
		t.Parallel()

		failure := assert.Rejects(t, "an undeclared coverage must fail",
			func(tb assert.TB) {
				backendtest.AssertCoveredFacts(tb, scripted(
					func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
						return nil, nil
					},
				))
			})
		assert.Contains(t, failure, "coverage",
			"the refusal names the missing declaration")
	})

	t.Run("accepts a total declaration whose refusals report", func(t *testing.T) {
		t.Parallel()

		setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			tb.Helper()

			r, held := wellBuilder(tb).
				Coverage(total(map[symbol.Fact]render.Verdict{
					symbol.FactAbstract: render.Refuses,
				})).
				Build().(plugin.Renderer)
			assert.True(tb, held, "the covered backend renders")
			f := wellFixture(tb)
			for u := range f.Emit.Units() {
				for _, d := range u.Decls {
					if s, isStruct := d.(*emit.Struct); isStruct {
						s.Abstract = true
					}
				}
			}
			return r, f
		}
		backendtest.AssertCoveredFacts(t, setup)
	})

	t.Run("rejects a declaration missing a fact", func(t *testing.T) {
		t.Parallel()

		partial := total(nil)
		delete(partial.Facts, symbol.FactAsync)
		failure := assert.Rejects(t, "a coverage hole must fail",
			func(tb assert.TB) {
				backendtest.AssertCoveredFacts(tb,
					func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
						tb.Helper()
						r, held := wellBuilder(tb).Coverage(partial).
							Build().(plugin.Renderer)
						assert.True(tb, held, "the covered backend renders")
						return r, wellFixture(tb)
					})
			})
		assert.Contains(t, failure, symbol.FactAsync.String(),
			"the refusal names the missing fact")
	})

	t.Run("rejects an exception on a kind that cannot state it", func(t *testing.T) {
		t.Parallel()

		astray := total(nil)
		astray.Except = map[symbol.Kind]map[symbol.Fact]render.Verdict{
			symbol.KindStruct: {symbol.FactTag: render.Refuses},
		}
		failure := assert.Rejects(t, "a stray exception must fail",
			func(tb assert.TB) {
				backendtest.AssertCoveredFacts(tb,
					func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
						tb.Helper()
						r, held := wellBuilder(tb).Coverage(astray).
							Build().(plugin.Renderer)
						assert.True(tb, held, "the covered backend renders")
						return r, wellFixture(tb)
					})
			})
		assert.Contains(t, failure, symbol.FactTag.String(),
			"the refusal names the stray fact")
	})

	t.Run("rejects a setup carrying no fixture", func(t *testing.T) {
		t.Parallel()

		failure := assert.Rejects(t, "a fixture without a store must fail",
			func(tb assert.TB) {
				backendtest.AssertCoveredFacts(tb, hollowSetup)
			})
		assert.Contains(t, failure, "fixture",
			"the refusal names what the setup did not carry")
	})

	t.Run("rejects a lowering that drops its input's origin", func(t *testing.T) {
		t.Parallel()

		failure := assert.Rejects(t, "a settle failing the plan must fail",
			func(tb assert.TB) {
				backendtest.AssertCoveredFacts(tb,
					func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
						tb.Helper()
						r, held := wellBuilder(tb).Coverage(total(nil)).
							Lower(drifting).Build().(plugin.Renderer)
						assert.True(tb, held, "the covered backend renders")
						return r, wellFixture(tb)
					})
			})
		assert.Contains(t, failure, "settle",
			"the refusal names the step that failed")
	})

	t.Run("rejects a fixture the settle reports on", func(t *testing.T) {
		t.Parallel()

		failure := assert.Rejects(t, "a settle withholding a declaration must fail",
			func(tb assert.TB) {
				backendtest.AssertCoveredFacts(tb,
					func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
						tb.Helper()
						r, held := wellBuilder(tb).Coverage(total(nil)).
							Respell(refusing).Build().(plugin.Renderer)
						assert.True(tb, held, "the covered backend renders")
						return r, wellFixture(tb)
					})
			})
		assert.Contains(t, failure, "settles clean",
			"the refusal names the fixture the coverage is read over")
	})

	t.Run("rejects a render that aborts", func(t *testing.T) {
		t.Parallel()

		aborting := func(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			r := &covering{coverage: total(nil)}
			r.render = func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
				return nil, errors.New("kaboom")
			}
			return r, &backendtest.Fixture{Emit: plugin.NewEmit()}
		}

		failure := assert.Rejects(t, "a fatal error must fail the check",
			func(tb assert.TB) {
				backendtest.AssertCoveredFacts(tb, aborting)
			})
		assert.Contains(t, failure, "render completes",
			"the refusal names the step that failed")
	})

	t.Run("rejects a fact the declaration takes no stance on", func(t *testing.T) {
		t.Parallel()

		silent := total(nil)
		silent.Except = map[symbol.Kind]map[symbol.Fact]render.Verdict{
			symbol.KindStruct: {symbol.FactAbstract: render.VerdictUndeclared},
		}
		failure := assert.Rejects(t, "an undeclared verdict must fail",
			func(tb assert.TB) {
				backendtest.AssertCoveredFacts(tb,
					func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
						tb.Helper()
						r, held := wellBuilder(tb).Coverage(silent).
							Build().(plugin.Renderer)
						assert.True(tb, held, "the covered backend renders")
						return r, abstractFixture(tb)
					})
			})
		assert.Contains(t, failure, "no verdict",
			"the refusal names the fact that met no stance")
	})
}

// abstractFixture returns the valid fixture with the abstract fact
// stated on its struct, so a render's coverage guard has a fact to
// take a stance on.
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

// A host template that ranges some member lists and forgets one
// drops those members with no finding, which the kind and fact
// checks never see, so this check is the only one holding a
// backend to what its member lists actually spell.
func TestAssertRenderedMembers(t *testing.T) {
	t.Parallel()

	t.Run("accepts host templates that place every member", func(t *testing.T) {
		t.Parallel()

		backendtest.AssertRenderedMembers(t, memberSpelt)
	})

	t.Run("rejects a host template that drops its members", func(t *testing.T) {
		t.Parallel()

		failure := assert.Rejects(t, "a dropped member must fail the check",
			func(tb assert.TB) {
				backendtest.AssertRenderedMembers(tb, memberDropped)
			})
		assert.Contains(t, failure, structField,
			"the refusal names the member that arrived nowhere")
		assert.Contains(t, failure, hostStruct, "and the host it hangs on")
	})

	t.Run("reports every member its host template drops", func(t *testing.T) {
		t.Parallel()

		rec := assert.NewRecorder()
		backendtest.AssertRenderedMembers(rec, memberDropped)
		reported := strings.Join(rec.Messages(), "\n")
		for _, member := range []string{
			structField, structMethod, ifaceField, ifaceMethod,
			enumVariant, sumVariant,
		} {
			assert.Contains(t, reported, member,
				"the check reads the member lists of every host kind")
		}
		assert.Equal(t, len(rec.Messages()), 6,
			"and reports each drop rather than stopping at the first")
	})

	t.Run("passes a member a finding already names", func(t *testing.T) {
		t.Parallel()

		rec := assert.NewRecorder()
		backendtest.AssertRenderedMembers(rec, memberExcused)
		assert.False(t, rec.Failed(),
			"a member the settle reported is excused from the bytes")
	})

	t.Run("rejects the same member where no finding names it", func(t *testing.T) {
		t.Parallel()

		silent := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			tb.Helper()

			r, held := memberBuilder(tb, dropsMembers()).Build().(plugin.Renderer)
			assert.True(tb, held, "the member backend renders")
			return r, collidingFixture(tb)
		}
		failure := assert.Rejects(t, "a dropped member no finding names must fail",
			func(tb assert.TB) {
				backendtest.AssertRenderedMembers(tb, silent)
			})
		assert.Contains(t, failure, structField,
			"the excusal is the settle's finding, not the fixture")
	})

	t.Run("rejects a lowering that drops its input's origin", func(t *testing.T) {
		t.Parallel()

		failure := assert.Rejects(t, "a settle failing the plan must fail",
			func(tb assert.TB) {
				backendtest.AssertRenderedMembers(tb, driftingSetup)
			})
		assert.Contains(t, failure, "settle",
			"the refusal names the step that failed")
	})
}

func TestAssertSpeltKinds(t *testing.T) {
	t.Parallel()

	t.Run("rejects a kind the language cannot spell", func(t *testing.T) {
		t.Parallel()

		unspelt := scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
			ctx.Sink.Errorf(render.UnspeltKind,
				position.Pos{File: "a.txt"}, ctx.Plugin,
				"printer holds no template for the enum kind")
			return nil, nil
		})

		failure := assert.Rejects(t, "an unspelt kind must fail the check",
			func(tb assert.TB) {
				backendtest.AssertSpeltKinds(tb, unspelt)
			})
		assert.Contains(t, failure, "spell",
			"the check names the missing spelling")
	})
}

func TestAssertPlacedContent(t *testing.T) {
	t.Parallel()

	t.Run("rejects content that went unplaced", func(t *testing.T) {
		t.Parallel()

		dropped := scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
			ctx.Sink.Errorf(render.DroppedSlots,
				position.Pos{File: "a.txt"}, ctx.Plugin,
				"gen's template placed no marker and 2 contributions are pending")
			return nil, nil
		})

		failure := assert.Rejects(t, "dropped slot content must fail the check",
			func(tb assert.TB) {
				backendtest.AssertPlacedContent(tb, dropped)
			})
		assert.Contains(t, failure, "whole",
			"the check holds every body to arriving whole")
	})
}

func TestAssertContinuedRender(t *testing.T) {
	t.Parallel()

	t.Run("rejects a renderer that aborts", func(t *testing.T) {
		t.Parallel()

		aborting := scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
			return nil, errors.New("kaboom")
		})

		failure := assert.Rejects(t, "a fatal error must fail the check",
			func(tb assert.TB) {
				backendtest.AssertContinuedRender(tb, aborting)
			})
		assert.Contains(t, failure, "continue",
			"the check names the continuation rule")
	})

	t.Run("rejects a finding without a position", func(t *testing.T) {
		t.Parallel()

		unpositioned := scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
			ctx.Sink.Errorf(render.UnformattedFile,
				position.Pos{}, ctx.Plugin, "somewhere, something broke")
			return nil, nil
		})

		failure := assert.Rejects(t, "an unpositioned finding must fail the check",
			func(tb assert.TB) {
				backendtest.AssertContinuedRender(tb, unpositioned)
			})
		assert.Contains(t, failure, "position",
			"the check names the positioning rule")
	})

	t.Run("rejects a finding under another plugin's origin", func(t *testing.T) {
		t.Parallel()

		foreign := scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
			ctx.Sink.Errorf(render.UnformattedFile,
				position.Pos{File: "a.txt"}, "outsider", "not mine")
			return nil, nil
		})

		failure := assert.Rejects(t, "a mis-attributed finding must fail the check",
			func(tb assert.TB) {
				backendtest.AssertContinuedRender(tb, foreign)
			})
		assert.Contains(t, failure, "origin",
			"the check names the attribution rule")
	})

	t.Run("rejects a returned file reported unformatted", func(t *testing.T) {
		t.Parallel()

		lying := scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
			ctx.Sink.Errorf(render.UnformattedFile,
				position.Pos{File: "a.txt"}, ctx.Plugin, "the formatter refused a.txt")
			return []plugin.RenderedFile{{Name: "a.txt", Body: []byte("x")}}, nil
		})

		failure := assert.Rejects(t, "a withheld file must stay withheld",
			func(tb assert.TB) {
				backendtest.AssertContinuedRender(tb, lying)
			})
		assert.Contains(t, failure, "withheld",
			"the check holds the withholding rule")
	})

	t.Run("holds a genuine format failure to continuation", func(t *testing.T) {
		t.Parallel()

		partial := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			f := wellFixture(tb)
			b := backend.New("half", "stub",
				plugin.CommentSyntax{Line: []string{"//"}}).
				KindTemplates(map[symbol.Kind]string{
					symbol.KindStruct:   "type {{.Name}} struct{}\n",
					symbol.KindFunction: "func {{.Name}}()\n",
				}).
				Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
				Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
					return []byte("\treturn\n"), nil
				}).
				Imports(func(*render.ImportSet) string { return "" }).
				Finalise(func(src []byte) ([]byte, error) {
					if strings.Contains(string(src), "Row") {
						return nil, errors.New("unparseable")
					}
					return src, nil
				}).
				Build()
			r, held := b.(plugin.Renderer)
			assert.True(tb, held, "the kit backend renders")
			return r, f
		}

		backendtest.AssertContinuedRender(t, partial)
	})
}
