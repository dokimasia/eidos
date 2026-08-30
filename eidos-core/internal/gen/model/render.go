// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package model

import "strings"

// The packages the generator writes into.
const (
	// SymbolPackage holds the Kind constants.
	SymbolPackage = "symbol"
	// NodePackage is the model a frontend produces.
	NodePackage = "node"
	// EmitPackage is the model a generator produces.
	EmitPackage = "emit"
)

// The qualifiers a generated model file reaches other packages
// through.
const (
	symbolQualifier   = SymbolPackage + "."
	positionQualifier = "position."
)

// The spellings rendering composes generated expressions from.
const (
	// receiverName is the receiver every generated method uses.
	receiverName = "x"
	// accessorSuffix names a slot's exported accessor.
	accessorSuffix = "Slot"
	// shadowSuffix names the struct a codec encodes through.
	shadowSuffix = "JSON"
	// slotType is the storage a slot-tagged field declares.
	slotType = "Slot"
	// itemsMethod, lenMethod and appendMethod are the slot's own
	// surface, which generated code reaches through.
	itemsMethod  = "Items"
	lenMethod    = "Len"
	appendMethod = "Append"
	// symbolsType is the named slice a marker-typed field declares.
	// It carries the codec that reads a child's kind before it
	// allocates one, which a plain interface slice cannot.
	symbolsType = "Symbols"
	// sliceMarker and pointerMarker open a Go type spelling.
	sliceMarker   = "[]"
	pointerMarker = "*"
	// zeroLiteral constructs an empty value of a kind, and addressOf
	// takes its address.
	zeroLiteral = "{}"
	addressOf   = "&"
)

// Field names the models treat by convention rather than by tag.
// Lowering carries them through and rendering recognizes them.
const (
	// docField holds a declaration's documentation.
	docField = "Doc"
	// posField holds a node symbol's source position.
	posField = "Pos"
	// idField holds a node declaration's canonical identity, which is
	// what makes the kind answer the Declaration interface.
	idField = "ID"
	// originField holds an emit value's node identity, which is what
	// the generated OriginOf answers.
	originField = "Origin"
	// typeField holds a declaration's own type reference, which is
	// what makes a kind satisfy the Typed interface.
	typeField = "Type"
	// typeRefKind is the kind such a field references.
	typeRefKind = "TypeRef"
)

// memberMethods are the Membered interface's methods, against the
// field names that answer each one.
var memberMethods = []struct {
	Method string
	Fields []string
}{
	{Method: "FieldList", Fields: []string{"Fields", "Variants"}},
	{Method: "MethodList", Fields: []string{"Methods"}},
	{Method: "EmbedList", Fields: []string{"Embeds"}},
}

// view is one kind prepared for a template.
//
// Every spelling a template needs is computed here, so the
// templates range over data and decide nothing. The lowering stays
// free of layout concerns, and a template never asks which side it
// is on.
type view struct {
	Name string
	// Shadow is the unexported struct the codec encodes through.
	Shadow string
	// Doc is the schema's documentation for the kind.
	Doc []string
	// Fields are the struct's fields, in declaration order.
	Fields []fieldView
	// PosExpr and DocExpr answer the Symbol interface, and HasPos
	// and HasDoc say whether the kind carries the field they read.
	PosExpr string
	DocExpr string
	HasPos  bool
	HasDoc  bool
	// Members answer the Membered interface. The slice is empty for
	// a kind that carries no member list, and an entry with an
	// empty Items answers nil.
	Members []memberView
	// TypeRefStorage is the field answering the Typed interface,
	// empty when the kind is not typed.
	TypeRefStorage string
	// IDStorage is the field answering the identity accessor, empty
	// on the emit side, which carries an origin instead.
	IDStorage string
	// OriginStorage is the field answering OriginOf, empty on the
	// node side and on emit kinds that derive from nothing.
	OriginStorage string
	// Walked are the fields the traversal descends into.
	Walked []fieldView
	// Slots are the fields that become slot storage.
	Slots []fieldView
	// Children are the statements a test uses to give the subject
	// one child in every traversed field.
	Children []string
	// WalkVisits is how many declarations a walk over that subject
	// reaches, the subject included.
	WalkVisits int
	// Malformed is an encoding naming this kind whose first field
	// holds the wrong JSON type, so a decoder places the kind and
	// then fails on its body.
	Malformed string
}

// IsMembered reports whether the kind carries any member list.
func (v view) IsMembered() bool { return len(v.Members) > 0 }

