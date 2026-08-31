// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backendtest_test

import (
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/backendtest"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
)

// call returns the one-line scaffold statement naming n.
func call(n string) emit.Stmt {
	return emit.Stmt{Kind: emit.StmtExpr, Value: emit.Expr{Kind: emit.ExprName, Name: n}}
}

// unit returns one flushed plan unit under the emitting fixture
// plugin, its word doubling as the family tag the way distinct
// declared outputs arrive as distinct accumulators.
func unit(word string, decls ...symbol.Symbol) plugin.Unit {
	return plugin.Unit{
		Plugin: "gen", Tag: word, Per: plugin.PerPlan, Word: word, Decls: decls,
	}
}

// fnOf returns a function declaration carrying body.
func fnOf(name string, body emit.Body) *emit.Function {
	f := &emit.Function{
		Origin: coretest.Struct(coretest.StorePath, name).ID,
		Name:   name,
	}
	f.Body = body
	return f
}

// wellBackend builds the fixture backend through the kit: two
// kinds, a scaffold for names and returns, and a pass-through
// formatter.
func wellBackend(tb assert.TB) plugin.Renderer {
	tb.Helper()

	b := eidos.NewBackend("printer", "stub",
		plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{
			symbol.KindStruct:   "type {{.Name}} struct{}\n",
			symbol.KindFunction: "func {{.Name}}() {\n{{body .}}}\n",
		}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
			switch s.Kind {
			case emit.StmtExpr:
				set.Add("stub/runtime")
				return []byte("\t" + s.Value.Name + "()\n"), nil
			case emit.StmtReturn:
				return []byte("\treturn\n"), nil
			default:
				return nil, errors.New("the fixture spells names and returns only")
			}
		}).
		Imports(func(set *render.ImportSet) string {
			if set.Len() == 0 {
				return ""
			}
			return "import (" + strings.Join(set.Paths(), " ") + ")\n"
		}).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Build()
	r, held := b.(plugin.Renderer)
	assert.True(tb, held, "the kit backend renders")
	return r
}

// wellFixture returns the valid fixture: a store carrying every
// kind the backend spells and a body in each of the four content
// forms, with the reference's slot content spliced through a
// marker-placing tree.
func wellFixture(tb assert.TB) *backendtest.Fixture {
	tb.Helper()

	refBody := emit.Body{Ref: &emit.TemplateRef{Name: "save.tpl"}}
	refBody.Prologue.Append(call("guard"))
	e := plugin.NewEmit()
	for _, u := range []plugin.Unit{
		unit("stub",
			&emit.Struct{
				Origin: coretest.Struct(coretest.StorePath, "Row").ID,
				Name:   "Row",
			},
			fnOf("Noop", emit.Body{}),
			fnOf("Load", emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}}),
		),
		unit("ref", fnOf("Save", refBody)),
		unit("raw", fnOf("Dump", emit.Body{Verbatim: "\tdump()\n"})),
	} {
		assert.NoError(tb, e.Add(u), "the fixture unit arrives")
	}
	return &backendtest.Fixture{
		Emit:     e,
		Schedule: []plugin.ID{"gen"},
		Trees: map[plugin.ID]fs.FS{
			"gen": fstest.MapFS{
				"save.tpl": &fstest.MapFile{Data: []byte("\tsaving()\n{{slots}}")},
			},
		},
	}
}

// wellRendered returns the valid setup: the kit backend over the
// valid fixture.
func wellRendered(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	return wellBackend(tb), wellFixture(tb)
}

// fake is a renderer the rejection tests script: the suite has to
// catch every way a renderer can cheat the rules.
type fake struct {
	render func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error)
}

func (f *fake) Render(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
	return f.render(ctx)
}

// scripted returns a setup handing the fake over a bare fixture.
func scripted(r func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error)) backendtest.Setup {
	return func(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
		return &fake{render: r}, &backendtest.Fixture{Emit: plugin.NewEmit()}
	}
}

// The suite is the contract a backend author tests against, so it
// has to accept a valid backend through and reject each way of
// cheating: that second half is what justifies it.
func TestAssertSettledShape(t *testing.T) {
	t.Parallel()

	t.Run("accepts the kit's clean settle", func(t *testing.T) {
		t.Parallel()

		backendtest.AssertSettledShape(t, wellRendered)
	})

	t.Run("rejects a setup that breaks its own isolation", func(t *testing.T) {
		t.Parallel()

		calls := 0
		setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			r, f := wellRendered(tb)
			calls++
			if calls > 1 {
				assert.NoError(tb,
					f.Emit.Add(unit("extra", fnOf("extra", emit.Body{}))),
					"the drifted unit arrives")
			}
			return r, f
		}
		failure := assert.Rejects(t, "a differing second build must fail",
			func(tb assert.TB) {
				backendtest.AssertSettledShape(tb, setup)
			})
		assert.Contains(t, failure, "unit", "the refusal names the drift")
	})
}

func TestRenderSettled(t *testing.T) {
	t.Parallel()

	t.Run("settles and renders the fixture once", func(t *testing.T) {
		t.Parallel()

		files := backendtest.RenderSettled(t, wellRendered)
		assert.True(t, len(files) > 0, "the settled fixture renders whole")
	})
}

func TestRunBackendSuite(t *testing.T) {
	t.Parallel()

	backendtest.RunBackendSuite(t, wellRendered)
}

