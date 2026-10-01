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
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The suite's fixture: the plugin every fixture unit comes from, the
// backends the cases build, and the words, names and bodies they
// render.
const (
	// fixtureEmitter is the plugin every fixture unit comes from.
	fixtureEmitter plugin.ID = "gen"
	// printerName, memberName and halfName name the fixture backends.
	printerName plugin.ID = "printer"
	memberName  plugin.ID = "member"
	halfName    plugin.ID = "half"
	// stubTarget is the target every fixture backend declares.
	stubTarget plugin.Target = "stub"
	// lineComment opens a line comment in the fixture language.
	lineComment = "//"
	// fileExt is the extension the fixture naming spells.
	fileExt = ".txt"
	// runtimePkg is the import the fixture scaffold records.
	runtimePkg = "stub/runtime"
	// refTemplate names the reference-form template.
	refTemplate = "save.tpl"
	// The unit words, which the naming spells into filenames.
	stubWord  = "stub"
	refWord   = "ref"
	rawWord   = "raw"
	hostsWord = "hosts"
	funcsWord = "funcs"
	sumsWord  = "sums"
	extraWord = "extra"
	richWord  = "rich"
	// The valid fixture's functions, and the calls their bodies make.
	noopName  = "Noop"
	loadName  = "Load"
	saveName  = "Save"
	dumpName  = "Dump"
	guardCall = "guard"
	dumpBody  = "\tdump()\n"
	// fileA and fileB name the files a scripted renderer returns, and
	// bodyX is the body of each.
	fileA = "a.txt"
	fileB = "b.txt"
	bodyX = "x\n"
	// abortMessage is the error a scripted render aborts with.
	abortMessage = "kaboom"
	// outsider is an origin no render context names.
	outsider plugin.ID = "outsider"
	// intType spells a parameter's type.
	intType = "int"
	// kindReason is the reason a fixture backend states for a kind it
	// refuses.
	kindReason = "the fixture target declares no such construct"
)

// The fixture language's spellings: a struct, and a function that
// places its body.
const (
	structTpl   = "type {{.Name}} struct{}\n"
	functionTpl = "func {{.Name}}() {\n{{" + render.BuiltinBody + " .}}}\n"
	// refSource is the reference-form template: a call, then the
	// marker that places every pending slot.
	refSource = "\tsaving()\n{{" + render.BuiltinSlots + "}}"
)

// The declarations the member fixture contains: one host per kind
// with members, and the members of each, so a check reading member
// names meets a field, a method and both variant forms.
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
	// prefixedField is a field name the host name Row contains.
	prefixedField = "Ro"
	// sumComment and variantComment are the comments the coverage
	// cases state on a sum and on its variant.
	sumComment     = "tagged"
	variantComment = "round"
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

// wordNaming spells every unit as its word under the fixture
// extension.
func wordNaming(u plugin.Unit) string { return u.Word + fileExt }

// passThrough is the fixture formatter, which returns its input.
func passThrough(src []byte) ([]byte, error) { return src, nil }

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

// wellFixture returns the valid fixture: a store with every kind
// the backend spells and a body in each of the four content forms,
// with the reference's slot content spliced through a
// marker-placing tree.
func wellFixture(tb assert.TB) *backendtest.Fixture {
	tb.Helper()

	refBody := emit.Body{Ref: &emit.TemplateRef{Name: refTemplate}}
	refBody.Prologue.Append(call(guardCall))
	e := plugin.NewEmit()
	for _, u := range []plugin.Unit{
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
	} {
		assert.NoError(tb, e.Add(u), "the fixture unit arrives")
	}
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

// wellRendered returns the valid setup: the kit backend over the
// valid fixture.
func wellRendered(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	return wellBackend(tb), wellFixture(tb)
}

// shapeOf returns the sum host of the member fixture: one variant,
// and the given comment on the sum and on its variant.
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

// refusingRendered is the valid setup with a sum beside the kinds
// the backend spells, which the backend declares refused, so the
// suite meets a declared refusal the way it does over the canonical
// fixture.
func refusingRendered(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := wellBuilder(tb).
		Coverage(total(nil)).
		RefusedKinds(map[symbol.Kind]string{symbol.KindSum: kindReason}).
		Build().(plugin.Renderer)
	assert.True(tb, held, "the refusing backend renders")
	f := wellFixture(tb)
	assert.NoError(tb, f.Emit.Add(unit(sumsWord, shapeOf("", ""))), "the refused unit arrives")
	return r, f
}

// fake is a renderer the failing cases script: the suite has to
// catch every way a renderer can cheat the rules.
type fake struct {
	render func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error)
}

