// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings that the capabilities read.
const (
	sentinelPrefix   = "Err"
	anySpelling      = spellAny
	comparableBound  = "comparable"
	witnessSpelling  = spellInt
	witnessBoundNone = 0
	// interfaceKeyword opens an inline interface body, and blankField is
	// the name of a field that no code can name.
	interfaceKeyword = "interface"
	blankField       = "_"
	// unionSep separates the terms of a type-set element, and
	// approxMark marks a term that admits every type of its
	// underlying type.
	unionSep   = "|"
	approxMark = "~"
)

// The methods of the error contract of Go: the method that returns the
// message, and the hooks that the package errors calls.
const (
	errorMethod  = "Error"
	unwrapMethod = "Unwrap"
	isMethod     = "Is"
	asMethod     = "As"
)

// typeSet is the handle of the type sets that the Go frontend stamps on
// an interface. A read resolves the handle in the registry of the facts
// of the view, and a view without the key reads no type set.
var typeSet = meta.Named[[]string](golang.TypeSetKey)

// The handles of the marks that the Go frontend stamps on a method with a
// pointer receiver, on an interface with type-set elements, and on a
// struct whose lowering left out a field.
var (
	pointerReceiver = meta.Named[bool](golang.ReceiverIsPointerKey)
	constraint      = meta.Named[bool](golang.ConstraintInterfaceKey)
	hiddenFields    = meta.Named[bool](golang.HiddenFieldsKey)
)

// MethodSet returns the methods of a Go type without regard to their
// receivers. For a struct, an interface and an enumeration, they are the
// methods that the member walk of the Go rules returns: the declared
// methods and the methods that the embeds promote, where two promoted
// methods of one name at one depth cancel each other. The file-level
// methods of the package of the type whose receiver names the type
// follow. A defined type has no members, so its file-level methods are all
// of its methods. MethodSet reads the package of the type alone, because
// Go declares a method in the package of its receiver. It returns nil for
// a symbol that is not a declaration.
//
// MethodSet allocates the list of methods and what the member walk
// allocates.
func MethodSet(decl symbol.Symbol, v rules.View) []*node.Method {
	d, declares := decl.(node.Declaration)
	if !declares {
		return nil
	}
	id := d.Identity()
	var out []*node.Method
	if set, membered := rules.NewBound(Rules{}, v, nil).MembersOf(decl); membered {
		out = make([]*node.Method, 0, len(set.Members))
		for _, m := range set.Members {
			if method, isMethod := m.Symbol.(*node.Method); isMethod {
				out = append(out, method)
			}
		}
	}
	if pkg, held := v.PackageOf(id); held {
		for _, f := range pkg.Files {
			for _, s := range f.Decls {
				if m, isMethod := s.(*node.Method); isMethod && m.Identity().Owner == id.Name {
					out = append(out, m)
				}
			}
		}
	}
	return out
}

// NullaryString returns the method of set with the name that takes
// nothing and returns one string, such as Error of the error interface
// and String of fmt.Stringer, and nil where set has none. The result is
// the predeclared string, directly or through transparent aliases that
// the view declares. A defined type over string is a type of its own, and
// a reference that the graph does not resolve is unknown, so neither
// counts. It allocates nothing beyond what the reads of the aliases
// record.
func NullaryString(set []*node.Method, name string, v rules.View) *node.Method {
	for _, m := range set {
		if m == nil || m.Name != name || len(m.Params) != 0 || len(m.Returns) != 1 || m.Returns[0] == nil {
			continue
		}
		t := rules.Unaliased(m.Returns[0].Type, v)
		if t != nil && t.Form == symbol.FormNamed && t.Target.IsZero() && nameOf(t) == stringType {
			return m
		}
	}
	return nil
}

// SentinelName returns the name of the error value of a base under the
// convention of Go. The name is Err followed by the base in PascalCase.
// It allocates the case conversion of the base and the joined name, two
// allocations.
func (Rules) SentinelName(base string) string { return sentinelPrefix + naming.Pascal(base) }

// SentinelBase returns the identifier after the prefix Err, where an
// upper-case rune opens it, and reports false for any other identifier.
// It allocates nothing.
func (Rules) SentinelBase(ident string) (string, bool) {
	rest, prefixed := strings.CutPrefix(ident, sentinelPrefix)
	if !prefixed || !token.IsExported(rest) {
		return "", false
	}
	return rest, true
}

