// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package double

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
)

// receiverFallback is the receiver of a method of a double whose name does
// not start with a letter. paramPrefix starts the name that a parameter
// without a name takes.
const (
	receiverFallback = "d"
	paramPrefix      = "a"
)

// Match is the part of the match of an interface that [Of] reads. Of reads
// the rules of the language of the interface, and reports at the
// interface. [sdk.InterfaceMatch] satisfies it.
type Match interface {
	Rules() rules.Bound
	Errorf(c diag.Code, format string, a ...any)
}

// Of returns the methods of the double named host of the interface i,
// which m matched. Each method mirrors an effective method of i through
// [sdk.Mirror], and has a receiver and a name for each parameter. The rules
// of the language of i walk its effective members, so a method that an
// embedded interface contributes is a method of the double too.
//
// Of returns false after it reports [IncompleteMembers] at i where the walk
// has gaps. It also returns false after it reports [Property] at an
// interface with a property, because a method of a double cannot implement
// a property.
//
// It allocates the members of the walk, what sdk.Mirror allocates for each
// method, and the receiver and the names that it adds.
func Of(m Match, host string, i *node.Interface) ([]*emit.Method, bool) {
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
		for _, p := range m.Params {
			param := *p
			c.Params = append(c.Params, &param)
		}
		for _, r := range m.Returns {
			result := *r
			c.Returns = append(c.Returns, &result)
		}
		out = append(out, c)
	}
	return out
}

// completed names each parameter of the method m that has no name by its
// position, and gives m a receiver. host is the name of the double. The
// receiver takes the first letter of host in lower case, or d where host
// does not start with a letter. Where a parameter or a result of m has that
// name, the receiver takes the letter followed by the smallest number from
// 0 that no parameter or result has.
//
// completed returns m. It allocates the set of the names of m, the receiver
// and the names that it adds.
func completed(host string, m *emit.Method) *emit.Method {
	taken := make(map[string]bool, len(m.Params)+len(m.Returns))
	for i, p := range m.Params {
		if p.Name == "" {
			p.Name = paramPrefix + strconv.Itoa(i)
		}
		taken[p.Name] = true
	}
	for _, r := range m.Returns {
		taken[r.Name] = true
	}
	letter := receiverFallback
	if first, _ := utf8.DecodeRuneInString(host); unicode.IsLetter(first) {
		letter = strings.ToLower(string(first))
	}
	name := letter
	for n := 0; taken[name]; n++ {
		name = letter + strconv.Itoa(n)
	}
	m.Receiver = &emit.Param{Name: name, Type: &emit.TypeRef{Spelling: host}}
	return m
}
