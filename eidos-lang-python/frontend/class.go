// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strconv"
	"strings"
	"unicode"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/spell"
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the bases, the keywords and the members that decide the
// kind and the members of a class, and the keywords of namedtuple of
// collections.
const (
	protocolName      = "Protocol"
	genericName       = "Generic"
	metaclassKeyword  = "metaclass"
	totalKeyword      = "total"
	falseLiteral      = "False"
	trueLiteral       = "True"
	ignoreName        = "_ignore_"
	nonmemberName     = "nonmember"
	castName          = "cast"
	fieldNamesKeyword = "field_names"
	renameKeyword     = "rename"
	defaultsKeyword   = "defaults"
)

// namedTupleBase is the identity of NamedTuple of typing, as typeshed
// declares the class. The class that namedtuple of collections declares
// extends it.
var namedTupleBase = symbol.Identity{Lang: Lang, Package: typingModule, Name: namedTupleName, Kind: symbol.KindStruct}

// The descriptions of the statements of a class that declare no member, and
// of a class that an enum declares, in the refusal of a carrier on them.
const (
	statementRefusal = "a statement"
	ignoreRefusal    = "the list _ignore_ of an enum"
	enumTypeRefusal  = "a class that an enum declares"
)

// enumClasses are the classes of enum whose subclass is an enum, and whose
// call declares one.
var enumClasses = map[string]bool{
	"Enum": true, "IntEnum": true, "StrEnum": true, "Flag": true, "IntFlag": true, "ReprEnum": true,
}

// enumMetaclasses are the metaclasses of enum that make a class an enum.
var enumMetaclasses = map[string]bool{"EnumMeta": true, "EnumType": true}

// enumUnderlying maps each class of enum whose members have a data type to
// the type.
var enumUnderlying = map[string]string{"IntEnum": "int", "IntFlag": "int", "StrEnum": "str"}

// classKind is the kind of declaration that a class statement loads as.
type classKind uint8

// The kinds of a class.
const (
	// structClass is a class that values are made of.
	structClass classKind = iota
	// protocolClass is a class with the base Protocol, an interface.
	protocolClass
	// typedDictClass is a class with the base TypedDict, an interface.
	typedDictClass
	// namedTupleClass is a class with the base NamedTuple, a struct of
	// immutable fields.
	namedTupleClass
	// enumClass is a class with a base of enum, an enum.
	enumClass
)

// methodBody is the body of one method of a class, whose assignments to the
// receiver declare fields: the block, the name of the receiver, and whether
// the method is __init__, whose every assignment to the receiver declares a
// field.
type methodBody struct {
	block treesitter.Node
	self  string
	init  bool
}

// classBody is the state of the lowering of the body of one class: the kind
// of the class and the totality of a TypedDict, the owner chain of its
// members, its members in the order of the walk, the names of its members
// for the check of a duplicate, the names that _ignore_ of an enum lists,
// the names of the type parameters of the class, and the bodies of the
// methods whose assignments to the receiver declare fields.
type classBody struct {
	kind      classKind
	total     bool
	owner     string
	fields    []*node.Field
	methods   []*node.Method
	types     []symbol.Symbol
	variants  []*node.EnumVariant
	declared  map[string]bool
	overloads map[string]bool
	accessors map[string]bool
	ignore    map[string]bool
	params    map[string]bool
	bodies    []methodBody
}

// newClassBody returns the empty body of a class of a kind, whose members
// have the owner chain owner, and which is total as a TypedDict. Each set of
// names of the body is nil until its first name.
func newClassBody(kind classKind, owner string) *classBody {
	return &classBody{kind: kind, owner: owner, total: true}
}

// declare records a name of a member, and reports false with a finding
// under [DuplicateName] for a name that the class declares already. The
// first declaration of a name is its declaration. The overloads of a name
// and the implementation after them, and the accessors of one property,
// declare one name.
func (c *classBody) declare(l *lowering, name string, at position.Pos, overload, accessor bool) bool {
	switch {
	case !c.declared[name]:
		setFlag(&c.declared, name, true)
		setFlag(&c.overloads, name, overload)
		setFlag(&c.accessors, name, accessor)
		return true
	case c.overloads[name] || accessor && c.accessors[name]:
		setFlag(&c.overloads, name, overload)
		return true
	default:
		l.u.Infof(DuplicateName, at, "the class declares %s a second time, and the first declaration is kept", name)
		return false
	}
}

