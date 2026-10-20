// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/lowering"
	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The parts of the declarations that the lowering writes: the decorator of
// a variant of a sum, the separator of the names that it joins, the names of
// the result of a callable and of the type of an alias, a constant or a
// variable in the name of a Protocol, and the mark of a hard-private name.
const (
	dataclassDecorator = "dataclass"
	frozenArg          = "frozen=True"
	nameJoin           = "_"
	resultName         = "result"
	typeName           = "type"
	hardMark           = "#"
	overloadDecorator  = "overload"
)

// Lower checks one declaration and reshapes the constructs that Python
// states in other declarations, before the settle respells the names:
//
//   - a sum becomes one frozen dataclass for each variant, named after the
//     sum and the variant, with the fields of the variant and the type
//     parameters of the sum, and an alias of the sum that joins the
//     dataclasses in a union;
//   - an intersection of interfaces becomes a Protocol that extends each
//     member, named after the declaration and the member that states the
//     type, and the declaration refers to the Protocol. An alias of an
//     intersection becomes the Protocol under the name of the alias;
//   - a hard-private member loses the # of its name and is private, which
//     Python writes with two underscores.
//
// Lower returns an error for each construct of this list, and the settle
// withholds the declaration:
//
//   - an annotation whose name is neither a dotted Python name nor a marker
//     of the satellite, an annotation on a declaration that Python does not
//     decorate, an assigned annotation, and an argument of a decorator that
//     does not parse as an argument of a call;
//   - a marker that the satellite does not declare, a marker on a
//     declaration of another kind, and a marker that a declaration states
//     twice;
//   - a value or a default that does not parse as Python;
//   - a parameter that a caller passes by keyword alone before a parameter
//     that a caller passes by position, a positional-only parameter after
//     one that a caller passes otherwise, a variadic parameter that is not
//     last in its form, and a parameter without a default after one with a
//     default among the parameters that a caller passes by position;
//   - a setter or a deleter of a property without a getter of its name
//     before it;
//   - a name that a class declares twice, outside the overloads of one
//     method and the accessors of one property;
//   - a sum without a variant, a sum with methods, and a variant with a
//     field without a name.
//
// A declaration without these constructs passes through and allocates
// nothing beyond the parse of its texts. Lower is the lowering of the
// backend kit, and any number of goroutines may call it.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	if err := checkDecl(s); err != nil {
		return nil, err
	}
	harden(s)
	if sum, is := s.(*emit.Sum); is {
		return lowerSum(sum)
	}
	if a, is := s.(*emit.Alias); is && a.Target != nil && a.Target.Form == symbol.FormIntersection {
		return []symbol.Symbol{&emit.Interface{
			Origin: a.Origin, Doc: a.Doc, Comment: a.Comment, Name: a.Name, Visibility: a.Visibility,
			TypeParams: a.TypeParams, Extends: a.Target.Elems,
		}}, nil
	}
	protocols, err := intersections(s)
	if err != nil || len(protocols) == 0 {
		return nil, err
	}
	return append(protocols, s), nil
}

// LowerStub lowers a declaration of a stub file as [Lower] lowers it, after
// it drops the body of each callable that the declaration is or contains. A
// stub file writes ... for every body, so it imports nothing that a body
// names. LowerStub allocates what Lower allocates.
func LowerStub(s symbol.Symbol) ([]symbol.Symbol, error) {
	dropBodies(s)
	return Lower(s)
}

// dropBodies clears the body of the function or the method s, and of each
// method of the class s and of the classes that it nests.
func dropBodies(s symbol.Symbol) {
	var methods []*emit.Method
	var types []symbol.Symbol
	switch d := s.(type) {
	case *emit.Function:
		d.Body = emit.Body{}
	case *emit.Method:
		d.Body = emit.Body{}
	case *emit.Struct:
		methods, types = d.Methods.Items(), d.Types.Items()
	case *emit.Interface:
		methods, types = d.Methods.Items(), d.Types.Items()
	case *emit.Enum:
		methods = d.Methods.Items()
	}
	for _, m := range methods {
		m.Body = emit.Body{}
	}
	for _, t := range types {
		dropBodies(t)
	}
}

