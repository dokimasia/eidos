// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"errors"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"text/template"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// stubNaming spells every unit as word, key stem and a fixture
// extension: the shape a target's naming returns.
func stubNaming(u plugin.Unit) string {
	stem := strings.TrimSuffix(path.Base(u.Key), ".go")
	if stem == "." {
		stem = "plan"
	}
	return stem + "_" + u.Word + ".txt"
}

// language returns the smallest valid language: a struct
// spelling, a callable spelling that places its body, a scaffold
// printer for names and returns, and a pass-through formatter.
func language() render.Language {
	return render.Language{
		Kinds: map[symbol.Kind]string{
			symbol.KindStruct:   "type {{.Name}} struct{}\n",
			symbol.KindFunction: "func {{.Name}}() {\n{{body .}}}\n",
		},
		Naming:   stubNaming,
		Scaffold: scaffold,
		Imports: func(set *render.ImportSet) string {
			if set.Len() == 0 {
				return ""
			}
			return "import (" + strings.Join(set.Paths(), " ") + ")\n"
		},
		Finalise: func(src []byte) ([]byte, error) { return src, nil },
	}
}

// scaffold spells the two statement kinds the fixtures use: a bare
// name evaluated for effect, and a return. A dotted name records
// its head as an import, the way a real printer records what it
// qualifies with.
func scaffold(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
	switch s.Kind {
	case emit.StmtExpr:
		if s.Value.Kind == emit.ExprValue {
			return nil, render.RefuseValue("fixture", "the fixture spells no value")
		}
		if head, _, qualified := strings.Cut(s.Value.Name, "."); qualified {
			set.Add(head)
		}
		return []byte("\t" + s.Value.Name + "()\n"), nil
	case emit.StmtReturn:
		return []byte("\treturn\n"), nil
	default:
		return nil, errors.New("the fixture spells names and returns only")
	}
}

// unspellable returns the statement carrying a value the fixture
// language has no form for.
func unspellable() emit.Stmt {
	return emit.Stmt{
		Kind:  emit.StmtExpr,
		Value: emit.ValueExpr(emit.Literal(emit.LiteralInt, "1")),
	}
}

// call returns the one-line scaffold statement naming n.
func call(n string) emit.Stmt {
	return emit.Stmt{Kind: emit.StmtExpr, Value: emit.Expr{Kind: emit.ExprName, Name: n}}
}

// fn returns a per-source unit holding one function carrying body.
func fn(key, name string, body emit.Body) plugin.Unit {
	u := unitOf("gen", key)
	f := &emit.Function{
		Origin: coretest.Struct(coretest.StorePath, name).ID,
		Name:   name,
	}
	f.Body = body
	u.Decls = append(u.Decls, f)
	return u
}

// unitOf returns one flushed unit carrying structs.
func unitOf(p plugin.ID, key string, names ...string) plugin.Unit {
	decls := make([]symbol.Symbol, 0, len(names))
	for _, n := range names {
		decls = append(decls, &emit.Struct{
			Origin: coretest.Struct(coretest.StorePath, n).ID,
			Name:   n,
		})
	}
	return plugin.Unit{
		Plugin: p, Tag: "", Per: plugin.PerSource,
		Word: "stub", Key: key, Decls: decls,
	}
}

// seeded returns an emit store holding the given units.
func seeded(tb assert.TB, units ...plugin.Unit) *plugin.Emit {
	tb.Helper()

	e := plugin.NewEmit()
	for _, u := range units {
		assert.NoError(tb, e.Add(u), "the fixture unit arrives")
	}
	return e
}

// runPass builds the pass over the language and renders the store.
func runPass(
	tb assert.TB, l render.Language, e *plugin.Emit,
) ([]plugin.RenderedFile, *diag.Sink) {
	tb.Helper()

	p, err := render.New("printer", l)
	assert.NoError(tb, err, "the language composes")
	sink := diag.NewSink()
	files, err := p.Render(&plugin.RenderContext{
		Emit: e, Sink: sink, Plugin: "printer",
	})
	assert.NoError(tb, err, "the pass runs whole")
	return files, sink
}

// refName is the template every reference-form fixture body names.
const refName = "method1.tpl"

// refTree returns the emitting plugin's tree holding src under
// [refName].
func refTree(src string) map[plugin.ID]fs.FS {
	return map[plugin.ID]fs.FS{
		"gen": fstest.MapFS{refName: &fstest.MapFile{Data: []byte(src)}},
	}
}

// refBody returns a body claiming [refName] and nothing else, so a
// case adds only the slots it is about.
func refBody() emit.Body {
	return emit.Body{Ref: &emit.TemplateRef{Name: refName}}
}

// refused returns a statement the fixture's scaffold cannot spell:
// the real printer failure an error path needs.
func refused() emit.Stmt {
	return emit.Stmt{Kind: emit.StmtGuard, Name: "err"}
}

// method returns a per-source unit holding one method carrying
// body, the second callable kind the body builtin takes.
func method(key, name string, body emit.Body) plugin.Unit {
	u := unitOf("gen", key)
	m := &emit.Method{
		Origin: coretest.Method(coretest.StorePath, coretest.StructName, name).ID,
		Name:   name,
	}
	m.Body = body
	u.Decls = append(u.Decls, m)
	return u
}

// renderRef renders one function whose body is b, with trees as the
// emitting plugin's template trees, and returns the file's bytes
// beside the run's findings. An empty answer means the file was
// withheld.
func renderRef(
	tb assert.TB, trees map[plugin.ID]fs.FS, b emit.Body,
) (string, *diag.Sink) {
	tb.Helper()

	p, err := render.New("printer", language())
	assert.NoError(tb, err, "the language composes")
	sink := diag.NewSink()
	files, err := p.Render(&plugin.RenderContext{
		Emit:  seeded(tb, fn("store.go", "Handle", b)),
		Trees: trees, Sink: sink, Plugin: "printer",
	})
	assert.NoError(tb, err, "the pass runs whole")
	if len(files) == 0 {
		return "", sink
	}
	return string(files[0].Body), sink
}

// reported returns the message of the first finding under code,
// which is what a case asserting the wording reads.
func reported(tb assert.TB, sink *diag.Sink, code diag.Code) string {
	tb.Helper()

	coretest.AssertReports(tb, sink, code)
	for d := range sink.All() {
		if d.Code == code {
			return d.Msg
		}
	}
	return ""
}