// ownerOf returns the owner chain of the members of a class body, and the
// empty chain for a class at the top level of a module.
func ownerOf(host *classBody) string {
	if host == nil {
		return ""
	}
	return host.owner
}

// class lowers a class statement. The kind of the class follows from its
// direct bases, which the bindings of the module resolve: an interface for
// a Protocol and for a TypedDict, an enum for a class of enum, and a struct
// for every other class. The bases of the class are its supertypes in
// Extends, without Generic, Protocol and TypedDict, and the bases of an enum
// are the stamp python.bases. The keyword arguments of the statement are
// the stamp python.keywords. The type parameters are the parameters of the
// syntax of PEP 695, and the type variables of Generic[...] or Protocol[...]
// among the bases, or else the type variables that the bases name. host is
// the body of the class that declares the class, and nil at the top level of
// a module. A struct with an abstract method is abstract, and a class under
// @final is final.
func (l *lowering) class(def, outermost treesitter.Node, decorators []decorator, host *classBody) symbol.Symbol {
	nameNode := def.Child(l.v.fieldName)
	name := nameNode.Text()
	at := l.pos(nameNode)
	vis, claimed := l.claim(name, at, host)
	if !claimed {
		l.skip(outermost)
		return nil
	}
	if host != nil && host.kind == enumClass {
		l.u.Warnf(UnmodeledConstruct, at, "the enum declares the class %s, and an enum of the model has no types", name)
		l.refuse(outermost, enumTypeRefusal)
		l.refuseMarkers(decorators, enumTypeRefusal)
		return nil
	}
	carriers, comment := l.declParts(outermost, def)
	var bases, keywords []treesitter.Node
	for arg := range def.Child(l.v.fieldSuperclasses).Content() {
		switch arg.Kind() {
		case l.v.keywordArgument, l.v.listSplat, l.v.dictionarySplat:
			keywords = append(keywords, arg)
		default:
			bases = append(bases, arg)
		}
	}
	kind := l.classKind(bases, keywords)
	c := newClassBody(kind, joinOwner(ownerOf(host), name))
	var spelled []string
	for _, kw := range keywords {
		spelled = append(spelled, kw.Text())
		total := kw.Kind() == l.v.keywordArgument && kw.Child(l.v.fieldName).Text() == totalKeyword
		if total && kw.Child(l.v.fieldValue).Text() == falseLiteral {
			c.total = false
		}
	}
	typeParams := l.classParams(def, bases)
	for _, tp := range typeParams {
		setFlag(&c.params, tp.Name, true)
	}
	body := def.Child(l.v.fieldBody)
	doc, _ := l.docstring(body)
	l.classStatements(body, c)
	if kind != enumClass {
		l.selfFields(c)
	}
	decl := l.classDecl(def, c, bases, typeParams, vis, doc, comment)
	annotations := l.decorate(decl, decorators)
	switch d := decl.(type) {
	case *node.Struct:
		d.Annotations = annotations
	case *node.Interface:
		d.Annotations = annotations
	case *node.Enum:
		d.Annotations = annotations
	}
	l.classMarks(decl, c, bases, spelled, decorators)
	l.u.AttachCarriers(decl, carriers, BadCarrier)
	if host != nil {
		host.types = append(host.types, decl)
		return nil
	}
	if l.classKinds == nil {
		l.classKinds = map[string]classKind{}
	}
	l.classKinds[name] = kind
	return decl
}

// classKind returns the kind of a class from its direct bases and its
// keywords: a Protocol makes an interface, TypedDict a TypedDict, NamedTuple
// a named tuple, and a class or a metaclass of enum an enum. A class of the
// module that is an enum or a TypedDict makes its subclass one, and the
// kind of a base of another module is a struct, because the parse reads one
// module.
func (l *lowering) classKind(bases, keywords []treesitter.Node) classKind {
	for _, kw := range keywords {
		if kw.Kind() != l.v.keywordArgument || kw.Child(l.v.fieldName).Text() != metaclassKeyword {
			continue
		}
		module, name := l.scope.canonical(kw.Child(l.v.fieldValue).Compact())
		if module == enumModule && enumMetaclasses[name] {
			return enumClass
		}
	}
	for _, b := range bases {
		value, _ := l.generic(b)
		if value.IsZero() {
			value = b
		}
		module, name := l.scope.canonical(value.Compact())
		switch {
		case module == typingModule && name == protocolName:
			return protocolClass
		case module == typingModule && name == typedDictName:
			return typedDictClass
		case module == typingModule && name == namedTupleName:
			return namedTupleClass
		case module == enumModule && enumClasses[name]:
			return enumClass
		case module == l.at.pkg && (l.classKinds[name] == enumClass || l.classKinds[name] == typedDictClass):
			return l.classKinds[name]
		}
	}
	return structClass
}

