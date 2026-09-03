// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fixture's package and the names it declares.
const (
	svcPath     = "svc"
	depPath     = "dep"
	rowName     = "Row"
	baseName    = "Base"
	derivedName = "Derived"
	storeName   = "Store"
	intSpelling = "int"
	strSpelling = "string"
)

// viewOver mints a view over a frozen graph with a fresh read set
// and the kernel's keys registered, returning the set and the facts
// so a case can stamp and read.
func viewOver(tb assert.TB, g *store.Graph) (rules.View, *store.ReadSet, *meta.Facts) {
	tb.Helper()

	reads := store.NewReadSet()
	reader, err := g.Reader(reads, nil)
	assert.NoError(tb, err, "the sealed graph hands out a reader")
	registry := meta.NewRegistry()
	keys, err := meta.Kernel(registry)
	assert.NoError(tb, err, "the kernel keys register")
	facts := meta.NewFacts(registry)
	return rules.View{Decls: reader, Facts: facts, Reads: reads, Kernel: keys}, reads, facts
}

// boundOver binds the fixture's rules to a view over the graph.
func boundOver(tb assert.TB, g *store.Graph) (rules.Bound, *store.ReadSet, *meta.Facts) {
	tb.Helper()

	v, reads, facts := viewOver(tb, g)
	return rules.NewBound(scripted(), v, nil), reads, facts
}

// native is the scripted rules speaking the fixture's language, so
// a contributor in the fixture walks under the scripted policy
// rather than under the absent one a foreign language gets. It
// keeps the scripted generics capability.
type native struct {
	rules.SourceRules
	rules.GenericsRules
}

// Lang returns the fixture's language.
func (native) Lang() symbol.Lang { return coretest.Lang }

// scripted returns the scripted rules bound to the fixture's
// language.
func scripted() rules.SourceRules {
	s := rulestest.Scripted()
	return native{SourceRules: s, GenericsRules: s.(rules.GenericsRules)}
}

// named returns a resolved reference to a declaration in the
// fixture's language.
func named(path, name string, kind symbol.Kind) *node.TypeRef {
	return &node.TypeRef{Spelling: name, Target: coretest.ID(path, name, kind)}
}

// builtin returns a reference the resolution step left without a
// target.
func builtin(spelling string) *node.TypeRef { return &node.TypeRef{Spelling: spelling} }

// field returns a typed field on a host.
func field(path, host, name string, typ *node.TypeRef) *node.Field {
	f := coretest.Field(path, host, name)
	f.Type = typ
	return f
}

// method returns a method on a host with one int parameter and one
// string result.
func method(path, host, name string) *node.Method {
	m := coretest.Method(path, host, name)
	m.Params = []*node.Param{{Name: "n", Type: builtin(intSpelling)}}
	m.Returns = []*node.Return{{Type: builtin(strSpelling)}}
	return m
}

// embedding returns an embed of a resolved struct.
func embedding(path, name string) *node.Embed {
	return &node.Embed{Ref: named(path, name, symbol.KindStruct)}
}

// embedOf returns an embed of a resolved declaration of any kind.
func embedOf(path, name string, kind symbol.Kind) *node.Embed {
	return &node.Embed{Ref: named(path, name, kind)}
}

// iface returns a bare interface, no members pre-populated.
func iface(path, name string) *node.Interface {
	return &node.Interface{ID: coretest.ID(path, name, symbol.KindInterface), Name: name}
}

// hierarchy builds the walk fixture: Base with a field and a method,
// Derived embedding Base and declaring a field of its own, Row with
// two builtin fields, and Store, an interface with one method.
func hierarchy() *node.Package {
	base := coretest.Struct(svcPath, baseName)
	base.Fields = []*node.Field{field(svcPath, baseName, "id", builtin(intSpelling))}
	base.Methods = []*node.Method{method(svcPath, baseName, "ID")}
	derived := coretest.Struct(svcPath, derivedName)
	derived.Embeds = []*node.Embed{embedding(svcPath, baseName)}
	derived.Fields = []*node.Field{field(svcPath, derivedName, "name", builtin(strSpelling))}
	row := coretest.Struct(svcPath, rowName)
	row.Fields = []*node.Field{
		field(svcPath, rowName, "name", builtin(strSpelling)),
		field(svcPath, rowName, "count", builtin(intSpelling)),
	}
	iface := coretest.Interface(svcPath, storeName)
	iface.Methods = []*node.Method{method(svcPath, storeName, "Get")}
	return coretest.Package(svcPath, base, derived, row, iface)
}

// policy overrides the scripted language's member policy, so a
// case exercises one shadowing rule or contribution list.
type policy struct {
	rules.SourceRules
	members rules.MemberPolicy
}

// Members returns the overriding policy.
func (p policy) Members() rules.MemberPolicy { return p.members }

// nongeneric hides the scripted language's generics capability.
type nongeneric struct {
	inner rules.SourceRules
}

func (n nongeneric) Lang() symbol.Lang           { return n.inner.Lang() }
func (n nongeneric) Members() rules.MemberPolicy { return n.inner.Members() }
func (n nongeneric) ParamRole(p *node.Param, v rules.View) rules.ParamRole {
	return n.inner.ParamRole(p, v)
}

func (n nongeneric) ReturnRoles(rs []*node.Return, v rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	return n.inner.ReturnRoles(rs, v)
}

func (n nongeneric) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	return n.inner.Builtin(ref, v)
}

func (n nongeneric) Resolve(
	s rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	return n.inner.Resolve(s, name, kind, v)
}

func (n nongeneric) SamplesOf(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	return n.inner.SamplesOf(ref, hint, v)
}

func (n nongeneric) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	return n.inner.ZeroValue(ref, v)
}

func (n nongeneric) LiteralFor(f *node.File, ref *node.TypeRef, text string, v rules.View) (emit.Value, bool) {
	return n.inner.LiteralFor(f, ref, text, v)
}

func (n nongeneric) TypeName(word, base string) string { return n.inner.TypeName(word, base) }
