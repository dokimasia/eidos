// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"errors"
	"io/fs"
	"path"
	"strings"
	"testing/fstest"

	"go.dokimi.dev/assert"

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
