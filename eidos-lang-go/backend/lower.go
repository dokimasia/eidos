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

// defaultUnderlying is the underlying type of the defined type of a
// lowered enum that states none. The lowering writes the value of each
// variant verbatim as a constant of the defined type.
const defaultUnderlying = "int"

// errorType is the type of the return that a declared failure lowers
// into, because Go returns a failure as an error value. errorName is the
// name of that return among named results.
const (
	errorType = "error"
	errorName = "err"
)

// iotaName is the name of the constant counter of Go. In a constant
// declared alone, iota is zero.
const iotaName = "iota"

// markerPrefix is the start of the name of the marker method of a sum.
// The name of the sum follows it.
const markerPrefix = "is"

// Lower rewrites the constructs that Go expresses through other
// declarations. An enum becomes a defined type and its constants, and a
// sum becomes an interface and one struct for each variant. A callable
// with declared failure types gets an error return, in place. Each
// rewrite removes the fact that it lowers, so a second settle does not
// change the result. Every other declaration passes through unchanged.
// Lower returns an error for a callable with failure types whose body has
// verbatim text, statements or a return statement in a slot, because the
// error return changes what each return statement returns.
//
// # Allocation contract
//
// A declaration that passes through allocates nothing. An enum allocates
// the list of outputs and the defined type, and the target of an enum
// without an underlying type. Each variant allocates the constant, its
// type and the two steps of its joined name. A variant name without a
// lower-case letter also allocates its lower-case form, and a variant
// whose doc opens with its name allocates the doc of its constant and the
// first line of that doc. A sum allocates what [lowerSum] states. A callable
// with declared failures allocates the error return, its type and the
// grown storage of its returns. A struct allocates the receiver of each
// method that it fills. A refusal allocates its error.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	switch d := s.(type) {
	case *emit.Enum:
		return lowerEnum(d)
	case *emit.Sum:
		return lowerSum(d)
	}
	if err := lowering.Callables(golang.Lang, s, throw); err != nil {
		return nil, err
	}
	if d, is := s.(*emit.Struct); is {
		receive(d.Name, d.Origin, d.TypeParams, d.Methods.Items())
	}
	return nil, nil
}

// receive sets the Receives type of the member methods of the struct
// name. Go declares a method at the package level, so the template
// renders each method after its type, and the receiver must refer to
// that type. The reference has the struct origin as its target, so the
// settle respells the receiver and the type together. A generic struct
// gets its type parameters as the arguments of the reference, because a
// Go receiver restates them. A method with its own receiver keeps it, so
// a generator can request a pointer or a named receiver.
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

// throw appends one error return to the returns of a callable with
// declared failure types. [lowering.Callables] calls it for each such
// callable and clears the failure types. A callable gets one error
// whatever the count, because Go failures are values of one interface,
// and a caller recovers the concrete types through errors.As. Where the
// other results have names, the error gets the first free name of err,
// err1 and err2, because Go refuses a result list with named and unnamed
// results. throw cannot fail, so it returns nil.
func throw(_ string, returns *emit.Slot[*emit.Return], _ []*emit.TypeRef) error {
	failure := &emit.Return{Type: &emit.TypeRef{Spelling: errorType}}
	rs := returns.Items()
	if slices.ContainsFunc(rs, func(r *emit.Return) bool { return r != nil && r.Name != "" }) {
		failure.Name = freeName(rs, errorName)
	}
	returns.Append(failure)
	return nil
}

// freeName returns base where no result has the name base. Otherwise it
// returns base followed by the smallest positive number that gives a
// name that no result has.
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