// ErrorContract returns the contract of a Go type whose method set has
// the method Error() string: a struct, an enumeration, an interface or a
// defined type. [MethodSet] returns the method set, and [NullaryString]
// finds the method Error in it.
//
//   - Pointer reads the mark of a pointer receiver on the method Error.
//   - Unwrap is Unwrap() error or Unwrap() []error, Is is
//     Is(error) bool, and As is As(any) bool.
//   - Cause is the first field of the type error that a struct declares,
//     where the struct declares Unwrap() error itself. An Unwrap that an
//     embed promotes returns the cause of the embed.
//
// A parameter or a result counts through transparent aliases that the
// view declares. ErrorContract reports false for every other declaration.
// It allocates what MethodSet allocates.
func (Rules) ErrorContract(decl symbol.Symbol, v rules.View) (rules.ErrorContract, bool) {
	set := MethodSet(decl, v)
	message := NullaryString(set, errorMethod, v)
	if message == nil {
		return rules.ErrorContract{}, false
	}
	_, pointer := rules.Fact(v, message.ID, pointerReceiver)
	c := rules.ErrorContract{Message: message, Pointer: pointer}
	// builtin returns the predeclared type of a reference without a
	// target, and the zero name for any other reference.
	builtin := func(ref *node.TypeRef) typeName {
		t := rules.Unaliased(ref, v)
		switch {
		case t == nil || !t.Target.IsZero():
			return typeName{}
		case t.Form == symbol.FormInline && emptyInterfaceBody(t.Spelling):
			return anyType
		default:
			return nameOf(t)
		}
	}
	single := false
	for _, m := range set {
		if m == nil || len(m.Returns) != 1 || m.Returns[0] == nil {
			continue
		}
		result := m.Returns[0].Type
		switch {
		case m.Name == unwrapMethod && len(m.Params) == 0 && builtin(result) == errorType:
			c.Unwrap, single = m, true
		case m.Name == unwrapMethod && len(m.Params) == 0 && result != nil && result.Form == symbol.FormList &&
			len(result.Elems) == 1 && builtin(result.Elems[0]) == errorType:
			c.Unwrap = m
		case m.Name == isMethod && len(m.Params) == 1 && m.Params[0] != nil &&
			builtin(m.Params[0].Type) == errorType && builtin(result) == boolType:
			c.Is = m
		case m.Name == asMethod && len(m.Params) == 1 && m.Params[0] != nil &&
			builtin(m.Params[0].Type) == anyType && builtin(result) == boolType:
			c.As = m
		}
	}
	if s, isStruct := decl.(*node.Struct); isStruct && single && c.Unwrap.ID.Owner == s.Name {
		for _, f := range s.Fields {
			if f != nil && builtin(f.Type) == errorType {
				c.Cause = f
				break
			}
		}
	}
	return c, true
}

// ValueReceiver reports whether a value of the type of m has m, as the
// method set of a Go type has a method with a value receiver and leaves
// out one with a pointer receiver. It reads the mark that the frontend
// stamps on a method with a pointer receiver, so a view without facts
// reports every method. It allocates nothing beyond what the view
// records.
func (Rules) ValueReceiver(m *node.Method, v rules.View) bool {
	if m == nil {
		return false
	}
	_, pointer := rules.Fact(v, m.ID, pointerReceiver)
	return !pointer
}

// TypeSet reports whether an interface has type-set elements, such as the
// constraint interface{ ~int | ~float64 }, which only a type argument
// satisfies. It reads the mark that the frontend stamps on such an
// interface, so a view without facts reports none. It allocates nothing
// beyond what the view records.
func (Rules) TypeSet(i *node.Interface, v rules.View) bool {
	if i == nil {
		return false
	}
	_, marked := rules.Fact(v, i.ID, constraint)
	return marked
}

// Tag reads one key of the struct tag of a field, under the key:"value"
// convention of Go, and reports whether the key is present. It allocates
// nothing for a value without escapes.
func (Rules) Tag(f *node.Field, key string) (string, bool) {
	if f == nil {
		return "", false
	}
	return reflect.StructTag(f.Tag).Lookup(key)
}

