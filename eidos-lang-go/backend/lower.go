// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go/scanner"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"unicode"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/lowering"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// underlying is the defined type an enum lowers over. A variant's
// stated value spells verbatim against it, so a generator stating
// values states int-shaped ones.
const underlying = "int"

// errorType is the return an announced failure lowers into, which
// is how Go declares one, and errorName is the name it takes beside
// named results.
const (
	errorType = "error"
	errorName = "err"
)

// iotaName spells Go's constant counter, which a constant declared
// alone evaluates to zero.
const iotaName = "iota"

// markerPrefix opens the name of a sum's marker method, and the sum's
// name follows it.
const markerPrefix = "is"

// The brackets of the type arguments that a variant's receiver restates.
const (
	argsOpen  = "["
	argsClose = "]"
)

// Lower reshapes the constructs Go states in other declarations: an
// enum becomes a defined type and its constants, a sum becomes an
// interface and one struct for each variant, and a callable announcing
// failure types gains an error return, in place. Each consumes its
// fact, so a second settle does not change the result. Everything else
// passes through unchanged.
//
// # Allocation contract
//
// A declaration that passes through allocates nothing. An enum
// allocates the list of outputs, the defined type and its target, and
// per variant the constant, its type and its joined name's two. A
// variant name without a lower-case letter adds its lower-cased form. A sum
// allocates what [lowerSum] states. A callable that announces failures
// allocates the error return, its type and the grown list of returns,
// and a struct allocates the receiver of each method it fills. A refusal
// allocates its error.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	switch d := s.(type) {
	case *emit.Enum:
		return lowerEnum(d)
	case *emit.Sum:
		return lowerSum(d)
	case *emit.Function:
		d.Returns, d.Throws = thrown(d.Returns, d.Throws), nil
	case *emit.Method:
		d.Returns, d.Throws = thrown(d.Returns, d.Throws), nil
	case *emit.Struct:
		if err := lowering.UniqueMethods(string(golang.Lang), d.Name, d.Methods.Items()); err != nil {
			return nil, err
		}
		lowerMembers(d.Methods.Items())
		receive(d.Name, d.Origin, d.TypeParams, d.Methods.Items())
	case *emit.Interface:
		if err := lowering.UniqueMethods(string(golang.Lang), d.Name, d.Methods.Items()); err != nil {
			return nil, err
		}
		lowerMembers(d.Methods.Items())
	}
	return nil, nil
}

// lowerMembers rewrites a host's member methods the way the
// file-level callables rewrite, because the lowering receives the
// host whole.
func lowerMembers(methods []*emit.Method) {
	for _, m := range methods {
		m.Returns, m.Throws = thrown(m.Returns, m.Throws), nil
	}
}

// receive gives a struct's member methods their receiver: Go
// states a method at the package level, so the template spells
// each one after its type and the receiver has to name that type.
// The reference names the struct's origin, so the settle
// respells receiver and type together, and a generic struct's type
// parameters as its arguments, because Go's receiver restates
// them. A method stating its own receiver keeps it, which is how a
// generator asks for a pointer or a named receiver.
func receive(name string, origin symbol.Identity, params []*emit.TypeParam, methods []*emit.Method) {
	for _, m := range methods {
		if m.Receiver != nil || m.Receives != nil {
			continue
		}
		m.Receives = &emit.TypeRef{Target: origin, Spelling: name}
		for _, p := range params {
			m.Receives.Args = append(m.Receives.Args, &emit.TypeRef{Spelling: p.Name})
		}
	}
}

// thrown appends the error return a callable's announced failure
// types lower into: one error whatever the count, because Go's
// failures are values of one interface, and a caller recovers the
// concrete types through errors.As. Where the other results are
// named, the error takes the first free name of err, err1, err2,
// because Go refuses a list mixing named and unnamed results. A
// callable announcing none keeps its returns untouched.
func thrown(returns []*emit.Return, throws []*emit.TypeRef) []*emit.Return {
	if len(throws) == 0 {
		return returns
	}
	failure := &emit.Return{Type: &emit.TypeRef{Spelling: errorType}}
	if slices.ContainsFunc(returns, func(r *emit.Return) bool { return r != nil && r.Name != "" }) {
		failure.Name = freeName(returns, errorName)
	}
	return append(returns, failure)
}

