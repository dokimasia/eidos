// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape/tools/registry"
)

// docLine is the width of the widest line of generated documentation. It
// is the width of the comment marker and a space, plus the width at which
// the generator wraps the text.
const docLine = len("// ") + 76

// The catalog path that the generated code imports, which the module of
// the catalog declares.
const catalogModule = "go.dokimi.dev/eidos/plugin/shape"

// The generated registry declares the summary keys, then for each spec
// its constants, its params struct, its reader and the handles of its
// keys, and finally Specs.
func TestDeclare(t *testing.T) {
	t.Parallel()

	t.Run("CatalogPath", func(t *testing.T) {
		t.Parallel()

		t.Run("is the module path of the catalog", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, registry.CatalogPath, catalogModule, "the generated wiring imports the catalog by its path")
		})
	})

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the summary keys first", func(t *testing.T) {
			t.Parallel()

			names := declaredNames(t, registryOf(t))
			assert.NotEmpty(t, names, "the registry declares something")
			assert.Equal(t, names[:5], []string{"KeyDetected", "KeyShape", "KeyMixed", "KeyMember", "KeyClassified"},
				"the registry opens with the summary keys")
		})

		t.Run("declares the constants, the struct, the reader and the handles of a shape", func(t *testing.T) {
			t.Parallel()

			want := []string{
				"ProbeReader",
				"ProbeReaderReads",
				"ProbeReaderKey",
				"ProbeReaderValue",
				"ProbeReaderParams",
				"ProbeReaderOf",
				"probeReaderShapeKey",
				"probeReaderKeyKey",
				"probeReaderValueKey",
				"probeReaderReadsKey",
			}
			assert.Equal(t, keptOf(declaredNames(t, registryOf(t)), want), want,
				"the shape has its name, its param, its bindings, its struct, its reader and its handles in order")
		})

		t.Run("declares the role constants and the id handle of a contract", func(t *testing.T) {
			t.Parallel()

			want := []string{
				"TxProbe",
				"TxProbeClosed",
				"TxProbeBegin",
				"TxProbeCommit",
				"TxProbeAbort",
				"TxProbeParams",
				"TxProbeOf",
				"txProbeRoleKey",
				"txProbeIDKey",
				"txProbeClosedKey",
			}
			assert.Equal(t, keptOf(declaredNames(t, registryOf(t)), want), want,
				"the contract has its name, its param, its roles, its struct, its reader and its handles in order")
		})

		t.Run("gives a contract its role and its id before its params", func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				fieldsOf(t, registryOf(t), "TxProbeParams"),
				[]string{"Role:Role", "ID:string", "Closed:symbol.Identity"},
				"the struct of the contract has the role, the id and the param",
			)
		})

		t.Run("types the field of each param by the type of the param", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldsOf(t, registryOf(t), "XSSSafeParams"), []string{
				"Limit:int64", "Mode:string", "Read:symbol.Identity", "Axis:symbol.Identity",
			}, "an int is an int64, a string a string, and a reference an identity")
		})

		t.Run("gives a shape its bindings before its params", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldsOf(t, registryOf(t), "ProbeReaderParams"), []string{
				"Key:symbol.Identity", "Value:symbol.Identity", "Reads:symbol.Identity",
			}, "the struct of the shape has the bindings and then the param")
		})

		t.Run("wraps every line of documentation within the width", func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, registryFile, registryOf(t), parser.ParseComments)
			assert.NoError(t, err, "the registry parses")
			assert.NotEmpty(t, file.Decls, "the registry declares something")
			last := file.Decls[len(file.Decls)-1].End()
			for _, group := range file.Comments {
				if group.Pos() < file.Package || group.Pos() > last {
					continue
				}
				for _, c := range group.List {
					expect.True(t, len(c.Text) <= docLine, "the line fits the width: "+c.Text)
				}
			}
		})
	})
}

// registryOf returns the registry that the run over the fixture catalog
// writes.
func registryOf(tb testing.TB) []byte {
	tb.Helper()

	files, findings := generated(tb, os.DirFS(fixtureRoot))
	assert.Empty(tb, findings, "the fixture catalog is valid")
	return files[registryFile]
}

// declaredNames returns the names of the top-level declarations of a Go
// source, in order.
func declaredNames(tb testing.TB, src []byte) []string {
	tb.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), registryFile, src, 0)
	assert.NoError(tb, err, "the source parses")
	var out []string
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			out = append(out, d.Name.Name)
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.ValueSpec:
					for _, n := range s.Names {
						out = append(out, n.Name)
					}
				case *ast.TypeSpec:
					out = append(out, s.Name.Name)
				}
			}
		}
	}
	return out
}

// fieldsOf returns the fields of a struct of a Go source, each as its name
// and its type joined by a colon, in order.
func fieldsOf(tb testing.TB, src []byte, name string) []string {
	tb.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, registryFile, src, 0)
	assert.NoError(tb, err, "the source parses")
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		spec, is := n.(*ast.TypeSpec)
		if !is || spec.Name.Name != name {
			return true
		}
		st, _ := spec.Type.(*ast.StructType)
		for _, f := range st.Fields.List {
			var typ strings.Builder
			assert.NoError(tb, printer.Fprint(&typ, fset, f.Type), "the type of the field prints")
			out = append(out, f.Names[0].Name+":"+typ.String())
		}
		return false
	})
	return out
}

// keptOf returns the names of a list that a second list has, in the order
// of the first.
func keptOf(names, kept []string) []string {
	var out []string
	for _, n := range names {
		if slices.Contains(kept, n) {
			out = append(out, n)
		}
	}
	return out
}
