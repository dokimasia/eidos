// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// kitSyntax returns the comment forms a fixture language declares.
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

// kitFuncs returns the fixture's shared template vocabulary.
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

// kitLanguage returns the same language as values, for the
// hand-built pass the kit's Build must lower to.
func kitLanguage() render.Language {
	kinds := kitStructs()
	maps.Copy(kinds, kitCallables())
	return render.Language{
		Kinds: kinds, File: kitFile, Funcs: kitFuncs(),
		Naming: kitNaming, Scaffold: kitScaffold,
		Imports: kitImports, Finalise: kitFinalise,
	}
}

// kitBackend returns the full fixture declaration, ready to Build
// or to break one piece of.
func kitBackend(name plugin.ID, target plugin.Target) *backend.Builder {
	return backend.New(name, target, kitSyntax()).
		FileTemplate(kitFile).
		KindTemplates(kitStructs()).
		KindTemplates(kitCallables()).
		Funcs(kitFuncs()).
		Naming(kitNaming).
		Scaffold(kitScaffold).
		Imports(kitImports).
		Finalise(kitFinalise)
}

// kitUnit returns one flushed plan unit holding a struct and a
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

// kitVersion is the version the fixture declares, so a built
// backend's answer is distinguishable from the undeclared empty
// one.
const kitVersion = "3.2.1"

// cursorSuffix names the companion declaration the fixture's
// lowering produces beside every struct.
const cursorSuffix = "Cursor"

// respellPrefix is what the fixture's name convention puts in
// front of every declared name.
const respellPrefix = "t_"

// kitLower is the fixture's construct lowering: a struct arrives
// as itself and a companion carrying the same origin, and every
// other declaration passes through untouched.
func kitLower(s symbol.Symbol) ([]symbol.Symbol, error) {
	row, held := s.(*emit.Struct)
	if !held {
		return nil, nil
	}
	return []symbol.Symbol{row, &emit.Struct{
		Origin: row.Origin, Name: row.Name + cursorSuffix,
	}}, nil
}

// kitRespell is the fixture's name convention: every declared name
// takes the prefix, whatever kind carries it.
func kitRespell(_, _ symbol.Kind, _ symbol.Visibility, name string) (string, error) {
	return respellPrefix + name, nil
}

// structNames reads the declared names off a lowering's answer,
// which the fixture hook returns as structs alone.
func structNames(tb assert.TB, decls []symbol.Symbol) []string {
	tb.Helper()

	out := make([]string, 0, len(decls))
	for _, d := range decls {
		s, held := d.(*emit.Struct)
		assert.True(tb, held, "the fixture lowering answers with structs")
		if !held {
			continue
		}
		out = append(out, s.Name)
	}
	return out
}

// kitStore returns the fixture store, unsettled.
func kitStore(tb assert.TB) *plugin.Emit {
	tb.Helper()

	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(kitUnit()), "the fixture unit arrives")
	return e
}

// kitUnsettled renders the fixture store through b without
// settling it first, and returns what the render answered.
func kitUnsettled(tb assert.TB, b plugin.Backend) error {
	tb.Helper()

	r, held := b.(plugin.Renderer)
	assert.True(tb, held, "a kit backend renders")
	_, err := r.Render(&plugin.RenderContext{
		Emit: kitStore(tb), Sink: diag.NewSink(), Plugin: "printer",
	})
	return err
}

// kitSettled settles the fixture store through b's declared seams
// and renders it, asserting both steps run clean.
func kitSettled(tb assert.TB, b plugin.Backend) []plugin.RenderedFile {
	tb.Helper()

	r, held := b.(plugin.Renderer)
	assert.True(tb, held, "a kit backend renders")
	e := kitStore(tb)
	sink := diag.NewSink()
	assert.NoError(tb, plugin.Settle(e, b, sink), "the plan settles once")
	coretest.AssertCodes(tb, sink)
	files, err := r.Render(&plugin.RenderContext{
		Emit: e, Sink: sink, Plugin: "printer",
	})
	assert.NoError(tb, err, "and the settled store renders")
	coretest.AssertCodes(tb, sink)
	return files
}

// kitRender renders the fixture store through b's renderer role
// and asserts the pass ran clean.
func kitRender(tb assert.TB, b plugin.Backend) []plugin.RenderedFile {
	tb.Helper()

	r, held := b.(plugin.Renderer)
	assert.True(tb, held, "a kit backend renders")
	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(kitUnit()), "the fixture unit arrives")
	sink := diag.NewSink()
	files, err := r.Render(&plugin.RenderContext{
		Emit: e, Sink: sink, Plugin: "printer",
	})
	assert.NoError(tb, err, "the pass runs whole")
	assert.Equal(tb, len(slices.Collect(sink.All())), 0,
		"the clean fixture reports nothing")
	return files
}

