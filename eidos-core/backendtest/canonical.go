// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest

import (
	"io/fs"
	"slices"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
)

// emitter is the one plugin the canonical fixture emits under:
// the schedule's only entry, and the tree the reference form
// resolves in.
const emitter plugin.ID = "gen"

// fixtureLang is the source language every canonical identity
// carries. The fixture is language-neutral, so the spelling names
// no satellite.
const fixtureLang symbol.Lang = "fixture"

// fixturePackage is the package path every canonical unit shares,
// and the name its package clause spells where a target writes
// one.
const fixturePackage = "svc"

// keyExt is the extension the canonical routing keys carry, so a
// target's naming proves it trims the source extension rather
// than the target's.
const keyExt = ".src"

// canonicalWord is the family word every canonical unit carries.
const canonicalWord = "gen"

// refTemplate is the template the reference-form body names,
// resolved in the emitter's tree.
const refTemplate = "save.tpl"

// verbatimBody is the literal text the verbatim-form body
// carries: one indented call, parseable where a formatter reads
// the file.
const verbatimBody = "\ttrace()\n"

// boundName is the one bound every generic canonical declaration
// constrains by, so each target proves one bound spelling.
const boundName = "Codec"

// canonicalKinds fixes which kinds the fixture holds a
// declaration for, in build order.
var canonicalKinds = []symbol.Kind{
	symbol.KindEnum,
	symbol.KindSum,
	symbol.KindStruct,
	symbol.KindInterface,
	symbol.KindFunction,
	symbol.KindMethod,
	symbol.KindAlias,
	symbol.KindConstant,
	symbol.KindVariable,
}

// CanonicalFixture returns the kernel's shared emit fixture,
// filtered to a backend's declared kind inventory: one unit per
// kind the inventory holds, the four body content forms on the
// functions and again on the struct's member methods, and the
// template tree the reference form resolves in.
//
// The inventory is the map a backend already declares its kind
// templates in; only its keys are read. Filtering is what keeps
// [AssertSpeltKinds] honest per backend: the fixture emits
// exactly what the backend claims to spell, so a kind outside the
// claim never renders and a kind inside it must. The fixture
// holds a declaration for every kind a backend spells today; an
// inventory naming any other kind fails the test, so a backend
// growing a new spelling extends this fixture before it can claim
// coverage.
//
// Member coverage does not depend on file-level callables: a
// backend whose inventory carries no function or method kind
// still reaches every content form through its struct template's
// members. Two calls build two isolated fixtures, the way [Setup]
// requires.
//
// The parameterizable kinds carry a generic sibling beside the
// plain declaration: a parameter list, one named bound, a
// reference restating a parameter as an argument, and a method
// declaring parameters of its own, so a backend's generic
// spellings render under the same suite. The siblings stay inside
// what every target spells; variance, defaults and value
// parameters stay in each satellite's own template tests.
//
// Every declared name spells in the neutral lower camel form, so
// a backend declaring a respell convention proves it as bytes:
// one fixture renders row as Row into Go, fetch as fetch into
// TypeScript and Java, and as fetch into Rust's snake case.
func CanonicalFixture(tb assert.TB, inventory map[symbol.Kind]string) *Fixture {
	tb.Helper()

	if _, valid := requestedKinds(tb, inventory); !valid {
		return nil
	}

	e := plugin.NewEmit()
	for _, k := range canonicalKinds {
		if _, held := inventory[k]; !held {
			continue
		}
		u := canonicalUnit(k)
		if err := e.Add(u); err != nil {
			tb.Errorf("the canonical %s unit arrives: %v", k, err)
			return nil
		}
	}
	return &Fixture{
		Emit:     e,
		Schedule: []plugin.ID{emitter},
		Trees:    map[plugin.ID]fs.FS{emitter: canonicalTree()},
	}
}

