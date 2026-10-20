// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/internal/value"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/lang/spoke"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names that the lowering gives the types and the fields that it
// synthesizes. A payload states the results of a method, an entry the pair
// of a map, and a type of an input object ends in the suffix of an input.
const (
	payloadSuffix    = "Payload"
	entrySuffix      = "Entry"
	inputSuffix      = "Input"
	entryKeySuffix   = "Key"
	entryValueSuffix = "Value"
	keyField         = "key"
	valueField       = "value"
	resultPrefix     = "result"
)

// hostNames is the number of member names that a host records in place.
// A host records the names of a larger type in a list that it allocates.
const hostNames = 16

// caser converts the names of the types that the lowering synthesizes. It
// converts as [spell.Name] does, without initialisms, so the respell keeps
// each name.
var caser = naming.New()

// path contains the names after which the lowering names a type that it
// synthesizes for a member: the name of the type, the name of the member,
// and the name of a parameter of the member. The path of a type of entries
// contains the name of that type and the name of its field. An empty name
// adds nothing to the name of a type.
type path [3]string

// name returns the name of a type that the lowering synthesizes under the
// path p: each name of p in Pascal case, followed by suffix. It allocates
// the name, and each name of p that is not in Pascal case.
func (p path) name(suffix string) string {
	var b strings.Builder
	b.Grow(len(p[0]) + len(p[1]) + len(p[2]) + len(suffix))
	for _, part := range p {
		b.WriteString(caser.Pascal(part))
	}
	b.WriteString(suffix)
	return b.String()
}

// host is the context in which the lowering lowers the members of one type:
// the name and the origin of the type, whether it is an input object,
// whether it is the root of subscriptions, and whether its members receive
// it, as the extension members at file level do. out are the types that the
// members need, which the lowering declares beside the type.
type host struct {
	name     string
	origin   symbol.Identity
	input    bool
	streams  bool
	receives bool
	// names are the names of the first [hostNames] members so far, and
	// more are the names of the members after them. n is the number of
	// members so far.
	names [hostNames]string
	more  []string
	n     int
	out   []symbol.Symbol
}

// member records the name of a member, and refuses a name that the type
// declares already, because a type declares the name of a field or of an
// enum value once. It allocates nothing for the first [hostNames] members
// of a type.
func (h *host) member(name string) error {
	if slices.Contains(h.names[:min(h.n, hostNames)], name) || slices.Contains(h.more, name) {
		return fmt.Errorf(refusalPrefix+"a type declares the name of a member once, and %s declares %s twice",
			h.name, name)
	}
	if h.n < hostNames {
		h.names[h.n] = name
	} else {
		h.more = append(h.more, name)
	}
	h.n++
	return nil
}

// field lowers the field f of the type. A field has a type, takes a default
// in an input object alone, and receives no type inside a type. Its default
// is a constant value of GraphQL. The wire name of a field is a GraphQL name
// that does not start with [graphql.ReservedPrefix], because the settle
// keeps the name as the generator wrote it and a schema cannot declare any
// other name.
func (h *host) field(f *emit.Field) error {
	if _, err := checkAnnotations(symbol.KindField, f.Name, f.Annotations); err != nil {
		return err
	}
	if err := h.member(f.Name); err != nil {
		return err
	}
	switch {
	case f.Wire && (!naming.IsIdentifier(f.Name) || strings.HasPrefix(f.Name, graphql.ReservedPrefix)):
		return fmt.Errorf(refusalPrefix+"a schema declares a field by a GraphQL name without two leading "+
			"underscores, and the wire name %q of %s is not one", f.Name, h.name)
	case f.Receives != nil && !h.receives:
		return fmt.Errorf(refusalPrefix+"an extension of GraphQL is a block of its own, and %s receives %s inside %s",
			f.Name, f.Receives.Spelling, h.name)
	case f.Type == nil:
		return fmt.Errorf(refusalPrefix+"a field of GraphQL has a type, and %s of %s states none", f.Name, h.name)
	case f.Value != "" && !h.input:
		return fmt.Errorf(refusalPrefix+"a field of an object type takes no default, and %s of %s states %s",
			f.Name, h.name, f.Value)
	}
	if err := checkValue(f.Name, f.Value); err != nil {
		return err
	}
	t, err := h.lowerType(path{h.name, f.Name}, f.Type, h.input, true)
	if err != nil {
		return err
	}
	f.Type = t
	return nil
}