// The backend kit is the write side's authoring builder: the
// declaration is data, Build lowers it to the render pass, and
// the returned value carries every role the plan validates.
func TestBackend(t *testing.T) {
	t.Parallel()

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the backend, renderer and syntax roles", func(t *testing.T) {
			t.Parallel()

			b := kitBackend("printer", "stub").Build()
			assert.Equal(t, b.Name(), "printer", "the name is the identity")
			assert.Equal(t, b.Target(), "stub", "the target is carried as declared")
			_, renders := b.(plugin.Renderer)
			assert.True(t, renders, "the kit backend holds the renderer role")
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
			assert.NoError(t, err, "the same language composes by hand")
			e := plugin.NewEmit()
			assert.NoError(t, e.Add(kitUnit()), "the fixture unit arrives")
			direct, err := pass.Render(&plugin.RenderContext{
				Emit: e, Sink: diag.NewSink(), Plugin: "printer",
			})
			assert.NoError(t, err, "the hand-built pass runs whole")
			assert.Equal(t, first, direct, "the kit is spelling, not semantics")
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
				{
					name: "one group spelt twice",
					build: func() {
						kitBackend("printer", "stub").
							Groups(map[render.GroupName]string{"block": "types\n"}).
							Groups(map[render.GroupName]string{"block": "again\n"}).
							Build()
					},
				},
				{
					name: "a cluster without group templates",
					build: func() {
						kitBackend("printer", "stub").
							Cluster(func([]symbol.Symbol) []render.Clustered {
								return nil
							}).
							Build()
					},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					assert.Panics(t, tt.build,
						"a wrong declaration panics on the first Build in any test")
				})
			}
		})

		t.Run("lowers the split to the pass", func(t *testing.T) {
			t.Parallel()

			b := kitBackend("printer", "stub").
				Split(func(u plugin.Unit) []plugin.Unit {
					out := make([]plugin.Unit, 0, len(u.Decls))
					for i, d := range u.Decls {
						su := u
						su.Decls = u.Decls[i : i+1]
						su.Word = strings.ToLower(d.Kind().String())
						out = append(out, su)
					}
					return out
				}).
				Build()
			files := kitRender(t, b)
			names := make([]string, 0, len(files))
			for _, f := range files {
				names = append(names, f.Name)
			}
			assert.Equal(t, names, []string{"function.txt", "struct.txt"},
				"the declared split reshapes units before the naming")
		})

		t.Run("declares the settle seams and refuses an unsettled store", func(t *testing.T) {
			t.Parallel()

			b := kitBackend("printer", "stub").
				Lower(func(s symbol.Symbol) ([]symbol.Symbol, error) {
					return []symbol.Symbol{s}, nil
				}).
				Respell(func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
					return name, nil
				}).
				Build()
			_, lowers := b.(plugin.Lowerer)
			assert.True(t, lowers, "the built backend declares the construct seam")
			_, respells := b.(plugin.Respeller)
			assert.True(t, respells, "and the name seam")
			plain := kitBackend("plain", "stub").Build()
			_, lowers = plain.(plugin.Lowerer)
			assert.False(t, lowers, "a hookless backend declares neither")

			r, held := b.(plugin.Renderer)
			assert.True(t, held, "a kit backend renders")
			e := plugin.NewEmit()
			assert.NoError(t, e.Add(kitUnit()), "the fixture unit arrives")
			_, err := r.Render(&plugin.RenderContext{
				Emit: e, Sink: diag.NewSink(), Plugin: "printer",
			})
			assert.HasError(t, err, "an unsettled store refuses at the first render")

			assert.NoError(t, plugin.Settle(e, b, diag.NewSink()), "the plan settles once")
			files, err := r.Render(&plugin.RenderContext{
				Emit: e, Sink: diag.NewSink(), Plugin: "printer",
			})
			assert.NoError(t, err, "and the settled store renders")
			assert.True(t, len(files) > 0, "whole")
		})

		t.Run("carries the declared coverage to the guard and back", func(t *testing.T) {
			t.Parallel()

			declared := render.Coverage{
				Facts: map[symbol.Fact]render.Verdict{
					symbol.FactAbstract: render.Refuses,
				},
			}
			b := kitBackend("printer", "stub").Coverage(declared).Build()
			c, held := b.(render.Coverer)
			assert.True(t, held, "a kit backend reads its coverage back")
			assert.Equal(t, c.Coverage().Of(symbol.KindStruct, symbol.FactAbstract),
				render.Refuses, "as declared")

			plain, held := kitBackend("plain", "stub").Build().(render.Coverer)
			assert.True(t, held, "an undeclared coverage still reads")
			assert.False(t, plain.Coverage().Declared(), "as undeclared")
		})

		t.Run("panics with every kit defect in one message", func(t *testing.T) {
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

		t.Run("panics with every language fault at once", func(t *testing.T) {
			t.Parallel()

			recovered := assert.Panics(t, func() {
				backend.New("printer", "stub", kitSyntax()).Build()
			}, "an empty language is a declaration defect")
			text := fmt.Sprint(recovered)
			for _, gap := range []string{
				"kinds", "filenames", "scaffolding", "import block", "formatter",
			} {
				assert.True(t, strings.Contains(text, gap),
					"the message names every gap, "+gap+" included")
			}
		})
	})

	t.Run("Version", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the declared version back", func(t *testing.T) {
			t.Parallel()

			b, held := kitBackend("printer", "stub").
				Version(kitVersion).Build().(plugin.Versioned)
			assert.True(t, held,
				"a kit backend contributes to the run fingerprint")
			assert.Equal(t, b.Version(), kitVersion,
				"the version the declaration bumped, as declared")
		})

		t.Run("reads empty where none was declared", func(t *testing.T) {
			t.Parallel()

			b, held := kitBackend("plain", "stub").Build().(plugin.Versioned)
			assert.True(t, held, "an undeclared version still reads")
			assert.Equal(t, b.Version(), "",
				"empty, so the fingerprint folds in nothing for it")
		})
	})

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		lowering := func() plugin.Backend {
			return kitBackend("printer", "stub").Lower(kitLower).Build()
		}

		t.Run("declares the construct seam alone", func(t *testing.T) {
			t.Parallel()

			b := lowering()
			_, lowers := b.(plugin.Lowerer)
			assert.True(t, lowers, "a declared lowering carries the construct seam")
			_, respells := b.(plugin.Respeller)
			assert.False(t, respells, "and no name seam, because none was declared")
		})

		t.Run("forwards a declaration to the declared hook", func(t *testing.T) {
			t.Parallel()

			l, held := lowering().(plugin.Lowerer)
			assert.True(t, held, "the built backend lowers")
			row := &emit.Struct{
				Origin: coretest.Struct(coretest.StorePath, coretest.StructName).ID,
				Name:   coretest.StructName,
			}
			out, err := l.Lower(row)
			assert.NoError(t, err, "the fixture hook takes a struct")
			assert.Equal(t, structNames(t, out),
				[]string{coretest.StructName, coretest.StructName + cursorSuffix},
				"the hook's own answer arrives, in its own order")
		})

		t.Run("refuses an unsettled store", func(t *testing.T) {
			t.Parallel()

			err := kitUnsettled(t, lowering())
			assert.HasError(t, err, "an unsettled store never reaches the pass")
			assert.Contains(t, err.Error(), "unsettled",
				"and the refusal names why no bytes were written")
		})

		t.Run("renders the lowered declarations once settled", func(t *testing.T) {
			t.Parallel()

			files := kitSettled(t, lowering())
			assert.Length(t, files, 1, "the fixture assembles one file")
			assert.ContainsInOrder(t, string(files[0].Body),
				[]string{"type ROW struct{}", "type ROWCURSOR struct{}"},
				"the companion the hook produced renders beside its input")
		})
	})

	t.Run("Respell", func(t *testing.T) {
		t.Parallel()

		respelling := func() plugin.Backend {
			return kitBackend("printer", "stub").Respell(kitRespell).Build()
		}

		t.Run("declares the name seam alone", func(t *testing.T) {
			t.Parallel()

			b := respelling()
			_, respells := b.(plugin.Respeller)
			assert.True(t, respells, "a declared respell carries the name seam")
			_, lowers := b.(plugin.Lowerer)
			assert.False(t, lowers, "and no construct seam, because none was declared")
		})

		t.Run("forwards a name to the declared hook", func(t *testing.T) {
			t.Parallel()

			r, held := respelling().(plugin.Respeller)
			assert.True(t, held, "the built backend respells")
			got, err := r.Respell(symbol.KindInvalid, symbol.KindStruct,
				symbol.VisibilityPublic, coretest.StructName)
			assert.NoError(t, err, "the fixture hook spells every name")
			assert.Equal(t, got, respellPrefix+coretest.StructName,
				"the hook's own answer arrives unchanged")
		})

		t.Run("refuses an unsettled store", func(t *testing.T) {
			t.Parallel()

			err := kitUnsettled(t, respelling())
			assert.HasError(t, err, "an unsettled store never reaches the pass")
			assert.Contains(t, err.Error(), "unsettled",
				"and the refusal names why no bytes were written")
		})

		t.Run("renders the respelt names once settled", func(t *testing.T) {
			t.Parallel()

			files := kitSettled(t, respelling())
			assert.Length(t, files, 1, "the fixture assembles one file")
			assert.ContainsInOrder(t, string(files[0].Body),
				[]string{"type T_ROW struct{}", "func t_Load() {"},
				"every declared name renders through the convention")
		})
	})
}
