// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest

import (
	"context"
	"path"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// ScriptedLang is the language every scripted declaration carries.
const ScriptedLang symbol.Lang = "fake"

// ScriptedTestKey is the classification key the scripted stamps
// write.
const ScriptedTestKey meta.KeyName = "fake.testFile"

// ScriptedBadFile is the scripted frontend's one finding: a file
// with no package line declares nothing.
var ScriptedBadFile = diag.MustRegister(diag.Prefix("FAKE"), diag.CodeSpec{
	Number:  1,
	Meaning: "a fake file opens without a package line",
})

// ScriptedOptions is the scripted frontend's declared
// configuration.
type ScriptedOptions struct {
	Tag string
}

// Scripted is the language the suite proves itself on and any
// consumer can drive: small enough to hold in the head, wide
// enough to reach every phase — packages, bindings, cross-package
// references, members, directives, classification stamps and a
// signature-sensitive declaration. One statement per line:
//
//	package PATH          the file's package path
//	import ALIAS PATH...  bind an alias to one or more packages
//	type NAME REF...      a struct, fields f0..fn typed by the refs
//	method NAME REF...    a method on the last type, params by ref
//	const name            a constant; skipped at signature depth
//	+NAME ARGS            a directive on the last type
//	stamp KEY VALUE       a classification stamp on the file
//
// It partitions by directory, one shared input when the tree
// carries mod.zz at its root, and the fields are open so a test
// can rename, re-version, re-claim or re-tag it.
type Scripted struct {
	ID   plugin.ID
	Ver  string
	Sel  []string
	Opts *ScriptedOptions
}

// NewScripted returns the scripted frontend under its usual claim.
func NewScripted() *Scripted {
	return &Scripted{
		ID:  "fakefront",
		Ver: "1",
		// The manifest is a shared input, never source: the claim
		// carves it out and the partition reads it instead.
		Sel:  []string{"**/*.zz", "!mod.zz", "!**/skip/**"},
		Opts: &ScriptedOptions{Tag: "steady"},
	}
}

// ScriptedKeys registers the classification key the scripted stamps write,
// in the shape a suite fixture declares its keys.
func ScriptedKeys(r *meta.Registry) error {
	if err := r.ClaimNamespace("fake", "fake"); err != nil {
		return err
	}
	_, err := meta.Register[string](r, meta.KeySpec{
		Name:  ScriptedTestKey,
		Kinds: []symbol.Kind{symbol.KindFile},
		Doc:   "marks a file the scripted language stamps as a test",
	})
	return err
}

// ScriptedSchemas declares the one directive the scripted carriers write,
// in the shape a suite fixture declares its schemas.
func ScriptedSchemas() []directive.Schema {
	return []directive.Schema{{
		Plugin: "gen",
		Name:   "table",
		Params: []directive.ParamSpec{{
			Key:  "name",
			Type: directive.TypeString,
			Doc:  "the table's own name",
		}},
		Doc: "names the table a scripted type maps onto",
	}}
}

// Name returns the declared name.
func (f *Scripted) Name() plugin.ID { return f.ID }

// Lang returns the one language every fake declaration carries.
func (*Scripted) Lang() symbol.Lang { return ScriptedLang }

// Version returns the declared version, which every unit key folds.
func (f *Scripted) Version() string { return f.Ver }

// Options returns the declared configuration.
func (f *Scripted) Options() any { return f.Opts }

// Selection returns the file claim.
func (f *Scripted) Selection() []string { return f.Sel }

// Syntax returns the language's one comment form.
func (*Scripted) Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{Line: []string{"//"}}
}

// Partition groups by directory, in path order, and declares the
// tree's mod.zz a shared input of every unit when it exists.
func (*Scripted) Partition(
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
func (f *Scripted) Parse(_ context.Context, u *plugin.SourceUnit) error {
	for _, ref := range u.Files() {
		if err := f.ParseFile(u, ref.Path); err != nil {
			return err
		}
	}
	return nil
}

// ParseFile reads and lowers one member, so a test can drive a
// unit partially — the shape a broken frontend takes.
func (f *Scripted) ParseFile(u *plugin.SourceUnit, path string) error {
	b, err := u.Read(path)
	if err != nil {
		return err
	}
	f.parseFile(u, path, string(b))
	return nil
}

// Resolve probes the file's bindings: "alias.Name" through each
// package the alias binds, a capitalized bare spelling in the
// file's own package, and anything else is a builtin.
func (*Scripted) Resolve(scope plugin.ImportScope, spelling string) []symbol.Identity {
	bindings, _ := scope.Bindings.(map[string][]string)
	if alias, name, qualified := strings.Cut(spelling, "."); qualified {
		var out []symbol.Identity
		for _, pkg := range bindings[alias] {
			out = append(out, symbol.Identity{Lang: ScriptedLang, Package: pkg, Name: name})
		}
		return out
	}
	if spelling[0] >= 'A' && spelling[0] <= 'Z' {
		return []symbol.Identity{{Lang: ScriptedLang, Package: scope.File.Package, Name: spelling}}
	}
	return nil
}

// parseFile lowers one file's statements.
func (*Scripted) parseFile(u *plugin.SourceUnit, filePath, content string) {
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
			u.Errorf(ScriptedBadFile, at, "%s opens with %q, not a package line", filePath, fields[0])
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
		case fields[0] == "stamp" && len(fields) == 3:
			gb.Stamp(file, meta.RawStamp{
				Key: meta.KeyName(fields[1]), Value: fields[2], Pos: at,
			})
		case strings.HasPrefix(fields[0], "+") && last != nil:
			raw, err := directive.Parse(strings.TrimPrefix(strings.TrimSpace(line), "+"))
			if err != nil {
				u.Errorf(ScriptedBadFile, at, "%s carries a directive outside the grammar: %v", filePath, err)
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
