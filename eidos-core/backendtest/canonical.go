// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

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

// canonicalKinds fixes which kinds the fixture holds a
// declaration for, in build order.
var canonicalKinds = []symbol.Kind{
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
func CanonicalFixture(tb assert.TB, inventory map[symbol.Kind]string) *Fixture {
	tb.Helper()

	if len(inventory) == 0 {
		tb.Errorf("the canonical fixture takes an inventory naming at least one kind")
		return nil
	}
	requested := make([]symbol.Kind, 0, len(inventory))
	for k := range inventory {
		requested = append(requested, k)
	}
	slices.Sort(requested)
	for _, k := range requested {
		if !slices.Contains(canonicalKinds, k) {
			tb.Errorf("the canonical fixture holds no %s declaration", k)
			return nil
		}
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

// canonicalUnit returns the one unit covering a kind.
func canonicalUnit(k symbol.Kind) plugin.Unit {
	switch k {
	case symbol.KindStruct:
		return unitFor("row", rowStruct())
	case symbol.KindInterface:
		return unitFor("store", storeInterface())
	case symbol.KindFunction:
		return unitFor("task", taskFunctions()...)
	case symbol.KindMethod:
		return unitFor("track", trackMethod())
	case symbol.KindAlias:
		return unitFor("id", idAlias())
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
		{Name: "Boot", Body: emit.Body{}},
		{Name: "Fetch", Body: emit.Body{Stmts: scaffoldStmts()}},
		{Name: "Push", Body: ref},
		{Name: "Trace", Body: emit.Body{Verbatim: verbatimBody}},
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
		Origin: originOf("Row", symbol.KindStruct),
		Doc:    []string{"Row holds one canonical record."},
		Name:   "Row",
	}
	s.Fields.Append(&emit.Field{
		Origin:  memberOf("Row", "Name", symbol.KindField),
		Doc:     []string{"Name keys the row."},
		Comment: "unique per store",
		Name:    "Name",
		Type:    typeRef("string"),
		Tag:     `json:"name"`,
	})
	for _, c := range formCallables() {
		s.Methods.Append(&emit.Method{
			Origin: memberOf("Row", c.Name, symbol.KindMethod),
			Name:   c.Name,
			Body:   c.Body,
		})
	}
	return s
}

// storeInterface returns the interface: one documented method
// signature with a parameter and a result, which is what the
// signature vocabulary renders.
func storeInterface() *emit.Interface {
	i := &emit.Interface{
		Origin: originOf("Store", symbol.KindInterface),
		Doc:    []string{"Store reads rows back."},
		Name:   "Store",
	}
	i.Methods.Append(&emit.Method{
		Origin:  memberOf("Store", "Get", symbol.KindMethod),
		Doc:     []string{"Get returns the row key names."},
		Name:    "Get",
		Params:  []*emit.Param{{Name: "key", Type: typeRef("string")}},
		Returns: []*emit.Return{{Type: typeRef("string")}},
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

// trackMethod returns the file-level method, attached to the
// struct by reference the way a target with receiver syntax
// spells it.
func trackMethod() *emit.Method {
	return &emit.Method{
		Origin:   memberOf("Row", "Track", symbol.KindMethod),
		Name:     "Track",
		Receives: typeRef("Row"),
		Body:     emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}},
	}
}

// idAlias returns the alias.
func idAlias() *emit.Alias {
	return &emit.Alias{
		Origin: originOf("ID", symbol.KindAlias),
		Doc:    []string{"ID names a row."},
		Name:   "ID",
		Target: typeRef("string"),
	}
}

// limitConstant returns the constant, untyped the way a target
// without one spells it anyway, its trailing comment stated so a
// target that renders one proves it.
func limitConstant() *emit.Constant {
	return &emit.Constant{
		Origin:  originOf("Limit", symbol.KindConstant),
		Doc:     []string{"Limit bounds one fetch."},
		Comment: "rows per call",
		Name:    "Limit",
		Value:   "8",
	}
}

// countVariable returns the variable, typed so every target has a
// spelling to write.
func countVariable() *emit.Variable {
	return &emit.Variable{
		Origin: originOf("Count", symbol.KindVariable),
		Doc:    []string{"Count tracks fetches."},
		Name:   "Count",
		Type:   typeRef("int"),
	}
}