// requestedKinds validates an inventory and returns its kinds in
// kind order; false means the calling test already failed. The
// fixtures hold a declaration for every kind a backend spells
// today, so an inventory naming any other kind is refused, and a
// backend growing a new spelling extends the fixtures before it
// can claim coverage.
func requestedKinds(
	tb assert.TB, inventory map[symbol.Kind]string,
) ([]symbol.Kind, bool) {
	tb.Helper()

	if len(inventory) == 0 {
		tb.Errorf("the fixture takes an inventory naming at least one kind")
		return nil, false
	}
	requested := make([]symbol.Kind, 0, len(inventory))
	for k := range inventory {
		requested = append(requested, k)
	}
	slices.Sort(requested)
	for _, k := range requested {
		if !slices.Contains(canonicalKinds, k) {
			tb.Errorf("the fixture holds no %s declaration", k)
			return nil, false
		}
	}
	return requested, true
}

// canonicalTree returns the emitter's template tree: the
// reference-form template, placing the slots marker so pending
// contributions splice rather than drop.
func canonicalTree() fs.FS {
	return fstest.MapFS{
		refTemplate: &fstest.MapFile{
			Data: []byte("\tsaving()\n{{" + render.BuiltinSlots + "}}"),
		},
	}
}

// canonicalUnit returns the one unit covering a kind, the plain
// declaration first and its generic sibling beside it where the
// kind parameterizes.
func canonicalUnit(k symbol.Kind) plugin.Unit {
	switch k {
	case symbol.KindEnum:
		return unitFor("phase", phaseEnum())
	case symbol.KindSum:
		return unitFor("shape", shapeSum(), packSum())
	case symbol.KindStruct:
		return unitFor("row", rowStruct(), boxStruct())
	case symbol.KindInterface:
		return unitFor("store", storeInterface(), keyedInterface())
	case symbol.KindFunction:
		return unitFor("task", append(taskFunctions(), sortFunction())...)
	case symbol.KindMethod:
		return unitFor("track", trackMethod(), foldMethod())
	case symbol.KindAlias:
		return unitFor("id", idAlias(), matchAlias())
	case symbol.KindConstant:
		return unitFor("limit", limitConstant())
	default: // symbol.KindVariable, by canonicalKinds
		return unitFor("count", countVariable())
	}
}

// unitFor returns one per-source unit under the emitter, keyed by
// stem, with its origins collected from the declarations.
func unitFor(stem string, decls ...symbol.Symbol) plugin.Unit {
	origins := make([]symbol.Identity, 0, len(decls))
	for _, d := range decls {
		if id, held := emit.OriginOf(d); held && !id.IsZero() {
			origins = append(origins, id)
		}
	}
	slices.SortFunc(origins, compareIdentity)
	origins = slices.CompactFunc(origins, func(a, b symbol.Identity) bool {
		return a == b
	})
	return plugin.Unit{
		Plugin:  emitter,
		Per:     plugin.PerSource,
		Word:    canonicalWord,
		Key:     fixturePackage + "/" + stem + keyExt,
		Pkg:     packageID(),
		Decls:   decls,
		Origins: origins,
	}
}

// compareIdentity orders identities by their string form, which
// is the canonical spelling everything durable sorts by.
func compareIdentity(a, b symbol.Identity) int {
	as, bs := a.String(), b.String()
	switch {
	case as < bs:
		return -1
	case as > bs:
		return 1
	default:
		return 0
	}
}

// packageID is the owning package every canonical unit shares.
func packageID() symbol.Identity {
	return symbol.Identity{
		Lang:    fixtureLang,
		Package: fixturePackage,
		Name:    fixturePackage,
		Kind:    symbol.KindPackage,
	}
}

// originOf returns a top-level canonical identity.
func originOf(name string, k symbol.Kind) symbol.Identity {
	return symbol.Identity{
		Lang:    fixtureLang,
		Package: fixturePackage,
		Name:    name,
		Kind:    k,
	}
}

// memberOf returns a member's canonical identity under its owner.
func memberOf(owner, name string, k symbol.Kind) symbol.Identity {
	id := originOf(name, k)
	id.Owner = owner
	return id
}

// typeRef returns a reference spelled as written, unresolved the
// way builtins stay.
func typeRef(spelling string) *emit.TypeRef {
	return &emit.TypeRef{Spelling: spelling}
}