// memberView is one Membered method and the expressions that answer
// it.
type memberView struct {
	Method string
	// Items ranges over the members, and Len sizes the result.
	// Both are empty when the kind has no field for this method.
	Items string
	Len   string
}

// fieldView is one field as one model side carries it.
type fieldView struct {
	// Doc and Comment are the schema's documentation for the field.
	Doc     []string
	Comment string
	// Name is the schema's field name; Storage is how the struct
	// declares it, which differs for a slot.
	Name    string
	Storage string
	// Decl is the type the struct declares.
	Decl string
	// Elem is the referenced kind, empty when the field references
	// none.
	Elem string
	// Items ranges over the field's values, and Len sizes them.
	Items string
	Len   string
	// Slice and Pointer describe the shape the traversal walks.
	Slice   bool
	Pointer bool
	// Accessor is the exported slot method, empty when the field is
	// not a slot.
	Accessor string
	// JSONName is the field's key in encoded form.
	JSONName string
	// JSONType is the type the codec's shadow struct declares. A
	// field typed by the marker becomes raw bytes, because its
	// concrete kind is only known from the encoded discriminator.
	JSONType string
	// Raw says the codec dispatches this field through the
	// kind-discriminated encoder rather than encoding it directly.
	Raw bool
}

// viewsFor prepares every kind for one model side.
//
// side is [NodePackage] or [EmitPackage]. Slot storage exists on the
// emit side alone, so a slot-tagged field renders as an ordinary
// slice on the node side.
func viewsFor(kinds []KindSpec, side string) []view {
	out := make([]view, 0, len(kinds))
	for _, kind := range kinds {
		out = append(out, viewOf(kind, side))
	}
	return out
}

// viewOf prepares one kind for one model side.
func viewOf(kind KindSpec, side string) view {
	v := view{
		Name:    kind.Name,
		Shadow:  unexport(kind.Name) + shadowSuffix,
		Doc:     kind.Doc,
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
		}
		if field.Walk {
			v.Walked = append(v.Walked, f)
		}
		if f.Accessor != "" {
			v.Slots = append(v.Slots, f)
		}
	}
	v.Members = membersOf(byName)
	v.Children, v.WalkVisits = childrenOf(v)
	v.Malformed = malformedOf(v)
	return v
}

// malformedOf writes an encoding that names the kind and then
// breaks: the first field carries a JSON type it cannot hold. A
// decoder therefore reaches the kind's own reader before it fails,
// which is the path a bad payload takes.
func malformedOf(v view) string {
	if len(v.Fields) == 0 {
		return ""
	}
	field := v.Fields[0]
	wrong := `"nope"`
	if !strings.HasPrefix(field.JSONType, sliceMarker) && field.JSONType == "string" {
		wrong = "[]"
	}
	return `{"kind":"` + v.Name + `","` + field.JSONName + `":` + wrong + `}`
}

// childrenOf writes the statements that give a subject one child in
// every traversed field, and counts what a walk then reaches.
//
// A child is empty, so it contributes exactly one visit. A field
// typed by the marker takes a value of the enclosing kind, which is
// always available and terminates for the same reason.
func childrenOf(v view) (stmts []string, visits int) {
	visits = 1
	const subject = "subject"
	for _, f := range v.Walked {
		named := v.Name
		if f.Elem != "" {
			named = f.Elem
		}
		child := addressOf + named + zeroLiteral
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

// membersOf answers the Membered methods a kind implements. It
// answers nothing when the kind carries no member list at all, so a
// kind that is not membered grows no methods.
func membersOf(byName map[string]fieldView) []memberView {
	var members []memberView
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
	f.Raw = field.IsSymbol
	switch {
	case field.IsSymbol && field.Slice:
		f.JSONType = symbolsType
	default:
		f.JSONType = qualify(field.Type)
	}
	f.Decl = f.JSONType

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

// selector is the field as generated code reads it off the
// receiver.
func (f fieldView) selector() string { return receiverName + "." + f.Storage }

// unexport lowers a name's leading run of capitals, so an acronym
// stays one word: ID becomes id, and TypeParams becomes typeParams.
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
		// The last capital opens the next word: IDList becomes idList.
		return strings.ToLower(name[:caps-1]) + name[caps-1:]
	}
}

// qualify renders a schema type spelling as a model package spells
// it: the marker becomes the shared Symbol interface, and a kind
// name stays bare because the model declares it in the same
// package.
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
