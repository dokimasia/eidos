// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos_test

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
)

// kitSyntax answers the comment forms a fixture language declares.
func kitSyntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{Line: []string{"//"}}
}

// kitFile is the fixture's file skeleton, spelling a comment so a
// declared skeleton is distinguishable from the kit default.
const kitFile = "// {{.Name}}\n{{imports}}{{decls}}"

// kitStructs and kitCallables split the kind inventory across two
// declarations, the way a satellite groups its spellings; the
// struct spelling calls the shared helper so the vocabulary flow
// is held too.
func kitStructs() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct: "type {{up .Name}} struct{}\n",
	}
}

func kitCallables() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindFunction: "func {{.Name}}() {\n{{body .}}}\n",
	}
}

// kitFuncs answers the fixture's shared template vocabulary.
func kitFuncs() template.FuncMap {
	return template.FuncMap{"up": strings.ToUpper}
}

// kitNaming spells every unit as its word under a fixture
// extension.
func kitNaming(u plugin.Unit) string { return u.Word + ".txt" }

// kitScaffold spells the one statement kind the fixture emits,
// recording an import the way a real printer records what it
// qualifies with.
func kitScaffold(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
	if s.Kind != emit.StmtReturn {
		return nil, errors.New("the fixture spells returns only")
	}
	set.Add("stub/runtime")
	return []byte("\treturn\n"), nil
}

// kitImports renders the collected set as a one-line block.
func kitImports(set *render.ImportSet) string {
	if set.Len() == 0 {
		return ""
	}
	return "import (" + strings.Join(set.Paths(), " ") + ")\n"
}

// kitFinalise is the pass-through fixture formatter.
func kitFinalise(src []byte) ([]byte, error) { return src, nil }

// kitLanguage answers the same language as values, for the floor
// the kit's Build must lower to.
func kitLanguage() render.Language {
	kinds := kitStructs()
	maps.Copy(kinds, kitCallables())
	return render.Language{
		Kinds: kinds, File: kitFile, Funcs: kitFuncs(),
		Naming: kitNaming, Scaffold: kitScaffold,
		Imports: kitImports, Finalise: kitFinalise,
	}
}

// kitBackend answers the full fixture declaration, ready to Build
// or to break one piece of.
func kitBackend(name plugin.ID, target plugin.Target) *eidos.BackendBuilder {
	return eidos.NewBackend(name, target, kitSyntax()).
		FileTemplate(kitFile).
		KindTemplates(kitStructs()).
		KindTemplates(kitCallables()).
		Funcs(kitFuncs()).
		Naming(kitNaming).
		Scaffold(kitScaffold).
		Imports(kitImports).
		Finalise(kitFinalise)
}

// kitUnit answers one flushed plan unit holding a struct and a
// function whose body scaffolds a return.
func kitUnit() plugin.Unit {
	f := &emit.Function{
		Origin: coretest.Struct(coretest.StorePath, "Load").ID,
		Name:   "Load",
	}
	f.Body = emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}}
	return plugin.Unit{
		Plugin: "gen", Per: plugin.PerPlan, Word: "stub",
		Decls: []symbol.Symbol{
			&emit.Struct{
				Origin: coretest.Struct(coretest.StorePath, "Row").ID,
				Name:   "Row",
			},
			f,
		},
	}
}

// kitRender renders the fixture store through b's renderer seat
// and asserts the pass ran clean.
func kitRender(tb assert.TB, b plugin.Backend) []plugin.RenderedFile {
	tb.Helper()

	r, held := b.(plugin.Renderer)
	assert.True(tb, held, "a kit backend renders")
	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(kitUnit()), "the fixture unit lands")
	sink := diag.NewSink()
	files, err := r.Render(&plugin.RenderContext{
		Emit: e, Sink: sink, Plugin: "printer",
	})
	assert.NoError(tb, err, "the pass runs whole")
	assert.Equal(tb, len(slices.Collect(sink.All())), 0,
		"the clean fixture reports nothing")
	return files
}

