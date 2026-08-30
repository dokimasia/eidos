// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package render_test

import (
	"errors"
	"path"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
)

// stubNaming spells every unit as word, key stem and a fixture
// extension: the shape a target's naming answers.
func stubNaming(u plugin.Unit) string {
	stem := strings.TrimSuffix(path.Base(u.Key), ".go")
	if stem == "." {
		stem = "plan"
	}
	return stem + "_" + u.Word + ".txt"
}

// language answers the smallest lawful language: a struct
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
		Finalise: func(src []byte) ([]byte, error) { return src, nil },
	}
}

// scaffold spells the two statement kinds the fixtures use: a bare
// name evaluated for effect, and a return.
func scaffold(s emit.Stmt) ([]byte, error) {
	switch s.Kind {
	case emit.StmtExpr:
		return []byte("\t" + s.Value.Name + "()\n"), nil
	case emit.StmtReturn:
		return []byte("\treturn\n"), nil
	default:
		return nil, errors.New("the fixture spells names and returns only")
	}
}

// call answers the one-line scaffold statement naming n.
func call(n string) emit.Stmt {
	return emit.Stmt{Kind: emit.StmtExpr, Value: emit.Expr{Kind: emit.ExprName, Name: n}}
}

// fn answers a per-source unit holding one function carrying body.
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

// unitOf answers one flushed unit carrying structs.
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

// seeded answers an emit store holding the given units.
func seeded(tb assert.TB, units ...plugin.Unit) *plugin.Emit {
	tb.Helper()

	e := plugin.NewEmit()
	for _, u := range units {
		assert.NoError(tb, e.Add(u), "the fixture unit lands")
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

// The pass is the ritual every language shares: group through the
// naming, render kinds in canonical order, finalise per file and
// continue past a failure.
func TestPass(t *testing.T) {
	t.Parallel()

	t.Run("groups units into files through the naming", func(t *testing.T) {
		t.Parallel()

		files, sink := runPass(t, language(), seeded(t,
			unitOf("gen", "store.go", "Alpha"),
			unitOf("gen", "user.go", "Beta"),
		))
		assert.False(t, sink.Failed(), "nothing to report")
		assert.Length(t, files, 2, "one file per name")
		assert.Equal(t, files[0].Name, "store_stub.txt",
			"files answer in name order")
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
			"contributions land in unit order, which is total")
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
			assert.False(t, sink.Failed(), "the default form is lawful")
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
			assert.False(t, sink.Failed(), "the scaffold form is lawful")
			assert.ContainsInOrder(t, string(files[0].Body),
				[]string{"pro", "content", "named", "epi"},
				"prologue, content, named slots, epilogue: the fixed composition")
		})

		t.Run("verbatim lands literally", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, language(), seeded(t,
				fn("store.go", "Handle", emit.Body{Verbatim: "\treturn nil\n"})))
			assert.False(t, sink.Failed(), "the verbatim form is lawful")
			assert.Contains(t, string(files[0].Body), "\treturn nil\n",
				"byte for byte, the sharp knife")
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

	t.Run("two runs answer the same bytes", func(t *testing.T) {
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
}

// BenchmarkPass prices the ritual at the canonical scale: 1000
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

	b.ReportAllocs()
	for b.Loop() {
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