// Render implements [plugin.Renderer] through the scripted function.
func (f *fake) Render(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
	return f.render(ctx)
}

// covering is a renderer declaring fact coverage without the backend
// roles, so a check reads a declaration off a renderer that never
// settles.
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

// hollowSetup hands a renderer over a fixture without a store,
// which every check fails before it reads anything. The renderer
// returns a whole file, so what a check fails on is the missing
// store and nothing further along.
func hollowSetup(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	return &fake{render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
		return []plugin.RenderedFile{{Path: fileA, Body: []byte(bodyX)}}, nil
	}}, &backendtest.Fixture{}
}

// hollowBacked hands a kit backend over a fixture without a store,
// so a check that takes the backend's seams still has nothing to
// settle.
func hollowBacked(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	return wellBackend(tb), &backendtest.Fixture{}
}

// drifting is a lowering that drops its input's provenance: the
// defect [plugin.Settle] fails the plan on, and reports as no
// declaration's finding.
func drifting(s symbol.Symbol) ([]symbol.Symbol, error) {
	st, isStruct := s.(*emit.Struct)
	if !isStruct {
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
// continues, so a check meets a settle that reports without failing.
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
// reports and withholds a declaration while the plan continues.
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

// memberFixture returns a store with one host per kind with members,
// each with the members a rendered file has to contain.
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

// collidingField is the second field name of the colliding fixture.
// It settles to the first one's spelling under a respell that
// lowers the leading letter, so the settle reports both names and
// keeps them as emitted.
const collidingField = "cell"

// collidingFixture returns a store whose struct has two fields one
// respell cannot tell apart.
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

// memberBuilder accumulates a backend over the kinds with members,
// with the given spellings, so one test renders members and another
// drops them.
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

// sumRefusing is the member backend with the sum kind refused, its
// other hosts spelling every member.
func sumRefusing(tb assert.TB) *backend.Builder {
	tb.Helper()

	kinds := spellsMembers()
	delete(kinds, symbol.KindSum)
	return memberBuilder(tb, kinds).
		RefusedKinds(map[symbol.Kind]string{symbol.KindSum: kindReason})
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

// The declarations of the reference fixture. Each name is one the
// settle rewrites and something else follows: the field's type
// names its own host, the method's body calls the function and
// guards on its parameter, and the named slot contains a call of
// its own.
const (
	richFn     = "boot"
	richStruct = "row"
	richField  = "next"
	richMethod = "touch"
	richParam  = "item"
	richSlot   = "extra"
)

// richFixture returns a store with every reference the settle
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
		Params: []*emit.Param{{Name: richParam, Type: &emit.TypeRef{Spelling: intType}}},
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
// a convention the settle applies to every reference that follows a
// declared name.
func upperFirst(
	_, _ symbol.Kind, _ symbol.Visibility, name string,
) (string, error) {
	if name == "" {
		return name, nil
	}
	return strings.ToUpper(name[:1]) + name[1:], nil
}

// duplicating is a lowering that emits a second declaration under
// its input's origin: a reshaping the declaration comparison would
// fail, which is why the check compares a lowering backend's unit
// keys and origins alone.
func duplicating(s symbol.Symbol) ([]symbol.Symbol, error) {
	st, isStruct := s.(*emit.Struct)
	if !isStruct {
		return nil, nil
	}
	return []symbol.Symbol{st, &emit.Struct{Origin: st.Origin, Name: st.Name}}, nil
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

// reportsOnce returns a setup whose renderer reports one Error under
// code at fileA and returns no file.
func reportsOnce(code diag.Code, msg string) backendtest.Setup {
	return scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
		ctx.Sink.Errorf(code, position.Pos{File: fileA}, ctx.Plugin, "%s", msg)
		return nil, nil
	})
}

// The suite is the contract a backend author tests against: it
// passes a valid backend and fails each way of cheating.
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
			assert.Contains(t, failure, structField,
				"the failure names the member that arrived nowhere")
			assert.Contains(t, failure, hostStruct, "and the host it belongs to")
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
			assert.Length(t, rec.Messages(), 6,
				"and reports each drop, not only the first")
		})

		t.Run("fails a member only another file spells", func(t *testing.T) {
			t.Parallel()

			// The struct's method shares its name with a function in a
			// second file, so the rendered bytes contain the name while
			// the host's own file drops the method.
			shadowed := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				tb.Helper()

				kinds := dropsMembers()
				kinds[symbol.KindFunction] = "func {{.Name}}() {}\n"
				r, held := memberBuilder(tb, kinds).Build().(plugin.Renderer)
				assert.True(tb, held, "the member backend renders")
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
			failure := assert.Rejects(t, "a member only another file spells must fail",
				func(tb assert.TB) {
					backendtest.AssertRenderedMembers(tb, shadowed)
				})
			assert.Contains(t, failure, structMethod, "the failure names the member its host dropped")
		})

		t.Run("fails a member whose name occurs only inside a longer word", func(t *testing.T) {
			t.Parallel()

			// The host name Row contains the member name Ro, and no
			// whole word spells Ro.
			prefixed := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				tb.Helper()

				r, held := memberBuilder(tb, dropsMembers()).Build().(plugin.Renderer)
				assert.True(tb, held, "the member backend renders")
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
			failure := assert.Rejects(t, "a member no whole word spells must fail",
				func(tb assert.TB) {
					backendtest.AssertRenderedMembers(tb, prefixed)
				})
			assert.Contains(t, failure, "member "+prefixedField+" of", "the failure names the member")
		})

		t.Run("passes a member a finding already names", func(t *testing.T) {
			t.Parallel()

			rec := assert.NewRecorder()
			backendtest.AssertRenderedMembers(rec, memberExcused)
			assert.False(t, rec.Failed(),
				"a member the settle reported is excused from the bytes")
		})

		t.Run("fails the same member where no finding names it", func(t *testing.T) {
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

		t.Run("passes the members of a host of a refused kind", func(t *testing.T) {
			t.Parallel()

			setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				tb.Helper()

				r, held := sumRefusing(tb).Build().(plugin.Renderer)
				assert.True(tb, held, "the refusing member backend renders")
				return r, memberFixture(tb)
			}
			rec := assert.NewRecorder()
			backendtest.AssertRenderedMembers(rec, setup)
			assert.False(t, rec.Failed(),
				"a refused host renders nothing, so no member of it is missing")
		})

		t.Run("fails a lowering that drops its input's origin", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a settle failing the plan must fail",
				func(tb assert.TB) {
					backendtest.AssertRenderedMembers(tb, driftingSetup)
				})
			assert.Contains(t, failure, "settle",
				"the failure names the step that failed")
		})
	})

	t.Run("AssertCoveredFacts", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a renderer declaring no coverage", func(t *testing.T) {
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
				"the failure names the missing declaration")
		})

		t.Run("passes a total declaration whose refusals report", func(t *testing.T) {
			t.Parallel()

			setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				tb.Helper()

				r, held := wellBuilder(tb).
					Coverage(total(map[symbol.Fact]render.Verdict{
						symbol.FactAbstract: render.Refuses,
					})).
					Build().(plugin.Renderer)
				assert.True(tb, held, "the covered backend renders")
				return r, abstractFixture(tb)
			}
			backendtest.AssertCoveredFacts(t, setup)
		})

		t.Run("counts a variant's refusal under the variant's kind", func(t *testing.T) {
			t.Parallel()

			// A Sum and its variant each state the refused comment. The
			// variant's finding names SumVariant, which contains Sum.
			setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				tb.Helper()

				r, held := memberBuilder(tb, spellsMembers()).
					Coverage(total(map[symbol.Fact]render.Verdict{symbol.FactComment: render.Refuses})).
					Build().(plugin.Renderer)
				assert.True(tb, held, "the covered member backend renders")
				e := plugin.NewEmit()
				assert.NoError(tb, e.Add(unit(hostsWord, shapeOf(sumComment, variantComment))),
					"the sum unit arrives")
				return r, &backendtest.Fixture{Emit: e, Schedule: []plugin.ID{fixtureEmitter}}
			}
			// Go randomises map iteration order per range, and the check
			// ranges over its expectations. Thirty runs visit the Sum key
			// before the SumVariant key at least once with high
			// probability, which is the order a substring match
			// misattributes under.
			for range 30 {
				rec := assert.NewRecorder()
				backendtest.AssertCoveredFacts(rec, setup)
				assert.False(t, rec.Failed(), "each refusal counts under the kind it names")
			}
		})

		t.Run("passes a refused fact stated on a declaration of a refused kind", func(t *testing.T) {
			t.Parallel()

			setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				tb.Helper()

				r, held := sumRefusing(tb).
					Coverage(total(map[symbol.Fact]render.Verdict{symbol.FactComment: render.Refuses})).
					Build().(plugin.Renderer)
				assert.True(tb, held, "the refusing member backend renders")
				e := plugin.NewEmit()
				assert.NoError(tb, e.Add(unit(hostsWord, shapeOf(sumComment, variantComment))),
					"the sum unit arrives")
				return r, &backendtest.Fixture{Emit: e, Schedule: []plugin.ID{fixtureEmitter}}
			}
			rec := assert.NewRecorder()
			backendtest.AssertCoveredFacts(rec, setup)
			assert.False(t, rec.Failed(),
				"a refused declaration renders nothing, so none of its facts reports")
		})

		t.Run("fails a declaration missing a fact", func(t *testing.T) {
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
				"the failure names the missing fact")
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
						func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
							tb.Helper()
							r, held := wellBuilder(tb).Coverage(astray).
								Build().(plugin.Renderer)
							assert.True(tb, held, "the covered backend renders")
							return r, wellFixture(tb)
						})
				})
			assert.Contains(t, failure, symbol.FactTag.String(),
				"the failure names the stray fact")
		})

		t.Run("fails a setup returning no fixture", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a fixture without a store must fail",
				func(tb assert.TB) {
					backendtest.AssertCoveredFacts(tb, hollowSetup)
				})
			assert.Contains(t, failure, "fixture",
				"the failure names what the setup did not return")
		})

		t.Run("fails a lowering that drops its input's origin", func(t *testing.T) {
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
				"the failure names the step that failed")
		})

		t.Run("fails a fixture the settle reports on", func(t *testing.T) {
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
				"the failure names the fixture the coverage is read over")
		})

		t.Run("fails a render that aborts", func(t *testing.T) {
			t.Parallel()

			aborting := func(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				r := &covering{coverage: total(nil)}
				r.render = func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
					return nil, errors.New(abortMessage)
				}
				return r, &backendtest.Fixture{Emit: plugin.NewEmit()}
			}

			failure := assert.Rejects(t, "a fatal error must fail the check",
				func(tb assert.TB) {
					backendtest.AssertCoveredFacts(tb, aborting)
				})
			assert.Contains(t, failure, "render completes",
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
						func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
							tb.Helper()
							r, held := wellBuilder(tb).Coverage(silent).
								Build().(plugin.Renderer)
							assert.True(tb, held, "the covered backend renders")
							return r, abstractFixture(tb)
						})
				})
			assert.Contains(t, failure, "no verdict",
				"the failure names the fact that met no stance")
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
			assert.Contains(t, failure, "renders clean",
				"the check names what a satellite's pins read")
		})

		t.Run("passes a run reporting a declared kind refusal", func(t *testing.T) {
			t.Parallel()

			rec := assert.NewRecorder()
			files := backendtest.RenderSettled(rec, refusingRendered)
			assert.False(t, rec.Failed(), "a declared refusal is the fixture's, not a defect")
			assert.NotEmpty(t, files, "and the spelt kinds render")
		})

		t.Run("fails a lowering that drops its input's origin", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a settle failing the plan must fail",
				func(tb assert.TB) {
					backendtest.RenderSettled(tb, driftingSetup)
				})
			assert.Contains(t, failure, "settle",
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
			setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
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
			assert.Contains(t, failure, "unit", "the failure names the drift")
		})

		t.Run("passes a respell every reference follows", func(t *testing.T) {
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
					word := extraWord + strconv.Itoa(i)
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
				"a hand-rolled renderer declares no settle to check")
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

		t.Run("fails a setup returning no store", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a fixture without a store must fail",
				func(tb assert.TB) {
					backendtest.AssertSettledShape(tb, hollowSetup)
				})
			assert.Contains(t, failure, "fixture",
				"the failure names what the setup did not return")
		})

		t.Run("fails a lowering that drops its input's origin", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a settle failing the plan must fail",
				func(tb assert.TB) {
					backendtest.AssertSettledShape(tb, driftingSetup)
				})
			assert.Contains(t, failure, "settle",
				"the failure names the step that failed")
		})

		t.Run("stops at the end of the shorter build", func(t *testing.T) {
			t.Parallel()

			calls := 0
			setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				r, f := wellRendered(tb)
				calls++
				if calls == 1 {
					assert.NoError(tb,
						f.Emit.Add(unit(extraWord, fnOf(extraWord, emit.Body{}))),
						"the drifted unit arrives")
				}
				return r, f
			}
			rec := assert.NewRecorder()
			backendtest.AssertSettledShape(rec, setup)
			assert.Length(t, rec.Failures(), 1,
				"the count is the one failure: the comparison stops where the "+
					"shorter build ends")
			assert.Contains(t, rec.Message(), "unit",
				"the failure names the unit contract")
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
			assert.Contains(t, failure, "unit",
				"the check demands a populated fixture")
		})

		t.Run("fails a fixture without a store", func(t *testing.T) {
			t.Parallel()

			rec := assert.NewRecorder()
			backendtest.AssertPopulatedFixture(rec, hollowSetup)
			assert.Length(t, rec.Failures(), 1,
				"the missing store is the one failure: the check stops and "+
					"ranges over nothing")
			assert.Contains(t, rec.Message(), "store",
				"the failure names what the fixture lacks")
		})
	})

	t.Run("AssertDeterministicRender", func(t *testing.T) {
		t.Parallel()

		t.Run("fails bytes that vary between runs", func(t *testing.T) {
			t.Parallel()

			var runs int
			varying := func(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				runs++
				stamp := strconv.Itoa(runs)
				return &fake{render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
					return []plugin.RenderedFile{{Path: fileA, Body: []byte(stamp)}}, nil
				}}, &backendtest.Fixture{Emit: plugin.NewEmit()}
			}

			failure := assert.Rejects(t, "bytes that depend on run state must fail the check",
				func(tb assert.TB) {
					backendtest.AssertDeterministicRender(tb, varying)
				})
			assert.Contains(t, failure, "same bytes",
				"the check names the byte-identity contract")
		})

		t.Run("passes one finding set reported in two orders", func(t *testing.T) {
			t.Parallel()

			var runs int
			swapping := func(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				runs++
				first := runs == 1
				r := &fake{render: func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
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
				}}
				return r, &backendtest.Fixture{Emit: plugin.NewEmit()}
			}

			backendtest.AssertDeterministicRender(t, swapping)
			assert.Equal(t, runs, 2,
				"the findings are the run's order and the check compares them as a set")
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
			assert.Contains(t, failure, "spell",
				"the check names the missing spelling")
		})

		t.Run("passes a kind the language declares refused", func(t *testing.T) {
			t.Parallel()

			rec := assert.NewRecorder()
			backendtest.AssertSpeltKinds(rec, reportsOnce(render.RefusedKind,
				"printer refuses the Sum kind: "+kindReason))
			assert.False(t, rec.Failed(), "a declared refusal is a spelling decision")
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
			assert.Contains(t, failure, "whole",
				"the check requires every body to arrive whole")
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
			assert.Contains(t, failure, "continue",
				"the check names the continuation rule")
		})

		t.Run("fails a finding without a position", func(t *testing.T) {
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

		t.Run("fails a finding under another plugin's origin", func(t *testing.T) {
			t.Parallel()

			foreign := scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
				ctx.Sink.Errorf(render.UnformattedFile,
					position.Pos{File: fileA}, outsider, "not the context's")
				return nil, nil
			})

			failure := assert.Rejects(t, "a mis-attributed finding must fail the check",
				func(tb assert.TB) {
					backendtest.AssertContinuedRender(tb, foreign)
				})
			assert.Contains(t, failure, "origin",
				"the check names the attribution rule")
		})

		t.Run("fails a returned file reported unformatted", func(t *testing.T) {
			t.Parallel()

			lying := scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
				ctx.Sink.Errorf(render.UnformattedFile,
					position.Pos{File: fileA}, ctx.Plugin, "the formatter refused %s", fileA)
				return []plugin.RenderedFile{{Path: fileA, Body: []byte(bodyX)}}, nil
			})

			failure := assert.Rejects(t, "a withheld file must remain withheld",
				func(tb assert.TB) {
					backendtest.AssertContinuedRender(tb, lying)
				})
			assert.Contains(t, failure, "withheld",
				"the check names the withholding rule")
		})

		t.Run("passes a format failure the render continues past", func(t *testing.T) {
			t.Parallel()

			partial := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
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
				r, held := b.(plugin.Renderer)
				assert.True(tb, held, "the kit backend renders")
				return r, f
			}

			backendtest.AssertContinuedRender(t, partial)
		})
	})
}
