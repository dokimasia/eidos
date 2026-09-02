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
// off the expression: the vocabulary the underlying-kind stamp
// carries.
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
		if builtins[t.Name] {
			return "basic"
		}
		return "named"
	default:
		return "named"
	}
}

// stampConstValues evaluates the file's package-level constants
// through the type checker's own machinery — the package's scope
// alone, imports stubbed, errors swallowed — and stamps the exact
// value of everything that still evaluated, iota arithmetic
// included. What crosses a package boundary stays unstamped:
// absence is unknown, never a wrong number.
func stampConstValues(
	u *plugin.SourceUnit, fset *token.FileSet, parsed *ast.File, file *node.File,
) {
	holdsConstants := false
	for _, decl := range file.Decls {
		switch decl.(type) {
		case *node.Constant, *node.Enum:
			holdsConstants = true
		}
	}
	if !holdsConstants {
		return
	}

	values := evaluate(fset, parsed)
	if len(values) == 0 {
		return
	}
	gb := u.Graph()
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

// evaluate runs the checker over one file and returns the exact
// values of the package-level constants it could settle.
func evaluate(fset *token.FileSet, parsed *ast.File) map[string]string {
	conf := types.Config{
		Error:       func(error) {},
		Importer:    stubImporter{},
		FakeImportC: true,
	}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}}
	pkg, _ := conf.Check(parsed.Name.Name, fset, []*ast.File{parsed}, info)
	if pkg == nil {
		return nil
	}
	out := map[string]string{}
	for ident, obj := range info.Defs {
		c, isConst := obj.(*types.Const)
		if !isConst || c.Val() == nil || obj.Parent() != pkg.Scope() {
			continue
		}
		if c.Val().Kind() == constant.Unknown {
			continue // an operand crossed the package: absent, never wrong
		}
		out[ident.Name] = c.Val().ExactString()
	}
	return out
}

// stubImporter refuses every import: the evaluation is the
// package's own scope by design.
type stubImporter struct{}

// Import refuses, so a constant crossing a package boundary stays
// unevaluated rather than wrong.
func (stubImporter) Import(string) (*types.Package, error) {
	return nil, errors.New("frontend: constant evaluation reads the package's own scope alone")
}