func TestAssertPopulatedFixture(t *testing.T) {
	t.Parallel()

	t.Run("rejects an empty store", func(t *testing.T) {
		t.Parallel()

		hollow := scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
			return nil, nil
		})

		failure := assert.Rejects(t, "an empty store must fail the check",
			func(tb assert.TB) {
				backendtest.AssertPopulatedFixture(tb, hollow)
			})
		assert.Contains(t, failure, "unit",
			"the check demands an populated fixture")
	})
}

func TestAssertDeterministicRender(t *testing.T) {
	t.Parallel()

	t.Run("rejects bytes that vary between runs", func(t *testing.T) {
		t.Parallel()

		var runs int
		varying := func(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			runs++
			stamp := strconv.Itoa(runs)
			return &fake{render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
				return []plugin.RenderedFile{{Name: "a.txt", Body: []byte(stamp)}}, nil
			}}, &backendtest.Fixture{Emit: plugin.NewEmit()}
		}

		failure := assert.Rejects(t, "bytes carrying run state must fail the check",
			func(tb assert.TB) {
				backendtest.AssertDeterministicRender(tb, varying)
			})
		assert.Contains(t, failure, "same bytes",
			"the check names the byte-identity contract")
	})
}

func TestAssertSpeltKinds(t *testing.T) {
	t.Parallel()

	t.Run("rejects a kind the language cannot spell", func(t *testing.T) {
		t.Parallel()

		unspelt := scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
			ctx.Sink.Errorf(render.UnspeltKind,
				position.Pos{File: "a.txt"}, ctx.Plugin,
				"printer holds no template for the enum kind")
			return nil, nil
		})

		failure := assert.Rejects(t, "an unspelt kind must fail the check",
			func(tb assert.TB) {
				backendtest.AssertSpeltKinds(tb, unspelt)
			})
		assert.Contains(t, failure, "spell",
			"the check names the missing spelling")
	})
}

func TestAssertPlacedContent(t *testing.T) {
	t.Parallel()

	t.Run("rejects content that went unplaced", func(t *testing.T) {
		t.Parallel()

		dropped := scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
			ctx.Sink.Errorf(render.DroppedSlots,
				position.Pos{File: "a.txt"}, ctx.Plugin,
				"gen's template placed no marker and 2 contributions are pending")
			return nil, nil
		})

		failure := assert.Rejects(t, "dropped slot content must fail the check",
			func(tb assert.TB) {
				backendtest.AssertPlacedContent(tb, dropped)
			})
		assert.Contains(t, failure, "whole",
			"the check holds every body to arriving whole")
	})
}

func TestAssertContinuedRender(t *testing.T) {
	t.Parallel()

	t.Run("rejects a renderer that aborts", func(t *testing.T) {
		t.Parallel()

		aborting := scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
			return nil, errors.New("kaboom")
		})

		failure := assert.Rejects(t, "a fatal error must fail the check",
			func(tb assert.TB) {
				backendtest.AssertContinuedRender(tb, aborting)
			})
		assert.Contains(t, failure, "continue",
			"the check names the continuation rule")
	})

	t.Run("rejects a finding without a position", func(t *testing.T) {
		t.Parallel()

		unpositioned := scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
			ctx.Sink.Errorf(render.UnformattedFile,
				position.Pos{}, ctx.Plugin, "somewhere, something broke")
			return nil, nil
		})

		failure := assert.Rejects(t, "an unpositioned finding must fail the check",
			func(tb assert.TB) {
				backendtest.AssertContinuedRender(tb, unpositioned)
			})
		assert.Contains(t, failure, "position",
			"the check names the positioning rule")
	})

	t.Run("rejects a finding under another plugin's origin", func(t *testing.T) {
		t.Parallel()

		foreign := scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
			ctx.Sink.Errorf(render.UnformattedFile,
				position.Pos{File: "a.txt"}, "outsider", "not mine")
			return nil, nil
		})

		failure := assert.Rejects(t, "a mis-attributed finding must fail the check",
			func(tb assert.TB) {
				backendtest.AssertContinuedRender(tb, foreign)
			})
		assert.Contains(t, failure, "origin",
			"the check names the attribution rule")
	})

	t.Run("rejects a returned file reported unformatted", func(t *testing.T) {
		t.Parallel()

		lying := scripted(func(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
			ctx.Sink.Errorf(render.UnformattedFile,
				position.Pos{File: "a.txt"}, ctx.Plugin, "the formatter refused a.txt")
			return []plugin.RenderedFile{{Name: "a.txt", Body: []byte("x")}}, nil
		})

		failure := assert.Rejects(t, "a withheld file must stay withheld",
			func(tb assert.TB) {
				backendtest.AssertContinuedRender(tb, lying)
			})
		assert.Contains(t, failure, "withheld",
			"the check holds the withholding rule")
	})

	t.Run("holds a genuine format failure to continuation", func(t *testing.T) {
		t.Parallel()

		partial := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			f := wellFixture(tb)
			b := eidos.NewBackend("half", "stub",
				plugin.CommentSyntax{Line: []string{"//"}}).
				KindTemplates(map[symbol.Kind]string{
					symbol.KindStruct:   "type {{.Name}} struct{}\n",
					symbol.KindFunction: "func {{.Name}}()\n",
				}).
				Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
				Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
					return []byte("\treturn\n"), nil
				}).
				Imports(func(*render.ImportSet) string { return "" }).
				Finalise(func(src []byte) ([]byte, error) {
					if strings.Contains(string(src), "Row") {
						return nil, errors.New("unparseable")
					}
					return src, nil
				}).
				Build()
			r, held := b.(plugin.Renderer)
			assert.True(tb, held, "the kit backend renders")
			return r, f
		}

		backendtest.AssertContinuedRender(t, partial)
	})
}