// The backend kit is the second builder on the root package: the
// declaration is data, Build lowers it to the render pass, and
// the answered value carries every seat the plan validates.
func TestBackendBuilder(t *testing.T) {
	t.Parallel()

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("answers the backend, renderer and syntax seats", func(t *testing.T) {
			t.Parallel()

			b := kitBackend("printer", "stub").Build()
			assert.Equal(t, b.Name(), "printer", "the name is the identity")
			assert.Equal(t, b.Target(), "stub", "the target rides the declaration")
			_, renders := b.(plugin.Renderer)
			assert.True(t, renders, "the kit backend holds the renderer seat")
			carrier, held := b.(interface{ Syntax() plugin.CommentSyntax })
			assert.True(t, held,
				"the comment syntax is carried for the output contract")
			assert.Equal(t, carrier.Syntax(), kitSyntax(), "as declared")
		})

		t.Run("renders byte-identically to the pass it lowers to", func(t *testing.T) {
			t.Parallel()

			first := kitRender(t, kitBackend("printer", "stub").Build())
			second := kitRender(t, kitBackend("printer", "stub").Build())
			assert.Equal(t, first, second, "two builds render one output")

			assert.Equal(t, len(first), 1, "the fixture assembles one file")
			assert.Equal(t, first[0].Name, "stub.txt", "named by the naming")
			assert.Equal(t, string(first[0].Body),
				"// stub.txt\n"+
					"import (stub/runtime)\n"+
					"type ROW struct{}\n"+
					"func Load() {\n\treturn\n}\n",
				"the declared skeleton, vocabulary, kinds and scaffold all spell")

			pass, err := render.New("printer", kitLanguage())
			assert.NoError(t, err, "the same language composes at the floor")
			e := plugin.NewEmit()
			assert.NoError(t, e.Add(kitUnit()), "the fixture unit lands")
			floor, err := pass.Render(&plugin.RenderContext{
				Emit: e, Sink: diag.NewSink(), Plugin: "printer",
			})
			assert.NoError(t, err, "the floor pass runs whole")
			assert.Equal(t, first, floor, "the kit is spelling, not semantics")
		})

		t.Run("panics on a declaration defect", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name  string
				build func()
			}{
				{
					name:  "an empty name",
					build: func() { kitBackend("", "stub").Build() },
				},
				{
					name:  "a zero target",
					build: func() { kitBackend("printer", "").Build() },
				},
				{
					name: "one kind spelt twice",
					build: func() {
						kitBackend("printer", "stub").KindTemplates(kitStructs()).Build()
					},
				},
				{
					name: "one helper declared twice",
					build: func() {
						kitBackend("printer", "stub").Funcs(kitFuncs()).Build()
					},
				},
				{
					name: "a template that does not parse",
					build: func() {
						kitBackend("printer", "stub").
							KindTemplates(map[symbol.Kind]string{symbol.KindEnum: "{{"}).
							Build()
					},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					assert.Panics(t, tt.build,
						"a wrong declaration fires on the first Build in any test")
				})
			}
		})

		t.Run("panics with every kit defect in one bill", func(t *testing.T) {
			t.Parallel()

			recovered := assert.Panics(t, func() {
				kitBackend("printer", "stub").
					KindTemplates(kitStructs()).
					Funcs(kitFuncs()).
					Build()
			}, "collected defects fire together")
			text := fmt.Sprint(recovered)
			assert.Contains(t, text, "Struct kind twice", "the kind defect is named")
			assert.Contains(t, text, "helper twice", "the helper defect is named")
		})

		t.Run("panics with the language's whole bill", func(t *testing.T) {
			t.Parallel()

			recovered := assert.Panics(t, func() {
				eidos.NewBackend("printer", "stub", kitSyntax()).Build()
			}, "an empty language is a declaration defect")
			text := fmt.Sprint(recovered)
			for _, gap := range []string{
				"kinds", "filenames", "scaffolding", "import block", "formatter",
			} {
				assert.True(t, strings.Contains(text, gap),
					"the bill names every gap, "+gap+" included")
			}
		})
	})
}