// checkDecl returns the first refusal of the lowering of s and of each
// declaration that s contains.
func checkDecl(s symbol.Symbol) error {
	switch d := s.(type) {
	case *emit.Struct:
		return checkHost(d, d.Name, d.Annotations, d.Fields.Items(), d.Methods.Items(), d.Types.Items())
	case *emit.Interface:
		return checkHost(d, d.Name, d.Annotations, d.Fields.Items(), d.Methods.Items(), d.Types.Items())
	case *emit.Enum:
		if err := checkAnnotations(d, d.Annotations); err != nil {
			return err
		}
		for _, v := range d.Variants.Items() {
			if err := checkAnnotations(v, v.Annotations); err != nil {
				return err
			}
			if err := checkText(v.Name, v.Value, valueText); err != nil {
				return err
			}
		}
		return checkHost(d, d.Name, nil, d.Fields.Items(), d.Methods.Items(), nil)
	case *emit.Sum:
		return checkSum(d)
	case *emit.Function:
		if err := checkAnnotations(d, d.Annotations); err != nil {
			return err
		}
		return checkParams(d.Name, d.Params.Items())
	case *emit.Method:
		return checkMethod(d)
	case *emit.Alias:
		return checkAnnotations(d, d.Annotations)
	case *emit.Constant:
		if err := checkAnnotations(d, d.Annotations); err != nil {
			return err
		}
		return checkText(d.Name, d.Value, valueText)
	case *emit.Variable:
		if err := checkAnnotations(d, d.Annotations); err != nil {
			return err
		}
		return checkText(d.Name, d.Value, valueText)
	default:
		return nil
	}
}

// checkHost checks a class host with its annotations, its fields, its
// methods and its nested types, and the names that it declares.
func checkHost(
	host symbol.Symbol, name string, annotations symbol.Annotations,
	fields []*emit.Field, methods []*emit.Method, types []symbol.Symbol,
) error {
	if err := checkAnnotations(host, annotations); err != nil {
		return err
	}
	for _, f := range fields {
		if err := checkAnnotations(f, f.Annotations); err != nil {
			return err
		}
		if err := checkText(f.Name, f.Value, valueText); err != nil {
			return err
		}
	}
	for _, m := range methods {
		if err := checkMethod(m); err != nil {
			return err
		}
	}
	for _, t := range types {
		if err := checkDecl(t); err != nil {
			return err
		}
	}
	return members(name, fields, methods)
}

// checkMethod checks a method, its parameters and their texts.
func checkMethod(m *emit.Method) error {
	if err := checkAnnotations(m, m.Annotations); err != nil {
		return err
	}
	return checkParams(m.Name, m.Params.Items())
}

