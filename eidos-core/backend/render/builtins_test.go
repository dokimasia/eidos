// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/symbol"
)

// The builtins are the names every template resolves against, so
// what each records and returns, and that no vocabulary claims one,
// are contract.
func TestBuiltins(t *testing.T) {
	t.Parallel()

	t.Run("assembles the file through the skeleton", func(t *testing.T) {
		t.Parallel()

		t.Run("the default skeleton is imports then declarations", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindStruct] = "{{use \"fmt\"}}type {{.Name}} struct{}\n"
			files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
			assert.False(t, sink.Failed(), "a recorded import is valid")
			assert.ContainsInOrder(t, string(files[0].Body),
				[]string{"import (fmt)", "type Alpha struct{}"},
				"the block renders above the declarations")
		})

		t.Run("a file template spells its own clause", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.File = "package {{.Pkg.Package}}\n\n{{imports}}{{decls}}"
			u := unitOf("gen", "example.com/store/store.go", "Alpha")
			u.Pkg = coretest.PackageID("example.com/store")
			files, sink := runPass(t, l, seeded(t, u))
			assert.False(t, sink.Failed(), "the skeleton is valid")
			assert.HasPrefix(t, string(files[0].Body), "package example.com/store\n",
				"the language spells its clause off the owning package")
		})

		t.Run("the scaffold records what it qualifies", func(t *testing.T) {
			t.Parallel()

			body := emit.Body{Stmts: []emit.Stmt{call("audit.Log")}}
			files, sink := runPass(t, language(), seeded(t,
				fn("store.go", "Handle", body)))
			assert.False(t, sink.Failed(), "the qualified call is valid")
			assert.ContainsInOrder(t, string(files[0].Body),
				[]string{"import (audit)", "audit.Log()"},
				"spelling fed the file's one import set")
		})

		t.Run("a use of the file's own package imports nothing", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindStruct] = "{{use \"example.com/store\"}}{{use \"fmt\"}}type {{.Name}} struct{}\n"
			u := unitOf("gen", "example.com/store/store.go", "Alpha")
			u.Pkg = coretest.PackageID("example.com/store")
			files, sink := runPass(t, l, seeded(t, u))
			assert.False(t, sink.Failed(), "both uses are valid")
			assert.Contains(t, string(files[0].Body), "import (fmt)\n",
				"the file's own package is absent from its imports")
		})

		t.Run("imports dedupe and sort per file", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindStruct] = "{{use \"zeta\"}}{{use \"alpha\"}}{{use \"zeta\"}}type {{.Name}} struct{}\n"
			files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
			assert.False(t, sink.Failed(), "repeated uses are valid")
			assert.Contains(t, string(files[0].Body), "import (alpha zeta)",
				"one mention per path, in path order")
		})
	})

	t.Run("a vocabulary claiming the nested builtin is a fault", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Funcs = template.FuncMap{
			"nested": func(string, symbol.Symbol) (string, error) { return "", nil },
		}
		_, err := render.New("printer", l)
		assert.HasError(t, err, "the builtin names stay the pass's")
		assert.Contains(t, err.Error(), "builtin", "naming the claim")
	})

	t.Run("records the bindings a use declares", func(t *testing.T) {
		t.Parallel()

		bound := func() render.Language {
			l := language()
			l.Imports = func(set *render.ImportSet) string {
				var b strings.Builder
				for _, e := range set.Entries() {
					b.WriteString("use " + e.Path + " as " + e.Name + "\n")
				}
				return b.String()
			}
			return l
		}

		t.Run("a second argument records the name the import binds", func(t *testing.T) {
			t.Parallel()

			l := bound()
			l.Kinds[symbol.KindStruct] = "{{use \"svc/store\" \"Store\"}}type {{.Name}} struct{}\n"
			files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
			assert.False(t, sink.Failed(), "one binding per call is valid")
			assert.Contains(t, string(files[0].Body), "use svc/store as Store\n",
				"the binding reaches the block beside its path")
		})

		t.Run("more than one binding refuses the declaration", func(t *testing.T) {
			t.Parallel()

			l := bound()
			l.Kinds[symbol.KindStruct] = "{{use \"svc/store\" \"Store\" \"Row\"}}type {{.Name}} struct{}\n"
			files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), "one binding",
				"the refusal names the rule: one call records one binding")
			assert.NotContains(t, string(files[0].Body), "Alpha",
				"and the declaration is skipped rather than half-qualified")
		})
	})
}