// lowerEnum rewrites the enum e into a defined type and one typed
// constant per variant. The type keeps the name of the enum, and its
// target is the underlying type of the enum, or int for an enum without
// one. The name of each constant joins the name of the type and the name
// of its variant in the neutral camel case, and its type refers to the
// defined type.
// Before the join, the lowering converts a variant name without a
// lower-case letter to lower case. The join then title-cases each word,
// so the protobuf variant UNSPECIFIED of the enum phase becomes the
// constant phaseUnspecified. Where the first word of the doc of a variant
// is the name of the variant, the doc of the constant opens with the name
// of the constant instead, as a Go doc comment opens with the name that it
// documents.
//
// A constant declared alone cannot count through iota, so the lowering
// writes each value as a Go constant group evaluates it:
//
//   - A variant without a value repeats the last value.
//   - A variant before the first value takes its ordinal.
//   - Every iota in a value becomes the position of the variant.
//
// Every output has the origin of the enum, and the outputs replace the
// enum. The lowering refuses an enum with fields or methods, because a
// constant group has no members and Go has no other closed value set.
func lowerEnum(e *emit.Enum) ([]symbol.Symbol, error) {
	if e.Fields.Len() > 0 || e.Methods.Len() > 0 {
		return nil, refuse("a constant group has no members, and %s states some", e.Name)
	}
	target := e.Underlying
	if target == nil {
		target = &emit.TypeRef{Spelling: defaultUnderlying}
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
		Target:      target,
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
		word := v.Name
		if !strings.ContainsFunc(word, unicode.IsLower) {
			// naming.Pascal returns an upper-case identifier unchanged, so
			// the join lowers it first and Pascal title-cases each word.
			word = strings.ToLower(word)
		}
		name := e.Name + naming.Pascal(word)
		doc := v.Doc
		if len(doc) > 0 {
			if rest, opens := strings.CutPrefix(doc[0], v.Name); opens && (rest == "" || rest[0] == ' ') {
				doc = slices.Clone(doc)
				doc[0] = name + rest
			}
		}
		out = append(out, &emit.Constant{
			Origin:      e.Origin,
			Doc:         doc,
			Comment:     v.Comment,
			Name:        name,
			Visibility:  e.Visibility,
			Type:        &emit.TypeRef{Spelling: e.Name},
			Value:       value,
			Annotations: v.Annotations,
		})
	}
	return out, nil
}

// atPosition returns the constant expression expr with every iota token
// replaced by position. It scans the expression as Go tokens, so an iota
// inside a string or a longer identifier stays as written.
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

// lowerSum rewrites the sum s into an interface and one struct for each
// variant, as protobuf-go generates a oneof. The interface keeps the
// name, the documentation, the annotations and the type parameters of
// the sum. Its one method is the marker, an unexported method without
// parameters or results whose name joins is and the name of the sum. A
// type of another package cannot declare that method, so only the
// variant structs and the types that embed one of them implement the
// interface.
//
// Each variant becomes a struct whose name joins the name of the sum and
// the name of the variant in the neutral camel case. The struct has the
// documentation, the annotations and the fields of the variant, and a
// copy of the type parameters of the sum. It implements the marker with
// an empty body and a pointer receiver. The receiver restates the type
// parameters as arguments. The type of the receiver refers to the origin
// of the sum, so the settle respells the receiver with the struct. Every
// output has the origin of the sum, and the outputs replace the sum.
//
// The lowering refuses a sum with methods, because no variant struct has
// a body for them. It refuses a variant field without a name, because a
// Go struct field has a name.
//
// # Allocation contract
//
// lowerSum allocates six times for the sum: the list of outputs, the
// interface, its marker method, the method list and the two steps of the
// name of the marker. Each variant adds ten allocations: its struct, the
// two steps of its name, the marker method and its list, the receiver,
// the pointer type of the receiver, the element list of that type, the
// element and the spelling of the pointer. A variant with fields adds
// the list of its fields. A sum of two variants, one of them with a
// field, allocates 27 times. A generic sum of n type parameters adds
// 2n+3 allocations for each variant: the copied parameter list and each
// copy with its bounds, the argument list of the receiver and each
// argument, and the spelling of the arguments. A refusal allocates its
// error.
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

// variantStruct returns the struct of the variant v of the sum s, with
// the marker method marker.
func variantStruct(s *emit.Sum, v *emit.SumVariant, marker string) (*emit.Struct, error) {
	if err := lowering.NamedPayload(golang.Lang, v); err != nil {
		return nil, err
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
	st.Fields.Append(v.Fields.Items()...)
	host := &emit.TypeRef{Target: s.Origin, Spelling: st.Name, Args: lowering.ParamArgs(s.TypeParams)}
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