// classParams returns the type parameters of a class: the parameters of
// the syntax of PEP 695, then the type variables of Generic[...] and of
// Protocol[...] among its bases, in their order, or, where neither states
// one, the type variables that the bases name, in the order of their first
// use.
func (l *lowering) classParams(def treesitter.Node, bases []treesitter.Node) []*node.TypeParam {
	params := l.typeParams(def.Child(l.v.fieldTypeParameters))
	taken := map[string]bool{}
	for _, tp := range params {
		taken[tp.Name] = true
	}
	var explicit, implicit []*node.TypeRef
	for _, b := range bases {
		value, args := l.generic(b)
		module, name := l.scope.canonical(value.Compact())
		if module == typingModule && (name == genericName || name == protocolName) {
			for _, a := range args {
				explicit = append(explicit, l.typeRef(a))
			}
			continue
		}
		implicit = append(implicit, l.typeRef(b))
	}
	if len(explicit) > 0 {
		return append(params, l.usedVars(explicit, taken)...)
	}
	return append(params, l.usedVars(implicit, taken)...)
}

// classDecl returns the declaration of a class of the kind of its body:
// an interface for a Protocol and for a TypedDict, an enum for an enum, and
// a struct for every other class, with the members of the body.
func (l *lowering) classDecl(def treesitter.Node, c *classBody, bases []treesitter.Node, params []*node.TypeParam,
	vis symbol.Visibility, doc []string, comment string,
) symbol.Symbol {
	nameNode := def.Child(l.v.fieldName)
	name, at := nameNode.Text(), l.pos(nameNode)
	if c.kind == enumClass {
		return &node.Enum{
			Name: name, Pos: at, Doc: doc, Comment: comment, Visibility: vis, Underlying: l.underlying(bases),
			Variants: c.variants, Fields: c.fields, Methods: c.methods,
		}
	}
	var extends []*node.TypeRef
	for _, b := range bases {
		value, _ := l.generic(b)
		if value.IsZero() {
			value = b
		}
		module, base := l.scope.canonical(value.Compact())
		dropped := module == typingModule && (base == genericName ||
			c.kind == protocolClass && base == protocolName || c.kind == typedDictClass && base == typedDictName)
		if !dropped {
			extends = append(extends, l.typeRef(b))
		}
	}
	if c.kind == protocolClass || c.kind == typedDictClass {
		return &node.Interface{
			Name: name, Pos: at, Doc: doc, Comment: comment, Visibility: vis, TypeParams: params,
			Fields: c.fields, Methods: c.methods, Types: c.types, Extends: extends,
		}
	}
	st := &node.Struct{
		Name: name, Pos: at, Doc: doc, Comment: comment, Visibility: vis, TypeParams: params,
		Fields: c.fields, Methods: c.methods, Types: c.types, Extends: extends,
	}
	for _, m := range c.methods {
		st.Abstract = st.Abstract || m.Abstract
	}
	return st
}

// classMarks stamps what a class states and the model has no field for:
// python.typedDict on a TypedDict, python.bases with the bases of an enum as
// written, and python.keywords with the keyword arguments of the class
// statement. A class under @final of typing is final.
func (l *lowering) classMarks(decl symbol.Symbol, c *classBody, bases []treesitter.Node, keywords []string,
	decorators []decorator,
) {
	gb := l.u.Graph()
	if st, is := decl.(*node.Struct); is {
		st.Final = decoratedBy(decorators, typingModule, finalName)
	}
	at := position.Pos{}
	if len(bases) > 0 {
		at = l.pos(bases[0])
	}
	if c.kind == typedDictClass {
		gb.Stamp(decl, meta.RawStamp{Key: python.TypedDictKey, Value: true, Pos: at})
	}
	if c.kind == enumClass && len(bases) > 0 {
		spelled := make([]string, 0, len(bases))
		for _, b := range bases {
			spelled = append(spelled, b.Compact())
		}
		gb.Stamp(decl, meta.RawStamp{Key: python.BasesKey, Value: spelled, Pos: at})
	}
	if len(keywords) > 0 {
		gb.Stamp(decl, meta.RawStamp{Key: python.KeywordsKey, Value: keywords, Pos: at})
	}
}

