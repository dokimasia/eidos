// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest

import (
	"io/fs"
	"slices"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// emitter is the ID of the one plugin that emits the canonical fixture.
// It is the only entry of the schedule, and the reference form resolves
// its template in the tree of this plugin.
const emitter plugin.ID = "gen"

// fixtureLang is the source language of every canonical identity. The
// fixture is language-neutral, so the value is not the name of a
// satellite.
const fixtureLang symbol.Lang = "fixture"

// fixturePackage is the package path of every canonical unit. A target
// that writes a package clause uses it as the package name.
const fixturePackage = "svc"

// keyExt is the extension of every canonical routing key. No target uses
// it, so the output names prove that the naming of a target removes the
// source extension and not its own.
const keyExt = ".src"

// canonicalWord is the family word of every canonical unit.
const canonicalWord = "gen"

// refTemplate is the name of the template that the body in the
// reference form refers to. The render resolves it in the tree of the
// emitter.
const refTemplate = "save.tpl"

// verbatimBody is the text of the body in the verbatim form. The text is
// one indented call, so a formatter that reads the file can parse it.
const verbatimBody = "\ttrace()\n"

// boundName is the bound of every generic canonical declaration, so each
// target renders the same bound.
const boundName = "Codec"

// Two or more canonical declarations use these names. itemName is the
// name of a field and of a parameter, and keyName is the name of a
// parameter. keyedName is the name of the generic interface that the
// generic alias refers to.
const (
	itemName  = "item"
	keyName   = "key"
	keyedName = "keyed"
)

// The scaffolded fetch calls a function of another package. These are
// the package, the class and the function of that call. The fixture
// needs the class because Java has no free functions.
const (
	auditPackage = "audit"
	auditOwner   = "Audit"
	auditName    = "Record"
)

// The struct tag of the canonical field has one entry with this key and
// this value.
const (
	tagKey   = "json"
	tagValue = "name"
)

// canonicalKinds lists, in build order, the kinds of the declarations
// that the fixture emits. It contains every kind that an emit
// declaration can have at file level.
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

// CanonicalFixture returns the shared emit fixture of the kernel. The
// fixture has one unit per canonical kind. The functions and the member
// methods of the struct each have the four content forms of a body. The
// fixture also has the template tree in which the reference form
// resolves. The scaffolded body calls a function of another package
// through a reference expression, so each target qualifies the call and
// imports its package.
//
// The fixture emits every kind that an emit declaration can have at file
// level, whatever the backend under test supports. Each kind renders,
// lowers into kinds that render, or reports under [render.RefusedKind]
// with the reason that the backend declares. [AssertSpeltKinds] fails on
// any other outcome. A backend that refuses the file-level callables
// still renders every content form through the members of its struct
// template. Two calls build two separate fixtures, as [Setup] requires.
//
// Each kind that takes type parameters has a generic sibling beside the
// plain declaration. The sibling has a parameter list, one named bound,
// a reference that restates a parameter as an argument, and a method
// that declares type parameters of its own. The suite therefore renders
// the generic forms of each backend. The siblings use only what every
// target can render. The tests of each satellite cover variance,
// defaults and value parameters.
//
// Every declared name is in the neutral lower camel case, so a backend
// with a respell convention proves the convention in its output bytes.
// The fixture renders row as Row in Go, and fetch as fetch in
// TypeScript, Java and Rust.
func CanonicalFixture(tb assert.TB) *Fixture {
	tb.Helper()

	e := plugin.NewEmit()
	for _, k := range canonicalKinds {
		assert.NoError(tb, e.Add(canonicalUnit(k)), "the canonical "+k.String()+" unit arrives")
	}
	return &Fixture{
		Emit:     e,
		Schedule: []plugin.ID{emitter},
		Trees:    map[plugin.ID]fs.FS{emitter: canonicalTree()},
	}
}

// canonicalTree returns the template tree of the emitter. Its one
// template is the template of the reference form. The template contains
// the slots marker, so the render splices every pending contribution and
// drops none.
func canonicalTree() fs.FS {
	return fstest.MapFS{
		refTemplate: &fstest.MapFile{
			Data: []byte("\tsaving()\n{{" + render.BuiltinSlots + "}}"),
		},
	}
}

// canonicalUnit returns the unit of the kind k. The unit contains the
// plain declaration, and the generic sibling where k takes type
// parameters.
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

// unitFor returns a per-source unit of the emitter with the declarations
// decls. The key of the unit derives from stem, and the declarations are
// in the order that a flush leaves.
func unitFor(stem string, decls ...symbol.Symbol) plugin.Unit {
	origins := flushOrder(decls)
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

// flushOrder sorts decls into the order that a flush leaves for origins
// without a position, which the canonical origins are. It sorts by origin
// identity and keeps the insertion order of equal origins. It returns the
// distinct origins in that order.
func flushOrder(decls []symbol.Symbol) []symbol.Identity {
	slices.SortStableFunc(decls, func(a, b symbol.Symbol) int {
		oa, _ := emit.OriginOf(a)
		ob, _ := emit.OriginOf(b)
		return oa.Compare(ob)
	})
	origins := make([]symbol.Identity, 0, len(decls))
	for _, d := range decls {
		if id, held := emit.OriginOf(d); held && !id.IsZero() {
			origins = append(origins, id)
		}
	}
	return slices.Compact(origins)
}

// packageID returns the package identity of every canonical unit.
func packageID() symbol.Identity {
	return symbol.Identity{
		Lang:    fixtureLang,
		Package: fixturePackage,
		Name:    fixturePackage,
		Kind:    symbol.KindPackage,
	}
}

// originOf returns the canonical identity of the top-level declaration
// name of the kind k.
func originOf(name string, k symbol.Kind) symbol.Identity {
	return symbol.Identity{
		Lang:    fixtureLang,
		Package: fixturePackage,
		Name:    name,
		Kind:    k,
	}
}

// memberOf returns the canonical identity of the member name of the kind
// k under owner.
func memberOf(owner, name string, k symbol.Kind) symbol.Identity {
	id := originOf(name, k)
	id.Owner = owner
	return id
}

// typeRef returns an unresolved reference with the given spelling, as
// the reference to a builtin type is.
func typeRef(spelling string) *emit.TypeRef {
	return &emit.TypeRef{Spelling: spelling}
}

// paramOf returns a reference to the type parameter name of host. Its
// target is the parameter, as Link sets it. A backend reads the target
// when it binds the type parameters of a receiver.
func paramOf(host, name string) *emit.TypeRef {
	return &emit.TypeRef{Spelling: name, Target: memberOf(host, name, symbol.KindTypeParam)}
}

// callExpr returns a call of the local function name with the local
// names args as its arguments.
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

// formCallable is a body in one content form and the name of the
// callable that contains it.
type formCallable struct {
	Name string
	Body emit.Body
}

// formCallables returns the four content forms with the names of their
// callables. The forms are an empty body, scaffolding that every target
// renders, a reference into the tree of the emitter with pending
// prologue content, and verbatim text. Each call builds new values, so
// two fixtures share no slot storage.
func formCallables() []formCallable {
	ref := emit.Body{Ref: &emit.TemplateRef{Name: refTemplate}}
	ref.Prologue.Append(emit.Stmt{Kind: emit.StmtExpr, Value: callExpr("audit")})
	return []formCallable{
		{Name: "boot", Body: emit.Body{}},
		{Name: "fetch", Body: emit.Body{Stmts: fetchStmts()}},
		{Name: "push", Body: ref},
		{Name: "trace", Body: emit.Body{Verbatim: verbatimBody}},
	}
}

// fetchStmts returns the scaffolding of the canonical fetch. It inserts
// a call of a function of another package into the statements of
// [scaffoldStmts], before the return. The call passes a local name, so
// each target proves that it qualifies a reference and records its
// import.
func fetchStmts() []emit.Stmt {
	stmts := scaffoldStmts()
	record := emit.RefExpr(symbol.Identity{
		Lang:    fixtureLang,
		Package: auditPackage,
		Owner:   auditOwner,
		Name:    auditName,
		Kind:    symbol.KindMethod,
	})
	audit := emit.Stmt{
		Kind:  emit.StmtExpr,
		Value: emit.Expr{Kind: emit.ExprCall, Fn: &record, Args: []emit.Expr{{Kind: emit.ExprName, Name: "state"}}},
	}
	return slices.Insert(stmts, len(stmts)-1, audit)
}

// scaffoldStmts returns statements that every target renders. They are
// an assignment that declares one name, a call that reads the name, and
// a bare return. The scaffold tests of each satellite cover assignments
// of two or more names and guards.
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

// rowStruct returns the struct. It has one documented field with a
// struct tag, and one member method in each of the four content forms. A
// backend that renders members inside the host meets every form through
// these methods.
func rowStruct() *emit.Struct {
	s := &emit.Struct{
		Origin:  originOf("row", symbol.KindStruct),
		Doc:     []string{"Row is one canonical record."},
		Comment: "one per fetch",
		Name:    "row",
	}
	s.Fields.Append(&emit.Field{
		Origin:  memberOf("row", "name", symbol.KindField),
		Doc:     []string{"Name keys the row."},
		Comment: "unique per store",
		Name:    "name",
		Type:    typeRef("string"),
		Tags:    emit.SlotOf(&emit.TagEntry{Key: tagKey, Value: tagValue}),
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

// phaseEnum returns the enum. It has two variants without payloads and
// without values, a shape that every target renders. A target that
// lowers an enum into a constant group renders it too. The tests of each
// satellite cover values and members, because the targets render them
// differently.
func phaseEnum() *emit.Enum {
	e := &emit.Enum{
		Origin:  originOf("phase", symbol.KindEnum),
		Doc:     []string{"phase names a lifecycle step."},
		Comment: "closed set",
		Name:    "phase",
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

// shapeSum returns the sum. It has one variant with a named payload
// field and one variant without a payload. Every target that renders
// sums renders this shape, and so does a target that lowers a sum into
// variant types. The tests of each satellite cover positional payloads
// and variant methods, because the targets render them differently.
func shapeSum() *emit.Sum {
	s := &emit.Sum{
		Origin:  originOf("shape", symbol.KindSum),
		Doc:     []string{"shape is one closed figure."},
		Comment: "tagged",
		Name:    "shape",
	}
	circle := &emit.SumVariant{
		Origin:  memberOf("shape", "circle", symbol.KindSumVariant),
		Doc:     []string{"circle bounds by a radius."},
		Comment: "round",
		Name:    "circle",
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

// packSum returns the generic sum. It has one type parameter, a variant
// with a payload that refers to the parameter, and a variant without a
// payload. A target that restates the parameter on each variant proves
// the restatement. The parameter has no bound, as the parameter of the
// generic struct has none.
func packSum() *emit.Sum {
	s := &emit.Sum{
		Origin:     originOf("pack", symbol.KindSum),
		Doc:        []string{"pack wraps one optional item."},
		Name:       "pack",
		TypeParams: []*emit.TypeParam{{Name: "T"}},
	}
	some := &emit.SumVariant{
		Origin: memberOf("pack", "some", symbol.KindSumVariant),
		Doc:    []string{"some contains the item."},
		Name:   "some",
	}
	some.Fields.Append(&emit.Field{
		Origin: memberOf("some", itemName, symbol.KindField),
		Name:   itemName,
		Type:   typeRef("T"),
	})
	s.Variants.Append(some, &emit.SumVariant{
		Origin: memberOf("pack", "none", symbol.KindSumVariant),
		Name:   "none",
	})
	return s
}

// boxStruct returns the generic struct. It has one type parameter, a
// field that refers to it, and a member method that declares a bounded
// type parameter of its own. A target that renders members inside the
// host proves through the method that it renders the type parameters of
// a member. The type parameter of the host has no bound, so a target
// that restates it on an impl block restates only the name.
func boxStruct() *emit.Struct {
	s := &emit.Struct{
		Origin:     originOf("box", symbol.KindStruct),
		Doc:        []string{"Box wraps one item."},
		Name:       "box",
		TypeParams: []*emit.TypeParam{{Name: "T"}},
	}
	s.Fields.Append(&emit.Field{
		Origin: memberOf("box", itemName, symbol.KindField),
		Doc:    []string{"Item is the wrapped value."},
		Name:   itemName,
		Type:   typeRef("T"),
	})
	s.Methods.Append(&emit.Method{
		Origin: memberOf("box", "map", symbol.KindMethod),
		Doc:    []string{"Map rewraps the item."},
		Name:   "map",
		TypeParams: []*emit.TypeParam{
			{Name: "U", Bounds: []*emit.TypeRef{typeRef(boundName)}},
		},
		Params:  emit.SlotOf(&emit.Param{Name: itemName, Type: typeRef("U")}),
		Returns: emit.SlotOf(&emit.Return{Type: typeRef("U")}),
	})
	return s
}

// storeInterface returns the interface. It has one documented method
// with a parameter and a result, for the signature vocabulary of each
// target. It extends one interface, and every target renders the
// extension in its own supertype form.
func storeInterface() *emit.Interface {
	i := &emit.Interface{
		Origin:  originOf("store", symbol.KindInterface),
		Doc:     []string{"Store reads rows back."},
		Comment: "read side",
		Name:    "store",
		Extends: []*emit.TypeRef{typeRef("Closer")},
	}
	i.Methods.Append(&emit.Method{
		Origin:  memberOf("store", "get", symbol.KindMethod),
		Doc:     []string{"Get returns the row key names."},
		Comment: "by key",
		Name:    "get",
		Params:  emit.SlotOf(&emit.Param{Name: keyName, Type: typeRef("string"), Comment: "the row key"}),
		Returns: emit.SlotOf(&emit.Return{Type: typeRef("string"), Comment: "the row"}),
	})
	return i
}

// keyedInterface returns the generic interface. It has one bounded type
// parameter, and the signature of its method refers to the parameter.
func keyedInterface() *emit.Interface {
	i := &emit.Interface{
		Origin: originOf(keyedName, symbol.KindInterface),
		Doc:    []string{"Keyed looks rows up."},
		Name:   keyedName,
		TypeParams: []*emit.TypeParam{
			{Name: "K", Bounds: []*emit.TypeRef{typeRef(boundName)}},
		},
	}
	i.Methods.Append(&emit.Method{
		Origin:  memberOf(keyedName, "pick", symbol.KindMethod),
		Doc:     []string{"Pick returns the row at a key."},
		Name:    "pick",
		Params:  emit.SlotOf(&emit.Param{Name: keyName, Type: typeRef("K")}),
		Returns: emit.SlotOf(&emit.Return{Type: typeRef("K")}),
	})
	return i
}

// taskFunctions returns the four content forms as file-level functions.
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

// sortFunction returns the generic function. It has one bounded type
// parameter, and its signature refers to the parameter.
func sortFunction() symbol.Symbol {
	return &emit.Function{
		Origin:  originOf("sort", symbol.KindFunction),
		Doc:     []string{"Sort orders items in place."},
		Comment: "stable",
		Name:    "sort",
		TypeParams: []*emit.TypeParam{
			{Name: "T", Bounds: []*emit.TypeRef{typeRef(boundName)}},
		},
		Params:  emit.SlotOf(&emit.Param{Name: "items", Type: typeRef("T")}),
		Returns: emit.SlotOf(&emit.Return{Type: typeRef("T")}),
	}
}

// trackMethod returns the file-level method. Its Receives field refers
// to the struct, as a target with receiver syntax declares a method.
func trackMethod() *emit.Method {
	return &emit.Method{
		Origin:   memberOf("row", "track", symbol.KindMethod),
		Comment:  "detached",
		Name:     "track",
		Receives: typeRef("row"),
		Body:     emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}},
	}
}

// foldMethod returns the generic file-level method. Its Receives
// reference restates the type parameter of the generic host as an
// argument, with the parameter as its target. The method declares a
// bounded type parameter of its own. Go renders that parameter after the
// name, and Rust renders it inside the impl block of the receiver.
func foldMethod() *emit.Method {
	return &emit.Method{
		Origin: memberOf("box", "fold", symbol.KindMethod),
		Doc:    []string{"Fold collapses the box."},
		Name:   "fold",
		Receives: &emit.TypeRef{
			Spelling: "box",
			Args:     []*emit.TypeRef{paramOf("box", "T")},
		},
		TypeParams: []*emit.TypeParam{
			{Name: "U", Bounds: []*emit.TypeRef{typeRef(boundName)}},
		},
		Params:  emit.SlotOf(&emit.Param{Name: itemName, Type: typeRef("U")}),
		Returns: emit.SlotOf(&emit.Return{Type: typeRef("U")}),
		Body:    emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}},
	}
}

// idAlias returns the alias.
func idAlias() *emit.Alias {
	return &emit.Alias{
		Origin:  originOf("id", symbol.KindAlias),
		Doc:     []string{"ID names a row."},
		Comment: "opaque",
		Name:    "id",
		Target:  typeRef("string"),
	}
}

// matchAlias returns the generic alias. It has a bounded type parameter,
// and its target reference restates the parameter as an argument. The
// alias proves that each target renders an argument list.
func matchAlias() *emit.Alias {
	return &emit.Alias{
		Origin: originOf("match", symbol.KindAlias),
		Doc:    []string{"Match names a keyed lookup."},
		Name:   "match",
		TypeParams: []*emit.TypeParam{
			{Name: "T", Bounds: []*emit.TypeRef{typeRef(boundName)}},
		},
		Target: &emit.TypeRef{
			Spelling: keyedName,
			Args:     []*emit.TypeRef{typeRef("T")},
		},
	}
}

// limitConstant returns the constant of the type int32. It has a
// trailing comment, so a target that renders trailing comments proves
// it.
func limitConstant() *emit.Constant {
	return &emit.Constant{
		Origin:  originOf("limit", symbol.KindConstant),
		Doc:     []string{"Limit bounds one fetch."},
		Comment: "rows per call",
		Name:    "limit",
		Type:    &emit.TypeRef{Spelling: "int32"},
		Value:   "8",
	}
}

// countVariable returns the variable. It has a type, so every target can
// write its declaration. It has an initializer, so a target that renders
// initializers proves it.
func countVariable() *emit.Variable {
	return &emit.Variable{
		Origin: originOf("count", symbol.KindVariable),
		Doc:    []string{"Count tracks fetches."},
		Name:   "count",
		Type:   typeRef("int"),
		Value:  "0",
	}
}
