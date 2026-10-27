// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// memberedKinds are the kinds of the declarations that have members, in
// the order in which [Bound.Implementers] reads them.
var memberedKinds = [...]symbol.Kind{symbol.KindStruct, symbol.KindInterface, symbol.KindEnum, symbol.KindSum}

// Satisfaction is the comparison of the member set of a type with the
// members of an interface. The zero value is the satisfaction of an
// interface without members by a type without gaps.
type Satisfaction struct {
	// Missing are the members of the interface that the type has no
	// member of the name of, in the order of the interface.
	Missing []Member
	// Mismatched are the members of the interface that the type has with
	// another signature or another field type.
	Mismatched []Mismatch
	// Gaps are the gaps of the member walk of either side. A gap leaves
	// the comparison unproven.
	Gaps []Gap
}

// Mismatch pairs a member of an interface with the member of the same
// name that a type has.
type Mismatch struct {
	Want Member // the member of the interface
	Got  Member // the first member of the name that the type has
}

// Satisfied reports whether the type has every member of the interface
// with its signature, and the walk of both sides is complete. It
// allocates nothing.
func (s Satisfaction) Satisfied() bool {
	return len(s.Missing) == 0 && len(s.Mismatched) == 0 && len(s.Gaps) == 0
}

// Satisfies compares the member set of the type that t names with the
// members of the interface that iface names, as [Bound.MembersOf] returns
// both. It substitutes the type arguments of each reference into the
// members that it compares, and compares a method by its parameter and
// return types and a field by its type. Where t is not an optional and
// the rules of the type implement [SatisfactionRules], a method that a
// value of the type does not have counts as missing. It reports false
// where t or iface does not name a declaration of the view, where iface
// names no interface, and where the rules of the interface report that it
// constrains a type set. Where the member policy of the rules sets
// [MemberPolicy.PackageScopedNames], a member of the interface of package
// visibility matches only a member whose owner is in the package of the
// owner of the interface member.
//
// # Allocation contract
//
// Satisfies allocates what the two member walks allocate, the list of the
// members of the interface, the signature of each method and the type of
// each field that it compares, a copy of each method whose types it
// substitutes, and the lists of a satisfaction with misses. A type that
// satisfies an interface of four methods costs seven allocations.
func (b Bound) Satisfies(t, iface *node.TypeRef) (Satisfaction, bool) {
	want, wantGaps, ok := b.contract(iface)
	if !ok || t == nil {
		return Satisfaction{}, false
	}
	pointer := t.Form == symbol.FormOptional && len(t.Elems) == 1
	named := t
	if pointer {
		named = t.Elems[0]
	}
	if named == nil {
		return Satisfaction{}, false
	}
	sym, held := b.view.Lookup(named.Target)
	if !held {
		return Satisfaction{}, false
	}
	return b.compare(sym, named.Args, pointer, want, wantGaps)
}

// Implementers returns the identities of the declarations of the view
// whose member sets satisfy the interface that iface names, in identity
// order. A type implements the interface where its value satisfies it,
// and, where its rules implement [SatisfactionRules], where a pointer to
// it does. It compares a generic type at its own type parameters, and
// leaves out the interface itself. It reads every declaration of the
// kinds that have members through the reader of the view, so a run that
// calls it reruns where a declaration of those kinds enters or leaves the
// graph. It returns nil where iface names no interface of the view.
//
// # Allocation contract
//
// Implementers allocates what [Bound.Satisfies] allocates for each
// candidate, and the list that it returns.
func (b Bound) Implementers(iface *node.TypeRef) []symbol.Identity {
	want, wantGaps, ok := b.contract(iface)
	reader := b.view.Reader()
	if !ok || reader == nil {
		return nil
	}
	var out []symbol.Identity
	for _, kind := range memberedKinds {
		for sym := range reader.ByKind(kind) {
			decl, names := sym.(node.Declaration)
			if !names || decl.Identity() == iface.Target {
				continue
			}
			if s, held := b.compare(sym, nil, false, want, wantGaps); held && s.Satisfied() {
				out = append(out, decl.Identity())
				continue
			}
			if _, refines := b.rulesOf(decl.Identity().Lang).(SatisfactionRules); !refines {
				continue
			}
			if s, held := b.compare(sym, nil, true, want, wantGaps); held && s.Satisfied() {
				out = append(out, decl.Identity())
			}
		}
	}
	slices.SortFunc(out, symbol.Identity.Compare)
	return out
}