// The pass is the procedure every language shares: group through the
// naming, render kinds in canonical order, finalise per file and
// continue past a failure.
func TestPass(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("names every gap of an empty language at once", func(t *testing.T) {
			t.Parallel()

			_, err := render.New("printer", render.Language{})
			assert.HasError(t, err, "a language declaring nothing composes into nothing")
			for _, gap := range []string{
				"spells no kinds", "spells no filenames", "spells no scaffolding",
				"renders no import block", "holds no formatter",
			} {
				assert.Contains(t, err.Error(), gap,
					"a composition reads every fault at once, "+gap+" included")
			}
		})

		t.Run("a kind template that does not parse is a fault", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindEnum] = "{{"
			_, err := render.New("printer", l)
			assert.HasError(t, err, "an unparseable spelling never reaches a render")
			assert.Contains(t, err.Error(), symbol.KindEnum.String(),
				"and the fault names the kind it could not parse")
		})

		t.Run("a group template that does not parse is a fault", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Groups = map[render.GroupName]string{"block": "{{"}
			_, err := render.New("printer", l)
			assert.HasError(t, err, "an unparseable group spelling is the same fault")
			assert.Contains(t, err.Error(), "block",
				"and the fault names the group it could not parse")
		})

		t.Run("a file skeleton that does not parse is a fault", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.File = "{{"
			_, err := render.New("printer", l)
			assert.HasError(t, err, "an unparseable skeleton never assembles a file")
			assert.Contains(t, err.Error(), "file skeleton",
				"and the fault names the skeleton")
		})
	})

	t.Run("Coverage", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the language's declaration back", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Coverage = render.Coverage{Facts: map[symbol.Fact]render.Verdict{
				symbol.FactAbstract: render.Refuses,
			}}
			p, err := render.New("printer", l)
			assert.NoError(t, err, "the language composes")
			assert.Equal(t, p.Coverage().Of(symbol.KindStruct, symbol.FactAbstract),
				render.Refuses,
				"a consumer holding the pass reads the data the guard reads")
		})

		t.Run("reads undeclared where the language declared none", func(t *testing.T) {
			t.Parallel()

			p, err := render.New("printer", language())
			assert.NoError(t, err, "the language composes")
			assert.False(t, p.Coverage().Declared(),
				"an undeclared coverage leaves the guard off")
		})
	})

	t.Run("refuses a context it cannot render from", func(t *testing.T) {
		t.Parallel()

		p, err := render.New("printer", language())
		assert.NoError(t, err, "the language composes")

		_, err = p.Render(nil)
		assert.HasError(t, err, "a nil context carries no store")
		assert.Contains(t, err.Error(), "emit store", "and names what it needs")

		_, err = p.Render(&plugin.RenderContext{Sink: diag.NewSink()})
		assert.HasError(t, err, "and neither does a context holding no store")

		_, err = p.Render(&plugin.RenderContext{Emit: seeded(t)})
		assert.HasError(t, err, "a context without a sink has nowhere to report")
		assert.Contains(t, err.Error(), "sink", "and names what it needs")
	})

	t.Run("groups units into files through the naming", func(t *testing.T) {
		t.Parallel()

		files, sink := runPass(t, language(), seeded(t,
			unitOf("gen", "store.go", "Alpha"),
			unitOf("gen", "user.go", "Beta"),
		))
		assert.False(t, sink.Failed(), "nothing to report")
		assert.Length(t, files, 2, "one file per name")
		assert.Equal(t, files[0].Name, "store_stub.txt",
			"files come back in name order")
		assert.Equal(t, files[1].Name, "user_stub.txt", "both spelled by the target")
	})

	t.Run("units sharing a name assemble one file", func(t *testing.T) {
		t.Parallel()

		files, _ := runPass(t, language(), seeded(t,
			unitOf("weaver", "store.go", "Omega"),
			unitOf("gen", "store.go", "Alpha"),
		))
		assert.Length(t, files, 1, "two plugins, one file")
		body := string(files[0].Body)
		assert.ContainsInOrder(t, body, []string{"Alpha", "Omega"},
			"contributions arrive in unit order, which is total")
	})

	t.Run("two packages spelling one filename stay two files", func(t *testing.T) {
		t.Parallel()

		left := unitOf("gen", "example.com/left/store.go", "Alpha")
		left.Pkg = coretest.Struct("example.com/left", "Anchor").ID
		right := unitOf("gen", "example.com/right/store.go", "Beta")
		right.Pkg = coretest.Struct("example.com/right", "Anchor").ID
		files, _ := runPass(t, language(), seeded(t, left, right))
		assert.Length(t, files, 2, "the owning package addresses the file")
		assert.Equal(t, files[0].Name, files[1].Name, "one spelled name")
		assert.NotEqual(t, files[0].Pkg, files[1].Pkg, "two owners, two files")
	})

	t.Run("renders declarations in the unit's canonical order", func(t *testing.T) {
		t.Parallel()

		files, _ := runPass(t, language(), seeded(t,
			unitOf("gen", "store.go", "Beta", "Alpha"),
		))
		assert.ContainsInOrder(t, string(files[0].Body), []string{"Beta", "Alpha"},
			"the unit's own declaration order holds: the flush ordered it")
	})

	t.Run("a missing kind template is a positioned finding", func(t *testing.T) {
		t.Parallel()

		l := language()
		u := unitOf("gen", "store.go", "Alpha")
		u.Decls = append(u.Decls, &emit.Method{
			Origin: coretest.Struct(coretest.StorePath, "M").ID,
			Name:   "Handle",
		})
		files, sink := runPass(t, l, seeded(t, u))
		assert.True(t, sink.Failed(), "a kind the language cannot spell reports")
		assert.Length(t, files, 1, "and the remaining declarations still render")
		assert.Contains(t, string(files[0].Body), "Alpha",
			"the file keeps what rendered")
	})

	t.Run("a file carries the derivation it was assembled from", func(t *testing.T) {
		t.Parallel()

		files, sink := runPass(t, language(), seeded(t,
			unitOf("beta", "store.go", "Beta"),
			unitOf("alpha", "store.go", "Alpha"),
		))
		assert.False(t, sink.Failed(), "the fixture renders clean")
		assert.Length(t, files, 1, "two plugins sharing a filename assemble one file")
		assert.Equal(t, files[0].Plugins, []plugin.ID{"alpha", "beta"},
			"every emitter that contributed, distinct and sorted")
		assert.Equal(t, files[0].Sources, []string{"store.go"},
			"and the routing key they share, named once")
	})

	t.Run("a plan file derives from nothing", func(t *testing.T) {
		t.Parallel()

		plan := unitOf("gen", "", "Registry")
		plan.Per = plugin.PerPlan
		files, _ := runPass(t, language(), seeded(t, plan))
		assert.Length(t, files, 1, "the plan unit renders")
		assert.Equal(t, files[0].Plugins, []plugin.ID{"gen"}, "its emitter alone")
		assert.Length(t, files[0].Sources, 0,
			"and no source, because a plan file derives from no declaration")
	})

	t.Run("findings carry the context's plugin as origin", func(t *testing.T) {
		t.Parallel()

		u := unitOf("gen", "store.go", "Alpha")
		u.Decls = append(u.Decls, &emit.Method{
			Origin: coretest.Struct(coretest.StorePath, "M").ID,
			Name:   "Handle",
		})

		p, err := render.New("printer", language())
		assert.NoError(t, err, "the language composes")
		sink := diag.NewSink()
		_, err = p.Render(&plugin.RenderContext{
			Emit: seeded(t, u), Sink: sink, Plugin: "composed",
		})
		assert.NoError(t, err, "the pass runs whole")
		for d := range sink.All() {
			assert.Equal(t, d.Origin, diag.Origin("composed"),
				"the composition's identity, not the pass's own name")
		}

		sink = diag.NewSink()
		_, err = p.Render(&plugin.RenderContext{Emit: seeded(t, u), Sink: sink})
		assert.NoError(t, err, "the pass runs whole")
		for d := range sink.All() {
			assert.Equal(t, d.Origin, diag.Origin("printer"),
				"a zero context identity falls back to the pass's name")
		}
	})

	t.Run("a format failure continues with the remaining files", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Finalise = func(src []byte) ([]byte, error) {
			if strings.Contains(string(src), "Broken") {
				return nil, errors.New("unparseable")
			}
			return src, nil
		}
		files, sink := runPass(t, l, seeded(t,
			unitOf("gen", "broken.go", "Broken"),
			unitOf("gen", "store.go", "Alpha"),
		))
		assert.True(t, sink.Failed(), "the failure is an Error")
		assert.Length(t, files, 1, "the sink never receives the unformatted file")
		assert.Equal(t, files[0].Name, "store_stub.txt",
			"while its siblings render whole")
	})

	t.Run("renders the body forms", func(t *testing.T) {
		t.Parallel()

		t.Run("a zero body renders nothing but its shape", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, language(), seeded(t,
				fn("store.go", "Handle", emit.Body{})))
			assert.False(t, sink.Failed(), "the default form is valid")
			assert.Equal(t, string(files[0].Body), "func Handle() {\n}\n",
				"the kind template's own shape, no content")
		})

		t.Run("scaffolding composes with the slots in the fixed order", func(t *testing.T) {
			t.Parallel()

			var body emit.Body
			body.Prologue.Append(call("pro"))
			body.Declare("checks").Append(call("named"))
			body.Epilogue.Append(call("epi"))
			body.Stmts = []emit.Stmt{call("content")}
			files, sink := runPass(t, language(), seeded(t,
				fn("store.go", "Handle", body)))
			assert.False(t, sink.Failed(), "the scaffold form is valid")
			assert.ContainsInOrder(t, string(files[0].Body),
				[]string{"pro", "content", "named", "epi"},
				"prologue, content, named slots, epilogue: the fixed composition")
		})

		t.Run("verbatim arrives literally", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, language(), seeded(t,
				fn("store.go", "Handle", emit.Body{Verbatim: "\treturn nil\n"})))
			assert.False(t, sink.Failed(), "the verbatim form is valid")
			assert.Contains(t, string(files[0].Body), "\treturn nil\n",
				"byte for byte, as literal text")
		})

		t.Run("two forms is a finding and the slots still render", func(t *testing.T) {
			t.Parallel()

			var body emit.Body
			body.Prologue.Append(call("pro"))
			body.Stmts = []emit.Stmt{call("content")}
			body.Verbatim = "\tnope\n"
			files, sink := runPass(t, language(), seeded(t,
				fn("store.go", "Handle", body)))
			assert.True(t, sink.Failed(), "a two-form body is a defect")
			assert.Contains(t, string(files[0].Body), "pro",
				"the standard slots survive the refusal")
			assert.NotContains(t, string(files[0].Body), "content",
				"and no contested content is guessed at")
		})

		t.Run("a scaffold the language cannot spell reports", func(t *testing.T) {
			t.Parallel()

			body := emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtGuard, Name: "err"}}}
			_, sink := runPass(t, language(), seeded(t,
				fn("store.go", "Handle", body)))
			assert.True(t, sink.Failed(), "the printer's refusal is an Error")
		})
	})

	t.Run("resolves references in the emitter's tree", func(t *testing.T) {
		t.Parallel()

		trees := func(tpl string) map[plugin.ID]fs.FS {
			return map[plugin.ID]fs.FS{
				"gen": fstest.MapFS{"method1.tpl": &fstest.MapFile{Data: []byte(tpl)}},
			}
		}
		refBody := func(data any) emit.Body {
			var b emit.Body
			b.Prologue.Append(call("pro"))
			b.Ref = &emit.TemplateRef{Name: "method1.tpl", Data: data}
			return b
		}
		runRef := func(t *testing.T, trees map[plugin.ID]fs.FS, b emit.Body) (string, *diag.Sink) {
			t.Helper()
			pass, err := render.New("printer", language())
			assert.NoError(t, err, "the language composes")
			sink := diag.NewSink()
			files, err := pass.Render(&plugin.RenderContext{
				Emit:  seeded(t, fn("store.go", "Handle", b)),
				Trees: trees, Sink: sink, Plugin: "printer",
			})
			assert.NoError(t, err, "the pass runs whole")
			if len(files) == 0 {
				return "", sink
			}
			return string(files[0].Body), sink
		}

		t.Run("the template drives the layout and the data is passed through", func(t *testing.T) {
			t.Parallel()

			body, sink := runRef(t,
				trees("\tref({{.Data.mark}}) for {{.Decl.Name}}\n{{slots}}"),
				refBody(map[string]any{"mark": "x"}))
			assert.False(t, sink.Failed(), "a placed marker is valid")
			assert.ContainsInOrder(t, body, []string{"ref(x) for Handle", "pro"},
				"content where the template says, then the marker's slots")
		})

		t.Run("a named marker places one slot", func(t *testing.T) {
			t.Parallel()

			b := refBody(nil)
			b.Declare("checks").Append(call("named"))
			body, sink := runRef(t,
				trees("{{slot \"checks\"}}\tmid()\n{{slots}}"), b)
			assert.False(t, sink.Failed(), "named markers are valid")
			assert.ContainsInOrder(t, body, []string{"named", "mid", "pro"},
				"the named slot arrives first, the catch-all takes the rest")
		})

		t.Run("the tree is the emitter's alone", func(t *testing.T) {
			t.Parallel()

			other := map[plugin.ID]fs.FS{
				"other": fstest.MapFS{"method1.tpl": &fstest.MapFile{Data: []byte("{{slots}}")}},
			}
			body, sink := runRef(t, other, refBody(nil))
			assert.True(t, sink.Failed(), "another plugin's tree resolves nothing")
			assert.Contains(t, body, "pro",
				"and the slots survive as the fallback")
		})

		t.Run("a dropped marker with pending content is an Error", func(t *testing.T) {
			t.Parallel()

			body, sink := runRef(t, trees("\tbare()\n"), refBody(nil))
			assert.True(t, sink.Failed(), "the pending prologue reports")
			found := false
			for d := range sink.All() {
				if d.Code == render.DroppedSlots {
					found = true
					assert.Contains(t, d.Msg, "gen", "naming the emitter")
				}
			}
			assert.True(t, found, "under the marker rule's code")
			assert.NotContains(t, body, "pro",
				"the template owns the layout, so nothing is appended for it")
		})

		t.Run("no pending content needs no marker", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			b.Ref = &emit.TemplateRef{Name: "method1.tpl"}
			_, sink := runRef(t, trees("\tbare()\n"), b)
			assert.False(t, sink.Failed(), "an empty slot set has nothing to drop")
		})

		t.Run("two plugins' templates of one name resolve in each plugin's tree", func(t *testing.T) {
			t.Parallel()

			unit := func(p plugin.ID, name string) plugin.Unit {
				u := unitOf(p, "store.go")
				f := &emit.Function{
					Origin: coretest.Struct(coretest.StorePath, name).ID,
					Name:   name,
				}
				f.Body = emit.Body{Ref: &emit.TemplateRef{Name: "method1.tpl"}}
				u.Decls = append(u.Decls, f)
				return u
			}
			pass, err := render.New("printer", language())
			assert.NoError(t, err, "the language composes")
			sink := diag.NewSink()
			files, err := pass.Render(&plugin.RenderContext{
				Emit: seeded(t, unit("alpha", "First"), unit("beta", "Second")),
				Trees: map[plugin.ID]fs.FS{
					"alpha": fstest.MapFS{"method1.tpl": &fstest.MapFile{Data: []byte("\talpha()\n{{slots}}")}},
					"beta":  fstest.MapFS{"method1.tpl": &fstest.MapFile{Data: []byte("\tbeta()\n{{slots}}")}},
				},
				Sink: sink, Plugin: "printer",
			})
			assert.NoError(t, err, "the pass runs whole")
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, 1, "both units share one file")
			assert.Equal(t, string(files[0].Body),
				"func First() {\n\talpha()\n}\nfunc Second() {\n\tbeta()\n}\n",
				"each body renders its own emitter's template, whatever the worker rendered first")
		})
	})

	t.Run("a declaration skipped mid-render records no imports", func(t *testing.T) {
		t.Parallel()

		load := fn("store.go", "Load", emit.Body{Stmts: []emit.Stmt{call("pkg.Run"), refused()}})
		load.Decls = append([]symbol.Symbol{&emit.Struct{
			Origin: coretest.Struct(coretest.StorePath, "Alpha").ID,
			Name:   "Alpha",
		}}, load.Decls...)
		files, sink := runPass(t, language(), seeded(t, load))
		coretest.AssertCodes(t, sink, render.RefusedTemplate)
		assert.Length(t, files, 1, "the file renders without the skipped declaration")
		assert.Equal(t, string(files[0].Body), "type Alpha struct{}\n",
			"and without the import the skipped declaration recorded before it failed")
	})

	t.Run("findings arrive in file order whatever order the workers finish in", func(t *testing.T) {
		t.Parallel()

		released := make(chan struct{})
		l := language()
		l.Finalise = func(src []byte) ([]byte, error) {
			switch {
			case strings.Contains(string(src), "Alpha"):
				// The first file waits for the second to finish, so
				// with two workers the second file's finding is
				// reported first. One worker cannot run the second
				// file concurrently, so the wait ends by timeout and
				// the order is file order either way.
				select {
				case <-released:
				case <-time.After(2 * time.Second):
				}
				return nil, errors.New("alpha is unformattable")
			default:
				defer close(released)
				return nil, errors.New("beta is unformattable")
			}
		}
		_, sink := runPass(t, l, seeded(t,
			unitOf("gen", "alpha.go", "Alpha"),
			unitOf("gen", "beta.go", "Beta"),
		))
		got := []string{}
		for d := range sink.All() {
			got = append(got, d.Pos.File)
		}
		assert.Equal(t, got, []string{"alpha_stub.txt", "beta_stub.txt"},
			"the report order is the file order")
	})

	t.Run("positions findings at the package-qualified file", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Finalise = func([]byte) ([]byte, error) { return nil, errors.New("unparseable") }
		u := unitOf("gen", "example.com/left/store.go", "Alpha")
		u.Pkg = coretest.Struct("example.com/left", "Anchor").ID
		_, sink := runPass(t, l, seeded(t, u))
		for d := range sink.All() {
			assert.Equal(t, d.Pos.File, "example.com/left/store_stub.txt",
				"the position names the package the file belongs to")
		}
		coretest.AssertReports(t, sink, render.UnformattedFile)
	})

	t.Run("positions a vanishing split at its unit under the context's identity", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Split = func(plugin.Unit) []plugin.Unit { return nil }
		p, err := render.New("printer", l)
		assert.NoError(t, err, "the language composes")
		sink := diag.NewSink()
		_, err = p.Render(&plugin.RenderContext{
			Emit: seeded(t, unitOf("gen", "store.go", "Alpha")), Sink: sink, Plugin: "composed",
		})
		assert.NoError(t, err, "the pass runs whole")
		coretest.AssertPositioned(t, sink)
		for d := range sink.All() {
			assert.Equal(t, d.Origin, diag.Origin("composed"),
				"the finding reports under the composition's identity")
			assert.Equal(t, d.Pos.File, "store.go", "at the unit it could not route")
		}
	})

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

	t.Run("merges the template vocabulary", func(t *testing.T) {
		t.Parallel()

		shouting := func() render.Language {
			l := language()
			l.Funcs = template.FuncMap{"shout": strings.ToUpper}
			l.Kinds[symbol.KindStruct] = "type {{shout .Name}} struct{}\n"
			return l
		}
		runMerged := func(
			t *testing.T, l render.Language, ctx *plugin.RenderContext,
		) (string, *diag.Sink) {
			t.Helper()
			pass, err := render.New("printer", l)
			assert.NoError(t, err, "the language composes")
			sink := diag.NewSink()
			ctx.Sink = sink
			ctx.Plugin = "printer"
			files, err := pass.Render(ctx)
			assert.NoError(t, err, "the pass runs whole")
			if len(files) == 0 {
				return "", sink
			}
			return string(files[0].Body), sink
		}

		t.Run("the shared vocabulary serves the kind templates", func(t *testing.T) {
			t.Parallel()

			body, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit: seeded(t, unitOf("gen", "store.go", "Alpha")),
			})
			assert.False(t, sink.Failed(), "the shared helper is valid")
			assert.Contains(t, body, "type ALPHA struct{}",
				"registered once, called anywhere")
		})

		t.Run("a declared override replaces the helper everywhere", func(t *testing.T) {
			t.Parallel()

			body, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit:      seeded(t, unitOf("gen", "store.go", "Alpha")),
				Schedule:  []plugin.ID{"gen", "styler"},
				Funcs:     map[plugin.ID]template.FuncMap{"styler": {"shout": strings.ToLower}},
				Overrides: map[plugin.ID][]string{"styler": {"shout"}},
			})
			assert.False(t, sink.Failed(), "the replace verb is declared and valid")
			assert.Contains(t, body, "type alpha struct{}",
				"the backend's own templates render through the override")
		})

		t.Run("the latest schedule position wins", func(t *testing.T) {
			t.Parallel()

			ctx := func(schedule ...plugin.ID) *plugin.RenderContext {
				return &plugin.RenderContext{
					Emit:     seeded(t, unitOf("gen", "store.go", "Alpha")),
					Schedule: schedule,
					Funcs: map[plugin.ID]template.FuncMap{
						"early": {"shout": func(s string) string { return "early_" + s }},
						"late":  {"shout": func(s string) string { return "late_" + s }},
					},
					Overrides: map[plugin.ID][]string{
						"early": {"shout"}, "late": {"shout"},
					},
				}
			}
			body, _ := runMerged(t, shouting(), ctx("early", "late"))
			assert.Contains(t, body, "late_Alpha",
				"the plugin that runs last changes how the construct renders")
			body, _ = runMerged(t, shouting(), ctx("late", "early"))
			assert.Contains(t, body, "early_Alpha",
				"and the order is the schedule's, not the map's")
		})

		t.Run("an undeclared shadow is refused and reported", func(t *testing.T) {
			t.Parallel()

			body, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit:     seeded(t, unitOf("gen", "store.go", "Alpha")),
				Schedule: []plugin.ID{"gen"},
				Funcs:    map[plugin.ID]template.FuncMap{"gen": {"shout": strings.ToLower}},
			})
			assert.True(t, sink.Failed(), "a shadow without a declaration reports")
			assert.Contains(t, body, "type ALPHA struct{}",
				"and the shared helper stands")
		})

		t.Run("a plugin's own helper serves its reference", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			b.Ref = &emit.TemplateRef{Name: "method1.tpl", Data: map[string]any{"x": "go"}}
			body, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit:     seeded(t, fn("store.go", "Handle", b)),
				Schedule: []plugin.ID{"gen"},
				Funcs: map[plugin.ID]template.FuncMap{"gen": {"mark": func(s string) string {
					return strings.ToUpper(s[:1]) + s[1:]
				}}},
				Trees: map[plugin.ID]fs.FS{
					"gen": fstest.MapFS{"method1.tpl": &fstest.MapFile{
						Data: []byte("\t{{mark .Data.x}}()\n{{slots}}"),
					}},
				},
			})
			assert.False(t, sink.Failed(), "a private helper is the plugin's own")
			assert.Contains(t, body, "\tGo()\n", "and its templates call it")
		})

		t.Run("a reserved name in the shared vocabulary is a fault", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Funcs = template.FuncMap{"body": strings.ToUpper}
			_, err := render.New("printer", l)
			assert.HasError(t, err, "the builtins' names are the pass's own")
			assert.Contains(t, err.Error(), "body", "naming the collision")
		})

		t.Run("a plugin the schedule does not hold still merges", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Ref.Data = map[string]any{"x": "go"}
			body, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit:  seeded(t, fn("store.go", "Handle", b)),
				Funcs: map[plugin.ID]template.FuncMap{"gen": {"mark": strings.ToUpper}},
				Trees: refTree("\t{{mark .Data.x}}()\n{{slots}}"),
			})
			assert.False(t, sink.Failed(), "a helper needs no schedule position to merge")
			assert.Contains(t, body, "\tGO()\n",
				"and a fixture without a schedule still renders through it")
		})

		t.Run("a plugin claiming a builtin is refused", func(t *testing.T) {
			t.Parallel()

			body, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit: seeded(t, fn("store.go", "Handle",
					emit.Body{Stmts: []emit.Stmt{call("content")}})),
				Schedule: []plugin.ID{"gen"},
				Funcs: map[plugin.ID]template.FuncMap{
					"gen": {render.BuiltinBody: strings.ToUpper},
				},
			})
			coretest.AssertCodes(t, sink, render.UndeclaredOverride)
			assert.Contains(t, body, "\tcontent()\n",
				"and the builtin stands, so the body still places its content")
		})

		t.Run("two plugins registering one helper collide", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Ref.Data = map[string]any{"x": "go"}
			body, sink := runMerged(t, shouting(), &plugin.RenderContext{
				Emit: seeded(t, fn("store.go", "Handle", b)),
				Funcs: map[plugin.ID]template.FuncMap{
					"alpha": {"mark": strings.ToUpper},
					"beta":  {"mark": strings.ToLower},
				},
				Trees: refTree("\t{{mark .Data.x}}()\n{{slots}}"),
			})
			coretest.AssertCodes(t, sink, render.HelperCollision)
			assert.Contains(t, body, "\tGO()\n",
				"the first registration in composition order stands")
		})
	})

	t.Run("two runs produce the same bytes", func(t *testing.T) {
		t.Parallel()

		build := func() *plugin.Emit {
			return seeded(t,
				unitOf("weaver", "store.go", "Omega"),
				unitOf("gen", "store.go", "Alpha", "Beta"),
				unitOf("gen", "user.go", "Gamma"),
			)
		}
		first, _ := runPass(t, language(), build())
		second, _ := runPass(t, language(), build())
		assert.Equal(t, second, first, "byte identity is the contract")
	})

	t.Run("splits units before naming", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Split = func(u plugin.Unit) []plugin.Unit {
			out := make([]plugin.Unit, 0, len(u.Decls))
			for _, d := range u.Decls {
				su := u
				su.Decls = []symbol.Symbol{d}
				su.Key = strings.ToLower(d.(*emit.Struct).Name) + ".go"
				out = append(out, su)
			}
			return out
		}
		files, sink := runPass(t, l, seeded(t,
			unitOf("gen", "store.go", "Alpha", "Beta"),
		))
		assert.False(t, sink.Failed(), "a split render reports nothing")
		names := make([]string, 0, len(files))
		for _, f := range files {
			names = append(names, f.Name)
		}
		assert.Equal(t, names, []string{"alpha_stub.txt", "beta_stub.txt"},
			"one file per split unit, named from the rewritten key")
	})

	t.Run("reports a split that vanishes a populated unit", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Split = func(plugin.Unit) []plugin.Unit { return nil }
		files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
		assert.Length(t, files, 0, "nothing routed, nothing rendered")
		assert.Contains(t, reported(t, sink, render.RefusedTemplate), "into nothing",
			"the vanishing reports instead of narrowing silently")
	})

	t.Run("clusters declarations under a group template", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Cluster = func(decls []symbol.Symbol) []render.Clustered {
			c := render.Clustered{Group: "block"}
			for _, d := range decls {
				if _, held := d.(*emit.Struct); held {
					c.Decls = append(c.Decls, d)
				}
			}
			if len(c.Decls) == 0 {
				return nil
			}
			return []render.Clustered{c}
		}
		l.Groups = map[render.GroupName]string{
			"block": "types (\n{{range .Decls}}\t{{.Name}}\n{{end}})\n",
		}

		u := unitOf("gen", "store.go", "Alpha")
		load := &emit.Function{
			Origin: coretest.Struct(coretest.StorePath, "Load").ID,
			Name:   "Load",
		}
		load.Body = emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}}
		beta := &emit.Struct{
			Origin: coretest.Struct(coretest.StorePath, "Beta").ID,
			Name:   "Beta",
		}
		u.Decls = append(u.Decls, load, beta)

		files, sink := runPass(t, l, seeded(t, u))
		assert.False(t, sink.Failed(), "a clustered render reports nothing")
		assert.Equal(t, len(files), 1, "one file")
		assert.Equal(t, string(files[0].Body),
			"types (\n\tAlpha\n\tBeta\n)\nfunc Load() {\n\treturn\n}\n",
			"the cluster renders at its first member's position, "+
				"the singleton through its kind template")
	})

	t.Run("a cluster naming no declared group is reported", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Cluster = func(decls []symbol.Symbol) []render.Clustered {
			return []render.Clustered{{Group: "ghost", Decls: decls}}
		}
		l.Groups = map[render.GroupName]string{
			"block": "types (\n{{range .Decls}}\t{{.Name}}\n{{end}})\n",
		}
		files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
		var codes []diag.Code
		for d := range sink.All() {
			codes = append(codes, d.Code)
		}
		assert.Equal(t, codes, []diag.Code{render.UnknownGroup},
			"the unknown group is one finding")
		assert.Equal(t, len(files), 1, "the file still renders")
		assert.Equal(t, string(files[0].Body), "",
			"without the skipped cluster's declarations")
	})

	t.Run("a cluster without group templates refuses to compose", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Cluster = func(decls []symbol.Symbol) []render.Clustered { return nil }
		_, err := render.New("printer", l)
		assert.HasError(t, err, "clustering needs group templates")
		assert.Contains(t, err.Error(), "group templates", "naming the gap")
	})

	t.Run("nests declarations through their kind templates", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Kinds[symbol.KindStruct] = "type {{.Name}} {\n" +
			"{{- range .Types.Items}}\n{{nested \"\t\" .}}{{- end}}\n}\n"
		host := &emit.Struct{Name: "Row"}
		inner := &emit.Struct{Name: "Inner"}
		inner.Types.Append(&emit.Struct{Name: "Deepest"})
		host.Types.Append(inner)
		u := unitOf("gen", "store.go")
		u.Decls = append(u.Decls, host)

		files, sink := runPass(t, l, seeded(t, u))
		assert.Length(t, slices.Collect(sink.All()), 0, "the nesting renders clean")
		assert.Equal(t, string(files[0].Body),
			"type Row {\n"+
				"\ttype Inner {\n"+
				"\t\ttype Deepest {\n"+
				"\t\t}\n"+
				"\t}\n"+
				"}\n",
			"each level indents once more, blank lines stay bare")
	})

	t.Run("a nested kind without a template reports and is skipped", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Kinds[symbol.KindStruct] = "type {{.Name}} {\n" +
			"{{- range .Types.Items}}\n{{nested \"\t\" .}}{{- end}}\n}\n"
		host := &emit.Struct{Name: "Row"}
		host.Types.Append(&emit.Enum{Name: "Phase"})
		u := unitOf("gen", "store.go")
		u.Decls = append(u.Decls, host)

		files, sink := runPass(t, l, seeded(t, u))
		var codes []diag.Code
		for d := range sink.All() {
			codes = append(codes, d.Code)
		}
		assert.Equal(t, codes, []diag.Code{render.UnspeltKind},
			"the unspelt nested kind is one finding")
		assert.Equal(t, string(files[0].Body), "type Row {\n\n}\n",
			"and the host renders without it")
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

	t.Run("places a body only where one is carried", func(t *testing.T) {
		t.Parallel()

		t.Run("a method's body composes like a function's", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindMethod] = "func (r) {{.Name}}() {\n{{body .}}}\n"
			files, sink := runPass(t, l, seeded(t, method("store.go", "Handle",
				emit.Body{Stmts: []emit.Stmt{call("content")}})))
			assert.False(t, sink.Failed(), "a method carries a body")
			assert.Equal(t, string(files[0].Body),
				"func (r) Handle() {\n\tcontent()\n}\n",
				"the same fixed composition, under the method's own spelling")
		})

		t.Run("a declaration carrying none refuses the template", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindStruct] = "type {{.Name}} struct{}\n{{body .}}"
			files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), "carries no body",
				"the refusal names what the builtin was handed")
			assert.Equal(t, string(files[0].Body), "",
				"and no fragment reaches the file the finding says was skipped")
		})
	})

	t.Run("a scaffold refusal fails the declaration wherever it sits", func(t *testing.T) {
		t.Parallel()

		placements := []struct {
			name string
			body func() emit.Body
		}{
			{
				name: "in the prologue",
				body: func() emit.Body {
					var b emit.Body
					b.Prologue.Append(refused())
					return b
				},
			},
			{
				name: "in a named slot",
				body: func() emit.Body {
					var b emit.Body
					b.Declare("checks").Append(refused())
					return b
				},
			},
			{
				name: "in the epilogue",
				body: func() emit.Body {
					var b emit.Body
					b.Epilogue.Append(refused())
					return b
				},
			},
		}
		for _, tt := range placements {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				files, sink := runPass(t, language(), seeded(t,
					fn("store.go", "Handle", tt.body())))
				coretest.AssertCodes(t, sink, render.RefusedTemplate)
				assert.Equal(t, string(files[0].Body), "",
					"the refusal withholds the whole declaration, "+
						"so no half-opened shape reaches the file")
			})
		}
	})

	t.Run("a value the language cannot spell reports under its own code", func(t *testing.T) {
		t.Parallel()

		var b emit.Body
		b.Stmts = []emit.Stmt{unspellable()}
		files, sink := runPass(t, language(), seeded(t, fn("store.go", "Handle", b)))
		coretest.AssertCodes(t, sink, render.UnspeltValue)
		assert.Contains(t, reported(t, sink, render.UnspeltValue), "spells no value",
			"the refusal carries the language's own reason")
		assert.Equal(t, string(files[0].Body), "",
			"and the declaration is skipped, the way any refused spelling is")
	})

	t.Run("a refusal that is not a value stays a template refusal", func(t *testing.T) {
		t.Parallel()

		var b emit.Body
		b.Stmts = []emit.Stmt{refused()}
		_, sink := runPass(t, language(), seeded(t, fn("store.go", "Handle", b)))
		coretest.AssertCodes(t, sink, render.RefusedTemplate)
	})

	t.Run("a reference the tree cannot serve falls back to the slots", func(t *testing.T) {
		t.Parallel()

		withPrologue := func() emit.Body {
			b := refBody()
			b.Prologue.Append(call("pro"))
			return b
		}

		t.Run("a name the tree does not hold", func(t *testing.T) {
			t.Parallel()

			body, sink := renderRef(t, map[plugin.ID]fs.FS{
				"gen": fstest.MapFS{"other.tpl": &fstest.MapFile{Data: []byte("{{slots}}")}},
			}, withPrologue())
			assert.Contains(t, reported(t, sink, render.UnresolvedRef), refName,
				"the finding names the template the tree does not hold")
			assert.Contains(t, body, "\tpro()\n",
				"and the slots survive the broken claim")
		})

		t.Run("a template that does not parse", func(t *testing.T) {
			t.Parallel()

			body, sink := renderRef(t, refTree("{{"), withPrologue())
			assert.Contains(t, reported(t, sink, render.UnresolvedRef), "does not parse",
				"the finding names why nothing resolved")
			assert.Contains(t, body, "\tpro()\n",
				"and the extension points survive it")
		})

		t.Run("a template that refuses at execute time", func(t *testing.T) {
			t.Parallel()

			body, sink := renderRef(t, refTree("{{slot \"ghost\"}}"), withPrologue())
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), "ghost",
				"the finding names the slot the body never declared")
			assert.Contains(t, body, "\tpro()\n",
				"and the fallback still places the pending content")
		})
	})

	t.Run("places the pending slots a marker asks for", func(t *testing.T) {
		t.Parallel()

		t.Run("the catch-all takes the named slots too", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Declare("checks").Append(call("named"))
			body, sink := renderRef(t, refTree("\tmid()\n{{slots}}"), b)
			assert.False(t, sink.Failed(), "a placed marker is valid")
			assert.ContainsInOrder(t, body, []string{"mid", "named"},
				"the catch-all places every slot no named marker claimed")
		})

		t.Run("a named marker skips the slots before it", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Declare("first").Append(call("early"))
			b.Declare("second").Append(call("late"))
			body, sink := renderRef(t, refTree("{{slot \"second\"}}{{slots}}"), b)
			assert.False(t, sink.Failed(), "named markers are valid")
			assert.ContainsInOrder(t, body, []string{"late", "early"},
				"the named slot arrives where the template says, the rest after")
		})

		t.Run("an unplaced named slot counts against the marker rule", func(t *testing.T) {
			t.Parallel()

			b := refBody()
			b.Declare("checks").Append(call("named"))
			body, sink := renderRef(t, refTree("\tbare()\n"), b)
			assert.Contains(t, reported(t, sink, render.DroppedSlots), "1 pending",
				"the finding counts the statements no marker placed")
			assert.NotContains(t, body, "named",
				"and nothing is appended for a template that owns its layout")
		})

		markers := []struct {
			name string
			tpl  string
			body func() emit.Body
		}{
			{
				name: "the catch-all over a refused prologue",
				tpl:  "{{slots}}",
				body: func() emit.Body {
					b := refBody()
					b.Prologue.Append(refused())
					return b
				},
			},
			{
				name: "the catch-all over a refused named slot",
				tpl:  "{{slots}}",
				body: func() emit.Body {
					b := refBody()
					b.Declare("checks").Append(refused())
					return b
				},
			},
			{
				name: "the catch-all over a refused epilogue",
				tpl:  "{{slots}}",
				body: func() emit.Body {
					b := refBody()
					b.Epilogue.Append(refused())
					return b
				},
			},
			{
				name: "a named marker over a refused slot",
				tpl:  "{{slot \"checks\"}}",
				body: func() emit.Body {
					b := refBody()
					b.Declare("checks").Append(refused())
					return b
				},
			},
		}
		for _, tt := range markers {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				body, sink := renderRef(t, refTree(tt.tpl), tt.body())
				assert.Contains(t, reported(t, sink, render.RefusedTemplate),
					refName, "the printer's refusal reaches the referencing template")
				assert.Equal(t, body, "",
					"the refusal withholds the whole declaration, "+
						"so no half-opened shape reaches the file")
			})
		}
	})

	t.Run("clusters claim a declaration once", func(t *testing.T) {
		t.Parallel()

		grouped := func(c render.Cluster) render.Language {
			l := language()
			l.Cluster = c
			l.Groups = map[render.GroupName]string{
				"first":  "first(\n{{range .Decls}}\t{{.Name}}\n{{end}})\n",
				"second": "second(\n{{range .Decls}}\t{{.Name}}\n{{end}})\n",
			}
			return l
		}

		t.Run("a member the unit does not hold is ignored", func(t *testing.T) {
			t.Parallel()

			stranger := &emit.Struct{
				Origin: coretest.Struct(coretest.StorePath, "Stranger").ID,
				Name:   "Stranger",
			}
			l := grouped(func([]symbol.Symbol) []render.Clustered {
				return []render.Clustered{{
					Group: "first", Decls: []symbol.Symbol{stranger},
				}}
			})
			files, sink := runPass(t, l, seeded(t,
				unitOf("gen", "store.go", "Alpha", "Beta")))
			assert.False(t, sink.Failed(), "an outside claim is ignored, not reported")
			assert.Equal(t, string(files[0].Body),
				"type Alpha struct{}\ntype Beta struct{}\n",
				"every declaration renders as the singleton it stayed")
		})

		t.Run("a declaration two clusters claim goes to the first", func(t *testing.T) {
			t.Parallel()

			l := grouped(func(decls []symbol.Symbol) []render.Clustered {
				return []render.Clustered{
					{Group: "first", Decls: decls[:1]},
					{Group: "second", Decls: decls[:1]},
				}
			})
			files, sink := runPass(t, l, seeded(t,
				unitOf("gen", "store.go", "Alpha", "Beta")))
			assert.False(t, sink.Failed(), "the second claim is dropped silently")
			assert.Equal(t, string(files[0].Body),
				"first(\n\tAlpha\n)\ntype Beta struct{}\n",
				"the first cluster renders it, and the second renders nothing")
		})

		t.Run("a group template refusing its cluster reports", func(t *testing.T) {
			t.Parallel()

			l := grouped(func(decls []symbol.Symbol) []render.Clustered {
				return []render.Clustered{{Group: "first", Decls: decls}}
			})
			l.Groups["first"] = "{{.Missing}}"
			files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), "group template",
				"the finding names the template that refused")
			assert.Equal(t, string(files[0].Body), "",
				"and the cluster's declarations render nowhere")
		})
	})

	t.Run("nests through the member's own kind template", func(t *testing.T) {
		t.Parallel()

		hosting := func() render.Language {
			l := language()
			l.Kinds[symbol.KindStruct] = "type {{.Name}} {\n" +
				"{{- range .Types.Items}}\n{{nested \"\t\" .}}{{- end}}\n}\n"
			return l
		}
		hostOf := func(inner symbol.Symbol) plugin.Unit {
			host := &emit.Struct{Name: "Row"}
			host.Types.Append(inner)
			u := unitOf("gen", "store.go")
			u.Decls = append(u.Decls, host)
			return u
		}

		t.Run("a member's refusal propagates to its host", func(t *testing.T) {
			t.Parallel()

			l := hosting()
			l.Kinds[symbol.KindEnum] = "{{.Missing}}"
			_, sink := runPass(t, l, seeded(t, hostOf(&emit.Enum{Name: "Phase"})))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), "Missing",
				"a host never renders around a half-spelt member")
		})

		t.Run("a member spelling nothing indents nothing", func(t *testing.T) {
			t.Parallel()

			l := hosting()
			l.Kinds[symbol.KindEnum] = ""
			files, sink := runPass(t, l, seeded(t, hostOf(&emit.Enum{Name: "Phase"})))
			assert.False(t, sink.Failed(), "an empty spelling is a spelling")
			assert.Equal(t, string(files[0].Body), "type Row {\n\n}\n",
				"the block carries no indentation of its own")
		})
	})

	t.Run("a skeleton refusing a file withholds it", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.File = "{{.Missing}}"
		files, sink := runPass(t, l, seeded(t, unitOf("gen", "store.go", "Alpha")))
		assert.Contains(t, reported(t, sink, render.RefusedTemplate), "store_stub.txt",
			"the finding names the file the skeleton refused")
		assert.Length(t, files, 0, "and the sink never receives a half-assembled file")
	})
}

