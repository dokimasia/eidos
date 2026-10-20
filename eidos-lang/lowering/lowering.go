// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package lowering

import (
	"fmt"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// CopyTypeParams returns a copy of a parameter list that shares no node
// with ps. Each parameter is new, and so are the references of its
// bounds, its default and its type. The walks of the settle then visit
// the list of each output once. For no parameters, it returns nil
// without allocating. A list allocates itself, each parameter, and the
// copies that [CopyTypeRef] makes of its references.
func CopyTypeParams(ps []*emit.TypeParam) []*emit.TypeParam {
	if len(ps) == 0 {
		return nil
	}
	out := make([]*emit.TypeParam, 0, len(ps))
	for _, p := range ps {
		c := *p
		c.Bounds = CopyTypeRefs(p.Bounds)
		c.Default = CopyTypeRef(p.Default)
		c.Type = CopyTypeRef(p.Type)
		out = append(out, &c)
	}
	return out
}

// CopyTypeRefs returns a copy of a list of references that shares no
// node with ts. For no references, it returns nil without allocating. A
// list allocates itself and the copies that [CopyTypeRef] makes.
func CopyTypeRefs(ts []*emit.TypeRef) []*emit.TypeRef {
	if len(ts) == 0 {
		return nil
	}
	out := make([]*emit.TypeRef, 0, len(ts))
	for _, t := range ts {
		out = append(out, CopyTypeRef(t))
	}
	return out
}

// CopyTypeRef returns a copy of one reference tree that shares no node
// with t. It copies the children of a form and the arguments of an
// instantiation, so a structural reference keeps its whole tree through
// a lowering. For a nil reference, it returns nil. It allocates each node
// of the tree, and the lists of children and arguments of each node.
func CopyTypeRef(t *emit.TypeRef) *emit.TypeRef {
	if t == nil {
		return nil
	}
	c := *t
	c.Elems = CopyTypeRefs(t.Elems)
	c.Args = CopyTypeRefs(t.Args)
	return &c
}

// ParamArgs returns one reference for each type parameter, in order, with
// the name of the parameter as its spelling. A declaration that a sum
// lowers into states them as the arguments of a reference to the sum. No
// parameters return nil without allocating.
//
// # Allocation contract
//
// ParamArgs allocates the list and each reference: 1+n allocations for n
// parameters.
func ParamArgs(params []*emit.TypeParam) []*emit.TypeRef {
	if len(params) == 0 {
		return nil
	}
	args := make([]*emit.TypeRef, 0, len(params))
	for _, p := range params {
		args = append(args, &emit.TypeRef{Spelling: p.Name})
	}
	return args
}

// NamedPayload returns an error under the prefix of lang where a field of
// the payload of v has no name. A backend that lowers a variant into a
// struct, a class or an interface calls it, because each of these
// declarations names its fields. It returns nil for a variant without a
// payload.
//
// # Allocation contract
//
// NamedPayload allocates nothing. An error allocates its message.
func NamedPayload(lang symbol.Lang, v *emit.SumVariant) error {
	for _, f := range v.Fields.Items() {
		if f.Name == "" {
			return fmt.Errorf("%s: each field of a variant's declaration has a name, "+
				"and a payload entry of %s does not have one", lang, v.Name)
		}
	}
	return nil
}

// Callables lowers the declared failures of the callables of s: the
// function or the method that s is, or each method of the struct or the
// interface that s is. For each callable with failures, it calls fold with
// the name, the returns and the failures of the callable, and it clears
// the failures once fold returns nil. fold rewrites the returns in place,
// such as with Append for an error return or with Replace for a result
// type. A callable without failures keeps its returns, and every other
// kind of declaration is left unchanged.
//
// The lowering changes the results of a callable and does not rewrite its
// body. The default body has no statement, and a template renders its
// body from the lowered results. Verbatim text, scaffolding statements
// and a return statement in a slot are written for the results before the
// lowering.
//
// Error modes:
//   - A struct or an interface that declares one method name twice
//     returns the error of [UniqueMethods] under the prefix of lang.
//   - A callable with failures whose body has verbatim text or
//     scaffolding statements, or a slot with a return statement at any
//     depth, returns an error under the prefix of lang, and the callable
//     keeps its failures.
//   - An error of fold is returned unchanged, and the callable keeps its
//     failures.
//
// # Allocation contract
//
// Callables allocates what [UniqueMethods] allocates for a host, and what
// fold allocates. It allocates nothing itself.
func Callables(lang symbol.Lang, s symbol.Symbol,
	fold func(name string, returns *emit.Slot[*emit.Return], throws []*emit.TypeRef) error,
) error {
	var host string
	var methods []*emit.Method
	switch d := s.(type) {
	case *emit.Function:
		return foldThrows(lang, d.Name, &d.Body, &d.Returns, &d.Throws, fold)
	case *emit.Method:
		return foldThrows(lang, d.Name, &d.Body, &d.Returns, &d.Throws, fold)
	case *emit.Struct:
		host, methods = d.Name, d.Methods.Items()
	case *emit.Interface:
		host, methods = d.Name, d.Methods.Items()
	default:
		return nil
	}
	if err := UniqueMethods(string(lang), host, methods); err != nil {
		return err
	}
	for _, m := range methods {
		if err := foldThrows(lang, m.Name, &m.Body, &m.Returns, &m.Throws, fold); err != nil {
			return err
		}
	}
	return nil
}

// UniqueMethods returns an error for the first method name that the host
// declares twice, under the prefix of lang. A backend for a language
// without overloads calls it before it spells the members of the host.
//
// # Allocation contract
//
// UniqueMethods allocates nothing for up to eight methods, whose set
// of names is on the stack, and the set at its final size for more,
// three allocations. An error allocates its message.
func UniqueMethods(lang, host string, methods []*emit.Method) error {
	seen := make(map[string]bool, len(methods))
	for _, m := range methods {
		if seen[m.Name] {
			return fmt.Errorf(
				"%s: method %s declared twice on %s, and %s overloads nothing",
				lang, m.Name, host, lang,
			)
		}
		seen[m.Name] = true
	}
	return nil
}

// DeleteAccessor returns an error under the prefix of lang for the first
// method with the delete accessor that s is or contains: the method that s
// is, or a method of the struct, the interface, the enum or the sum that s
// is, and of each type that they nest. Python is the one language that
// deletes a property, so the backend of every other language calls it and
// the settle withholds the declaration. It returns nil for every other
// declaration.
//
// # Allocation contract
//
// DeleteAccessor allocates nothing. An error allocates its message.
func DeleteAccessor(lang symbol.Lang, s symbol.Symbol) error {
	var methods []*emit.Method
	var nested []symbol.Symbol
	switch d := s.(type) {
	case *emit.Method:
		methods = []*emit.Method{d}
	case *emit.Struct:
		methods, nested = d.Methods.Items(), d.Types.Items()
	case *emit.Interface:
		methods, nested = d.Methods.Items(), d.Types.Items()
	case *emit.Enum:
		methods = d.Methods.Items()
	case *emit.Sum:
		methods = d.Methods.Items()
	default:
		return nil
	}
	for _, m := range methods {
		if m.Accessor == symbol.AccessorDelete {
			return fmt.Errorf("%s: a property has no deleter in %s, and %s states one", lang, lang, m.Name)
		}
	}
	for _, t := range nested {
		if err := DeleteAccessor(lang, t); err != nil {
			return err
		}
	}
	return nil
}

// foldThrows calls fold for one callable with failures, and clears the
// failures once fold returns nil. A callable without failures is left
// unchanged, and an error of fold is returned unchanged. It returns an
// error under the prefix of lang, before it calls fold, for a body that
// [Callables] refuses.
func foldThrows(lang symbol.Lang, name string, body *emit.Body,
	returns *emit.Slot[*emit.Return], throws *[]*emit.TypeRef,
	fold func(name string, returns *emit.Slot[*emit.Return], throws []*emit.TypeRef) error,
) error {
	if len(*throws) == 0 {
		return nil
	}
	written := body.Verbatim != "" || len(body.Stmts) > 0 ||
		returnsIn(body.Prologue.Items()) || returnsIn(body.Epilogue.Items())
	for _, s := range body.Slots {
		written = written || returnsIn(s.Slot.Items())
	}
	if written {
		return fmt.Errorf("%s: the failures of %s move into its results, "+
			"and the lowering does not rewrite the statements or the text of its body", lang, name)
	}
	if err := fold(name, returns, *throws); err != nil {
		return err
	}
	*throws = nil
	return nil
}

// returnsIn reports whether a statement of stmts is a return statement or
// has one among the actions of a guard, at any depth.
func returnsIn(stmts []emit.Stmt) bool {
	for _, s := range stmts {
		if s.Kind == emit.StmtReturn || returnsIn(s.Then) {
			return true
		}
	}
	return false
}