// checkSum checks a sum before its lowering.
func checkSum(s *emit.Sum) error {
	switch {
	case s.Variants.Len() == 0:
		return refuse("a union joins at least one type, and the sum %s states no variant", s.Name)
	case s.Methods.Len() > 0:
		return refuse("a union has no members, and the sum %s states methods", s.Name)
	}
	if err := checkAnnotations(s, s.Annotations); err != nil {
		return err
	}
	for _, v := range s.Variants.Items() {
		if err := checkAnnotations(v, v.Annotations); err != nil {
			return err
		}
		if err := lowering.NamedPayload(python.Lang, v); err != nil {
			return err
		}
		for _, f := range v.Fields.Items() {
			if err := checkAnnotations(f, f.Annotations); err != nil {
				return err
			}
			if err := checkText(f.Name, f.Value, valueText); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkAnnotations checks the annotations of the declaration d by the rules
// of [Lower].
func checkAnnotations(d symbol.Symbol, as symbol.Annotations) error {
	kind := d.Kind()
	for i, a := range as {
		m := python.Marker(a.Name)
		switch {
		case strings.HasPrefix(a.Name, python.MarkerPrefix) && !m.Valid():
			return refuse("the satellite declares no marker %s", a.Name)
		case strings.HasPrefix(a.Name, python.MarkerPrefix) && !m.Marks(kind):
			return refuse("the marker %s marks no %s", a.Name, kind)
		case strings.HasPrefix(a.Name, python.MarkerPrefix):
			for _, b := range as[:i] {
				if b.Name == a.Name {
					return refuse("a declaration states the marker %s once, and %s states it twice", a.Name,
						declaredName(d))
				}
			}
			continue
		case !decorated(kind):
			return refuse("Python decorates a class or a callable, and the %s %s states the annotation %s",
				kind, declaredName(d), a.Name)
		case !dottedName(a.Name):
			return refuse("a decorator is a dotted name, and %q is not one", a.Name)
		case a.Assigned:
			return refuse("a decorator passes its arguments in parentheses, and %s assigns its argument", a.Name)
		}
		for _, arg := range a.Args {
			if !texts.parses(arg, argumentText) {
				return refuse("an argument of a decorator is Python source, and %q of %s does not parse", arg, a.Name)
			}
		}
	}
	return nil
}

// decorated reports whether Python decorates a declaration of the kind k: a
// class or a callable.
func decorated(k symbol.Kind) bool {
	switch k {
	case symbol.KindStruct, symbol.KindInterface, symbol.KindEnum, symbol.KindFunction, symbol.KindMethod:
		return true
	default:
		return false
	}
}

// dottedName reports whether name is a dotted name of Python: identifiers
// separated by dots.
func dottedName(name string) bool {
	for part := range strings.SplitSeq(name, dot) {
		if !spell.IsIdentifier(part) {
			return false
		}
	}
	return true
}

// checkText checks that the text of the declaration name parses as the
// construct k, and accepts an empty text.
func checkText(name, text string, k textKind) error {
	if text == "" || texts.parses(text, k) {
		return nil
	}
	return refuse("the text of %s is Python source, and %q does not parse", name, text)
}

// checkParams checks the order of the parameters of the callable name and
// their defaults by the rules of [Lower].
func checkParams(name string, ps []*emit.Param) error {
	passedOtherwise, keywords, variadic, kwargs, defaulted := false, false, false, false, false
	for _, p := range ps {
		if err := checkAnnotations(p, p.Annotations); err != nil {
			return err
		}
		if err := checkText(p.Name, p.Default, defaultText); err != nil {
			return err
		}
		switch {
		case kwargs:
			return refuse("the keyword variadic parameter of %s is its last parameter", name)
		case p.Variadic == symbol.VariadicKeyword:
			kwargs = true
		case p.Variadic == symbol.VariadicPositional && (variadic || keywords):
			return refuse("the positional variadic parameter of %s precedes its keyword parameters", name)
		case p.Variadic == symbol.VariadicPositional:
			variadic = true
		case p.Passing == symbol.PassingKeyword:
			keywords = true
		case keywords || variadic:
			return refuse("Python orders the parameters of a signature by how a caller passes them, "+
				"and %s of %s follows a parameter that a caller passes by keyword", p.Name, name)
		case p.Passing == symbol.PassingPositional && passedOtherwise:
			return refuse("Python orders the parameters of a signature by how a caller passes them, "+
				"and the positional parameter %s of %s follows a parameter that a caller passes otherwise",
				p.Name, name)
		case p.Default == "" && !p.Optional && defaulted:
			return refuse("a parameter without a default follows a parameter with a default in %s, "+
				"and Python refuses %s there", name, p.Name)
		default:
			passedOtherwise = passedOtherwise || p.Passing == symbol.PassingDefault
			defaulted = defaulted || p.Default != "" || p.Optional
		}
	}
	return nil
}

// members returns an error for a name that the class host declares twice:
// a field and another member of one name, two methods of one name unless
// all but the last are overloads or each is another accessor of one
// property, and a setter or a deleter without a getter of its name before
// it, which Python decorates through the property of the getter.
func members(host string, fields []*emit.Field, methods []*emit.Method) error {
	for i, f := range fields {
		for _, g := range fields[:i] {
			if g.Name == f.Name {
				return refuse("%s declares two fields named %s, and a class declares a name once", host, f.Name)
			}
		}
		for _, m := range methods {
			if m.Name == f.Name && !m.Indexer {
				return refuse("%s declares a field and a method named %s, and a class declares a name once",
					host, f.Name)
			}
		}
	}
	for i, m := range methods {
		if m.Indexer {
			continue
		}
		getter := m.Accessor == symbol.AccessorGet
		for _, earlier := range methods[:i] {
			if earlier.Indexer || earlier.Name != m.Name {
				continue
			}
			getter = getter || earlier.Accessor == symbol.AccessorGet
			accessors := m.Accessor != symbol.AccessorNone && earlier.Accessor != symbol.AccessorNone &&
				m.Accessor != earlier.Accessor
			if !accessors && !stated(earlier.Annotations, overloadDecorator) {
				return refuse("%s declares two methods named %s, and a class declares a name once "+
					"outside the overloads and the accessors of one name", host, m.Name)
			}
		}
		switch {
		case m.Accessor == symbol.AccessorSet && !getter:
			return refuse("a setter belongs to a property, and %s declares the setter %s before its getter",
				host, m.Name)
		case m.Accessor == symbol.AccessorDelete && !getter:
			return refuse("a deleter belongs to a property, and %s declares the deleter %s before its getter",
				host, m.Name)
		}
	}
	return nil
}

// harden strips the # of the name of each hard-private member of a class
// and of the classes that it nests, and makes the member private, which
// Python writes with two underscores.
func harden(s symbol.Symbol) {
	var fields []*emit.Field
	var methods []*emit.Method
	var types []symbol.Symbol
	switch d := s.(type) {
	case *emit.Struct:
		fields, methods, types = d.Fields.Items(), d.Methods.Items(), d.Types.Items()
	case *emit.Interface:
		fields, methods, types = d.Fields.Items(), d.Methods.Items(), d.Types.Items()
	default:
		return
	}
	for _, f := range fields {
		if f.Hard {
			f.Name = strings.TrimPrefix(f.Name, hardMark)
			f.Visibility = symbol.VisibilityPrivate
		}
	}
	for _, m := range methods {
		if m.Hard {
			m.Name = strings.TrimPrefix(m.Name, hardMark)
			m.Visibility = symbol.VisibilityPrivate
		}
	}
	for _, t := range types {
		harden(t)
	}
}

// lowerSum lowers a sum into a frozen dataclass for each variant and an
// alias that joins them.
func lowerSum(s *emit.Sum) ([]symbol.Symbol, error) {
	variants := s.Variants.Items()
	out := make([]symbol.Symbol, 0, len(variants)+1)
	refs := make([]*emit.TypeRef, 0, len(variants))
	for _, v := range variants {
		name, err := spell.Name(symbol.KindInvalid, symbol.KindStruct, s.Visibility, s.Name+nameJoin+v.Name)
		if err != nil {
			return nil, err
		}
		class := &emit.Struct{
			Origin: s.Origin, Doc: v.Doc, Comment: v.Comment, Name: name, Visibility: s.Visibility,
			TypeParams:  lowering.CopyTypeParams(s.TypeParams),
			Annotations: symbol.Annotations{{Name: dataclassDecorator, Args: []string{frozenArg}}},
		}
		class.Fields.Append(v.Fields.Items()...)
		out = append(out, class)
		refs = append(refs, &emit.TypeRef{Spelling: name, Args: lowering.ParamArgs(s.TypeParams)})
	}
	return append(out, &emit.Alias{
		Origin: s.Origin, Doc: s.Doc, Comment: s.Comment, Name: s.Name, Visibility: s.Visibility,
		TypeParams: s.TypeParams, Target: &emit.TypeRef{Form: symbol.FormUnion, Elems: refs},
	}), nil
}

// intersections declares a Protocol for each intersection that the types of
// s state, and replaces each intersection with a reference to its Protocol.
// A Protocol is named after the declaration and the member that states the
// type: a field, a parameter, the result of a callable, or the type of an
// alias, a constant or a variable, so its name differs from the name of the
// declaration. A second intersection of one member takes a number. It
// returns the error of a name that the convention of Python cannot spell.
func intersections(s symbol.Symbol) ([]symbol.Symbol, error) {
	p := protocols{origin: originOf(s), visibility: visibilityOf(s)}
	switch d := s.(type) {
	case *emit.Struct:
		p.host(d.Name, d.Fields.Items(), d.Methods.Items())
	case *emit.Interface:
		p.host(d.Name, d.Fields.Items(), d.Methods.Items())
	case *emit.Function:
		p.callable(d.Name, d.Params.Items(), d.Returns.Items())
	case *emit.Alias:
		p.walk(d.Name, typeName, d.Target)
	case *emit.Constant:
		p.walk(d.Name, typeName, d.Type)
	case *emit.Variable:
		p.walk(d.Name, typeName, d.Type)
	}
	return p.out, p.err
}

// protocols collects the Protocols of the intersections of one declaration,
// and the first error of a name of one.
type protocols struct {
	origin     symbol.Identity
	visibility symbol.Visibility
	out        []symbol.Symbol
	err        error
}

// host walks the types of the fields and of the methods of a class.
func (p *protocols) host(name string, fields []*emit.Field, methods []*emit.Method) {
	for _, f := range fields {
		p.walk(name, f.Name, f.Type)
	}
	for _, m := range methods {
		p.callable(name+nameJoin+m.Name, m.Params.Items(), m.Returns.Items())
	}
}

// callable walks the types of the parameters and of the results of a
// callable.
func (p *protocols) callable(name string, params []*emit.Param, returns []*emit.Return) {
	for _, param := range params {
		p.walk(name, param.Name, param.Type)
	}
	for _, r := range returns {
		p.walk(name, resultName, r.Type)
	}
}

// walk replaces each intersection of the tree of t with a reference to a
// new Protocol named after the declaration name and the member member.
func (p *protocols) walk(name, member string, t *emit.TypeRef) {
	if t == nil || p.err != nil {
		return
	}
	for _, e := range t.Elems {
		p.walk(name, member, e)
	}
	for _, a := range t.Args {
		p.walk(name, member, a)
	}
	if t.Form != symbol.FormIntersection {
		return
	}
	join := name
	if member != "" {
		join += nameJoin + member
	}
	protocol, err := spell.Name(symbol.KindInvalid, symbol.KindInterface, p.visibility, join)
	if err != nil {
		p.err = err
		return
	}
	if n := p.count(protocol); n > 0 {
		protocol += strconv.Itoa(n + 1)
	}
	p.out = append(p.out, &emit.Interface{
		Origin: p.origin, Name: protocol, Visibility: p.visibility, Extends: t.Elems,
	})
	*t = emit.TypeRef{Spelling: protocol}
}

// count returns the number of Protocols that the walk declared under name,
// or under name with a number after it.
func (p *protocols) count(name string) int {
	n := 0
	for _, s := range p.out {
		if strings.HasPrefix(emit.DeclaredName(s), name) {
			n++
		}
	}
	return n
}

// originOf returns the origin of a declaration, and the zero identity for a
// declaration without one.
func originOf(s symbol.Symbol) symbol.Identity {
	id, _ := emit.OriginOf(s)
	return id
}