// BenchmarkPass measures the procedure at the canonical scale: 1000
// per-package files of 200 declarations, 200k template executions,
// through a pass-through formatter, so the number is the pass and
// the engine, not a real language's spelling.
func BenchmarkPass(b *testing.B) {
	const packages, decls = 1_000, 200
	e := plugin.NewEmit()
	for i := range packages {
		path := "example.com/pkg" + strconv.Itoa(i)
		u := unitOf("gen", path+"/store.go")
		u.Pkg = coretest.Struct(path, "Anchor").ID
		for d := range decls {
			u.Decls = append(u.Decls, &emit.Struct{
				Origin: coretest.Struct(path, "S"+strconv.Itoa(d)).ID,
				Name:   "S" + strconv.Itoa(d),
			})
		}
		if err := e.Add(u); err != nil {
			b.Fatalf("Add: unexpected error: %v", err)
		}
	}
	p, err := render.New("printer", language())
	if err != nil {
		b.Fatalf("New: unexpected error: %v", err)
	}

	c := bench.Start(b).MaxAllocs(680_000)
	defer c.End()
	for c.Loop() {
		sink := diag.NewSink()
		files, err := p.Render(&plugin.RenderContext{
			Emit: e, Sink: sink, Plugin: "printer",
		})
		if err != nil {
			b.Fatalf("Render: unexpected error: %v", err)
		}
		if len(files) != packages || sink.Failed() {
			b.Fatal("every file renders clean")
		}
	}
}

