// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"strconv"
	"strings"
)

// The generator writes into these packages. The generated tests of
// each package declare its black-box twin.
const (
	// SymbolPackage is the package of the Kind and Fact constants.
	SymbolPackage     = "symbol"
	SymbolTestPackage = "symbol_test"

	// RootPackage is the authoring surface that contains the match
	// constructors.
	RootPackage     = "eidos"
	RootTestPackage = "eidos_test"
	// NodePackage is the model a frontend produces.
	NodePackage     = "node"
	NodeTestPackage = "node_test"
	// EmitPackage is the model a generator produces.
	EmitPackage     = "emit"
	EmitTestPackage = "emit_test"
)

// A generated model file qualifies the declarations of other packages
// with these qualifiers.
const (
	symbolQualifier   = SymbolPackage + "."
	positionQualifier = "position."
)

// Rendering composes generated expressions from these spellings.
const (
	// receiverName is the receiver of every generated method.
	receiverName = "x"
	// accessorSuffix ends the name of the exported accessor of a slot.
	accessorSuffix = "Slot"
	// slotType is the storage type of a slot-tagged field.
	slotType = "Slot"
	// itemsMethod, lenMethod and appendMethod are the methods of a
	// slot that generated code calls.
	itemsMethod  = "Items"
	lenMethod    = "Len"
	appendMethod = "Append"
	// symbolsType is the named slice type of a marker-typed field. Its
	// codec reads the kind of a child before it allocates the child. A
	// plain interface slice cannot do that.
	symbolsType = "Symbols"
	// sliceMarker and pointerMarker open a Go type spelling.
	sliceMarker   = "[]"
	pointerMarker = "*"
	// zeroLiteral constructs an empty value of a kind, and addressOf
	// takes its address.
	zeroLiteral = "{}"
	addressOf   = "&"
)

// The binary codec writes the value of a node-side field with one of
// these codecs, and its template switches on them. A field whose type
// matches none renders text that gofmt refuses, so a schema type that
// the codec cannot write fails the generation.
const (
	codecString      = "string"
	codecBool        = "bool"
	codecInt         = "int"
	codecIdentity    = "identity"
	codecPos         = "pos"
	codecStrings     = "strings"
	codecAnnotations = "annotations"
	codecEnum        = "enum"
	codecRef         = "ref"
	codecRefs        = "refs"
	codecSymbols     = "symbols"
)

// The binary codec writes the types of these schema spellings by name.
const (
	stringType   = "string"
	boolType     = "bool"
	intType      = "int"
	identityType = symbolQualifier + "Identity"
	posType      = positionQualifier + "Pos"
	stringsType  = sliceMarker + stringType
)

// The models treat these field names by convention and not by tag.
// Lowering passes them through, and rendering recognizes them.
const (
	// docField is the documentation of a declaration.
	docField = "Doc"
	// posField is the source position of a node symbol.
	posField = "Pos"
	// idField is the canonical identity of a node declaration. The
	// generated Identity method returns it.
	idField = "ID"
	// originField is the node identity of an emit value. The generated
	// OriginOf returns it.
	originField = "Origin"
	// typeField is the type reference of a declaration. A kind with it
	// satisfies the Typed interface.
	typeField = "Type"
	// typeRefKind is the kind that such a field references.
	typeRefKind = "TypeRef"
	// visField is the normalized visibility of a declaration. The
	// respell traversal passes it to the hook.
	visField = "Visibility"
)

// memberMethods are the methods of the Membered interface, each with
// the field names that satisfy it.
var memberMethods = []struct {
	Method string
	Fields []string
}{
	{Method: "FieldList", Fields: []string{"Fields"}},
	{Method: "MethodList", Fields: []string{"Methods"}},
	{Method: "EmbedList", Fields: []string{"Embeds"}},
}

