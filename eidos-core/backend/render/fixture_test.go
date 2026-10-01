// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"errors"
	"io/fs"
	"path"
	"strings"
	"testing/fstest"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fixture's names: the pass, the plugin that emits, the routing
// key it emits under, the file the naming spells for that key, and
// the declarations the cases render.
const (
	// passName is the identity every fixture pass composes under.
	passName plugin.ID = "printer"
	// emitter is the plugin a fixture unit comes from.
	emitter plugin.ID = "gen"
	// storeKey is the routing key a fixture unit emits under.
	storeKey = "store.go"
	// storeFile is the filename [stubNaming] spells for storeKey.
	storeFile = "store_stub.txt"
	// alphaName and betaName name the fixture's structs.
	alphaName = "Alpha"
	betaName  = "Beta"
	// handleName names the fixture's callables.
	handleName = "Handle"
	// blockGroup names the group template a fixture cluster selects.
	blockGroup render.GroupName = "block"
	// missingField is a field no fixture value has, so failing, the
	// template that reads it, fails at execute time.
	missingField = "Missing"
	failing      = "{{." + missingField + "}}"
	// refusalReason is the reason a fixture language states for a
	// kind it refuses.
	refusalReason = "the fixture language declares no such construct"
)

// The binding fixture: a helper that qualifies a name through the
// file's import set, the packages it qualifies with, and the name the
// first package binds.
const (
	// qualifyHelper, itemHelper and claimHelper are the vocabulary
	// helpers [qualifying] declares.
	qualifyHelper = "qualify"
	itemHelper    = "item"
	claimHelper   = "claim"
	// storePkg and legacyPkg are two packages whose last segment is
	// storeLocal, so the second to bind takes a suffix.
	storePkg  = "svc/store"
	legacyPkg = "legacy/store"
	// storeLocal is the name storePkg binds when no other binding
	// takes it.
	storeLocal = "store"
	// rowName is the declaration a qualified spelling names.
	rowName = "Row"
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

// The fixture language's spellings: a struct, and a function that
// places its body.
const (
	structTpl   = "type {{.Name}} struct{}\n"
	functionTpl = "func {{.Name}}() {\n{{body .}}}\n"
)

// language returns the smallest valid language: a struct
// spelling, a callable spelling that places its body, a scaffold
// printer for names and returns, and a pass-through formatter.
func language() render.Language {
	return render.Language{
		Kinds: map[symbol.Kind]string{
			symbol.KindStruct:   structTpl,
			symbol.KindFunction: functionTpl,
		},
		Naming:   stubNaming,
		Scaffold: scaffold,
		Imports:  pathImports,
		Finalise: func(src []byte) ([]byte, error) { return src, nil },
	}
}

// pathImports renders the set's paths on one line, and nothing for an
// empty set.
func pathImports(set *render.ImportSet) string {
	if set.Len() == 0 {
		return ""
	}
	return "import (" + strings.Join(set.Paths(), " ") + ")\n"
}

// action spells one template action over the given words, so a
// fixture template names a builtin or a helper through its constant.
func action(words ...string) string {
	return "{{" + strings.Join(words, " ") + "}}"
}

// helpers returns a vocabulary that binds nothing to the file's
// import set: every file calls the same functions.
func helpers(fm template.FuncMap) func(*render.ImportSet) template.FuncMap {
	return func(*render.ImportSet) template.FuncMap { return fm }
}

// qualifying is a vocabulary bound to the file's import set, the way a
// backend's speller is:
//
//   - qualify binds a package under the package's last segment and
//     spells a name through the bound name, or bare where the package
//     is the file's own.
//   - item imports one declaration and spells the name it binds.
//   - claim claims one declaration's simple name and spells the name,
//     or the path and the name where another binding takes it.
func qualifying(set *render.ImportSet) template.FuncMap {
	return template.FuncMap{
		qualifyHelper: func(pkg, name string) string {
			local := set.Bind(pkg, path.Base(pkg))
			if local == "" {
				return name
			}
			return local + "." + name
		},
		itemHelper: func(pkg, name string) string {
			return set.BindItem(pkg, name, false)
		},
		claimHelper: func(pkg, name string) string {
			if set.Claim(pkg, name) {
				return name
			}
			return pkg + "." + name
		},
	}
}

// binding returns the fixture language with the [qualifying]
// vocabulary and an import block that writes every entry on a line
// of its own, the bound name beside the path.
func binding() render.Language {
	l := language()
	l.Funcs = qualifying
	l.Imports = namedImports
	return l
}

// namedImports renders every entry on a line of its own, the bound
// name beside the path, so a case reads which name each import binds.
func namedImports(set *render.ImportSet) string {
	var b strings.Builder
	for _, e := range set.Entries() {
		b.WriteString("use " + e.Path + " as " + e.Name + "\n")
	}
	return b.String()
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

// unspellable returns a statement whose value the fixture language
// has no form for.
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

// fn returns a per-source unit containing one function whose body is
// body.
func fn(key, name string, body emit.Body) plugin.Unit {
	u := unitOf(emitter, key)
	f := &emit.Function{
		Origin: coretest.Struct(coretest.StorePath, name).ID,
		Name:   name,
	}
	f.Body = body
	u.Decls = append(u.Decls, f)
	return u
}

// besideAlpha returns u with the struct alphaName before its
// declarations, so a case that skips a declaration of u still has a
// file to read.
func besideAlpha(u plugin.Unit) plugin.Unit {
	u.Decls = append([]symbol.Symbol{unitOf(emitter, u.Key, alphaName).Decls[0]}, u.Decls...)
	return u
}

// unitOf returns one flushed unit of structs with the given names.
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

// seeded returns an emit store containing the given units.
func seeded(tb assert.TB, units ...plugin.Unit) *plugin.Emit {
	tb.Helper()

	e := plugin.NewEmit()
	for _, u := range units {
		assert.NoError(tb, e.Add(u), "the fixture unit is added")
	}
	return e
}

// runPass builds the pass over the language and renders the store,
// routed into files through the pass's own filename half.
func runPass(
	tb assert.TB, l render.Language, e *plugin.Emit,
) ([]plugin.RenderedFile, *diag.Sink) {
	tb.Helper()

	p, err := render.New(passName, l)
	assert.NoError(tb, err, "the language composes")
	sink := diag.NewSink()
	files, err := p.Render(&plugin.RenderContext{
		Emit: e, Files: backendtest.Files(e, p), Sink: sink, Plugin: passName,
	})
	assert.NoError(tb, err, "the pass renders every file")
	return files, sink
}

// refName is the template every reference-form fixture body names.
const refName = "method1.tpl"

// refTree returns the emitting plugin's tree containing src under
// [refName].
func refTree(src string) map[plugin.ID]fs.FS {
	return map[plugin.ID]fs.FS{
		emitter: fstest.MapFS{refName: &fstest.MapFile{Data: []byte(src)}},
	}
}

// refBody returns a body referencing [refName] and nothing else, so
// a case adds only the slots it tests.
func refBody() emit.Body {
	return emit.Body{Ref: &emit.TemplateRef{Name: refName}}
}

// refused returns a statement the fixture's scaffold cannot spell:
// the printer failure an error path needs.
func refused() emit.Stmt {
	return emit.Stmt{Kind: emit.StmtGuard, Name: "err"}
}

// method returns a per-source unit containing one method whose body
// is body, the second callable kind the body builtin takes.
func method(key, name string, body emit.Body) plugin.Unit {
	u := unitOf(emitter, key)
	m := &emit.Method{
		Origin: coretest.Method(coretest.StorePath, coretest.StructName, name).ID,
		Name:   name,
	}
	m.Body = body
	u.Decls = append(u.Decls, m)
	return u
}

// renderRef renders one function whose body is b through the
// language l, with trees as the emitting plugin's template trees, and
// returns the file's bytes beside the run's findings. An empty result
// means the file was withheld.
func renderRef(
	tb assert.TB, l render.Language, trees map[plugin.ID]fs.FS, b emit.Body,
) (string, *diag.Sink) {
	tb.Helper()

	p, err := render.New(passName, l)
	assert.NoError(tb, err, "the language composes")
	sink := diag.NewSink()
	e := seeded(tb, fn(storeKey, handleName, b))
	files, err := p.Render(&plugin.RenderContext{
		Emit: e, Files: backendtest.Files(e, p),
		Trees: trees, Sink: sink, Plugin: passName,
	})
	assert.NoError(tb, err, "the pass renders every file")
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
