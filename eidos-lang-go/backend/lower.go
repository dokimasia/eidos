// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go/scanner"
	"go/token"
	"slices"
	"strconv"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/lowering"
	"go.dokimi.dev/eidos/lang/naming"
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

// Lower reshapes the constructs Go states in other declarations:
// an enum becomes a defined type and its constants, and a callable
// announcing failure types gains an error return, in place. Both
// consume their fact, so a second settle changes nothing, and
// everything else passes through unchanged, a sum included, whose
// unspelt kind the render reports.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	switch d := s.(type) {
	case *emit.Enum:
		return lowerEnum(d)
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
// camel form, and its type references the defined type. A constant
// declared alone cannot count through iota, so each value is
// spelled the way a Go constant group evaluates it: a variant
// stating no value repeats the last stated value, a variant before
// any stated value takes its ordinal, and every iota in a value is
// the variant's position. Every output names the enum's origin, and
// none restates the enum.
//
// An enum with fields or methods refuses: a constant group has no
// members, and Go declares no other closed value set.
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
		out = append(out, &emit.Constant{
			Origin:      e.Origin,
			Doc:         v.Doc,
			Comment:     v.Comment,
			Name:        e.Name + naming.Pascal(v.Name),
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
