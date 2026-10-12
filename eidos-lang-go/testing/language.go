// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"errors"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/nodetest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The syntax of an inline body that [Language] writes on one line, of a
// list of types, of a variadic parameter and of the close of a type
// argument list.
const (
	structWord    = "struct"
	interfaceWord = "interface"
	funcWord      = "func"
	bodyOpen      = "{ "
	bodyClose     = " }"
	emptyBody     = "{}"
	memberSep     = "; "
	listSep       = ", "
	fieldSep      = " "
	variadicMark  = "..."
	typeArgsClose = "]"
)

// errMixedBody is the refusal of an inline body with both fields and
// methods, which Go writes as neither a struct nor an interface.
var errMixedBody = errors.New("testing: an inline body of Go has fields or methods, and not both")

// Language is the spelling adapter of Go for a fixture of nodetest. It
// writes each composite type through [golang.Restate], the writer of the
// composite syntax of Go that the spoke and the backend use, and each
// inline body on one line, with its members separated by semicolons. A
// reference to a type of another package has the name of the package and
// a dot before the name of the type. Go declares no overloads.
//
// # Concurrency
//
// A Language is safe for concurrent use, because it has no state.
type Language struct{}

// Language is the spelling adapter of Go.
var _ nodetest.Language = Language{}

// Lang returns the identity of Go.
func (Language) Lang() symbol.Lang { return golang.Lang }

// Overloads reports false, because Go declares one callable of a name in a
// scope.
func (Language) Overloads() bool { return false }

// Spell writes ref in the syntax of Go. It writes a pointer, a slice, an
// array, a map, a channel and a function type through [golang.Restate].
// It writes an inline body without members as struct{}, one with fields as
// a struct, and one with methods as an interface. A child with type
// arguments has them in the brackets of Go.
//
// Error modes: an inline body with both fields and methods returns an
// error, and every form that [golang.Restate] refuses, such as a tuple or a
// union, returns its error.
func (Language) Spell(ref *node.TypeRef) (string, error) {
	if ref.Form == symbol.FormInline {
		return inline(ref)
	}
	elems := make([]string, 0, len(ref.Elems))
	for _, elem := range ref.Elems {
		elems = append(elems, spelled(elem))
	}
	return golang.Restate(&emit.TypeRef{Form: ref.Form, Split: ref.Split, Length: ref.Length}, elems)
}

// Qualify returns the name of pkg, a dot and name.
func (Language) Qualify(pkg *node.Package, name string) string {
	return pkg.Name + qualifierSep + name
}

// inline writes an inline body on one line: struct{} without members, a
// struct of its fields, or an interface of its methods. It returns
// errMixedBody for a body with both.
func inline(ref *node.TypeRef) (string, error) {
	switch {
	case len(ref.Fields) > 0 && len(ref.Methods) > 0:
		return "", errMixedBody
	case len(ref.Methods) > 0:
		members := make([]string, 0, len(ref.Methods))
		for _, m := range ref.Methods {
			signature, err := methodSignature(m)
			if err != nil {
				return "", err
			}
			members = append(members, m.Name+signature)
		}
		return interfaceWord + bodyOpen + strings.Join(members, memberSep) + bodyClose, nil
	case len(ref.Fields) > 0:
		members := make([]string, 0, len(ref.Fields))
		for _, f := range ref.Fields {
			members = append(members, f.Name+fieldSep+spelled(f.Type))
		}
		return structWord + bodyOpen + strings.Join(members, memberSep) + bodyClose, nil
	default:
		return structWord + emptyBody, nil
	}
}

// methodSignature writes the parameters and the results of a method of an
// inline interface, as [golang.Restate] writes a function type without its
// keyword. A positional variadic parameter has the variadic mark before its
// type.
func methodSignature(m *node.Method) (string, error) {
	types := make([]string, 0, len(m.Params)+len(m.Returns))
	for _, p := range m.Params {
		t := spelled(p.Type)
		if p.Variadic == symbol.VariadicPositional {
			t = variadicMark + t
		}
		types = append(types, t)
	}
	for _, r := range m.Returns {
		types = append(types, spelled(r.Type))
	}
	fn, err := golang.Restate(&emit.TypeRef{Form: symbol.FormFunc, Split: len(m.Params)}, types)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(fn, funcWord), nil
}

// spelled returns the spelling of a child with its type arguments in the
// brackets of Go, and the empty string for a nil reference.
func spelled(ref *node.TypeRef) string {
	if ref == nil {
		return ""
	}
	if len(ref.Args) == 0 {
		return ref.Spelling
	}
	args := make([]string, 0, len(ref.Args))
	for _, arg := range ref.Args {
		args = append(args, spelled(arg))
	}
	return ref.Spelling + typeArgsOpen + strings.Join(args, listSep) + typeArgsClose
}
