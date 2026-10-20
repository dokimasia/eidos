// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the forms that an assignment declares through: the
// annotation of an explicit alias, the calls of typing that declare a
// distinct type, a named tuple and a TypedDict, and the call of collections
// that declares a named tuple.
const (
	typeAliasName  = "TypeAlias"
	newTypeName    = "NewType"
	namedTupleName = "NamedTuple"
	typedDictName  = "TypedDict"
	namedtupleCall = "namedtuple"
)

// The descriptions of an assignment that declares no name, and of the
// assignment of __all__, in the refusal of a carrier on it.
const (
	noNameRefusal = "an assignment that declares no name"
	allRefusal    = "the list __all__"
)

// nodeBuffer is the number of the targets of an assignment, and of the
// names of one target, that the lowering collects on the stack. A longer
// chain of targets grows its list on the heap, and so does every tuple of
// targets.
const nodeBuffer = 4

// moduleAssignment lowers an assignment at the top level of a module, and
// appends the declarations that it binds to the declarations of the file.
// __all__ is the exports of the file and declares nothing. A name annotated
// TypeAlias is an alias of its value, and so is the implicit alias of
// [lowering.implicitAlias]. A call of TypeVar, ParamSpec, TypeVarTuple,
// NewType, NamedTuple, TypedDict or a class of enum declares what the call
// declares. Every other assignment binds variables, or constants under
// Final.
func (l *lowering) moduleAssignment(stmt treesitter.Node) {
	typed := stmt.Child(l.v.fieldType)
	var buf [nodeBuffer]treesitter.Node
	targets, value := l.targets(stmt, buf[:0])
	single := len(targets) == 1 && targets[0].Kind() == l.v.identifier
	if single && string(targets[0].Bytes()) == allName {
		l.refuse(stmt, allRefusal)
		return
	}
	if single && typed.IsZero() && value.Kind() == l.v.call && l.callDeclaration(targets[0], value, stmt) {
		return
	}
	var a annotation
	if !typed.IsZero() {
		a = l.annotationOf(typed)
	}
	if single && (a.typeAlias || l.implicitAlias(stmt, typed, value)) {
		l.declareAlias(l.alias(targets[0], treesitter.Node{}, value, stmt, false, nil))
		return
	}
	l.bindings(stmt, targets, value, typed, a)
}

// declareAlias appends an alias to the declarations of the file and records
// its name, and does nothing for the nil alias of a name that the module
// declares already or that the signature depth leaves out.
func (l *lowering) declareAlias(a *node.Alias) {
	if a != nil {
		l.file.Decls = append(l.file.Decls, a)
		setFlag(&l.aliases, a.Name, true)
	}
}

// implicitAlias reports whether an assignment of one name is an implicit
// alias of the typing specification: the assignment has no annotation and
// no type comment, and its value is a type expression of
// [lowering.typeExpression]. typed is the annotation node of the assignment.
func (l *lowering) implicitAlias(stmt, typed, value treesitter.Node) bool {
	if !typed.IsZero() || !l.typeExpression(value) {
		return false
	}
	text, _ := l.typeComment(stmt)
	return text == ""
}