// BenchmarkPassReferences measures the body-claiming path at the
// same scale: 1000 files of 200 functions, each body a reference to
// one template in the emitting plugin's tree, so every declaration
// executes a parsed reference template.
func BenchmarkPassReferences(b *testing.B) {
	const packages, decls = 1_000, 200
	e := plugin.NewEmit()
	for i := range packages {
		path := "example.com/pkg" + strconv.Itoa(i)
		u := unitOf("gen", path+"/store.go")
		u.Pkg = coretest.Struct(path, "Anchor").ID
		for d := range decls {
			f := &emit.Function{
				Origin: coretest.Struct(path, "F"+strconv.Itoa(d)).ID,
				Name:   "F" + strconv.Itoa(d),
			}
			f.Body = refBody()
			u.Decls = append(u.Decls, f)
		}
		if err := e.Add(u); err != nil {
			b.Fatalf("Add: unexpected error: %v", err)
		}
	}
	p, err := render.New("printer", language())
	if err != nil {
		b.Fatalf("New: unexpected error: %v", err)
	}
	trees := refTree("\tref()\n{{slots}}")

	c := bench.Start(b).MaxAllocs(3_500_000)
	defer c.End()
	for c.Loop() {
		sink := diag.NewSink()
		files, err := p.Render(&plugin.RenderContext{
			Emit: e, Trees: trees, Sink: sink, Plugin: "printer",
		})
		if err != nil {
			b.Fatalf("Render: unexpected error: %v", err)
		}
		if len(files) != packages || sink.Failed() {
			b.Fatal("every file renders clean")
		}
	}
}