// method lowers the method m of the type into a field with arguments: each
// parameter is an argument, and the results are its type. A method of an
// input object, a method without a result and a method that receives a
// type inside a type are refused. Two or more results become one result of
// a payload type, [host.payload].
func (h *host) method(m *emit.Method) error {
	if _, err := checkAnnotations(symbol.KindMethod, m.Name, m.Annotations); err != nil {
		return err
	}
	if err := h.member(m.Name); err != nil {
		return err
	}
	results := m.Returns.Items()
	switch {
	case h.input:
		return fmt.Errorf(refusalPrefix+"an input field takes no arguments, "+
			"and the input object %s declares the method %s", h.name, m.Name)
	case m.Receives != nil && !h.receives:
		return fmt.Errorf(refusalPrefix+"an extension of GraphQL is a block of its own, and %s receives %s inside %s",
			m.Name, m.Receives.Spelling, h.name)
	case len(results) == 0:
		return fmt.Errorf(refusalPrefix+"a field of GraphQL has a type, and the method %s of %s returns nothing",
			m.Name, h.name)
	}
	if err := h.params(h.name, m.Name, m.Params.Items()); err != nil {
		return err
	}
	if len(results) > 1 {
		return h.payload(path{h.name, m.Name}, m)
	}
	if results[0].Type == nil {
		return fmt.Errorf(refusalPrefix+"a field of GraphQL has a type, and the result of %s of %s states none",
			m.Name, h.name)
	}
	t, err := h.lowerType(path{h.name, m.Name}, results[0].Type, false, true)
	if err != nil {
		return err
	}
	results[0].Type = t
	return nil
}

// params lowers the parameters ps of a field with arguments or of an
// executable definition. member is the name of the field or of the
// definition, and owner is the name of the type of the field, which is empty
// for a definition. A parameter has a name and a type, and its default is a
// constant value of GraphQL. An argument is an input, so a map is a list of
// an input object of entries. A positional variadic parameter is a list
// argument, and a keyword variadic parameter is refused.
func (h *host) params(owner, member string, ps []*emit.Param) error {
	for _, p := range ps {
		if _, err := checkAnnotations(symbol.KindParam, p.Name, p.Annotations); err != nil {
			return err
		}
		switch {
		case p.Name == "":
			return fmt.Errorf(refusalPrefix+"an argument of GraphQL has a name, "+
				"and a parameter of %s states none", member)
		case p.Type == nil:
			return fmt.Errorf(refusalPrefix+"an argument of GraphQL has a type, "+
				"and %s of %s states none", p.Name, member)
		case p.Variadic == symbol.VariadicKeyword:
			return fmt.Errorf(refusalPrefix+"GraphQL has no keyword arguments, and %s of %s takes them", p.Name, member)
		}
		if err := checkValue(p.Name, p.Default); err != nil {
			return err
		}
		t, err := h.lowerType(path{owner, member, p.Name}, p.Type, true, false)
		if err != nil {
			return err
		}
		if p.Variadic == symbol.VariadicPositional {
			t, p.Variadic = &emit.TypeRef{Spelling: t.Spelling, Form: symbol.FormList, Elems: []*emit.TypeRef{t}}, 0
		}
		p.Type = t
	}
	return nil
}

// payload lowers the two or more results of the method m into one result:
// a reference to the payload type that the lowering names after the path p
// and Payload, and declares with a field for each result. A named result
// gives its field its name, and an unnamed result is result followed by its
// position. Relay names the result type of a mutation after the mutation
// with the suffix Payload, and the name of the type in front keeps two
// payloads of one field name apart.
func (h *host) payload(p path, m *emit.Method) error {
	name := p.name(payloadSuffix)
	results := m.Returns.Items()
	fields := make([]*emit.Field, len(results))
	inner := &host{name: name, origin: h.origin}
	for k, r := range results {
		field := r.Name
		if field == "" {
			field = resultPrefix + strconv.Itoa(k+1)
		}
		fields[k] = &emit.Field{Name: field, Type: r.Type, Comment: r.Comment}
		if err := inner.field(fields[k]); err != nil {
			return err
		}
	}
	h.out = append(h.out, &emit.Struct{Origin: h.origin, Name: name, Fields: emit.SlotOf(fields...)})
	h.out = append(h.out, inner.out...)
	m.Returns.Replace(&emit.Return{Type: &emit.TypeRef{Spelling: name}})
	return nil
}