// freeName returns base where no result is named base, and base
// with the smallest positive number appended that no result is
// named otherwise.
func freeName(returns []*emit.Return, base string) string {
	taken := func(name string) bool {
		return slices.ContainsFunc(returns, func(r *emit.Return) bool { return r != nil && r.Name == name })
	}
	name := base
	for n := 1; taken(name); n++ {
		name = base + strconv.Itoa(n)
	}
	return name
}

// lowerEnum reshapes an enum into a defined type and one typed
// constant per variant: the type keeps the enum's name, each
// constant joins the type's name and its variant's in the neutral
// camel form, and its type references the defined type. Before the
// join, the lowering lower-cases a variant name without a lower-case
// letter. The join then title-cases each word, so the protobuf variant
// UNSPECIFIED of the enum phase becomes the constant phaseUnspecified.
//
// A constant declared alone cannot count through iota, so the lowering
// spells each value the way a Go constant group evaluates it:
//
//   - A variant without a stated value repeats the last stated value.
//   - A variant before any stated value takes its ordinal.
//   - Every iota in a value becomes the variant's position.
//
// Every output has the enum's origin, and the outputs replace the enum.
// The lowering refuses an enum with fields or methods, because a
// constant group has no members and Go does not declare another closed
// value set.
func lowerEnum(e *emit.Enum) ([]symbol.Symbol, error) {
	if e.Fields.Len() > 0 || e.Methods.Len() > 0 {
		return nil, refuse("a constant group has no members, and %s states some", e.Name)
	}
	variants := e.Variants.Items()
	out := make([]symbol.Symbol, 0, 1+len(variants))
	out = append(out, &emit.Alias{
		Origin:      e.Origin,
		Doc:         e.Doc,
		Comment:     e.Comment,
		Name:        e.Name,
		Visibility:  e.Visibility,
		Defined:     true,
		Target:      &emit.TypeRef{Spelling: underlying},
		Annotations: e.Annotations,
	})
	var last string
	for i, v := range variants {
		value := v.Value
		switch {
		case value != "":
			last = value
		case last != "":
			value = last
		default:
			value = strconv.Itoa(i)
		}
		value = atPosition(value, i)
		name := v.Name
		if !strings.ContainsFunc(name, unicode.IsLower) {
			// naming.Pascal returns an upper-case identifier unchanged, so
			// the join lowers it first and Pascal title-cases each word.
			name = strings.ToLower(name)
		}
		out = append(out, &emit.Constant{
			Origin:      e.Origin,
			Doc:         v.Doc,
			Comment:     v.Comment,
			Name:        e.Name + naming.Pascal(name),
			Visibility:  e.Visibility,
			Type:        &emit.TypeRef{Spelling: e.Name},
			Value:       value,
			Annotations: v.Annotations,
		})
	}
	return out, nil
}

// atPosition spells a constant expression with every iota token
// replaced by its position. The expression is scanned as Go
// tokens, so an iota inside a string or a longer identifier is
// kept as written.
func atPosition(expr string, position int) string {
	if !strings.Contains(expr, iotaName) {
		return expr
	}
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(expr))
	var s scanner.Scanner
	s.Init(file, []byte(expr), nil, 0)
	var b strings.Builder
	from := 0
	for {
		at, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.IDENT && lit == iotaName {
			offset := file.Offset(at)
			b.WriteString(expr[from:offset])
			b.WriteString(strconv.Itoa(position))
			from = offset + len(iotaName)
		}
	}
	b.WriteString(expr[from:])
	return b.String()
}