// typeExpression reports whether the value of an assignment is a type
// expression by its syntax and the declarations of the module:
//
//   - a subscript of a class of builtins, of a name of typing, of
//     collections.abc or of collections, and of a class or an alias that the
//     module declares before the assignment;
//   - a name of a class of builtins or of collections.abc, Any,
//     LiteralString, Never and NoReturn of typing, and a class or an alias
//     that the module declares before the assignment;
//   - a union with | of None, names, dotted names and subscripts, in which
//     None or another type expression is an operand. The operator | of a
//     type takes only another type.
//
// The parse reads one module, so the kind of a name that an import binds
// from another module is unknown. A value that names such a name alone is
// no type expression, and neither is any other value.
func (l *lowering) typeExpression(n treesitter.Node) bool {
	n = l.inner(n)
	switch n.Kind() {
	case l.v.binaryOperator:
		if n.Child(l.v.fieldOperator).Text() != unionMark {
			return false
		}
		var operands []treesitter.Node
		l.flatten(n, &operands)
		typed := false
		for _, op := range operands {
			switch op.Kind() {
			case l.v.none:
				typed = true
			case l.v.identifier, l.v.attribute, l.v.subscript:
				typed = typed || l.typeExpression(op)
			default:
				return false
			}
		}
		return typed
	case l.v.subscript:
		module, name := l.scope.canonical(n.Child(l.v.fieldValue).Compact())
		switch module {
		case builtinsModule:
			return python.IsBuiltinClass(name)
		case typingModule, abcModule, collectionsModule:
			return true
		default:
			return module == l.at.pkg && l.declaresType(name)
		}
	case l.v.identifier, l.v.attribute:
		module, name := l.scope.canonical(n.Compact())
		switch module {
		case builtinsModule:
			return python.IsBuiltinClass(name)
		case abcModule:
			return true
		case typingModule:
			return name == anyForm || name == literalStringForm || name == neverForm || name == noReturnForm
		default:
			return module == l.at.pkg && l.declaresType(name)
		}
	default:
		return false
	}
}

// declaresType reports whether the module declares a class or an alias of
// the name before the statement under lowering.
func (l *lowering) declaresType(name string) bool {
	if _, class := l.classKinds[name]; class {
		return true
	}
	return l.aliases[name]
}

// targets appends to buf the targets of an assignment, each target of a
// chain a = b = v, and returns them with the value of the last assignment
// of the chain.
func (l *lowering) targets(stmt treesitter.Node, buf []treesitter.Node) ([]treesitter.Node, treesitter.Node) {
	for n := stmt; ; {
		buf = append(buf, n.Child(l.v.fieldLeft))
		right := n.Child(l.v.fieldRight)
		if right.Kind() != l.v.assignment {
			return buf, right
		}
		n = right
	}
}

// names returns the names that a target binds: the target itself where it
// is a name, appended to buf, and each name of a tuple of targets in a new
// list. It reports whether the target is one name, which takes the value of
// its assignment. A target of any other form binds no name of the module.
func (l *lowering) names(target treesitter.Node, buf []treesitter.Node) ([]treesitter.Node, bool) {
	switch target.Kind() {
	case l.v.identifier:
		return append(buf, target), true
	case l.v.patternList, l.v.tuplePattern:
		var out []treesitter.Node
		for element := range target.Content() {
			if element.Kind() == l.v.identifier {
				out = append(out, element)
			}
		}
		return out, false
	default:
		return buf, false
	}
}

// bindings lowers the names that an assignment binds at the top level of a
// module, and appends their declarations to the declarations of the file: a
// constant for each name under Final, and a mutable variable for each other
// name, with the type of the annotation a or of the type comment and the
// value as written. typed is the annotation node of the assignment, and the
// zero Node for an assignment without one. A name of a tuple of targets has
// no value. Each takes the docstring of the attribute, the trailing comment
// of the assignment and the metadata of an Annotated, and the first takes
// its carriers. bindings returns the declarations that it appends.
func (l *lowering) bindings(
	stmt treesitter.Node,
	targets []treesitter.Node,
	value, typed treesitter.Node,
	a annotation,
) []symbol.Symbol {
	if typed.IsZero() {
		if text, at := l.typeComment(stmt); text != "" {
			l.parseAt(text, at, func(t treesitter.Node) { a.ref = l.typeRef(t) })
		}
	}
	carriers, comment := l.declParts(stmt, stmt)
	doc := l.attributeDoc(stmt)
	start := len(l.file.Decls)
	var spelled [nodeBuffer]treesitter.Node
	for _, target := range targets {
		names, single := l.names(target, spelled[:0])
		for _, name := range names {
			text := name.Text()
			if !l.declare(text, l.pos(name), false) {
				continue
			}
			vis := l.moduleVisibility(text)
			if l.signatures && vis != symbol.VisibilityPublic {
				continue
			}
			ref := a.ref
			if len(l.file.Decls) > start {
				ref = node.CloneRef(a.ref)
			}
			assigned := ""
			if single {
				assigned = value.Text()
			}
			var decl symbol.Symbol
			if a.final {
				decl = &node.Constant{
					Name:       text,
					Pos:        l.pos(name),
					Doc:        doc,
					Comment:    comment,
					Visibility: vis,
					Type:       ref,
					Value:      assigned,
				}
			} else {
				decl = &node.Variable{
					Name: text, Pos: l.pos(name), Doc: doc, Comment: comment, Visibility: vis,
					Mutability: symbol.MutabilityMutable, Type: ref, Value: assigned,
				}
			}
			l.annotatedStamp(decl, a)
			l.file.Decls = append(l.file.Decls, decl)
		}
	}
	added := l.file.Decls[start:]
	if len(added) == 0 {
		l.u.RefuseCarriers(UnaddressedCarrier, carriers, noNameRefusal)
		return nil
	}
	l.u.AttachCarriers(added[0], carriers, BadCarrier)
	return added
}