// view is one kind, prepared for a template.
//
// The view computes every spelling that a template needs, so the
// templates range over data and decide nothing. The lowering has no
// layout concerns, and a template does not depend on its side.
type view struct {
	Name string
	// Doc is the documentation of the kind in the schema.
	Doc []string
	// Fields are the fields of the struct in declaration order.
	Fields []fieldView
	// PosExpr and DocExpr satisfy the Symbol interface, and HasPos
	// and HasDoc report whether the kind has the field they read.
	PosExpr string
	DocExpr string
	HasPos  bool
	HasDoc  bool
	// Subject marks a kind that the dispatch surface triggers on. The
	// match template ranges over these kinds.
	Subject bool
	// Fallible reports whether the binary encoding of the kind can
	// fail. It can fail when the kind has a field that admits any kind,
	// or a field of a kind whose encoding can fail. Only a symbol from
	// outside the model fails an encoding.
	Fallible bool
	// ZeroIdentity reports a node-side kind without an ID field. Its
	// Identity method returns the zero identity, so the kind still
	// satisfies the Declaration interface.
	ZeroIdentity bool
	// Members satisfy the Membered interface. The slice is empty for
	// a kind without a member list, and an entry with an empty Items
	// returns nil.
	Members []memberView
	// TypeRefStorage is the field that the TypeRef method returns. It
	// is empty when the kind is not typed.
	TypeRefStorage string
	// IDStorage is the field that the identity accessor returns. It is
	// empty for an emit kind and for a node kind without an ID field.
	IDStorage string
	// OriginStorage is the field that OriginOf returns. It is empty on
	// the node side and on emit kinds that derive from nothing.
	OriginStorage string
	// Walked are the fields the traversal descends into.
	Walked []fieldView
	// Slots are the fields that become slot storage.
	Slots []fieldView
	// Bodies are the storage names of the body fields of an emit kind.
	// The generated seal of the kind seals the slots of each body beside
	// the slots of the kind.
	Bodies []string
	// Children are the statements a test uses to give the subject
	// one child in every traversed field.
	Children []string
	// WalkVisits is the number of declarations that a walk over that
	// subject visits, including the subject.
	WalkVisits int
	// NameStorage is the field of the declared name of the kind. It is
	// empty when the kind declares no name. VisStorage is the field of
	// its visibility, and it is empty when the kind has none.
	NameStorage string
	VisStorage  string
	// NameWalked are the traversed fields that lead to a declared
	// name. They are the descent set of the respell traversal.
	NameWalked []fieldView
	// NameChildren are the statements that give the subject of a test
	// one named child per descent field. NameVisits is the number of
	// names that the respell traversal then visits.
	NameChildren []string
	NameVisits   int
	// Facts are the facts of the fields of the kind, in schema field
	// order. FactWalked are the traversed fields that lead to a kind
	// with facts. They are the descent set of the facts traversal.
	Facts      []factView
	FactWalked []fieldView
	// FactChildren build one child with one fact per descent field
	// that leads to a kind with facts, and record what the traversal
	// yields for each. It is empty when no descent field leads to such
	// a kind.
	FactChildren []factChildView
	// Malformed is an encoding of this kind whose first field has the
	// wrong JSON type. A decoder therefore selects the kind and then
	// fails on its body.
	Malformed string
	// Populate are the statements that give the subject of a test a
	// value other than the zero value in every field that the binary
	// codec writes as a value. Each field gets a value of its own.
	Populate []string
}

// IsMembered reports whether the kind has any member list.
func (v view) IsMembered() bool { return len(v.Members) > 0 }

// memberView is one Membered method and the expressions that satisfy
// it.
type memberView struct {
	Method string
	// Items ranges over the members, and Len sizes the result.
	// Both are empty when the kind has no field for this method.
	Items string
	Len   string
}

// factView is one stated fact as the facts traversal reads it.
type factView struct {
	// Fact is the suffix of the generated constant, from the schema
	// tag.
	Fact string
	// Stated is the expression that reports the fact as stated. It
	// reads the receiver of the traversal.
	Stated string
	// Set is the statement that states the fact on the subject of a
	// test.
	Set string
}

// factChildView is the statements that give the subject of a test one
// child with one fact. It also records what the traversal then yields.
type factChildView struct {
	// Field is the descent field that contains the child. The
	// generated case has its name.
	Field string
	Stmts []string
	Kind  string
	Fact  string
}

// multiReturnFact is the one fact whose statedness does not follow the
// shape of the field. A callable returns several values when the list
// has more than one entry, so the threshold is two and not presence.
const multiReturnFact = "MultiReturn"