// lowerType lowers the type t of a member into the forms that GraphQL
// writes: a named reference, a nullable form, a list and, as the whole type
// of a field of the root of subscriptions, a stream. A map becomes a list of
// the type of entries that [host.entry] declares under the path p. top
// reports whether t is the whole type of a member. lowerType refuses a
// reference with type arguments, a nullable form of a nullable form, a form
// with the wrong number of children, a stream elsewhere, and every other
// form.
func (h *host) lowerType(p path, t *emit.TypeRef, input, top bool) (*emit.TypeRef, error) {
	switch t.Form {
	case symbol.FormNamed:
		if len(t.Args) > 0 {
			return nil, fmt.Errorf(refusalPrefix+"%s states type arguments, "+
				"and GraphQL has no generic types", t.Spelling)
		}
		return t, nil
	case symbol.FormOptional, symbol.FormList, symbol.FormStream:
		if len(t.Elems) != 1 {
			return nil, fmt.Errorf(refusalPrefix+"%s is %s of %d children, and the form has one", t.Spelling,
				spoke.Describe(rules.TypeShape{Form: t.Form, Async: t.Async}), len(t.Elems))
		}
		switch {
		case t.Form == symbol.FormOptional && t.Elems[0].Form == symbol.FormOptional:
			return nil, fmt.Errorf(refusalPrefix+"a GraphQL type has one nullability, and %s is optional twice",
				t.Spelling)
		case t.Form == symbol.FormStream && (!top || !h.streams || input):
			return nil, fmt.Errorf(refusalPrefix+"a field streams on the root of subscriptions alone, "+
				"and %s of %s is a stream", t.Spelling, h.name)
		}
		child, err := h.lowerType(p, t.Elems[0], input, false)
		if err != nil {
			return nil, err
		}
		t.Elems[0] = child
		return t, nil
	case symbol.FormMap:
		if len(t.Elems) != 2 {
			return nil, fmt.Errorf(refusalPrefix+"a map has a key and a value, and %s has %d children", t.Spelling,
				len(t.Elems))
		}
		return h.entry(p, t.Elems[0], t.Elems[1], input)
	default:
		return nil, fmt.Errorf(refusalPrefix+"%s is %s, which GraphQL does not spell", t.Spelling,
			spoke.Describe(rules.TypeShape{Form: t.Form, Async: t.Async}))
	}
}

// entry declares the type of the entries of a map of the keys key and the
// values value: an object type named after the path p and Entry, or an
// input object named after p and EntryInput in an input, with the fields
// key and value. It returns a list of the type, which GraphQL writes for a
// map.
func (h *host) entry(p path, key, val *emit.TypeRef, input bool) (*emit.TypeRef, error) {
	name := p.name(entrySuffix)
	k, err := h.lowerType(path{name, entryKeySuffix}, key, input, false)
	if err != nil {
		return nil, err
	}
	v, err := h.lowerType(path{name, entryValueSuffix}, val, input, false)
	if err != nil {
		return nil, err
	}
	e := &emit.Struct{
		Origin: h.origin, Name: name,
		Fields: emit.SlotOf(&emit.Field{Name: keyField, Type: k}, &emit.Field{Name: valueField, Type: v}),
	}
	if input {
		e.Name += inputSuffix
		e.Annotations = symbol.Annotations{{Name: string(graphql.InputMarker)}}
	}
	h.out = append(h.out, e)
	ref := &emit.TypeRef{Spelling: e.Name}
	return &emit.TypeRef{Spelling: e.Name, Form: symbol.FormList, Elems: []*emit.TypeRef{ref}}, nil
}

// checkValue refuses the default text of the member name where it is not
// empty and does not parse as a constant value of GraphQL.
func checkValue(name, text string) error {
	if text == "" {
		return nil
	}
	if _, err := value.Parse(text); err != nil {
		return fmt.Errorf(refusalPrefix+"a default is a value of GraphQL, and the default of %s is not: %w", name, err)
	}
	return nil
}
