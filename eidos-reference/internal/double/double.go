// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package double

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
)

// receiverFallback is the receiver of a method of a double whose name does
// not start with a letter.
const receiverFallback = "d"

// Of returns the methods of the double named host of the interface i,
// which m matched. Of reads the rules of the language of i and a fact of i
// through m, and reports at i. Each method mirrors an effective method of
// i through [sdk.Mirror], and has a receiver and a name for each
// parameter. The rules of the language of i walk its effective members, so
// a method that an embedded interface contributes is a method of the
// double too.
//
// Of returns false after it reports [Constraint] at an interface that the
// Go satellite marks as a constraint interface, because no value has the
// type of such an interface. It returns false after it reports
// [IncompleteMembers] at i where the walk has gaps. It also returns false
// after it reports [Property] at an interface with a property, because a
// method of a double cannot implement a property.
//
// It allocates the members of the walk, what sdk.Mirror allocates for each
// method, and the receiver and the names that it adds.
func Of(m sdk.SubjectMatch, host string, i *node.Interface) ([]*emit.Method, bool) {
	if constraint, _ := sdk.FactOf(m, i.ID, meta.Named[bool](golang.ConstraintInterfaceKey)); constraint {
		m.Errorf(Constraint,
			"%s has type-set elements and only constrains type parameters, so it has no double", i.Name)
		return nil, false
	}
	set, _ := m.Rules().MembersOf(i)
	if !set.Complete() {
		m.Errorf(IncompleteMembers, "the members of %s are incomplete, so it has no double", i.Name)
		return nil, false
	}
	out := make([]*emit.Method, 0, len(set.Members))
	for _, member := range set.Members {
		switch s := member.Symbol.(type) {
		case *node.Method:
			out = append(out, completed(host, sdk.Mirror(host, s)))
		case *node.Field:
			m.Errorf(
				Property,
				"%s has the property %s, which a method of a double cannot implement, so it has no double",
				i.Name,
				s.Name,
			)
			return nil, false
		}
	}
	return out, true
}

// Copy returns the methods of the double named host of the emitted
// interface i. Each method is a copy of a method of i from [Signatures],
// with a receiver and a name for each parameter. It allocates what
// Signatures allocates, and the receiver and the names that it adds.
func Copy(host string, i *emit.Interface) []*emit.Method {
	out := Signatures(i.Methods.Items())
	for _, m := range out {
		completed(host, m)
	}
	return out
}

// Signatures returns a copy of the signature of each method of ms, in
// order. A copy has the origin, the documentation, the name, the
// visibility, the type parameters, the parameters, the results and the
// thrown types of its method. It has lists of parameters and results of
// its own, and shares the type references of ms, which the settle respells
// in place for both.
//
// A copy has no receiver and no body. A weaver appends into the methods
// with a receiver alone, so it passes over a copy.
//
// Signatures allocates each copy and its lists.
func Signatures(ms []*emit.Method) []*emit.Method {
	out := make([]*emit.Method, 0, len(ms))
	for _, m := range ms {
		c := &emit.Method{
			Origin:     m.Origin,
			Doc:        slices.Clone(m.Doc),
			Name:       m.Name,
			Visibility: m.Visibility,
			Async:      m.Async,
			TypeParams: slices.Clone(m.TypeParams),
			Throws:     slices.Clone(m.Throws),
		}
		for _, p := range m.Params.Items() {
			param := *p
			c.Params.Append(&param)
		}
		for _, r := range m.Returns.Items() {
			result := *r
			c.Returns.Append(&result)
		}
		out = append(out, c)
	}
	return out
}

// completed names each parameter of the method m that has no name through
// [naming.Params], renames each parameter and result with the name of a
// predeclared identifier of Go through [golang.RenamePredeclared], and gives
// m a receiver. host is the name of the double. The body of a double calls
// predeclared functions, such as append in a stub, and a parameter of the
// same name would hide the function. The receiver takes the first letter of
// host in lower case, or d where host does not start with a letter. Where m
// has that name already, the receiver takes the name followed by the
// smallest number from 0 that m does not have, as [naming.Unique] chooses.
//
// completed returns m. It allocates what naming.Params and
// golang.RenamePredeclared allocate, the set of the names of m and the
// receiver.
func completed(host string, m *emit.Method) *emit.Method {
	golang.RenamePredeclared(naming.Params(m))
	params, returns := m.Params.Items(), m.Returns.Items()
	taken := make(map[string]bool, len(m.TypeParams)+len(params)+len(returns))
	for _, tp := range m.TypeParams {
		taken[tp.Name] = true
	}
	for _, p := range params {
		taken[p.Name] = true
	}
	for _, r := range returns {
		taken[r.Name] = true
	}
	letter := receiverFallback
	if first, _ := utf8.DecodeRuneInString(host); unicode.IsLetter(first) {
		letter = strings.ToLower(string(first))
	}
	m.Receiver = &emit.Param{Name: naming.Unique(taken, letter), Type: &emit.TypeRef{Spelling: host}}
	return m
}