// Derive returns int as the witness for a type parameter whose bound
// admits int. These bounds admit int:
//   - no bound
//   - the predeclared any or comparable
//   - an alias of a bound that admits int
//   - an interface in the view without methods, embeds and supertypes,
//     whose type set has int in every element. An interface without a
//     stamped type set has no such restriction, and a view without facts
//     reads no type set.
//
// Derive reads the declaration of the bound where the view contains one,
// and its spelling otherwise. Every other bound has an authored witness
// or none. Derive allocates the witness, one allocation, and what the
// read of the type-set fact allocates for an interface bound.
func (r Rules) Derive(p *node.TypeParam, v rules.View) (*node.TypeRef, bool) {
	if p == nil {
		return nil, false
	}
	switch len(p.Bounds) {
	case witnessBoundNone:
		return &node.TypeRef{Spelling: witnessSpelling}, true
	case 1:
		if r.admitsWitness(p.Bounds[0], v, 0) {
			return &node.TypeRef{Spelling: witnessSpelling}, true
		}
	}
	return nil, false
}

// admitsWitness reports whether int satisfies one bound.
func (r Rules) admitsWitness(bound *node.TypeRef, v rules.View, depth int) bool {
	if bound == nil || depth > deriveDepth {
		return false
	}
	if bound.Target.IsZero() {
		switch named(bound) {
		case anySpelling, comparableBound:
			return true
		default:
			return false
		}
	}
	sym, held := v.Lookup(bound.Target)
	if !held {
		return false
	}
	switch d := sym.(type) {
	case *node.Alias:
		return r.admitsWitness(d.Target, v, depth+1)
	case *node.Interface:
		if len(d.Methods) > 0 || len(d.Embeds) > 0 || len(d.Extends) > 0 {
			return false
		}
		elements, stamped := rules.Fact(v, d.ID, typeSet)
		return !stamped || typeSetNames(elements, witnessSpelling)
	default:
		return false
	}
}

// typeSetNames reports whether every type-set element has the type name
// among the terms of its union, with the approximation mark of each term
// stripped.
func typeSetNames(elements []string, name string) bool {
	for _, element := range elements {
		named := false
		for term := range strings.SplitSeq(element, unionSep) {
			if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(term), approxMark)) == name {
				named = true
				break
			}
		}
		if !named {
			return false
		}
	}
	return true
}

// Reified reports false, because Go erases type arguments at run time. A
// generic value has no reflection of its parameters that a generator
// could read. It allocates nothing.
func (Rules) Reified() bool { return false }

// Settable returns the fields that a constructor in another package can
// set. They are the exported fields, including the fields that embedding
// promotes, in the order of the member walk, with the gaps of the walk. A
// struct that the walk does not recognize returns an empty set.
//
// # Allocation contract
//
// Settable allocates the binding on which the member walk runs, what the
// walk allocates, and the list of settable fields, sized to the members
// of the walk. A struct that embeds one type costs four allocations.
func (r Rules) Settable(s *node.Struct, v rules.View) rules.MemberSet {
	set, is := rules.NewBound(r, v, nil).MembersOf(s)
	if !is {
		return rules.MemberSet{}
	}
	out := rules.MemberSet{Gaps: set.Gaps, Members: make([]rules.Member, 0, len(set.Members))}
	for _, m := range set.Members {
		f, isField := m.Symbol.(*node.Field)
		if isField && token.IsExported(f.Name) {
			out.Members = append(out.Members, m)
		}
	}
	return out
}

// Comparable reports whether a type is comparable with == in Go:
//   - A slice, a map and a function type are never comparable.
//   - A pointer, a channel and an interface, an inline interface
//     included, are always comparable.
//   - An array is comparable where its element is.
//   - An inline struct is comparable where every field that the reference
//     records is, and its spelling does not declare an embedded or a blank
//     field, which the reference leaves out.
//   - A builtin is comparable by its table. So are time.Duration and the
//     types of the standard library that the table lists as comparable,
//     where the view does not contain them, with a target or without one.
//   - A workspace type is comparable by its declaration. A struct is
//     comparable through every field and embed, an alias through its
//     target, and an enumeration always. A struct whose lowering left out
//     a field, golang.hiddenFields, is comparable only where the table of
//     the standard library lists it as comparable.
//
// An instantiated generic type checks its declaration with its type
// arguments substituted. Any other named reference outside the workspace
// reports false, because the model has no declaration that proves it.
// Comparable returns the references that break comparability as the
// problems.
//
// # Allocation contract
//
// Comparable allocates the problem list as it grows, the restated fields
// of an instantiated generic type, and the parse of the spelling of an
// inline struct. Any other type that compares allocates nothing.
func (r Rules) Comparable(ref *node.TypeRef, v rules.View) (bool, []*node.TypeRef) {
	var problems []*node.TypeRef
	ok := r.comparable(ref, v, map[symbol.Identity]bool{}, &problems)
	return ok, problems
}