// callExpr returns the call of one local name.
func callExpr(name string, args ...string) emit.Expr {
	e := emit.Expr{
		Kind: emit.ExprCall,
		Fn:   &emit.Expr{Kind: emit.ExprName, Name: name},
	}
	for _, a := range args {
		e.Args = append(e.Args, emit.Expr{Kind: emit.ExprName, Name: a})
	}
	return e
}

// formCallable pairs a content form's carrier name with its body.
type formCallable struct {
	Name string
	Body emit.Body
}

// formCallables returns the four content forms under their
// carrier names, fresh per call so no two fixtures share slot
// storage: a body holding nothing, scaffolding every target
// spells, a reference into the emitter's tree with pending
// prologue content, and verbatim text.
func formCallables() []formCallable {
	ref := emit.Body{Ref: &emit.TemplateRef{Name: refTemplate}}
	ref.Prologue.Append(emit.Stmt{Kind: emit.StmtExpr, Value: callExpr("audit")})
	return []formCallable{
		{Name: "boot", Body: emit.Body{}},
		{Name: "fetch", Body: emit.Body{Stmts: scaffoldStmts()}},
		{Name: "push", Body: ref},
		{Name: "trace", Body: emit.Body{Verbatim: verbatimBody}},
	}
}

// scaffoldStmts returns the statements every target spells: a
// single-name declaring assignment, a call reading it, and a bare
// return. The multi-name and guard divergences stay in each
// satellite's own scaffold tests.
func scaffoldStmts() []emit.Stmt {
	return []emit.Stmt{
		{
			Kind:    emit.StmtAssign,
			Names:   []string{"state"},
			Declare: true,
			Value:   callExpr("begin"),
		},
		{Kind: emit.StmtExpr, Value: callExpr("commit", "state")},
		{Kind: emit.StmtReturn},
	}
}

// rowStruct returns the struct: one documented field, and the
// four content forms as member methods, which is how a backend
// whose members render inside the host reaches every form.
func rowStruct() *emit.Struct {
	s := &emit.Struct{
		Origin: originOf("row", symbol.KindStruct),
		Doc:    []string{"Row holds one canonical record."},
		Name:   "row",
	}
	s.Fields.Append(&emit.Field{
		Origin:  memberOf("row", "name", symbol.KindField),
		Doc:     []string{"Name keys the row."},
		Comment: "unique per store",
		Name:    "name",
		Type:    typeRef("string"),
		Tag:     `json:"name"`,
	})
	for _, c := range formCallables() {
		s.Methods.Append(&emit.Method{
			Origin: memberOf("row", c.Name, symbol.KindMethod),
			Name:   c.Name,
			Body:   c.Body,
		})
	}
	return s
}

// phaseEnum returns the enum: two payloadless variants and no
// stated values, which is the shape every target spells, a target
// lowering it into a constant group included. Values and members
// stay in each satellite's own tests, because their spellings
// diverge.
func phaseEnum() *emit.Enum {
	e := &emit.Enum{
		Origin: originOf("phase", symbol.KindEnum),
		Doc:    []string{"phase names a lifecycle step."},
		Name:   "phase",
	}
	e.Variants.Append(
		&emit.EnumVariant{
			Origin: memberOf("phase", "open", symbol.KindEnumVariant),
			Doc:    []string{"open admits writes."},
			Name:   "open",
		},
		&emit.EnumVariant{
			Origin: memberOf("phase", "closed", symbol.KindEnumVariant),
			Name:   "closed",
		},
	)
	return e
}

// shapeSum returns the sum: one variant carrying a named payload
// field and one carrying none, which is the shape every target
// claiming the kind spells, a target lowering it into variant
// types included. Positional payloads and variant methods stay in
// each satellite's own tests, because their spellings diverge.
func shapeSum() *emit.Sum {
	s := &emit.Sum{
		Origin: originOf("shape", symbol.KindSum),
		Doc:    []string{"shape is one closed figure."},
		Name:   "shape",
	}
	circle := &emit.SumVariant{
		Origin: memberOf("shape", "circle", symbol.KindSumVariant),
		Doc:    []string{"circle bounds by a radius."},
		Name:   "circle",
	}
	circle.Fields.Append(&emit.Field{
		Origin: memberOf("circle", "radius", symbol.KindField),
		Name:   "radius",
		Type:   typeRef("int"),
	})
	s.Variants.Append(circle, &emit.SumVariant{
		Origin: memberOf("shape", "empty", symbol.KindSumVariant),
		Name:   "empty",
	})
	return s
}

