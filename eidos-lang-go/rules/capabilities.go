// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go/token"
	"reflect"
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
	// interfaceKeyword opens an inline interface body.
	interfaceKeyword = "interface"
	// unionSep separates the terms of a type-set element, and
	// approxMark marks a term that admits every type of its
	// underlying type.
	unionSep   = "|"
	approxMark = "~"
)

// typeSet is the handle of the type sets that the Go frontend stamps on
// an interface. A read resolves the handle in the registry of the facts
// of the view, and a view without the key reads no type set.
var typeSet = meta.Named[[]string](golang.TypeSetKey)

// SentinelName returns the name of the error value of a base under the
// convention of Go. The name is Err followed by the base in PascalCase.
// It allocates the case conversion of the base and the joined name, two
// allocations.
func (Rules) SentinelName(base string) string { return sentinelPrefix + naming.Pascal(base) }

// IsSentinelName reports whether an identifier has the prefix Err
// followed by an upper-case rune. It allocates nothing.
func (Rules) IsSentinelName(ident string) bool {
	rest, prefixed := strings.CutPrefix(ident, sentinelPrefix)
	return prefixed && token.IsExported(rest)
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
//   - A builtin is comparable by its table.
//   - A workspace type is comparable by its declaration. A struct is
//     comparable through every field and embed, an alias through its
//     target, and an enumeration always.
//
// An instantiated generic type checks its declaration with its type
// arguments substituted. A named reference outside the workspace and an
// inline struct report false, because the model has no declaration that
// proves either. Comparable returns the references that break
// comparability as the problems.
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
		return true // the outer walk on the path decides it
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