// comparable is [Rules.Comparable] with the declarations on the path, so
// a self-referential type settles without endless recursion.
func (r Rules) comparable(
	ref *node.TypeRef, v rules.View, visiting map[symbol.Identity]bool, problems *[]*node.TypeRef,
) bool {
	if ref == nil {
		return false
	}
	switch ref.Form {
	case symbol.FormList, symbol.FormMap, symbol.FormFunc:
		*problems = append(*problems, ref)
		return false
	case symbol.FormOptional, symbol.FormStream:
		return true
	case symbol.FormArray:
		return r.comparable(child(ref, 0), v, visiting, problems)
	case symbol.FormInline:
		return r.comparableInline(ref, v, visiting, problems)
	case symbol.FormNamed:
	default:
		*problems = append(*problems, ref)
		return false
	}
	if ref.Target.IsZero() {
		if comparableBuiltin(ref) {
			return true
		}
		*problems = append(*problems, ref)
		return false
	}
	if visiting[ref.Target] {
		return true // the outer walk on the path decides it
	}
	visiting[ref.Target] = true
	sym, held := v.Lookup(ref.Target)
	if !held {
		if comparableBuiltin(ref) {
			return true
		}
		*problems = append(*problems, ref)
		return false
	}
	switch d := sym.(type) {
	case *node.Struct:
		if _, hidden := rules.Fact(v, d.ID, hiddenFields); hidden {
			if stdComparable[typeName{pkg: d.ID.Package, name: d.ID.Name}] {
				return true
			}
			*problems = append(*problems, ref)
			return false
		}
		ok := true
		for _, f := range d.Fields {
			if f == nil || !r.comparable(rules.Substitute(f.Type, d.TypeParams, ref.Args), v, visiting, problems) {
				ok = false
			}
		}
		for _, e := range d.Embeds {
			if e == nil || !r.comparable(rules.Substitute(e.Ref, d.TypeParams, ref.Args), v, visiting, problems) {
				ok = false
			}
		}
		return ok
	case *node.Interface, *node.Enum:
		return true
	case *node.Alias:
		if d.Target == nil {
			*problems = append(*problems, ref)
			return false
		}
		return r.comparable(rules.Substitute(d.Target, d.TypeParams, ref.Args), v, visiting, problems)
	default:
		*problems = append(*problems, ref)
		return false
	}
}

// comparableInline is [Rules.comparable] for an inline reference. An
// inline interface is comparable. An inline struct is comparable where
// every field that the reference records is, and where go/parser finds no
// embedded or blank field in its spelling, because the reference leaves
// both out. Every other inline type, and a spelling that does not parse,
// is the problem.
func (r Rules) comparableInline(
	ref *node.TypeRef, v rules.View, visiting map[symbol.Identity]bool, problems *[]*node.TypeRef,
) bool {
	spelling := named(ref)
	if strings.HasPrefix(spelling, interfaceKeyword) {
		return true
	}
	expr, err := parser.ParseExpr(spelling)
	st, isStruct := expr.(*ast.StructType)
	leftOut := func(f *ast.Field) bool {
		blank := func(n *ast.Ident) bool { return n.Name == blankField }
		return len(f.Names) == 0 || slices.ContainsFunc(f.Names, blank)
	}
	if err != nil || !isStruct || slices.ContainsFunc(st.Fields.List, leftOut) {
		*problems = append(*problems, ref)
		return false
	}
	ok := true
	for _, f := range ref.Fields {
		if f == nil || !r.comparable(f.Type, v, visiting, problems) {
			ok = false
		}
	}
	return ok
}
