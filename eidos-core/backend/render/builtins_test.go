// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"strconv"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/symbol"
)

// The builtins' fixture: the paths a use records and the package a
// file belongs to.
const (
	// fmtPkg, alphaPkg and zetaPkg are paths a use records.
	fmtPkg   = "fmt"
	alphaPkg = "alpha"
	zetaPkg  = "zeta"
	// examplePkg is the package a file belongs to.
	examplePkg = "example.com/store"
	// auditCall is a qualified callee whose head the scaffold records,
	// and auditFunc is the name of the callee.
	auditCall = auditHead + "." + auditFunc
	auditHead = "audit"
	auditFunc = "Log"
	// sampleHelper is a vocabulary helper that returns a literal value of
	// the text sevenText.
	sampleHelper = "sample"
	sevenText    = "7"
)

// TestBuiltins checks what each builtin of the pass records and
// returns. It also checks that no vocabulary claims the name of a
// builtin.
func TestBuiltins(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a vocabulary helper named after a builtin", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Funcs = helpers(template.FuncMap{
				render.BuiltinNested: func(string, symbol.Symbol) (string, error) { return "", nil },
			})
			_, err := render.New(passName, l)
			assert.HasError(t, err, "the builtin names are the pass's own")
			assert.Contains(t, err.Error(), render.BuiltinNested, "naming the claim")
		})

		t.Run("returns an error for a vocabulary helper named header", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Funcs = helpers(template.FuncMap{
				render.BuiltinHeader: func() *emit.File { return nil },
			})
			_, err := render.New(passName, l)
			assert.HasError(t, err, "the header is a builtin of the pass")
			assert.Contains(t, err.Error(), render.BuiltinHeader, "naming the claim")
		})

		t.Run("returns an error for a vocabulary helper named value", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Funcs = helpers(template.FuncMap{
				render.BuiltinValue: func(emit.Value) (string, error) { return "", nil },
			})
			_, err := render.New(passName, l)
			assert.HasError(t, err, "the value builtin is a builtin of the pass")
			assert.Contains(t, err.Error(), render.BuiltinValue, "naming the claim")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("renders the import block above the declarations under the default skeleton", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindStruct] = use(fmtPkg) + structTpl
			files, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			coretest.AssertCodes(t, sink)
			assert.ContainsInOrder(t, string(files[0].Body),
				[]string{"import (" + fmtPkg + ")", "type " + alphaName + " struct{}"},
				"imports, then declarations")
		})

		t.Run("renders the skeleton over the file's package", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.File = "package {{.Pkg.Package}}\n\n" +
				action(render.BuiltinImports) + action(render.BuiltinDecls)
			u := unitOf(emitter, examplePkg+"/"+storeKey, alphaName)
			u.Pkg = coretest.PackageID(examplePkg)
			files, sink := runPass(t, l, seeded(t, u))
			coretest.AssertCodes(t, sink)
			assert.HasPrefix(t, string(files[0].Body), "package "+examplePkg+"\n",
				"the language spells its clause from the file's package")
		})

		t.Run("renders the imports the scaffold records", func(t *testing.T) {
			t.Parallel()

			body := emit.Body{Stmts: []emit.Stmt{call(auditCall)}}
			files, sink := runPass(t, language(), seeded(t, fn(storeKey, handleName, body)))
			coretest.AssertCodes(t, sink)
			assert.ContainsInOrder(t, string(files[0].Body),
				[]string{"import (" + auditHead + ")", auditCall + "()"},
				"the statement's qualifier is imported")
		})

		t.Run("records no import for a use of the file's own package", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindStruct] = use(examplePkg) + use(fmtPkg) + structTpl
			u := unitOf(emitter, examplePkg+"/"+storeKey, alphaName)
			u.Pkg = coretest.PackageID(examplePkg)
			files, sink := runPass(t, l, seeded(t, u))
			coretest.AssertCodes(t, sink)
			assert.Contains(t, string(files[0].Body), "import ("+fmtPkg+")\n",
				"the other package alone is imported")
		})

		t.Run("renders each recorded path once in path order", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindStruct] = use(zetaPkg) + use(alphaPkg) + use(zetaPkg) + structTpl
			files, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			coretest.AssertCodes(t, sink)
			assert.Contains(t, string(files[0].Body), "import ("+alphaPkg+" "+zetaPkg+")",
				"one mention per path, in path order")
		})

		t.Run("records the name a use binds beside its path", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Imports = namedImports
			l.Kinds[symbol.KindStruct] = use(storePkg, storeName) + structTpl
			files, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			coretest.AssertCodes(t, sink)
			assert.Contains(t, string(files[0].Body), "use "+storePkg+" as "+storeName+"\n",
				"the binding is rendered beside its path")
		})

		t.Run("reports RefusedTemplate for a use whose name another use binds", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Imports = namedImports
			l.TemplateImport = func(_, name string) (string, string, bool) { return name, "", name != "" }
			l.Kinds[symbol.KindStruct] = use(storePkg, storeName) + use(legacyPkg, storeName) + structTpl
			_, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), strconv.Quote(legacyPkg),
				"the finding names the import whose name the file binds already")
		})

		t.Run("records a use that binds no name outside the name assignment", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.TemplateImport = func(_, name string) (string, string, bool) { return name, "", name != "" }
			l.Kinds[symbol.KindStruct] = use(fmtPkg) + structTpl
			files, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			coretest.AssertCodes(t, sink)
			assert.Contains(t, string(files[0].Body), "import ("+fmtPkg+")", "the bare path is recorded")
		})

		t.Run("reports RefusedTemplate for a use that binds two names", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindStruct] = use(storePkg, storeName, rowName) + structTpl
			_, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), "one binding",
				"the finding names the rule")
		})

		t.Run("skips a declaration whose use binds two names", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindStruct] = use(storePkg, storeName, rowName) + structTpl
			files, _ := runPass(t, l, seeded(t, besideAlpha(fn(storeKey, handleName, emit.Body{}))))
			assert.NotContains(t, string(files[0].Body), alphaName,
				"the declaration is absent from the file")
		})

		t.Run("writes a value through the writer of the language in a kind template", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, sampling(), seeded(t, unitOf(emitter, storeKey, alphaName)))
			coretest.AssertCodes(t, sink)
			assert.Contains(t, string(files[0].Body), "type "+alphaName+" = "+sevenText+"\n",
				"the kind template writes the value of the helper")
		})

		t.Run("writes a value in a body template", func(t *testing.T) {
			t.Parallel()

			b := emit.Body{Ref: &emit.TemplateRef{Name: refName, Data: emit.Literal(emit.LiteralInt, sevenText)}}
			body, sink := renderRef(t, language(),
				refTree("\treturn "+action(render.BuiltinValue, ".Data")+"\n"+action(render.BuiltinSlots)), b)
			coretest.AssertCodes(t, sink)
			assert.Contains(t, body, "\treturn "+sevenText+"\n", "the body template writes the value of its data")
		})

		t.Run("writes a value in the template of a claimed file", func(t *testing.T) {
			t.Parallel()

			body, sink := renderClaim(t, claimTree(recordsRange+action(render.BuiltinValue, ".Data")+rangeEnd),
				claimedUnit(handleName, emit.Literal(emit.LiteralInt, sevenText), emit.Body{}))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, body, sevenText, "the file template writes the value of the record")
		})

		t.Run("records the imports of a value into the file", func(t *testing.T) {
			t.Parallel()

			callee := symbol.Identity{
				Lang: coretest.Lang, Package: auditHead, Name: auditFunc, Kind: symbol.KindFunction,
			}
			b := emit.Body{Ref: &emit.TemplateRef{Name: refName, Data: emit.Name(callee)}}
			body, sink := renderRef(t, language(),
				refTree("\t"+action(render.BuiltinValue, ".Data")+"()\n"+action(render.BuiltinSlots)), b)
			coretest.AssertCodes(t, sink)
			assert.ContainsInOrder(t, body, []string{"import (" + auditHead + ")", auditCall + "()"},
				"the import of the callee is in the import block of the file")
		})

		t.Run("reports RefusedTemplate for a value in a language without a writer", func(t *testing.T) {
			t.Parallel()

			l := sampling()
			l.Value = nil
			_, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), string(passName),
				"the finding names the language")
		})

		t.Run("reports UnspeltValue for a value that the writer refuses", func(t *testing.T) {
			t.Parallel()

			l := sampling()
			l.Funcs = helpers(template.FuncMap{
				sampleHelper: func() emit.Value { return emit.Literal(emit.LiteralInt, "") },
			})
			_, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			coretest.AssertReports(t, sink, render.UnspeltValue)
		})
	})
}

// sampling returns the fixture language with a vocabulary helper that
// returns a literal value of sevenText, and a struct spelling that writes
// the value through the value builtin.
func sampling() render.Language {
	l := language()
	l.Funcs = helpers(template.FuncMap{
		sampleHelper: func() emit.Value { return emit.Literal(emit.LiteralInt, sevenText) },
	})
	l.Kinds[symbol.KindStruct] = "type {{.Name}} = " + action(render.BuiltinValue, sampleHelper) + "\n"
	return l
}

// use spells a use builtin over the given arguments, each quoted.
func use(args ...string) string {
	words := append(make([]string, 0, 1+len(args)), render.BuiltinUse)
	for _, a := range args {
		words = append(words, strconv.Quote(a))
	}
	return action(words...)
}