// underlying returns the data type of the members of an enum: int for
// IntEnum and IntFlag, str for StrEnum, and the mixin type T of
// class E(T, Enum). An enum without a data type has none.
func (l *lowering) underlying(bases []treesitter.Node) *node.TypeRef {
	var mixin *node.TypeRef
	for _, b := range bases {
		module, name := l.scope.canonical(b.Compact())
		if t, typed := enumUnderlying[name]; typed && module == enumModule {
			return &node.TypeRef{Spelling: t, Pos: l.pos(b)}
		}
		if module == enumModule && enumClasses[name] || module == l.at.pkg && l.classKinds[name] == enumClass {
			continue
		}
		if mixin == nil {
			mixin = l.typeRef(b)
		}
	}
	return mixin
}

// classStatements lowers the statements of one block of the body of a
// class into its members: the methods and the nested classes, the
// attributes, and the blocks of the compound statements that the checks of
// conditions select. A statement that declares nothing refuses its
// carriers.
func (l *lowering) classStatements(list treesitter.Node, c *classBody) {
	for stmt := range list.Content() {
		switch stmt.Kind() {
		case l.v.classDefinition, l.v.functionDefinition, l.v.decoratedDefinition:
			l.definition(stmt, c)
		case l.v.assignment:
			l.classAssignment(stmt, c)
		case l.v.ifStatement:
			l.branches(stmt, func(block treesitter.Node) { l.classStatements(block, c) })
		case l.v.tryStatement, l.v.withStatement, l.v.forStatement, l.v.whileStatement, l.v.matchStatement:
			l.allBodies(stmt, func(block treesitter.Node) { l.classStatements(block, c) })
		case l.v.stringNode, l.v.concatenatedString, l.v.passStatement, l.v.ellipsis:
		default:
			l.refuse(stmt, statementRefusal)
		}
	}
}

// classAssignment lowers an assignment of the body of a class. In an enum,
// each attribute that the typing specification counts as a member is a
// variant with its value as written, and _ignore_ lists the names that are
// no members. Every other attribute is a field: a field of its annotation,
// at type level under ClassVar, immutable under Final, ReadOnly and in a
// named tuple, and optional in a TypedDict under NotRequired and, without
// Required, under total=False. An attribute without an annotation is
// shared by the instances, a field at type level without a type. The value
// of an attribute of an enum that nonmember wraps is the argument of
// nonmember, and a cast inside it states the type of the field. A field of
// InitVar is stamped python.initVar. A name annotated TypeAlias, and the
// implicit alias of [lowering.implicitAlias] outside an enum, is an alias
// that the class declares.
func (l *lowering) classAssignment(stmt treesitter.Node, c *classBody) {
	typed := stmt.Child(l.v.fieldType)
	var buf [nodeBuffer]treesitter.Node
	targets, value := l.targets(stmt, buf[:0])
	one := len(targets) == 1 && targets[0].Kind() == l.v.identifier
	if c.kind == enumClass && one && (typed.IsZero() || !value.IsZero()) {
		name := targets[0].Text()
		if name == ignoreName {
			l.ignored(value, c)
			l.refuse(stmt, ignoreRefusal)
			return
		}
		if l.enumMember(name, value, c) {
			l.variant(targets[0], value, stmt, c)
			return
		}
	}
	var a annotation
	if !typed.IsZero() {
		a = l.annotationOf(typed)
	} else if text, at := l.typeComment(stmt); text != "" {
		l.parseAt(text, at, func(t treesitter.Node) { a.ref = l.typeRef(t) })
	}
	if inner, cast, wrapped := l.nonmember(value); c.kind == enumClass && one && wrapped {
		value = inner
		if a.ref == nil && !cast.IsZero() {
			a.ref = l.typeRef(cast)
		}
	}
	if one && (a.typeAlias || c.kind != enumClass && l.implicitAlias(stmt, typed, value)) {
		if decl := l.alias(targets[0], treesitter.Node{}, value, stmt, false, c); decl != nil {
			c.types = append(c.types, decl)
		}
		return
	}
	carriers, comment := l.declParts(stmt, stmt)
	doc := l.attributeDoc(stmt)
	var first *node.Field
	var spelled [nodeBuffer]treesitter.Node
	for _, target := range targets {
		names, single := l.names(target, spelled[:0])
		for _, name := range names {
			text := name.Text()
			if !c.declare(l, text, l.pos(name), false, false) {
				continue
			}
			vis := memberVisibility(text)
			if l.signatures && vis != symbol.VisibilityPublic {
				continue
			}
			f := &node.Field{
				Name: text, Pos: l.pos(name), Doc: doc, Comment: comment, Visibility: vis,
				Hard: vis == symbol.VisibilityPrivate, Mutability: symbol.MutabilityMutable, Type: a.ref,
			}
			if first != nil {
				f.Type = node.CloneRef(a.ref)
			}
			if single {
				f.Value = value.Text()
			}
			l.qualify(f, a, typed.IsZero(), c)
			c.fields = append(c.fields, f)
			if first == nil {
				first = f
			}
		}
	}
	if first == nil {
		l.u.RefuseCarriers(UnaddressedCarrier, carriers, noNameRefusal)
		return
	}
	l.u.AttachCarriers(first, carriers, BadCarrier)
}

