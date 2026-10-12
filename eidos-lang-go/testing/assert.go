// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The names that [AssertParallel] reads in a generated test file: the
// prefix of a test function, the package and the type of its parameter,
// and the methods that start a subtest and make a test parallel.
const (
	testPrefix     = "Test"
	testingPackage = "testing"
	testingType    = "T"
	runMethod      = "Run"
	parallelMethod = "Parallel"
)

// unnamedSubtest names a subtest whose name is not a string literal in the
// messages of [AssertParallel].
const unnamedSubtest = "a subtest"

// AssertVets runs go vet over the generated output under ctx, in a project
// that the Go adapter lays out. It is the assertion that Go adds to the
// set of the kernel. A refusal records a failure whose detail contains the
// error of go vet.
//
// Vet finds code that compiles and is still wrong, such as a format verb
// that does not match its argument, a lock copied by value or an
// unreachable return. A generator produces this class of defect most
// easily, because a template that assembles a call site correctly for one
// type can assemble it wrongly for the next.
func AssertVets(ctx context.Context, tb assert.TB, g toolchain.Generated) {
	tb.Helper()

	dir, done := toolchain.Prepare(tb, New(), g)
	if dir == "" {
		return
	}
	defer done()

	_, err := toolchain.Run(ctx, dir, goEnv, goBinary, "vet", allPackages)
	expect.NoError(tb, err, string(golang.Lang)+": go vet accepts the generated output")
}

// AssertParallel requires every test of the generated test files of g to
// call Parallel on its testing.T: each test function, and each subtest
// that a test starts through Run with a function literal. The call is a
// statement of the body of the test, so a call inside a nested function
// literal does not count for the test around it. A subtest that Run starts
// with a function value other than a literal is outside the check.
//
// A generated suite runs in the CI of every consumer, once for each
// generated declaration, so a test that runs serially slows every
// consumer. Each test without the call reports on its own, with its
// position. A fixture without a test fails, because a check over no test
// proves nothing. The check parses the files with the standard library and
// needs no toolchain. It reads the testing package under its own name.
func AssertParallel(tb assert.TB, g toolchain.Generated) {
	tb.Helper()

	fset := token.NewFileSet()
	tests := 0
	for _, path := range slices.Sorted(maps.Keys(g.Files)) {
		if !strings.HasSuffix(path, testSuffix) {
			continue
		}
		f, err := parser.ParseFile(fset, path, g.Files[path], parser.SkipObjectResolution)
		expect.NoError(tb, err, string(golang.Lang)+": the generated test file "+path+" parses")
		if err != nil {
			continue
		}
		for _, decl := range f.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || fn.Recv != nil || fn.Body == nil || !isTestName(fn.Name.Name) {
				continue
			}
			if param := testingParam(fn.Type); param != "" {
				tests += parallelTests(tb, fset, fn.Name.Name, param, fn.Body)
			}
		}
	}
	expect.NotEqual(tb, tests, 0, string(golang.Lang)+": the generated output declares a test")
}

// parallelTests checks that the test name, whose testing.T is param and
// whose body is body, calls Parallel, and then checks each subtest that
// the body starts through Run with a function literal. It returns the
// number of tests that it checked.
func parallelTests(tb assert.TB, fset *token.FileSet, name, param string, body *ast.BlockStmt) int {
	tb.Helper()

	expect.True(tb, callsParallel(body, param), string(golang.Lang)+": the generated test "+name+" at "+
		fset.Position(body.Lbrace).String()+" calls "+param+"."+parallelMethod)
	checked := 1
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			// A function literal is a scope of its own. A subtest literal is
			// checked from the call of Run that starts it.
			return false
		case *ast.CallExpr:
			lit, sub := subtest(x, param)
			if lit == nil {
				return true
			}
			label := unnamedSubtest
			if s, isLit := x.Args[0].(*ast.BasicLit); isLit && s.Kind == token.STRING {
				if unquoted, err := strconv.Unquote(s.Value); err == nil {
					label = unquoted
				}
			}
			checked += parallelTests(tb, fset, name+"/"+label, sub, lit.Body)
			return false
		}
		return true
	})
	return checked
}

// subtest returns the function literal and the name of its testing.T where
// call is param.Run(name, func(t *testing.T) {...}), and nil otherwise.
func subtest(call *ast.CallExpr, param string) (*ast.FuncLit, string) {
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel || sel.Sel.Name != runMethod || len(call.Args) != 2 {
		return nil, ""
	}
	if recv, isIdent := sel.X.(*ast.Ident); !isIdent || recv.Name != param {
		return nil, ""
	}
	lit, isLit := call.Args[1].(*ast.FuncLit)
	if !isLit {
		return nil, ""
	}
	sub := testingParam(lit.Type)
	if sub == "" {
		return nil, ""
	}
	return lit, sub
}

// callsParallel reports whether a statement of body calls param.Parallel.
func callsParallel(body *ast.BlockStmt, param string) bool {
	for _, stmt := range body.List {
		expr, isExpr := stmt.(*ast.ExprStmt)
		if !isExpr {
			continue
		}
		call, isCall := expr.X.(*ast.CallExpr)
		if !isCall || len(call.Args) != 0 {
			continue
		}
		sel, isSel := call.Fun.(*ast.SelectorExpr)
		if !isSel || sel.Sel.Name != parallelMethod {
			continue
		}
		if recv, isIdent := sel.X.(*ast.Ident); isIdent && recv.Name == param {
			return true
		}
	}
	return false
}

// testingParam returns the name of the one parameter of ft where its type
// is *testing.T, and the empty string otherwise.
func testingParam(ft *ast.FuncType) string {
	if ft.Params == nil || len(ft.Params.List) != 1 || len(ft.Params.List[0].Names) != 1 {
		return ""
	}
	field := ft.Params.List[0]
	star, isStar := field.Type.(*ast.StarExpr)
	if !isStar {
		return ""
	}
	sel, isSel := star.X.(*ast.SelectorExpr)
	if !isSel || sel.Sel.Name != testingType {
		return ""
	}
	if pkg, isIdent := sel.X.(*ast.Ident); !isIdent || pkg.Name != testingPackage {
		return ""
	}
	return field.Names[0].Name
}

// isTestName reports whether go test runs a function of the name as a
// test: the name is Test, or Test followed by a rune that is not a lower
// case letter.
func isTestName(name string) bool {
	rest, isTest := strings.CutPrefix(name, testPrefix)
	if !isTest {
		return false
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return rest == "" || !unicode.IsLower(r)
}