// wanted is one member of an interface with the key that it compares by.
type wanted struct {
	member Member
	field  bool   // the member is a field, and key is its type
	key    string // the signature of a method, or the type of a field
}

// contract returns the members of the interface that iface names with
// their keys at the type arguments of iface, and the gaps of its member
// walk. It reports false where iface names no interface of the view, and
// where the rules of the interface report that it constrains a type set.
func (b Bound) contract(iface *node.TypeRef) ([]wanted, []Gap, bool) {
	if iface == nil {
		return nil, nil, false
	}
	sym, held := b.view.Lookup(iface.Target)
	i, is := sym.(*node.Interface)
	if !held || !is || i == nil {
		return nil, nil, false
	}
	if refined, refines := b.rulesOf(i.ID.Lang).(SatisfactionRules); refines && refined.TypeSet(i, b.view) {
		return nil, nil, false
	}
	set, _ := b.membersOf(i)
	want := make([]wanted, 0, len(set.Members))
	for _, m := range set.Members {
		if field, key, keyed := memberKey(m.Symbol, i.TypeParams, iface.Args); keyed {
			want = append(want, wanted{member: m, field: field, key: key})
		}
	}
	return want, set.Gaps, true
}

// compare compares the member set of sym at the type arguments args with
// the members of an interface. pointer reports whether the type is
// reached through a pointer, so a method that only a pointer has counts.
// It reports false for a declaration without members.
func (b Bound) compare(
	sym symbol.Symbol, args []*node.TypeRef, pointer bool, want []wanted, wantGaps []Gap,
) (Satisfaction, bool) {
	got, held := b.membersOf(sym)
	if !held {
		return Satisfaction{}, false
	}
	typ, _ := memberedOf(sym)
	refined, refines := b.rulesOf(typ.id.Lang).(SatisfactionRules)
	scoped := b.source.Members().PackageScopedNames
	var s Satisfaction
	for _, w := range want {
		name := nameOf(&w.member, scoped)
		var first *Member
		matched := false
		for n := range got.Members {
			c := &got.Members[n]
			if nameOf(c, scoped) != name {
				continue
			}
			if m, isMethod := c.Symbol.(*node.Method); isMethod && !pointer && refines &&
				!refined.ValueReceiver(m, b.view) {

				continue
			}
			if first == nil {
				first = c
			}
			if field, key, keyed := memberKey(c.Symbol, typ.params, args); keyed && field == w.field && key == w.key {
				matched = true
				break
			}
		}
		switch {
		case matched:
		case first == nil:
			s.Missing = append(s.Missing, w.member)
		default:
			s.Mismatched = append(s.Mismatched, Mismatch{Want: w.member, Got: *first})
		}
	}
	if len(wantGaps)+len(got.Gaps) > 0 {
		s.Gaps = append(slices.Clone(wantGaps), got.Gaps...)
	}
	return s, true
}

// memberKey returns what a member compares by, with args substituted for
// params: the signature of a method, and the type of a field, which field
// reports. It reports false for any other member.
func memberKey(sym symbol.Symbol, params []*node.TypeParam, args []*node.TypeRef) (field bool, key string, ok bool) {
	switch m := sym.(type) {
	case *node.Method:
		if m == nil {
			return false, "", false
		}
		if len(params) > 0 && len(params) == len(args) {
			c := *m
			c.Params = substituteParams(m.Params, params, args)
			c.Returns = substituteReturns(m.Returns, params, args)
			m = &c
		}
		return false, signature(m), true
	case *node.Field:
		if m == nil {
			return false, "", false
		}
		var b strings.Builder
		typeKey(&b, Substitute(m.Type, params, args))
		return true, b.String(), true
	default:
		return false, "", false
	}
}