// qualify sets the fields of a field that the qualifiers of its annotation
// and the kind of its class state, and stamps what the model has no field
// for.
func (l *lowering) qualify(f *node.Field, a annotation, unannotated bool, c *classBody) {
	switch c.kind {
	case typedDictClass:
		f.Optional = a.notRequired || !c.total && !a.required
	case namedTupleClass:
		f.Mutability = symbol.MutabilityImmutable
	case structClass, protocolClass, enumClass:
	}
	if a.final || a.readOnly {
		f.Mutability = symbol.MutabilityImmutable
	}
	if a.classVar || unannotated && c.kind != typedDictClass && c.kind != namedTupleClass {
		f.Level = symbol.LevelType
	}
	if a.initVar {
		l.u.Graph().Stamp(f, meta.RawStamp{Key: python.InitVarKey, Value: true, Pos: a.pos})
	}
	l.annotatedStamp(f, a)
}

// enumMember reports whether an attribute of an enum is a member, as the
// typing specification counts the members of an enum: a name that is no
// dunder, no sunder and no private name, that _ignore_ does not list, and
// whose value is neither nonmember(...), a lambda nor absent.
func (l *lowering) enumMember(name string, value treesitter.Node, c *classBody) bool {
	sunder := len(name) > 2 && strings.HasPrefix(name, privateMark) && strings.HasSuffix(name, privateMark)
	special := dunder(name) || sunder || strings.HasPrefix(name, dunderMark)
	if value.IsZero() || special || c.ignore[name] || value.Kind() == l.v.lambda {
		return false
	}
	if value.Kind() == l.v.call {
		module, callee := l.scope.canonical(value.Child(l.v.fieldFunction).Compact())
		return module != enumModule || callee != nonmemberName
	}
	return true
}

// nonmember returns the value that nonmember of enum wraps, and the type
// that a cast of typing states for it: the argument of nonmember(value), and
// the value and the type of nonmember(cast("type", value)), which is how the
// backend writes an attribute of an enum with a value. It reports false for
// any other value.
func (l *lowering) nonmember(value treesitter.Node) (inner, cast treesitter.Node, wrapped bool) {
	if value.Kind() != l.v.call {
		return treesitter.Node{}, treesitter.Node{}, false
	}
	module, callee := l.scope.canonical(value.Child(l.v.fieldFunction).Compact())
	args := slices.Collect(value.Child(l.v.fieldArguments).Content())
	if module != enumModule || callee != nonmemberName || len(args) != 1 {
		return treesitter.Node{}, treesitter.Node{}, false
	}
	if args[0].Kind() == l.v.call {
		module, callee = l.scope.canonical(args[0].Child(l.v.fieldFunction).Compact())
		casted := slices.Collect(args[0].Child(l.v.fieldArguments).Content())
		if module == typingModule && callee == castName && len(casted) == 2 {
			return casted[1], casted[0], true
		}
	}
	return args[0], treesitter.Node{}, true
}