// callDeclaration lowers the assignment of a call that declares, and
// appends its declaration to the declarations of the file: the variable of
// a type variable, stamped python.typeVar with its callee; the defined
// alias of NewType; and the class of the functional forms of NamedTuple,
// TypedDict, the classes of enum and namedtuple of collections. It reports
// false for a call of any other callee, which binds a variable.
func (l *lowering) callDeclaration(name, call, stmt treesitter.Node) bool {
	module, callee := l.scope.canonical(call.Child(l.v.fieldFunction).Compact())
	_, typeVariable := typeVarMarks[callee]
	switch {
	case module == typingModule && typeVariable:
		text := name.Text()
		if l.typeVars == nil {
			l.typeVars = map[string]*typeVar{}
		}
		l.typeVars[text] = l.typeVarCall(text, call, callee)
		for _, d := range l.bindings(stmt, []treesitter.Node{name}, call, treesitter.Node{}, annotation{}) {
			l.u.Graph().Stamp(d, meta.RawStamp{Key: python.TypeVarKey, Value: callee, Pos: l.pos(call)})
		}
		return true
	case module == typingModule && callee == newTypeName:
		var target treesitter.Node
		positional := 0
		for arg := range call.Child(l.v.fieldArguments).Content() {
			if positional++; positional == 2 {
				target = arg
			}
		}
		l.declareAlias(l.alias(name, treesitter.Node{}, target, stmt, true, nil))
		return true
	case module == typingModule && (callee == namedTupleName || callee == typedDictName),
		module == enumModule && enumClasses[callee], module == collectionsModule && callee == namedtupleCall:
		if decl := l.functionalClass(name, call, stmt, callee); decl != nil {
			l.file.Decls = append(l.file.Decls, decl)
		}
		return true
	default:
		return false
	}
}

// typeAlias lowers a type statement of PEP 695: an alias of its name, its
// type parameters and its value.
func (l *lowering) typeAlias(stmt treesitter.Node) {
	left := l.inner(stmt.Child(l.v.fieldLeft))
	nameNode, params := left, treesitter.Node{}
	if left.Kind() == l.v.genericType {
		nameNode, params = firstNamed(left), left.FirstOf(l.v.typeParameter)
	}
	l.declareAlias(l.alias(nameNode, params, stmt.Child(l.v.fieldRight), stmt, false, nil))
}