// lowerSum reshapes a sum into an interface and one struct for each
// variant, as protobuf-go generates a oneof. The interface keeps the
// sum's name, documentation, annotations and type parameters. Its one
// method is the marker: an unexported method named is and the sum's
// name, without parameters or results. A type of another package cannot
// declare that method, so only the variant structs and the types that
// embed one of them implement the interface.
//
// Each variant becomes a struct whose name joins the sum's name and the
// variant's name in the neutral camel form. The struct has the variant's
// documentation, annotations and fields. It also has a copy of the sum's
// type parameters. It implements the marker with an empty body and a
// pointer receiver. The receiver restates the type parameters as
// arguments. Its type references the sum's origin, so the settle
// respells the receiver with the struct. Every output has the sum's
// origin, and the outputs replace the sum.
//
// The lowering refuses a sum with methods, because no variant struct has
// a body for them. It refuses a variant field without a name, because a
// Go struct field has a name.
//
// # Allocation contract
//
// lowerSum allocates six times for the sum: the list of outputs, the
// interface, its marker method, the method's list and the two steps of
// the marker's name. Each variant adds ten: its struct, the two steps of
// its name, the marker method and its list, the receiver, the receiver's
// pointer type, the type's list of one element, the element and the
// pointer's spelling. A variant with fields adds the list of its fields.
// A sum of two variants, one of them with a field, allocates 27 times. A
// generic sum of n type parameters adds 2n+3 for each variant: the
// copied list of parameters and each copy with its bounds, the list of
// the receiver's arguments and each argument, and the spelling of the
// arguments. A refusal allocates its error.
func lowerSum(s *emit.Sum) ([]symbol.Symbol, error) {
	if s.Methods.Len() > 0 {
		return nil, refuse("a variant struct has no body for a sum's methods, and %s states some", s.Name)
	}
	marker := markerPrefix + naming.Pascal(s.Name)
	variants := s.Variants.Items()
	out := make([]symbol.Symbol, 0, 1+len(variants))
	iface := &emit.Interface{
		Origin:      s.Origin,
		Doc:         s.Doc,
		Comment:     s.Comment,
		Name:        s.Name,
		Visibility:  s.Visibility,
		TypeParams:  s.TypeParams,
		Annotations: s.Annotations,
	}
	iface.Methods.Append(&emit.Method{Name: marker, Visibility: symbol.VisibilityPackage})
	out = append(out, iface)
	for _, v := range variants {
		st, err := variantStruct(s, v, marker)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

// variantStruct builds the struct of one variant of a sum, with the
// marker method named marker.
func variantStruct(s *emit.Sum, v *emit.SumVariant, marker string) (*emit.Struct, error) {
	fields := v.Fields.Items()
	for _, f := range fields {
		if f.Name == "" {
			return nil, refuse("a struct field has a name, and a payload entry in %s states none", v.Name)
		}
	}
	st := &emit.Struct{
		Origin:      s.Origin,
		Doc:         v.Doc,
		Comment:     v.Comment,
		Name:        s.Name + naming.Pascal(v.Name),
		Visibility:  s.Visibility,
		TypeParams:  lowering.CopyTypeParams(s.TypeParams),
		Annotations: v.Annotations,
	}
	st.Fields.Append(fields...)
	host := &emit.TypeRef{Target: s.Origin, Spelling: st.Name}
	if len(s.TypeParams) > 0 {
		host.Args = make([]*emit.TypeRef, 0, len(s.TypeParams))
		for _, p := range s.TypeParams {
			host.Args = append(host.Args, &emit.TypeRef{Spelling: p.Name})
		}
	}
	st.Methods.Append(&emit.Method{
		Name:       marker,
		Visibility: symbol.VisibilityPackage,
		Receiver: &emit.Param{Type: &emit.TypeRef{
			Spelling: pointerMark + spellref.Spell(host, argsOpen, argsClose, Anonymous),
			Form:     symbol.FormOptional,
			Elems:    []*emit.TypeRef{host},
		}},
	})
	return st, nil
}