// ignored records the names that the value of _ignore_ lists: the strings
// of a list or a tuple, and the names of a string separated by spaces.
func (l *lowering) ignored(value treesitter.Node, c *classBody) {
	if text, is := l.stringValue(value); is {
		for name := range strings.FieldsSeq(text) {
			setFlag(&c.ignore, name, true)
		}
		return
	}
	for e := range value.Content() {
		if name, is := l.stringValue(e); is {
			setFlag(&c.ignore, name, true)
		}
	}
}

// variant lowers a member of an enum to a variant, with its value as
// written, the docstring of the attribute, the trailing comment of the
// assignment and its carriers.
func (l *lowering) variant(nameNode, value, stmt treesitter.Node, c *classBody) {
	name := nameNode.Text()
	if !c.declare(l, name, l.pos(nameNode), false, false) {
		l.skip(stmt)
		return
	}
	carriers, comment := l.declParts(stmt, stmt)
	v := &node.EnumVariant{
		Name: name, Pos: l.pos(nameNode), Doc: l.attributeDoc(stmt), Comment: comment, Value: value.Text(),
	}
	c.variants = append(c.variants, v)
	l.u.AttachCarriers(v, carriers, BadCarrier)
}

// selfFields lowers the fields that the methods of a class declare through
// their receivers: each annotated assignment to self.<name> in the body of
// a method, and every assignment to self.<name> in the body of __init__,
// for each name that the body of the class does not declare. The fields
// follow the fields of the class body, in the order of the methods.
func (l *lowering) selfFields(c *classBody) {
	for _, b := range c.bodies {
		l.selfAssignments(b.block, b, c)
	}
}

// selfAssignments lowers the assignments to the receiver of one block of a
// method, and of the blocks of its compound statements, outside a callable
// and a class nested in it.
func (l *lowering) selfAssignments(block treesitter.Node, b methodBody, c *classBody) {
	for stmt := range block.Content() {
		switch stmt.Kind() {
		case l.v.assignment:
			left, typed := stmt.Child(l.v.fieldLeft), stmt.Child(l.v.fieldType)
			receiver := left.Kind() == l.v.attribute && left.Child(l.v.fieldObject).Text() == b.self
			if !receiver || !b.init && typed.IsZero() {
				continue
			}
			attr := left.Child(l.v.fieldAttribute)
			name := attr.Text()
			if c.declared[name] {
				continue
			}
			setFlag(&c.declared, name, true)
			carriers, comment := l.declParts(stmt, stmt)
			vis := memberVisibility(name)
			f := &node.Field{
				Name: name, Pos: l.pos(attr), Doc: l.attributeDoc(stmt), Comment: comment, Visibility: vis,
				Hard: vis == symbol.VisibilityPrivate, Mutability: symbol.MutabilityMutable,
			}
			if !typed.IsZero() {
				a := l.annotationOf(typed)
				f.Type = a.ref
				l.qualify(f, a, false, c)
			}
			c.fields = append(c.fields, f)
			l.u.AttachCarriers(f, carriers, BadCarrier)
		case l.v.ifStatement, l.v.tryStatement, l.v.withStatement, l.v.forStatement, l.v.whileStatement,
			l.v.matchStatement:
			l.allBodies(stmt, func(inner treesitter.Node) { l.selfAssignments(inner, b, c) })
		}
	}
}

// allBodies calls each with every block of a compound statement in order:
// the branches of an if statement, the clauses of a try statement, the body
// and the else of a loop, the body of a with statement and the case
// clauses of a match statement. The block of a match statement contains its
// case clauses.
func (l *lowering) allBodies(stmt treesitter.Node, each func(treesitter.Node)) {
	for child := range stmt.NamedChildren() {
		switch child.Kind() {
		case l.v.block:
			if stmt.Kind() == l.v.matchStatement {
				l.allBodies(child, each)
				continue
			}
			each(child)
		case l.v.elifClause, l.v.elseClause, l.v.exceptClause, l.v.finallyClause, l.v.caseClause:
			l.allBodies(child, each)
		}
	}
}