// packSum returns the generic sum: one type parameter, a variant
// whose payload references it, and a payloadless variant beside
// it, so a target restating the parameter over its variants
// proves the restatement. The parameter stays unbounded the way
// the generic struct's does.
func packSum() *emit.Sum {
	s := &emit.Sum{
		Origin:     originOf("pack", symbol.KindSum),
		Doc:        []string{"pack carries one optional item."},
		Name:       "pack",
		TypeParams: []*emit.TypeParam{{Name: "T"}},
	}
	some := &emit.SumVariant{
		Origin: memberOf("pack", "some", symbol.KindSumVariant),
		Doc:    []string{"some holds the item."},
		Name:   "some",
	}
	some.Fields.Append(&emit.Field{
		Origin: memberOf("some", "item", symbol.KindField),
		Name:   "item",
		Type:   typeRef("T"),
	})
	s.Variants.Append(some, &emit.SumVariant{
		Origin: memberOf("pack", "none", symbol.KindSumVariant),
		Name:   "none",
	})
	return s
}

// boxStruct returns the generic struct: one type parameter, a
// field referencing it, and a member method declaring a bounded
// parameter of its own, which is how a target rendering members
// inside the host spells a method's own list. The host's
// parameter stays unbounded, so a target restating it over an
// impl block restates the name alone.
func boxStruct() *emit.Struct {
	s := &emit.Struct{
		Origin:     originOf("box", symbol.KindStruct),
		Doc:        []string{"Box wraps one item."},
		Name:       "box",
		TypeParams: []*emit.TypeParam{{Name: "T"}},
	}
	s.Fields.Append(&emit.Field{
		Origin: memberOf("box", "item", symbol.KindField),
		Doc:    []string{"Item is the wrapped value."},
		Name:   "item",
		Type:   typeRef("T"),
	})
	s.Methods.Append(&emit.Method{
		Origin: memberOf("box", "map", symbol.KindMethod),
		Doc:    []string{"Map rewraps the item."},
		Name:   "map",
		TypeParams: []*emit.TypeParam{
			{Name: "U", Bounds: []*emit.TypeRef{typeRef(boundName)}},
		},
		Params:  []*emit.Param{{Name: "item", Type: typeRef("U")}},
		Returns: []*emit.Return{{Type: typeRef("U")}},
	})
	return s
}

// storeInterface returns the interface: one documented method
// signature with a parameter and a result, which is what the
// signature vocabulary renders, and one widened contract, which
// every target spells in its own supertype form.
func storeInterface() *emit.Interface {
	i := &emit.Interface{
		Origin:  originOf("store", symbol.KindInterface),
		Doc:     []string{"Store reads rows back."},
		Name:    "store",
		Extends: []*emit.TypeRef{typeRef("Closer")},
	}
	i.Methods.Append(&emit.Method{
		Origin:  memberOf("store", "get", symbol.KindMethod),
		Doc:     []string{"Get returns the row key names."},
		Name:    "get",
		Params:  []*emit.Param{{Name: "key", Type: typeRef("string")}},
		Returns: []*emit.Return{{Type: typeRef("string")}},
	})
	return i
}

// keyedInterface returns the generic interface: one bounded type
// parameter its method signature references.
func keyedInterface() *emit.Interface {
	i := &emit.Interface{
		Origin: originOf("keyed", symbol.KindInterface),
		Doc:    []string{"Keyed looks rows up."},
		Name:   "keyed",
		TypeParams: []*emit.TypeParam{
			{Name: "K", Bounds: []*emit.TypeRef{typeRef(boundName)}},
		},
	}
	i.Methods.Append(&emit.Method{
		Origin:  memberOf("keyed", "pick", symbol.KindMethod),
		Doc:     []string{"Pick returns the row at a key."},
		Name:    "pick",
		Params:  []*emit.Param{{Name: "key", Type: typeRef("K")}},
		Returns: []*emit.Return{{Type: typeRef("K")}},
	})
	return i
}

