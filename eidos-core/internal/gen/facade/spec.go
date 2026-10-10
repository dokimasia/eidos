// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"strconv"

	"go.dokimi.dev/eidos/core/internal/genfile"
)

// The three things a facade spec pins, and the case each one names.
const (
	// typeCase pins an alias to the kernel's type.
	typeCase = "is the kernel's type"
	// valueCase pins a constant or a variable to the kernel's value.
	valueCase = "equals the kernel's value"
	// signatureCase pins a wrapper to the kernel function's signature.
	signatureCase = "has the kernel function's signature"
)

// specCase is one pinned re-export: the symbol's name, the case's
// name, and the assertion the case runs.
type specCase struct {
	symbol    string
	name      string
	assertion string
	// reflects reports whether the assertion reads types through the
	// reflect package, which the spec then imports.
	reflects bool
}

// renderSpec renders the black-box spec of one facade package. The
// spec pins every re-export to its kernel counterpart: an alias names
// the kernel's own type, a constant or a variable equals the kernel's
// value, and a wrapper has the kernel function's signature.
//
// A generic type or function has no type to compare until it is
// instantiated, and its type parameters' constraints decide what can
// instantiate it, so the spec leaves it out. A constraint interface
// names a type set that only a type parameter's constraint can use, so
// the spec leaves it out too. The compiler checks a generic wrapper's
// forwarding call where the facade declares it.
//
// It returns an error for a package that re-exports nothing the spec
// can pin.
func renderSpec(ps *PackageSurface) ([]byte, error) {
	var cases []specCase
	for _, f := range ps.Files {
		for _, d := range f.Decls {
			cases = append(cases, specCasesOf(ps.Name, d)...)
		}
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("facade: %s re-exports nothing the spec can pin", ps.KernelPath)
	}

	reflects := false
	for _, c := range cases {
		reflects = reflects || c.reflects
	}
	var out bytes.Buffer
	out.WriteString(genfile.Header(generatorName))
	fmt.Fprintf(&out, "package %s_test\n\nimport (\n", ps.Name)
	if reflects {
		out.WriteString("\t\"reflect\"\n")
	}
	fmt.Fprintf(&out, "\t\"testing\"\n\n\t\"go.dokimi.dev/assert\"\n\n\t%s %s\n\t%s\n)\n\n",
		counterpartAlias, strconv.Quote(ps.KernelPath), strconv.Quote(ps.FacadeImportPath()))
	fmt.Fprintf(&out, "// The facade re-exports %s: every type is the kernel's\n", ps.KernelPath)
	out.WriteString("// own type, every constant and variable equals the kernel's value,\n" +
		"// and every wrapper has the kernel function's signature.\n" +
		"func TestFacade(t *testing.T) {\n\tt.Parallel()\n")
	for _, c := range cases {
		fmt.Fprintf(&out, "\n\tt.Run(%q, func(t *testing.T) {\n\t\tt.Parallel()\n\n", c.symbol)
		fmt.Fprintf(&out, "\t\tt.Run(%q, func(t *testing.T) {\n\t\t\tt.Parallel()\n\n", c.name)
		fmt.Fprintf(&out, "\t\t\t%s\n\t\t})\n\t})\n", c.assertion)
	}
	out.WriteString("}\n")
	return out.Bytes(), nil
}

// specCasesOf returns the cases of one top-level declaration: one for
// each exported, non-generic name it declares. A method has no case of
// its own, because the alias of its receiver's type carries it.
func specCasesOf(facadeName string, d ast.Decl) []specCase {
	switch d := d.(type) {
	case *ast.FuncDecl:
		if d.Recv != nil || !ast.IsExported(d.Name.Name) || d.Type.TypeParams != nil {
			return nil
		}
		return []specCase{{
			symbol: d.Name.Name,
			name:   signatureCase,
			assertion: fmt.Sprintf(
				"assert.Equal(t, reflect.TypeOf(%[1]s.%[2]s), reflect.TypeOf(%[3]s.%[2]s), %[4]q)",
				facadeName, d.Name.Name, counterpartAlias, "the wrapper has the kernel function's signature"),
			reflects: true,
		}}
	case *ast.GenDecl:
		switch d.Tok {
		case token.TYPE:
			return typeCases(facadeName, d)
		case token.CONST, token.VAR:
			return valueCases(facadeName, d)
		default:
			// An import declaration declares no name of the facade.
		}
	}
	return nil
}

// typeCases returns a case for every exported type a type declaration
// names that is neither generic nor a constraint interface.
func typeCases(facadeName string, d *ast.GenDecl) []specCase {
	var out []specCase
	for _, spec := range d.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok || !ast.IsExported(ts.Name.Name) || ts.TypeParams != nil || constraint(ts.Type) {
			continue
		}
		out = append(out, specCase{
			symbol: ts.Name.Name,
			name:   typeCase,
			assertion: fmt.Sprintf(
				"assert.Equal(t, reflect.TypeFor[%[1]s.%[2]s](), reflect.TypeFor[%[3]s.%[2]s](), %[4]q)",
				facadeName, ts.Name.Name, counterpartAlias, "the facade aliases the kernel's type"),
			reflects: true,
		})
	}
	return out
}

// constraint reports whether a type expression is a constraint
// interface: an interface whose elements include a union, an
// approximation or the predeclared comparable, which declares a type
// set rather than a method set.
func constraint(typ ast.Expr) bool {
	it, ok := typ.(*ast.InterfaceType)
	if !ok || it.Methods == nil {
		return false
	}
	for _, f := range it.Methods.List {
		if len(f.Names) > 0 {
			continue
		}
		switch e := f.Type.(type) {
		case *ast.BinaryExpr:
			return true
		case *ast.UnaryExpr:
			if e.Op == token.TILDE {
				return true
			}
		case *ast.Ident:
			if e.Name == comparableName {
				return true
			}
		}
	}
	return false
}

// comparableName is the predeclared constraint that admits every
// comparable type.
const comparableName = "comparable"

// valueCases returns a case for every exported name a constant or
// variable declaration names.
func valueCases(facadeName string, d *ast.GenDecl) []specCase {
	var out []specCase
	for _, spec := range d.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, name := range vs.Names {
			if !ast.IsExported(name.Name) {
				continue
			}
			out = append(out, specCase{
				symbol: name.Name,
				name:   valueCase,
				assertion: fmt.Sprintf("assert.Equal(t, %[1]s.%[2]s, %[3]s.%[2]s, %[4]q)",
					facadeName, name.Name, counterpartAlias, "the facade re-declares the kernel's value"),
			})
		}
	}
	return out
}