// functionalClass lowers the functional form of a class: NamedTuple with a
// list of pairs of a name and a type, or with keywords of the names; a
// TypedDict with a dictionary of the names and their types; a class of
// enum with the names of its members as a string, a list of names, a list
// of pairs of a name and a value, or a dictionary; and namedtuple of
// collections with the names of its fields, its rename and its defaults.
// The declaration is the declaration of the class syntax, with the stamps
// of that syntax. A named tuple of collections extends NamedTuple of
// typing, which its call does not name.
func (l *lowering) functionalClass(nameNode, call, stmt treesitter.Node, callee string) symbol.Symbol {
	name := nameNode.Text()
	vis, claimed := l.claim(name, l.pos(nameNode), nil)
	if !claimed {
		l.skip(stmt)
		return nil
	}
	carriers, comment := l.declParts(stmt, stmt)
	doc := l.attributeDoc(stmt)
	var positional, keywords []treesitter.Node
	for arg := range call.Child(l.v.fieldArguments).Content() {
		if arg.Kind() == l.v.keywordArgument {
			keywords = append(keywords, arg)
			continue
		}
		positional = append(positional, arg)
	}
	var members treesitter.Node
	if len(positional) > 1 {
		members = positional[1]
	}
	c := newClassBody(structClass, name)
	var spelled []string
	for _, kw := range keywords {
		spelled = append(spelled, kw.Text())
		if kw.Child(l.v.fieldName).Text() == totalKeyword && kw.Child(l.v.fieldValue).Text() == falseLiteral {
			c.total = false
		}
	}
	function := call.Child(l.v.fieldFunction)
	at := l.pos(nameNode)
	var decl symbol.Symbol
	switch callee {
	case namedTupleName:
		c.kind = namedTupleClass
		l.pairs(members, c)
		if members.IsZero() {
			for _, kw := range keywords {
				l.member(kw.Child(l.v.fieldName), kw.Child(l.v.fieldValue), c)
			}
			spelled = nil
		}
		decl = &node.Struct{
			Name: name, Pos: at, Doc: doc, Comment: comment, Visibility: vis, Fields: c.fields,
			Extends: []*node.TypeRef{l.named(function, function.Compact())},
		}
	case namedtupleCall:
		c.kind = namedTupleClass
		rename, defaults := false, treesitter.Node{}
		for _, kw := range keywords {
			switch value := kw.Child(l.v.fieldValue); kw.Child(l.v.fieldName).Text() {
			case fieldNamesKeyword:
				members = value
			case renameKeyword:
				rename = value.Text() == trueLiteral
			case defaultsKeyword:
				defaults = value
			}
		}
		l.tupleFields(members, rename, c)
		l.tupleDefaults(defaults, c)
		spelled = nil
		base := &node.TypeRef{
			Spelling: namedTupleName,
			Pos:      l.pos(function),
			Package:  typingModule,
			Target:   namedTupleBase,
		}
		decl = &node.Struct{
			Name: name, Pos: at, Doc: doc, Comment: comment, Visibility: vis, Fields: c.fields,
			Extends: []*node.TypeRef{base},
		}
	case typedDictName:
		c.kind = typedDictClass
		for pair := range members.Content() {
			if pair.Kind() == l.v.pair {
				l.member(pair.Child(l.v.fieldKey), pair.Child(l.v.fieldValue), c)
			}
		}
		decl = &node.Interface{Name: name, Pos: at, Doc: doc, Comment: comment, Visibility: vis, Fields: c.fields}
		l.u.Graph().Stamp(decl, meta.RawStamp{Key: python.TypedDictKey, Value: true, Pos: l.pos(call)})
	default:
		l.enumMembers(members, c)
		e := &node.Enum{Name: name, Pos: at, Doc: doc, Comment: comment, Visibility: vis, Variants: c.variants}
		if t, typed := enumUnderlying[callee]; typed {
			e.Underlying = &node.TypeRef{Spelling: t, Pos: l.pos(function)}
		}
		decl = e
		bases := []string{function.Compact()}
		l.u.Graph().Stamp(decl, meta.RawStamp{Key: python.BasesKey, Value: bases, Pos: l.pos(call)})
	}
	if len(spelled) > 0 {
		l.u.Graph().Stamp(decl, meta.RawStamp{Key: python.KeywordsKey, Value: spelled, Pos: l.pos(call)})
	}
	l.u.AttachCarriers(decl, carriers, BadCarrier)
	return decl
}