// fieldView is one field as one model side declares it.
type fieldView struct {
	// Doc and Comment are the documentation of the field in the
	// schema.
	Doc     []string
	Comment string
	// Name is the name of the field in the schema. Storage is the name
	// that the struct declares, and it differs for a slot.
	Name    string
	Storage string
	// Decl is the type that the struct declares.
	Decl string
	// Elem is the referenced kind. It is empty when the field
	// references no kind.
	Elem string
	// Items ranges over the values of the field, and Len sizes them.
	Items string
	Len   string
	// Accessor is the exported slot method. It is empty when the field
	// is not a slot. Child is the expression that constructs one value
	// that the slot admits.
	Accessor string
	Child    string
	// JSONName is the key of the field in encoded form.
	JSONName string
	// JSONType is the type that the value of the field decodes into.
	// It is the declared type, and a slice of the marker becomes the
	// kind-discriminated [symbolsType] slice.
	JSONType string
	// Codec is one of the codec constants, and the binary codec writes
	// the field with it. It is empty for a type that the codec cannot
	// write. Fallible reports whether the write of the field can fail.
	Codec    string
	Fallible bool
	// Slice and Pointer describe the shape the traversal walks.
	Slice   bool
	Pointer bool
}

// viewsFor prepares every kind for one model side.
//
// side is [NodePackage] or [EmitPackage]. Slot storage exists on the
// emit side alone, so a slot-tagged field renders as an ordinary
// slice on the node side.
func viewsFor(kinds []KindSpec, side string) []view {
	reach, named := nameReach(kinds, side)
	out := make([]view, 0, len(kinds))
	for _, kind := range kinds {
		out = append(out, viewOf(kind, side, reach, named))
	}
	fillFacts(kinds, side, out)
	fillCodec(kinds, side, out)
	return out
}

// fillCodec prepares the views of the binary codec on the node side. It
// marks the kinds and fields whose encoding can fail, and writes the
// statements that give each field a value of its own. It runs after
// viewOf, because fallibility follows the fields of every kind.
func fillCodec(kinds []KindSpec, side string, views []view) {
	if side != NodePackage {
		return
	}
	fallible := fallibleReach(kinds)
	for i := range views {
		v := &views[i]
		v.Fallible = fallible[v.Name]
		for j := range v.Fields {
			f := &v.Fields[j]
			f.Fallible = f.Codec == codecSymbols ||
				((f.Codec == codecRef || f.Codec == codecRefs) && fallible[f.Elem])
			if stmt := populateOf(*f, j); stmt != "" {
				v.Populate = append(v.Populate, stmt)
			}
		}
	}
}

