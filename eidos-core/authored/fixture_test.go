// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package authored_test

import (
	"fmt"
	"strings"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/authored"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The fixture's names.
const (
	pkgPath     = coretest.StorePath
	boxName     = "Box"
	alphaName   = "Alpha"
	itemName    = "Item"
	loadName    = "Load"
	paramT      = "T"
	fixtureFile = "box.go"
)

// fixture is the graph the cases run over: a generic Box[T] holding
// one field, a plain Alpha a witness can name, and a Load function
// with one parameter and one return.
type fixture struct {
	graph *store.Graph
	box   *node.Struct
	item  *node.Field
	alpha *node.Struct
	load  *node.Function
}

// build returns an unfrozen fixture graph.
func build(tb assert.TB) fixture {
	tb.Helper()

	box := coretest.Struct(pkgPath, boxName)
	box.Pos = position.Pos{File: fixtureFile, Line: 3, Col: 1}
	box.TypeParams = []*node.TypeParam{{
		ID: symbol.Identity{
			Lang: coretest.Lang, Package: pkgPath, Owner: boxName, Name: paramT, Kind: symbol.KindTypeParam,
		},
		Name: paramT,
		Pos:  position.Pos{File: fixtureFile, Line: 3, Col: 10},
	}}
	item := coretest.Field(pkgPath, boxName, itemName)
	item.Type = &node.TypeRef{Spelling: paramT}
	item.Pos = position.Pos{File: fixtureFile, Line: 4, Col: 2}
	box.Fields = []*node.Field{item}
	alpha := coretest.Struct(pkgPath, alphaName)
	alpha.Pos = position.Pos{File: fixtureFile, Line: 8, Col: 1}
	load := coretest.Function(pkgPath, loadName)
	load.Pos = position.Pos{File: fixtureFile, Line: 12, Col: 1}
	g := store.New()
	assert.NoError(tb, g.AddPackage(coretest.Package(pkgPath, box, alpha, load)),
		"the fixture package is admitted")
	return fixture{graph: g, box: box, item: item, alpha: alpha, load: load}
}

// composed returns a workspace listing both authored annotators,
// the fixture rules and one plan, as a composition would.
func composed(tb assert.TB) *workspace.Workspace {
	tb.Helper()

	gen, held := eidos.NewPlugin("mirror").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error { return nil })).
		Build().(plugin.Generator)
	assert.True(tb, held, "an emitter handler makes a generator")
	w, err := workspace.New().
		Annotators(authored.Sample(), authored.Witness()).
		Rules(fixtureRules{}).
		Targets("fixture").
		Plans(workspace.Plan{
			Name: "plan", Generators: []plugin.Generator{gen},
			Backend: fakeBackend{},
		}).
		Build()
	assert.NoError(tb, err, "the composition composes")
	return w
}

// attach attaches one raw kernel directive to a subject: the name,
// then key=value pairs.
func attach(tb assert.TB, g *store.Graph, subject symbol.Identity, line int, name directive.Name, pairs ...string) {
	tb.Helper()

	raw := directive.Raw{Name: name, Pos: position.Pos{File: fixtureFile, Line: line, Col: 1}}
	for _, pair := range pairs {
		key, value, _ := strings.Cut(pair, "=")
		raw.Args = append(raw.Args, directive.RawArg{Key: key, Value: directive.RawValue{Text: value}})
	}
	assert.NoError(tb, g.AttachDirectives(subject, []directive.Raw{raw}), "the directive attaches")
}

// fakeBackend is a backend by name and target alone.
type fakeBackend struct{}

func (fakeBackend) Name() plugin.ID       { return "printer" }
func (fakeBackend) Target() plugin.Target { return "fixture" }

// fixtureRules is the scripted rules speaking the fixture's
// language, resolving a type name as a struct in the fixture
// package.
type fixtureRules struct{}

func (fixtureRules) Lang() symbol.Lang { return coretest.Lang }

func (fixtureRules) Members() rules.MemberPolicy { return rulestest.Scripted().Members() }

func (fixtureRules) ParamRole(p *node.Param, v rules.View) rules.ParamRole {
	return rulestest.Scripted().ParamRole(p, v)
}

func (fixtureRules) ReturnRoles(rs []*node.Return, v rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	return rulestest.Scripted().ReturnRoles(rs, v)
}

func (fixtureRules) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	return rulestest.Scripted().Builtin(ref, v)
}

func (fixtureRules) Resolve(
	_ rules.Scope, name string, _ directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	if sym, held := v.Lookup(coretest.ID(pkgPath, name, symbol.KindStruct)); held {
		return sym, nil
	}
	return nil, fmt.Errorf("nothing in %s is named %s", pkgPath, name)
}

func (fixtureRules) SamplesOf(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	return rulestest.Scripted().SamplesOf(ref, hint, v)
}

func (fixtureRules) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	return rulestest.Scripted().ZeroValue(ref, v)
}

func (fixtureRules) LiteralFor(f *node.File, ref *node.TypeRef, text string, v rules.View) (emit.Value, bool) {
	return rulestest.Scripted().LiteralFor(f, ref, text, v)
}

func (fixtureRules) TypeName(word, base string) string {
	return rulestest.Scripted().TypeName(word, base)
}