// pairs lowers the fields of a list of pairs of a name and a type, as the
// functional form of NamedTuple states them.
func (l *lowering) pairs(list treesitter.Node, c *classBody) {
	for pair := range list.Content() {
		parts := make([]treesitter.Node, 0, 2)
		for part := range pair.Content() {
			parts = append(parts, part)
		}
		if pair.Kind() == l.v.tuple && len(parts) == 2 {
			l.member(parts[0], parts[1], c)
		}
	}
}

// tupleFields lowers the fields of namedtuple of collections from the names
// of a string, separated by spaces or commas, or from the strings of a list
// or a tuple. A field is immutable and has no type. Under rename, a name
// that is no identifier, a keyword, a name with a leading underscore and a
// name that an earlier field has take an underscore and the index of the
// field, as namedtuple renames them.
func (l *lowering) tupleFields(names treesitter.Node, rename bool, c *classBody) {
	text, joined := l.stringValue(names)
	var spelled []string
	var nodes []treesitter.Node
	if joined {
		spelled = strings.FieldsFunc(text, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	} else {
		for e := range names.Content() {
			if name, is := l.stringValue(e); is {
				spelled, nodes = append(spelled, name), append(nodes, e)
			}
		}
	}
	for i, name := range spelled {
		at := l.pos(names)
		if !joined {
			at = l.pos(nodes[i])
		}
		invalid := !spell.IsIdentifier(name) || spell.IsKeyword(name) || strings.HasPrefix(name, privateMark)
		if rename && (invalid || c.declared[name]) {
			name = privateMark + strconv.Itoa(i)
		}
		if !c.declare(l, name, at, false, false) {
			continue
		}
		f := &node.Field{Name: name, Pos: at, Visibility: memberVisibility(name), Mutability: symbol.MutabilityMutable}
		l.qualify(f, annotation{}, true, c)
		c.fields = append(c.fields, f)
	}
}

// tupleDefaults sets the values of a list or a tuple of the defaults of
// namedtuple of collections on the last fields, one value each in order. A
// value that is no literal list or tuple, and a list of more values than
// fields, which namedtuple refuses, set none.
func (l *lowering) tupleDefaults(defaults treesitter.Node, c *classBody) {
	values := slices.Collect(defaults.Content())
	if k := defaults.Kind(); k != l.v.list && k != l.v.tuple || len(values) > len(c.fields) {
		return
	}
	for i, v := range values {
		c.fields[len(c.fields)-len(values)+i].Value = v.Text()
	}
}

// member lowers one field of a functional form from the node of its name,
// a string or a name, and the node of its type.
func (l *lowering) member(nameNode, typed treesitter.Node, c *classBody) {
	name, quoted := l.stringValue(nameNode)
	if !quoted {
		name = nameNode.Text()
	}
	if !c.declare(l, name, l.pos(nameNode), false, false) {
		return
	}
	a := l.annotationOf(typed)
	f := &node.Field{
		Name: name, Pos: l.pos(nameNode), Visibility: memberVisibility(name), Mutability: symbol.MutabilityMutable,
		Type: a.ref,
	}
	l.qualify(f, a, false, c)
	c.fields = append(c.fields, f)
}

// enumMembers lowers the variants of the functional form of an enum: the
// names of a string, separated by spaces or commas, the strings of a list
// or a tuple, the pairs of a name and a value of a list or a tuple, and the
// pairs of a dictionary. A variant of a pair has the value as written.
func (l *lowering) enumMembers(members treesitter.Node, c *classBody) {
	if text, is := l.stringValue(members); is {
		for _, name := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
			c.variants = append(c.variants, &node.EnumVariant{Name: name, Pos: l.pos(members)})
		}
		return
	}
	for e := range members.Content() {
		var nameNode, value treesitter.Node
		switch e.Kind() {
		case l.v.pair:
			nameNode, value = e.Child(l.v.fieldKey), e.Child(l.v.fieldValue)
		case l.v.tuple:
			for part := range e.Content() {
				if nameNode.IsZero() {
					nameNode = part
					continue
				}
				value = part
			}
		default:
			nameNode = e
		}
		if name, is := l.stringValue(nameNode); is {
			c.variants = append(c.variants, &node.EnumVariant{Name: name, Pos: l.pos(nameNode), Value: value.Text()})
		}
	}
}
