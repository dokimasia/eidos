// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/lang/naming"
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
)

// SentinelName spells the error value a base names under Go's
// convention: Err followed by the base in PascalCase.
func (Rules) SentinelName(base string) string { return sentinelPrefix + naming.Pascal(base) }

// IsSentinelName reports whether an identifier follows the
// convention: the Err prefix followed by an upper-case rune.
func (Rules) IsSentinelName(ident string) bool {
	rest, prefixed := strings.CutPrefix(ident, sentinelPrefix)
	if !prefixed || rest == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return unicode.IsUpper(r)
}

// Tag reads one key of a field's struct tag, under Go's
// key:"value" convention, and reports whether the key is present.
func (Rules) Tag(f *node.Field, key string) (string, bool) {
	if f == nil {
		return "", false
	}
	return reflect.StructTag(f.Tag).Lookup(key)
}

// Derive returns a witness for a type parameter whose type set is
// knowable without loading the declaring package: no bound, any
// or comparable, each satisfied by int. Every other bound is
// authored or nothing.
func (Rules) Derive(p *node.TypeParam, _ rules.View) (*node.TypeRef, bool) {
	if p == nil {
		return nil, false
	}
	switch len(p.Bounds) {
	case witnessBoundNone:
		return &node.TypeRef{Spelling: witnessSpelling}, true
	case 1:
		switch named(p.Bounds[0]) {
		case anySpelling, comparableBound:
			return &node.TypeRef{Spelling: witnessSpelling}, true
		}
	}
	return nil, false
}

// Substitute restates a reference with type arguments bound: a
// named reference spelling a parameter becomes the argument at the
// parameter's position, and a structural or instantiated reference
// restates its children and arguments. A reference naming no
// parameter is returned as it is.
func (r Rules) Substitute(
	ref *node.TypeRef,
	params []*node.TypeParam,
	args []*node.TypeRef,
) *node.TypeRef {
	if ref == nil || len(params) != len(args) {
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
	if len(ref.Elems) == 0 && len(ref.Args) == 0 {
		return ref
	}
	c := *ref
	c.Elems = r.substituteAll(ref.Elems, params, args)
	c.Args = r.substituteAll(ref.Args, params, args)
	return &c
}

// substituteAll restates a list of references.
func (r Rules) substituteAll(
	refs []*node.TypeRef,
	params []*node.TypeParam,
	args []*node.TypeRef,
) []*node.TypeRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]*node.TypeRef, 0, len(refs))
	for _, held := range refs {
		out = append(out, r.Substitute(held, params, args))
	}
	return out
}

// Reified reports that Go erases type arguments at run time: a
// generic value carries no reflection of its parameters a
// generator could read.
func (Rules) Reified() bool { return false }

// Settable returns the fields a constructor in another package can
// set: the exported fields, promotion through embedding included,
// in the member walk's order.
func (r Rules) Settable(s *node.Struct, v rules.View) []rules.Member {
	set, is := rules.NewBound(r, v, nil).MembersOf(s)
	if !is {
		return nil
	}
	var out []rules.Member
	for _, m := range set.Members {
		f, isField := m.Symbol.(*node.Field)
		if isField && exported(f.Name) {
			out = append(out, m)
		}
	}
	return out
}

// Comparable reports whether a type works where Go demands ==: a
// slice, a map and a function type never do, a pointer, a channel
// and an interface always do, an array compares as its element, a
// builtin by its table, and a workspace type by its declaration,
// a struct through every field and embed, an alias through its
// target, an enumeration always. A named reference outside the
// workspace is unprovable, so it reports false. The references
// that break comparability come back as the problems.
func (r Rules) Comparable(ref *node.TypeRef, v rules.View) (bool, []*node.TypeRef) {
	var problems []*node.TypeRef
	ok := r.comparable(ref, v, map[symbol.Identity]bool{}, &problems)
	return ok, problems
}

// comparable is [Rules.Comparable] with the declarations on the
// path, so a self-referential type settles rather than recursing.
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
	case symbol.FormNamed:
	default:
		*problems = append(*problems, ref)
		return false
	}
	if ref.Target.IsZero() {
		if comparableBuiltin(named(ref)) {
			return true
		}
		*problems = append(*problems, ref)
		return false
	}
	if visiting[ref.Target] {
		return true // on the path already: the outer walk settles it
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
			if f == nil || !r.comparable(f.Type, v, visiting, problems) {
				ok = false
			}
		}
		for _, e := range d.Embeds {
			if e == nil || !r.comparable(e.Ref, v, visiting, problems) {
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
		return r.comparable(d.Target, v, visiting, problems)
	default:
		*problems = append(*problems, ref)
		return false
	}
}