// fallibleReach computes the kinds whose binary encoding can fail. Such
// a kind has a node-side field that admits any kind, or a node-side
// field of a kind whose encoding can fail. Only a symbol from outside
// the model fails an encoding.
func fallibleReach(kinds []KindSpec) map[string]bool {
	fallible := map[string]bool{}
	for _, k := range kinds {
		for _, f := range k.Fields {
			if f.IsSymbol && f.Side.OnNode() {
				fallible[k.Name] = true
				break
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, k := range kinds {
			if fallible[k.Name] {
				continue
			}
			for _, f := range k.Fields {
				if f.Side.OnNode() && fallible[f.Elem] {
					fallible[k.Name] = true
					changed = true
					break
				}
			}
		}
	}
	return fallible
}

// codecOf returns the codec that writes a field. It decides by the
// shape of a field that references kinds, and by the type of any other
// field. It returns the empty string for a type that the codec cannot
// write.
func codecOf(field FieldSpec) string {
	switch {
	case field.IsSymbol && field.Slice:
		return codecSymbols
	case field.Elem != "" && field.Slice:
		return codecRefs
	case field.Elem != "":
		return codecRef
	case field.Type == stringType:
		return codecString
	case field.Type == boolType:
		return codecBool
	case field.Type == intType:
		return codecInt
	case field.Type == stringsType:
		return codecStrings
	case field.Type == identityType:
		return codecIdentity
	case field.Type == posType:
		return codecPos
	case field.Type == annotationsType:
		return codecAnnotations
	case strings.HasPrefix(field.Type, symbolQualifier):
		return codecEnum
	}
	return ""
}

// populateOf writes the statement that gives one field of a test
// subject a value other than the zero value. The value differs from the
// value of every other field. A string gets the name of the field, and
// a number gets its ordinal. A field that references kinds gets its
// value from the children of the test, so populateOf writes no
// statement for it.
func populateOf(f fieldView, ordinal int) string {
	target := "subject." + f.Storage
	n := strconv.Itoa(ordinal + 1)
	switch f.Codec {
	case codecString:
		return target + ` = "` + f.Name + `"`
	case codecBool:
		return target + " = true"
	case codecInt:
		return target + " = -" + n
	case codecEnum:
		return target + " = " + n
	case codecIdentity:
		return target + ` = symbol.Identity{Lang: "lang", Package: "pkg/` + f.Name +
			`", Owner: "Owner", Name: "` + f.Name + `", Kind: symbol.KindStruct, Disc: "disc"}`
	case codecPos:
		return target + ` = position.Pos{File: "` + f.Name + `.go", Line: ` + n + `, Col: ` + n + `}`
	case codecStrings:
		return target + ` = []string{"` + f.Name + `", "", "` + f.Name + `"}`
	case codecAnnotations:
		return target + ` = symbol.Annotations{{Name: "` + f.Name +
			`", Args: []string{"x", "y"}}, {Name: "bare"}, {Name: "assigned", Args: []string{"x"}, Assigned: true}}`
	}
	return ""
}

// fillFacts prepares the views of the facts traversal on the emit
// side. It records the facts of each kind, the descent fields that lead
// to a kind with facts, and one child with a fact for the generated
// twin. It runs after viewOf, so the setter of a child can use the
// first fact of any kind.
func fillFacts(kinds []KindSpec, side string, views []view) {
	if side != EmitPackage {
		return
	}
	reach := factReach(kinds)
	first := map[string]factView{}
	for i, k := range kinds {
		v := &views[i]
		for _, f := range k.Fields {
			if f.Fact == "" || !f.Side.OnEmit() {
				continue
			}
			fv := fieldOf(f, side)
			v.Facts = append(v.Facts, factView{
				Fact:   f.Fact,
				Stated: factStated(f, fv),
				Set:    factSetter(f, fv, "subject", k.Name),
			})
			if _, held := first[k.Name]; !held {
				first[k.Name] = factView{
					Fact: f.Fact,
					Set:  factSetter(f, fv, "child", k.Name),
				}
			}
		}
		for _, f := range k.Fields {
			if !f.Walk || !f.Side.OnEmit() || (!f.IsSymbol && !reach[f.Elem]) {
				continue
			}
			v.FactWalked = append(v.FactWalked, fieldOf(f, side))
		}
	}
	for i, k := range kinds {
		views[i].FactChildren = factChildrenOf(k, first, side)
	}
}

// factReach computes the kinds that the facts traversal descends into.
// Such a kind states a fact or walks to a kind that does. A
// marker-typed walk field leads to such a kind by construction,
// because it admits any kind.
func factReach(kinds []KindSpec) map[string]bool {
	reach := map[string]bool{}
	for _, k := range kinds {
		for _, f := range k.Fields {
			if f.Fact != "" && f.Side.OnEmit() {
				reach[k.Name] = true
				break
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, k := range kinds {
			if reach[k.Name] {
				continue
			}
			for _, f := range k.Fields {
				if f.Walk && f.Side.OnEmit() && (f.IsSymbol || reach[f.Elem]) {
					reach[k.Name] = true
					changed = true
					break
				}
			}
		}
	}
	return reach
}

// factStated writes the expression that reports one fact as stated on
// the receiver of the traversal. Statedness follows the shape of the
// field. A bool is stated when true, a string when not empty, a shared
// vocabulary value when not zero, a reference when not nil, and a list
// when not empty. The several-returns fact alone needs more than one
// entry, because a single result is the ordinary case of every
// language. A shape outside the set renders text that gofmt refuses, so
// a mis-tagged field fails the generation and never walks wrong.
func factStated(field FieldSpec, f fieldView) string {
	switch {
	case field.Fact == multiReturnFact:
		return f.Len + " > 1"
	case field.Type == boolType:
		return f.selector()
	case field.Type == stringType:
		return f.selector() + ` != ""`
	case field.Type == annotationsType:
		return "len(" + f.selector() + ") > 0"
	case strings.HasPrefix(field.Type, symbolQualifier):
		return f.selector() + " != 0"
	case field.Slice:
		return f.Len + " > 0"
	case f.Pointer:
		return f.selector() + " != nil"
	}
	return "" // gofmt refuses the rendered file and names the fault
}

// factSetter writes the statement that states one fact on the given
// receiver in a test.
func factSetter(field FieldSpec, f fieldView, receiver, enclosing string) string {
	target := receiver + "." + f.Storage
	switch {
	case field.Fact == multiReturnFact && f.Accessor != "":
		child := addressOf + field.Elem + zeroLiteral
		return receiver + "." + f.Accessor + "()." + appendMethod + "(" + child + ", " + child + ")"
	case field.Fact == multiReturnFact:
		return target + " = " + qualify(field.Type) + "{{}, {}}"
	case field.Type == boolType:
		return target + " = true"
	case field.Type == stringType:
		return target + ` = "x"`
	case field.Type == annotationsType:
		return target + " = " + annotationsType + "{{}}"
	case strings.HasPrefix(field.Type, symbolQualifier):
		return target + " = 1"
	case field.Slice:
		if f.Accessor != "" {
			elem := field.Elem
			if elem == "" {
				elem = enclosing
			}
			return receiver + "." + f.Accessor + "()." + appendMethod +
				"(" + addressOf + elem + zeroLiteral + ")"
		}
		return target + " = " + qualify(field.Type) + "{{}}"
	case f.Pointer:
		return target + " = " + addressOf +
			strings.TrimPrefix(qualify(field.Type), pointerMarker) + zeroLiteral
	}
	return "" // gofmt refuses the rendered file and names the fault
}

// factChildrenOf writes the statements that give a subject one child
// with a fact, for every descent field whose element states a fact
// directly. It records what the traversal yields for that child. It
// skips a descent field that has a fact tag itself, because a value in
// it would state the fact of the subject beside the fact of the child.
//
// Every such field gets an entry, not only the first. Each is a
// recursion of its own in the generated traversal, and no test covers
// a descent that no case fills.
func factChildrenOf(k KindSpec, first map[string]factView, side string) []factChildView {
	var out []factChildView
	for _, f := range k.Fields {
		if !f.Walk || !f.Side.OnEmit() || f.Fact != "" {
			continue
		}
		elem := f.Elem
		if f.IsSymbol {
			elem = k.Name
		}
		fv, held := first[elem]
		if !held {
			continue
		}
		field := fieldOf(f, side)
		stmts := []string{
			"child := " + addressOf + elem + zeroLiteral,
			fv.Set,
		}
		target := "subject." + field.Storage
		switch {
		case field.Accessor != "":
			stmts = append(stmts,
				"subject."+field.Accessor+"()."+appendMethod+"(child)")
		case field.Slice:
			stmts = append(stmts, target+" = append("+target+", child)")
		default:
			stmts = append(stmts, target+" = child")
		}
		out = append(out, factChildView{
			Field: field.Name, Stmts: stmts, Kind: elem, Fact: fv.Fact,
		})
	}
	return out
}

// factsOf collects the suffixes of the fact constants that the schema
// declares, in the order of first use across the kinds in schema
// order. That order fixes the values of the generated constants.
func factsOf(kinds []KindSpec) []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range kinds {
		for _, f := range k.Fields {
			if f.Fact == "" || seen[f.Fact] {
				continue
			}
			seen[f.Fact] = true
			out = append(out, f.Fact)
		}
	}
	return out
}

// nameReach computes the kinds that the respell traversal descends
// into. Its first result contains every kind that declares a name or
// walks to a kind that does. named maps a kind to its name field. A
// marker-typed walk field leads to such a kind by construction,
// because it admits any kind.
func nameReach(
	kinds []KindSpec, side string,
) (reach map[string]bool, named map[string]string) {
	onSide := func(f FieldSpec) bool {
		if side == NodePackage {
			return f.Side.OnNode()
		}
		return f.Side.OnEmit()
	}
	named = map[string]string{}
	for _, k := range kinds {
		for _, f := range k.Fields {
			if f.IsName && onSide(f) {
				named[k.Name] = f.Name
				break
			}
		}
	}
	reach = map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, k := range kinds {
			if reach[k.Name] {
				continue
			}
			r := named[k.Name] != ""
			for _, f := range k.Fields {
				if r {
					break
				}
				if f.Walk && onSide(f) && (f.IsSymbol || reach[f.Elem]) {
					r = true
				}
			}
			if r {
				reach[k.Name] = true
				changed = true
			}
		}
	}
	return reach, named
}

// viewOf prepares one kind for one model side.
func viewOf(kind KindSpec, side string, reach map[string]bool, named map[string]string) view {
	v := view{
		Name:    kind.Name,
		Doc:     kind.Doc,
		Subject: kind.Subject,
		PosExpr: positionQualifier + "Pos{}",
		DocExpr: "nil",
	}

	byName := map[string]fieldView{}
	for _, field := range kind.Fields {
		if (side == NodePackage && !field.Side.OnNode()) ||
			(side == EmitPackage && !field.Side.OnEmit()) {

			continue
		}
		f := fieldOf(field, side)
		v.Fields = append(v.Fields, f)
		byName[f.Name] = f

		switch {
		case f.Name == posField:
			v.PosExpr, v.HasPos = "x."+f.Storage, true
		case f.Name == docField:
			v.DocExpr, v.HasDoc = "x."+f.Storage, true
		case f.Name == idField:
			v.IDStorage = f.Storage
		case f.Name == originField:
			v.OriginStorage = f.Storage
		case f.Name == typeField && f.Elem == typeRefKind:
			v.TypeRefStorage = f.Storage
		case f.Name == visField:
			v.VisStorage = f.Storage
		}
		if field.IsName {
			v.NameStorage = f.Storage
		}
		if field.Walk {
			v.Walked = append(v.Walked, f)
			if field.IsSymbol || reach[field.Elem] {
				v.NameWalked = append(v.NameWalked, f)
			}
		}
		if f.Accessor != "" {
			f.Child = childOf(f, kind.Name)
			v.Slots = append(v.Slots, f)
		}
		if side == EmitPackage && field.Type == BodyMarkerName {
			v.Bodies = append(v.Bodies, f.Storage)
		}
	}
	v.ZeroIdentity = side == NodePackage && v.IDStorage == ""
	v.Members = membersOf(byName)
	v.Children, v.WalkVisits = childrenOf(v)
	v.NameChildren, v.NameVisits = nameChildrenOf(v, named)
	v.Malformed = malformedOf(v)
	return v
}

// nameChildrenOf writes the statements that give a subject one named
// child per respell descent field. It counts the names that a
// traversal then visits, including the name of the subject. A marker
// field takes a named value of the enclosing kind, as [childrenOf]
// fills it. A descent field whose element declares no name contributes
// a bare child and no visit.
func nameChildrenOf(v view, named map[string]string) (stmts []string, visits int) {
	if v.NameStorage != "" {
		visits++
	}
	const subject = "subject"
	for _, f := range v.NameWalked {
		elem := f.Elem
		if elem == "" {
			if v.NameStorage == "" {
				continue
			}
			elem = v.Name
		}
		child := addressOf + elem + zeroLiteral
		if storage, has := named[elem]; has {
			child = addressOf + elem + `{` + storage + `: "beta"}`
			visits++
		}
		target := subject + "." + f.Storage
		switch {
		case f.Accessor != "":
			stmts = append(stmts,
				subject+"."+f.Accessor+"()."+appendMethod+"("+child+")")
		case f.Slice:
			stmts = append(stmts, target+" = append("+target+", "+child+")")
		default:
			stmts = append(stmts, target+" = "+child)
		}
	}
	return stmts, visits
}

// malformedOf writes an encoding of the kind whose first field has a
// JSON type that the field cannot decode. A decoder therefore enters
// the reader of the kind before it fails. A bad payload takes the same
// path.
func malformedOf(v view) string {
	if len(v.Fields) == 0 {
		return ""
	}
	field := v.Fields[0]
	wrong := `"nope"`
	if !strings.HasPrefix(field.JSONType, sliceMarker) && field.JSONType == stringType {
		wrong = "[]"
	}
	return `{"kind":"` + v.Name + `","` + field.JSONName + `":` + wrong + `}`
}

// childrenOf writes the statements that give a subject one child in
// every traversed field, and counts what a walk then visits.
//
// A child is empty, so it contributes exactly one visit. A field of
// the marker type takes a value of the enclosing kind. Such a value is
// always available, and it terminates for the same reason.
func childrenOf(v view) (stmts []string, visits int) {
	visits = 1
	const subject = "subject"
	for _, f := range v.Walked {
		child := childOf(f, v.Name)
		target := subject + "." + f.Storage
		switch {
		case f.Accessor != "":
			stmts = append(stmts,
				subject+"."+f.Accessor+"()."+appendMethod+"("+child+")")
		case f.Slice:
			stmts = append(stmts, target+" = append("+target+", "+child+")")
		default:
			stmts = append(stmts, target+" = "+child)
		}
		visits++
	}
	return stmts, visits
}

// childOf writes the expression that constructs one value that a field
// admits. The value is of the kind that the field references. A
// marker-typed field admits any kind, so it gets a value of the
// enclosing kind.
func childOf(f fieldView, enclosing string) string {
	elem := f.Elem
	if elem == "" {
		elem = enclosing
	}
	return addressOf + elem + zeroLiteral
}

// membersOf returns the Membered methods that a kind implements. It
// returns nil when the kind has no member list, so a kind that is not
// membered gets no methods.
func membersOf(byName map[string]fieldView) []memberView {
	members := make([]memberView, 0, len(memberMethods))
	found := false
	for _, method := range memberMethods {
		entry := memberView{Method: method.Method}
		for _, name := range method.Fields {
			if field, ok := byName[name]; ok {
				entry.Items, entry.Len = field.Items, field.Len
				found = true
				break
			}
		}
		members = append(members, entry)
	}
	if !found {
		return nil
	}
	return members
}

// fieldOf prepares one field for one model side.
func fieldOf(field FieldSpec, side string) fieldView {
	f := fieldView{
		Doc:     field.Doc,
		Comment: field.Comment,
		Name:    field.Name,
		Storage: field.Name,
		Decl:    qualify(field.Type),
		Elem:    field.Elem,
		Slice:   field.Slice,
		Pointer: strings.HasPrefix(field.Type, "*"),
	}
	f.JSONName = unexport(field.Name)
	switch {
	case field.IsSymbol && field.Slice:
		f.JSONType = symbolsType
	default:
		f.JSONType = qualify(field.Type)
	}
	f.Decl = f.JSONType
	f.Codec = codecOf(field)

	if side == EmitPackage && field.Slot != "" {
		element := strings.TrimPrefix(qualify(field.Type), sliceMarker)
		f.Accessor = field.Name + accessorSuffix
		f.Decl = slotType + "[" + element + "]"
		f.Items = f.selector() + "." + itemsMethod + "()"
		f.Len = f.selector() + "." + lenMethod + "()"
		return f
	}
	f.Items = f.selector()
	f.Len = "len(" + f.selector() + ")"
	return f
}

// selector returns the expression that reads the field from the
// receiver.
func (f fieldView) selector() string { return receiverName + "." + f.Storage }

// unexport lowers the leading run of capitals of a name, so an acronym
// remains one word. ID becomes id, and TypeParams becomes typeParams.
func unexport(name string) string {
	caps := 0
	for caps < len(name) && name[caps] >= 'A' && name[caps] <= 'Z' {
		caps++
	}
	switch {
	case caps == 0:
		return name
	case caps == len(name):
		return strings.ToLower(name)
	case caps == 1:
		return strings.ToLower(name[:1]) + name[1:]
	default:
		// The last capital opens the next word, so IDList becomes idList.
		return strings.ToLower(name[:caps-1]) + name[caps-1:]
	}
}

// qualify renders a schema type spelling as a model package writes it.
// The marker becomes the shared Symbol interface. A kind name remains
// bare, because the model declares it in the same package.
func qualify(spelling string) string {
	prefix, bare := "", spelling
	for {
		if rest, ok := strings.CutPrefix(bare, "[]"); ok {
			prefix, bare = prefix+"[]", rest
			continue
		}
		if rest, ok := strings.CutPrefix(bare, "*"); ok {
			prefix, bare = prefix+"*", rest
			continue
		}
		break
	}
	if bare == MarkerName {
		return prefix + symbolQualifier + MarkerName
	}
	return prefix + bare
}
