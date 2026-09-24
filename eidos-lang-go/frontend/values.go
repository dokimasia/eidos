// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"errors"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// underlyingKind classifies a defined type's target shape, read
// off the expression, in the underlying-kind stamp's vocabulary. A
// predeclared basic type is basic, and every other name, the
// predeclared interfaces included, is named.
func underlyingKind(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.ParenExpr:
		return underlyingKind(t.X)
	case *ast.StarExpr:
		return "pointer"
	case *ast.ArrayType:
		if t.Len == nil {
			return "slice"
		}
		return "array"
	case *ast.MapType:
		return "map"
	case *ast.ChanType:
		return "chan"
	case *ast.FuncType:
		return "func"
	case *ast.Ident:
		if golang.Basic(t.Name) {
			return "basic"
		}
		return "named"
	default:
		return "named"
	}
}

// stampConstValues evaluates one package's package-level constants
// through the type checker's own machinery — every included file
// together, imports stubbed, function bodies skipped, errors
// swallowed — and stamps the exact value of everything that still
// evaluated, iota arithmetic and cross-file references included.
// What crosses a package boundary is left unstamped: absence is
// unknown, never a wrong number.
func stampConstValues(u *plugin.SourceUnit, fset *token.FileSet, b *constBatch) {
	holdsConstants := false
	for _, file := range b.files {
		for _, decl := range file.Decls {
			switch decl.(type) {
			case *node.Constant, *node.Enum:
				holdsConstants = true
			}
		}
	}
	if !holdsConstants {
		return
	}

	values := literalValues(b.parsed)
	if values == nil {
		values = evaluate(fset, b.name, b.parsed)
	}
	if len(values) == 0 {
		return
	}
	gb := u.Graph()
	for _, file := range b.files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *node.Constant:
				if v, evaluated := values[d.Name]; evaluated {
					gb.Stamp(d, meta.RawStamp{Key: golang.ConstValueKey, Value: v, Pos: d.Pos})
				}
			case *node.Enum:
				for _, variant := range d.Variants {
					if v, evaluated := values[variant.Name]; evaluated {
						gb.Stamp(variant, meta.RawStamp{
							Key: golang.ConstValueKey, Value: v, Pos: variant.Pos,
						})
					}
				}
			}
		}
	}
}

// evaluate runs the checker once over the package's files and
// returns the exact values of the package-level constants it could
// settle. The package scope contains them all, so no checker maps
// are requested, and the files are pruned first: a constant reads
// types, imports and other constants alone, so functions and
// variables are never checked. A constant reading a variable's
// layout through unsafe is left absent, as the boundary contract
// states.
func evaluate(fset *token.FileSet, name string, parsed []*ast.File) map[string]string {
	conf := types.Config{
		Error:            func(error) {},
		Importer:         stubImporter{},
		FakeImportC:      true,
		IgnoreFuncBodies: true,
	}
	pruned := make([]*ast.File, 0, len(parsed))
	for _, f := range parsed {
		pruned = append(pruned, constFile(f))
	}
	pkg, _ := conf.Check(name, fset, pruned, nil)
	if pkg == nil {
		return nil
	}
	scope := pkg.Scope()
	names := scope.Names()
	out := make(map[string]string, len(names))
	for _, ident := range names {
		c, isConst := scope.Lookup(ident).(*types.Const)
		if !isConst || c.Val() == nil {
			continue
		}
		if c.Val().Kind() == constant.Unknown {
			continue // an operand crossed the package: absent, never wrong
		}
		out[ident] = c.Val().ExactString()
	}
	return out
}

// literalValues evaluates the package's constants without the
// checker when every spec is an untyped literal — one name, one
// literal value, no declared type — which is the shape most
// constants take. One spec outside that shape returns nil and the
// checker evaluates the package instead: a declared type converts
// the value, an iota row needs the group's arithmetic, and a
// reference needs resolution, none of which a literal read can do.
func literalValues(parsed []*ast.File) map[string]string {
	out := map[string]string{}
	for _, f := range parsed {
		for _, decl := range f.Decls {
			g, isGen := decl.(*ast.GenDecl)
			if !isGen || g.Tok != token.CONST {
				continue
			}
			for _, spec := range g.Specs {
				s, isValue := spec.(*ast.ValueSpec)
				if !isValue || s.Type != nil || len(s.Names) != len(s.Values) {
					return nil
				}
				for i, ident := range s.Names {
					v, literal := literalValue(s.Values[i])
					if !literal {
						return nil
					}
					if ident.Name != "_" {
						out[ident.Name] = v
					}
				}
			}
		}
	}
	return out
}

// literalValue evaluates one literal expression, a numeric sign
// admitted, and reports whether the expression is one.
func literalValue(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		exact := constant.MakeFromLiteral(v.Value, v.Kind, 0)
		if exact.Kind() == constant.Unknown {
			return "", false
		}
		return exact.ExactString(), true
	case *ast.UnaryExpr:
		lit, isLit := v.X.(*ast.BasicLit)
		if !isLit || (v.Op != token.ADD && v.Op != token.SUB) {
			return "", false
		}
		if lit.Kind != token.INT && lit.Kind != token.FLOAT && lit.Kind != token.IMAG {
			return "", false
		}
		exact := constant.MakeFromLiteral(lit.Value, lit.Kind, 0)
		if exact.Kind() == constant.Unknown {
			return "", false
		}
		return constant.UnaryOp(v.Op, exact, 0).ExactString(), true
	default:
		return "", false
	}
}

// constFile returns a shallow copy of one file with only what
// constant evaluation can read: imports, type declarations and
// constant groups.
func constFile(f *ast.File) *ast.File {
	kept := make([]ast.Decl, 0, len(f.Decls))
	for _, d := range f.Decls {
		g, isGen := d.(*ast.GenDecl)
		if !isGen || g.Tok == token.VAR {
			continue
		}
		kept = append(kept, d)
	}
	return &ast.File{
		Name: f.Name, Package: f.Package, Decls: kept,
		FileStart: f.FileStart, FileEnd: f.FileEnd, Imports: f.Imports,
	}
}

// stubImporter refuses every import: the evaluation is the
// package's own scope by design.
type stubImporter struct{}

// Import refuses, so a constant crossing a package boundary is left
// unevaluated, never wrong.
func (stubImporter) Import(string) (*types.Package, error) {
	return nil, errors.New("frontend: constant evaluation reads the package's own scope alone")
}
