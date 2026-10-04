// Copyright ThesmOS B.V. 2026
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
	// auditCall is a qualified callee whose head the scaffold records.
	auditCall = "audit.Log"
	auditHead = "audit"
)

// The builtins are the names every template resolves against, so
// what each records and returns, and that no vocabulary claims one,
// are contract.
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
	})
}

// use spells a use builtin over the given arguments, each quoted.
func use(args ...string) string {
	words := []string{render.BuiltinUse}
	for _, a := range args {
		words = append(words, strconv.Quote(a))
	}
	return action(words...)
}
