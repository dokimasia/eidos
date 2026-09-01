// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"context"
	"path"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// fakeLang is the scripted language the tests drive, one statement
// per line:
//
//	package PATH          the file's package path
//	import ALIAS PATH...  bind an alias to one or more packages
//	type NAME REF...      a struct, fields f0..fn typed by the refs
//	method NAME REF...    a method on the last type, params by ref
//	const name            a constant; skipped at signature depth
//	+NAME ARGS            a directive on the last type
const fakeLang symbol.Lang = "fake"

// fakeBadFile is the fake frontend's one finding: a file with no
// package line declares nothing.
var fakeBadFile = diag.MustRegister(diag.Prefix("FAKE"), diag.CodeSpec{
	Number:  1,
	Meaning: "a fake file opens without a package line",
})

// fakeOptions is the fake frontend's declared configuration.
type fakeOptions struct {
	Tag string
}

// fake drives the pipeline in tests: partition by directory, one
// shared input when the tree carries mod.zz at its root.
type fake struct {
	name    plugin.ID
	version string
	sel     []string
	opts    *fakeOptions
}

// newFake returns the fake frontend under its usual claim.
func newFake() *fake {
	return &fake{
		name:    "fakefront",
		version: "1",
		// The manifest is a shared input, never source: the claim
		// carves it out and the partition reads it instead.
		sel:  []string{"**/*.zz", "!mod.zz", "!**/skip/**"},
		opts: &fakeOptions{Tag: "steady"},
	}
}

func (f *fake) Name() plugin.ID     { return f.name }
func (*fake) Lang() symbol.Lang     { return fakeLang }
func (f *fake) Version() string     { return f.version }
func (f *fake) Options() any        { return f.opts }
func (f *fake) Selection() []string { return f.sel }
func (*fake) Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{Line: []string{"//"}}
}

// Partition groups by directory, in path order, and declares the
// tree's mod.zz a shared input of every unit when it exists.
func (*fake) Partition(
	_ context.Context, files []plugin.SourceRef, r plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	var shared []string
	if _, err := r.Read("mod.zz"); err == nil {
		shared = []string{"mod.zz"}
	}
	byDir := map[string][]plugin.SourceRef{}
	var dirs []string
	for _, ref := range files {
		dir := path.Dir(ref.Path)
		if _, met := byDir[dir]; !met {
			dirs = append(dirs, dir)
		}
		byDir[dir] = append(byDir[dir], plugin.SourceRef{Path: ref.Path, Shared: shared})
	}
	out := make([][]plugin.SourceRef, 0, len(dirs))
	for _, dir := range dirs {
		out = append(out, byDir[dir])
	}
	return out, nil
}

// Parse reads each member line by line into the unit's builder.
func (f *fake) Parse(_ context.Context, u *plugin.SourceUnit) error {
	for _, ref := range u.Files() {
		b, err := u.Read(ref.Path)
		if err != nil {
			return err
		}
		f.parseFile(u, ref.Path, string(b))
	}
	return nil
}

// parseFile lowers one file's statements.
func (*fake) parseFile(u *plugin.SourceUnit, filePath, content string) {
	gb := u.Graph()
	var (
		file     *node.File
		bindings = map[string][]string{}
		last     *node.Struct
	)
	for i, line := range strings.Split(content, "\n") {
		at := position.Pos{File: filePath, Line: i + 1}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch {
		case fields[0] == "package" && len(fields) == 2:
			pkg := gb.Package(fields[1])
			pkg.Name = path.Base(fields[1])
			file = &node.File{Path: filePath, Pos: at}
			pkg.Files = append(pkg.Files, file)
		case file == nil:
			u.Errorf(fakeBadFile, at, "%s opens with %q, not a package line", filePath, fields[0])
			return
		case fields[0] == "import" && len(fields) >= 3:
			bindings[fields[1]] = append(bindings[fields[1]], fields[2:]...)
		case fields[0] == "type" && len(fields) >= 2:
			last = &node.Struct{Name: fields[1], Pos: at}
			for j, spelling := range fields[2:] {
				last.Fields = append(last.Fields, &node.Field{
					Name: "f" + string(rune('0'+j)),
					Pos:  at,
					Type: &node.TypeRef{Spelling: spelling, Pos: at},
				})
			}
			file.Decls = append(file.Decls, last)
		case fields[0] == "method" && len(fields) >= 2 && last != nil:
			m := &node.Method{Name: fields[1], Pos: at}
			for _, spelling := range fields[2:] {
				m.Params = append(m.Params, &node.Param{
					Type: &node.TypeRef{Spelling: spelling, Pos: at},
				})
			}
			last.Methods = append(last.Methods, m)
		case fields[0] == "const" && len(fields) == 2:
			if u.Depth() == plugin.DepthSignatures {
				continue
			}
			file.Decls = append(file.Decls, &node.Constant{Name: fields[1], Value: "0", Pos: at})
		case strings.HasPrefix(fields[0], "+") && last != nil:
			raw, err := directive.Parse(strings.TrimPrefix(strings.TrimSpace(line), "+"))
			if err != nil {
				u.Errorf(fakeBadFile, at, "%s carries a directive outside the grammar: %v", filePath, err)
				continue
			}
			raw.Pos = at
			gb.Attach(last, raw)
		}
	}
	if file != nil {
		gb.Scope(file, bindings)
	}
}

// Resolve probes the file's bindings: "alias.Name" through each
// package the alias binds, a capitalized bare spelling in the
// file's own package, and anything else is a builtin.
func (*fake) Resolve(scope plugin.ImportScope, spelling string) []symbol.Identity {
	bindings, _ := scope.Bindings.(map[string][]string)
	if alias, name, qualified := strings.Cut(spelling, "."); qualified {
		var out []symbol.Identity
		for _, pkg := range bindings[alias] {
			out = append(out, symbol.Identity{Lang: fakeLang, Package: pkg, Name: name})
		}
		return out
	}
	if spelling[0] >= 'A' && spelling[0] <= 'Z' {
		return []symbol.Identity{{Lang: fakeLang, Package: scope.File.Package, Name: spelling}}
	}
	return nil
}