// alias lowers an alias of a name: its type parameters of PEP 695 and the
// type variables of the module that its target names, its target and the
// metadata of an Annotated that wraps the target. A defined alias is a
// distinct type, as NewType declares one. host is the body of the class
// whose member the alias is, and nil at the top level of a module. alias
// returns nil for a name that the module or the class declares already,
// and for an alias that the signature depth leaves out, and takes the
// comments of either.
func (l *lowering) alias(nameNode, params, value, stmt treesitter.Node, defined bool, host *classBody) *node.Alias {
	name := nameNode.Text()
	vis, claimed := l.claim(name, l.pos(nameNode), host)
	if !claimed {
		l.skip(stmt)
		return nil
	}
	carriers, comment := l.declParts(stmt, stmt)
	target := l.annotationOf(value)
	a := &node.Alias{
		Name: name, Pos: l.pos(nameNode), Doc: l.attributeDoc(stmt), Comment: comment, Visibility: vis,
		Defined: defined, TypeParams: l.typeParams(params), Target: target.ref,
	}
	taken := map[string]bool{}
	for _, tp := range a.TypeParams {
		taken[tp.Name] = true
	}
	a.TypeParams = append(a.TypeParams, l.usedVars([]*node.TypeRef{a.Target}, taken)...)
	l.annotatedStamp(a, target)
	l.u.AttachCarriers(a, carriers, BadCarrier)
	return a
}

// prescan reads the names of __all__ before the walk, because the
// visibility of every declaration of the module depends on them wherever
// the module states __all__. It reads the assignments of __all__, its
// augmented assignments, and the calls of its methods extend and append,
// in every branch of the module. It compares the names in the source and
// allocates nothing for a statement of another name. It also counts the
// statements that can declare a name, the capacity of the declarations of
// the file.
func (l *lowering) prescan(list treesitter.Node) {
	for stmt := range list.Content() {
		switch stmt.Kind() {
		case l.v.classDefinition, l.v.functionDefinition, l.v.decoratedDefinition, l.v.typeAliasStatement:
			l.declaring++
		case l.v.assignment, l.v.augmentedAssignment:
			if stmt.Kind() == l.v.assignment {
				l.declaring++
			}
			left := stmt.Child(l.v.fieldLeft)
			if left.Kind() == l.v.identifier && string(left.Bytes()) == allName {
				l.addAll(stmt.Child(l.v.fieldRight))
			}
		case l.v.call:
			method := stmt.Child(l.v.fieldFunction)
			if method.Kind() != l.v.attribute || string(method.Child(l.v.fieldObject).Bytes()) != allName {
				continue
			}
			attr := string(method.Child(l.v.fieldAttribute).Bytes())
			if attr == extendName || attr == appendName {
				for arg := range stmt.Child(l.v.fieldArguments).Content() {
					l.addAll(arg)
				}
			}
		case l.v.ifStatement, l.v.tryStatement, l.v.withStatement, l.v.forStatement, l.v.whileStatement,
			l.v.matchStatement:
			l.allBodies(stmt, l.prescan)
		}
	}
}

// addAll adds the names of a value of __all__: each string of a list or a
// tuple, and of both operands of a +, once each in their order.
func (l *lowering) addAll(n treesitter.Node) {
	if l.all == nil {
		l.all = map[string]bool{}
	}
	switch n.Kind() {
	case l.v.list, l.v.tuple:
		for e := range n.Content() {
			l.addAll(e)
		}
	case l.v.binaryOperator:
		l.addAll(n.Child(l.v.fieldLeft))
		l.addAll(n.Child(l.v.fieldRight))
	case l.v.parenthesizedExpression:
		l.addAll(firstNamed(n))
	case l.v.stringNode, l.v.concatenatedString:
		if name, is := l.stringValue(n); is && !l.all[name] {
			l.all[name] = true
			l.allNames = append(l.allNames, &node.Binding{Pos: l.pos(n), Name: name})
		}
	}
}

// allExport returns the export of the names of __all__, with the position
// of the first name.
func (l *lowering) allExport(root treesitter.Node) *node.Export {
	at := root.Pos()
	if len(l.allNames) > 0 {
		at = l.allNames[0].Pos
	}
	return &node.Export{Pos: at, Names: l.allNames}
}
