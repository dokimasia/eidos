// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"reflect"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings the capabilities read.
const (
	sentinelPrefix   = "Err"
	anySpelling      = spellAny
	comparableBound  = "comparable"
	witnessSpelling  = spellInt
	witnessBoundNone = 0
	// interfaceKeyword opens an inline interface body.
	interfaceKeyword = "interface"
	// unionSep separates the terms of a type-set element, and
	// approxMark marks a term that admits every type of its
	// underlying type.
	unionSep   = "|"
	approxMark = "~"
)

// SentinelName spells the error value a base names under Go's
// convention: Err followed by the base in PascalCase. It allocates the
// base's case conversion and the joined name, two allocations.
func (Rules) SentinelName(base string) string { return sentinelPrefix + naming.Pascal(base) }

// IsSentinelName reports whether an identifier follows the
// convention: the Err prefix followed by an upper-case rune. It
// allocates nothing.
func (Rules) IsSentinelName(ident string) bool {
	rest, prefixed := strings.CutPrefix(ident, sentinelPrefix)
	if !prefixed || rest == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return unicode.IsUpper(r)
}

// Tag reads one key of a field's struct tag, under Go's
// key:"value" convention, and reports whether the key is present. It
// allocates nothing for a value without escapes.
func (Rules) Tag(f *node.Field, key string) (string, bool) {
	if f == nil {
		return "", false
	}
	return reflect.StructTag(f.Tag).Lookup(key)
}

// Derive returns int as the witness for a type parameter whose bound
// admits it: no bound, the predeclared any or comparable, an alias of
// an admitting bound, and an interface in the view that states no
// methods, no embeds and no supertypes and whose type set, where the
// frontend stamped one, names int in every element. The bound's
// declaration is read where the view contains one, and its spelling
// otherwise. Every other bound is authored or nothing. It allocates the
// witness, one allocation, and what the type-set fact's read allocates
// for an interface bound.
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
		key, keyed := r.typeSetKey(v)
		if !keyed {
			return false
		}
		elements, stamped := rules.Fact(v, d.ID, key)
		return !stamped || typeSetNames(elements, witnessSpelling)
	default:
		return false
	}
}

// typeSetNames reports whether every type-set element names a type
// among its union's terms, a term's approximation mark stripped.
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

// typeSetKey returns the handle the frontend's type sets stamp
// under, and false where the view has no facts or the composition
// registered no such key.
func (Rules) typeSetKey(v rules.View) (meta.Key[[]string], bool) {
	if v.Facts == nil {
		return meta.Key[[]string]{}, false
	}
	return meta.Lookup[[]string](v.Facts.Registry(), golang.TypeSetKey)
}

// Substitute restates a reference with type arguments bound: a
// named reference spelling a parameter becomes the argument at the
// parameter's position, and a structural or instantiated reference
// restates its children and arguments. It copies what it rewrites, and
// returns a reference naming no parameter as it is, its children
// included.
//
// # Allocation contract
//
// Substitute allocates the copy of each reference on the path to a
// parameter it binds, with the list that holds it, and nothing for a
// reference that names no parameter or for an empty parameter list.
func (r Rules) Substitute(
	ref *node.TypeRef,
	params []*node.TypeParam,
	args []*node.TypeRef,
) *node.TypeRef {
	if ref == nil || len(params) == 0 || len(params) != len(args) {
		return ref
	}
	if ref.Form == symbol.FormNamed && len(ref.Args) == 0 {
		for i, p := range params {
			if p != nil && ref.Spelling == p.Name && args[i] != nil {
				c := *args[i]
				return &c
			}
		}
	}
	elems, elemsBound := r.substituteAll(ref.Elems, params, args)
	bound, argsBound := r.substituteAll(ref.Args, params, args)
	if !elemsBound && !argsBound {
		return ref
	}
	c := *ref
	c.Elems, c.Args = elems, bound
	return &c
}

// substituteAll restates a list of references, and reports whether
// any of them names a parameter. A list naming none is returned as it
// is, and the first restated reference copies the list.
func (r Rules) substituteAll(
	refs []*node.TypeRef,
	params []*node.TypeParam,
	args []*node.TypeRef,
) ([]*node.TypeRef, bool) {
	var out []*node.TypeRef
	for i, ref := range refs {
		restated := r.Substitute(ref, params, args)
		if restated != ref && out == nil {
			out = slices.Clone(refs)
		}
		if out != nil {
			out[i] = restated
		}
	}
	if out == nil {
		return refs, false
	}
	return out, true
}

// Reified reports that Go erases type arguments at run time: a
// generic value has no reflection of its parameters a generator
// could read. It allocates nothing.
func (Rules) Reified() bool { return false }

// Settable returns the fields a constructor in another package can
// set: the exported fields, promotion through embedding included,
// in the member walk's order, with the walk's gaps. A struct the
// walk does not recognize returns an empty set.
//
// # Allocation contract
//
// Settable allocates the binding the member walk runs on, what the
// walk allocates, and the list of settable fields sized to the walk's
// members: four allocations for a struct that embeds one type.
func (r Rules) Settable(s *node.Struct, v rules.View) rules.MemberSet {
	set, is := rules.NewBound(r, v, nil).MembersOf(s)
	if !is {
		return rules.MemberSet{}
	}
	out := rules.MemberSet{Gaps: set.Gaps, Members: make([]rules.Member, 0, len(set.Members))}
	for _, m := range set.Members {
		f, isField := m.Symbol.(*node.Field)
		if isField && exported(f.Name) {
			out.Members = append(out.Members, m)
		}
	}
	return out
}

// Comparable reports whether a type works where Go demands ==: a
// slice, a map and a function type never do, a pointer, a channel
// and an interface always do, an inline interface included, an
// array compares as its element, a builtin by its table, and a
// workspace type by its declaration, a struct through every field
// and embed, an alias through its target, an enumeration always. An
// instantiated generic type checks its declaration with its type
// arguments substituted. A named reference outside the workspace
// and an inline struct are unprovable, because the model has no
// declaration for either, so each reports false. The references
// that break comparability are returned as the problems.
//
// # Allocation contract
//
// Comparable allocates the problem list as it grows, and the restated
// fields of an instantiated generic type. A type that compares
// allocates nothing.
func (r Rules) Comparable(ref *node.TypeRef, v rules.View) (bool, []*node.TypeRef) {
	var problems []*node.TypeRef
	ok := r.comparable(ref, v, map[symbol.Identity]bool{}, &problems)
	return ok, problems
}

// comparable is [Rules.Comparable] with the declarations on the
// path, so a self-referential type settles without recursing.
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
		if strings.HasPrefix(named(ref), interfaceKeyword) {
			return true
		}
		*problems = append(*problems, ref)
		return false
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
		return true // on the path already: the outer walk decides it
	}
	visiting[ref.Target] = true
	sym, held := v.Lookup(ref.Target)
	if !held {
		*problems = append(*problems, ref)
		return false
	}
	switch d := sym.(type) {
	case *node.Struct:
		ok := true
		for _, f := range d.Fields {
			if f == nil || !r.comparable(r.Substitute(f.Type, d.TypeParams, ref.Args), v, visiting, problems) {
				ok = false
			}
		}
		for _, e := range d.Embeds {
			if e == nil || !r.comparable(r.Substitute(e.Ref, d.TypeParams, ref.Args), v, visiting, problems) {
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
		return r.comparable(r.Substitute(d.Target, d.TypeParams, ref.Args), v, visiting, problems)
	default:
		*problems = append(*problems, ref)
		return false
	}
}