// taskFunctions returns the four content forms as file-level
// functions.
func taskFunctions() []symbol.Symbol {
	cs := formCallables()
	fns := make([]symbol.Symbol, 0, len(cs))
	for _, c := range cs {
		fns = append(fns, &emit.Function{
			Origin: originOf(c.Name, symbol.KindFunction),
			Name:   c.Name,
			Body:   c.Body,
		})
	}
	return fns
}

// sortFunction returns the generic function: one bounded type
// parameter its signature references.
func sortFunction() symbol.Symbol {
	return &emit.Function{
		Origin: originOf("sort", symbol.KindFunction),
		Doc:    []string{"Sort orders items in place."},
		Name:   "sort",
		TypeParams: []*emit.TypeParam{
			{Name: "T", Bounds: []*emit.TypeRef{typeRef(boundName)}},
		},
		Params:  []*emit.Param{{Name: "items", Type: typeRef("T")}},
		Returns: []*emit.Return{{Type: typeRef("T")}},
	}
}

// trackMethod returns the file-level method, attached to the
// struct by reference the way a target with receiver syntax
// spells it.
func trackMethod() *emit.Method {
	return &emit.Method{
		Origin:   memberOf("row", "track", symbol.KindMethod),
		Name:     "track",
		Receives: typeRef("row"),
		Body:     emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}},
	}
}

// foldMethod returns the generic file-level method: the receiver
// restates the generic host's parameter as an argument, and the
// method declares a bounded parameter of its own, which Go spells
// behind the name and Rust inside the impl block the receiver
// opens.
func foldMethod() *emit.Method {
	return &emit.Method{
		Origin: memberOf("box", "fold", symbol.KindMethod),
		Doc:    []string{"Fold collapses the box."},
		Name:   "fold",
		Receives: &emit.TypeRef{
			Spelling: "box",
			Args:     []*emit.TypeRef{typeRef("T")},
		},
		TypeParams: []*emit.TypeParam{
			{Name: "U", Bounds: []*emit.TypeRef{typeRef(boundName)}},
		},
		Params:  []*emit.Param{{Name: "item", Type: typeRef("U")}},
		Returns: []*emit.Return{{Type: typeRef("U")}},
		Body:    emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}},
	}
}

// idAlias returns the alias.
func idAlias() *emit.Alias {
	return &emit.Alias{
		Origin: originOf("id", symbol.KindAlias),
		Doc:    []string{"ID names a row."},
		Name:   "id",
		Target: typeRef("string"),
	}
}

// matchAlias returns the generic alias: a bounded parameter the
// target reference restates as an argument, which is what proves
// an argument list spells.
func matchAlias() *emit.Alias {
	return &emit.Alias{
		Origin: originOf("match", symbol.KindAlias),
		Doc:    []string{"Match names a keyed lookup."},
		Name:   "match",
		TypeParams: []*emit.TypeParam{
			{Name: "T", Bounds: []*emit.TypeRef{typeRef(boundName)}},
		},
		Target: &emit.TypeRef{
			Spelling: "keyed",
			Args:     []*emit.TypeRef{typeRef("T")},
		},
	}
}

// limitConstant returns the constant, untyped the way a target
// without one spells it anyway, its trailing comment stated so a
// target that renders one proves it.
func limitConstant() *emit.Constant {
	return &emit.Constant{
		Origin:  originOf("limit", symbol.KindConstant),
		Doc:     []string{"Limit bounds one fetch."},
		Comment: "rows per call",
		Name:    "limit",
		Value:   "8",
	}
}

// countVariable returns the variable, typed so every target has a
// spelling to write, its initializer stated so a target that
// renders one proves it.
func countVariable() *emit.Variable {
	return &emit.Variable{
		Origin: originOf("count", symbol.KindVariable),
		Doc:    []string{"Count tracks fetches."},
		Name:   "count",
		Type:   typeRef("int"),
		Value:  "0",
	}
}
