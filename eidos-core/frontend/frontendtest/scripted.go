// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest

import (
	"context"
	"errors"
	"io/fs"
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

// ScriptedLang is the language of every scripted declaration.
const ScriptedLang symbol.Lang = "fake"

// ScriptedID is the name the scripted frontend declares, and the
// origin of every finding it reports.
const ScriptedID plugin.ID = "fakefront"

// ScriptedTestKey is the classification key the scripted stamps
// write.
const ScriptedTestKey meta.KeyName = "fake.testFile"

// ScriptedBadFile is the scripted frontend's one finding: a file
// with no package line declares nothing.
var ScriptedBadFile = diag.MustRegister(diag.Prefix("FAKE"), diag.CodeSpec{
	Number:  1,
	Meaning: "a fake file opens without a package line",
})

// ScriptedStore is the store the scripted language in the dependent
// role reads its dependency units from.
const ScriptedStore = "fake"

// ScriptedPublish is the import alias whose packages a file of the
// scripted language in the exporter role publishes.
const ScriptedPublish = "pub"

// scriptedExt is the suffix of every scripted source file, and
// scriptedManifest the name of the manifest at the tree's root.
const (
	scriptedExt      = ".zz"
	scriptedManifest = "mod" + scriptedExt
)

// ScriptedOptions is the scripted frontend's declared
// configuration.
type ScriptedOptions struct {
	Tag string
}

// Scripted is the language the suite proves itself on and any
// consumer can drive. Its grammar is small, and it exercises every
// load phase: packages, bindings, cross-package references, type
// parameters, members, directives, classification stamps and a
// signature-sensitive declaration. One statement per line:
//
//	package PATH          the file's package path
//	import ALIAS PATH...  bind an alias to one or more packages, one
//	                      import record per path
//	type NAME REF...      a struct, fields f0..fn typed by the refs
//	typeparam NAME        a type parameter on the last type
//	method NAME REF...    a method on the last type, params by ref
//	on NAME               the last type is NAME, which an earlier
//	                      member of the unit declared: the methods
//	                      after it fold onto that type the way a Go
//	                      receiver folds a method from another file
//	const name            a constant; skipped at signature depth
//	+NAME ARGS            a directive on the last type
//	// TEXT               a comment, split by the kernel: its
//	                      documentation and its carrier lines under
//	                      the load's brand attach to the next type,
//	                      and a tool:name line lowers as an
//	                      annotation. Above the package line it is
//	                      the file's header and lowers to nothing
//	stamp KEY VALUE       a classification stamp on the file
//	pkgnote NAME ARGS     a directive on the package node itself
//
// It partitions by directory, with one shared input when the tree
// has mod.zz at its root. The fields are open so a test can rename,
// re-version, re-claim or re-tag it, or declare it a language that
// cannot overload.
type Scripted struct {
	ID   plugin.ID
	Ver  string
	Sel  []string
	Opts *ScriptedOptions
	// Overloading is what Overloads reports. A method's parameters
	// are single-token spellings, which need no normalizing.
	Overloading bool
}

// NewScripted returns the scripted frontend under its usual claim,
// as a language that overloads.
func NewScripted() *Scripted {
	return &Scripted{
		ID:  ScriptedID,
		Ver: "1",
		// The manifest is a shared input, never source: the claim
		// carves it out and the partition reads it instead.
		Sel:         []string{"**/*" + scriptedExt, "!" + scriptedManifest, "!**/skip/**"},
		Opts:        &ScriptedOptions{Tag: "steady"},
		Overloading: true,
	}
}

// ScriptedKeys registers the classification key the scripted stamps write,
// in the shape a suite fixture declares its keys.
func ScriptedKeys(r *meta.Registry) error {
	if err := r.ClaimNamespace(ScriptedTestKey.Namespace()); err != nil {
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

// Lang returns the language of every scripted declaration.
func (*Scripted) Lang() symbol.Lang { return ScriptedLang }

// Version returns the declared version, which every unit key folds.
func (f *Scripted) Version() string { return f.Ver }

// Options returns the declared configuration.
func (f *Scripted) Options() any { return f.Opts }

// Selection returns the file claim.
func (f *Scripted) Selection() []string { return f.Sel }

// Syntax returns the language's one comment form with the tool
// directive convention declared. The scripted language reads its
// comments through the kernel's split, so a regression in the split
// fails the kernel's own tests as well as a satellite's.
func (*Scripted) Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{Line: []string{"//"}, Directives: true}
}

// Overloads reports the declared [Scripted.Overloading].
func (f *Scripted) Overloads() bool { return f.Overloading }

// Partition groups by directory, in path order, and declares the
// tree's mod.zz a shared input of every unit when it exists.
func (*Scripted) Partition(
	_ context.Context, files []plugin.SourceRef, r plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	var shared []string
	if _, err := r.Read(scriptedManifest); err == nil {
		shared = []string{scriptedManifest}
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

// Parse reads each member line by line into the unit's builder. The
// members share one table of the types they declare, so an on
// statement in one member finds a type an earlier member declared.
func (f *Scripted) Parse(_ context.Context, u *plugin.SourceUnit) error {
	types := map[string]*node.Struct{}
	for _, ref := range u.Files() {
		if err := f.parseMember(u, ref.Path, types); err != nil {
			return err
		}
	}
	return nil
}

// ParseFile reads and lowers one member, so a test can drive a
// unit partially, the shape a broken frontend takes. The member sees
// no type another member declared.
func (f *Scripted) ParseFile(u *plugin.SourceUnit, path string) error {
	return f.parseMember(u, path, map[string]*node.Struct{})
}

// Resolve probes the file's bindings in one tier: "alias.Name"
// through each package the alias binds, a capitalized bare spelling
// in the file's own package, and nothing for any other spelling,
// which is a builtin.
func (*Scripted) Resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	bindings, _ := scope.Bindings.(map[string][]string)
	if alias, name, qualified := strings.Cut(spelling, "."); qualified {
		var tier []symbol.Identity
		for _, pkg := range bindings[alias] {
			tier = append(tier, symbol.Identity{Lang: ScriptedLang, Package: pkg, Name: name})
		}
		if len(tier) == 0 {
			return nil
		}
		return plugin.Candidates{tier}
	}
	if spelling[0] >= 'A' && spelling[0] <= 'Z' {
		return plugin.Candidates{{{Lang: ScriptedLang, Package: scope.File.Package, Name: spelling}}}
	}
	return nil
}

// parseMember reads one member and lowers it against the unit's
// table of declared types.
func (f *Scripted) parseMember(u *plugin.SourceUnit, path string, types map[string]*node.Struct) error {
	b, err := u.Read(path)
	if err != nil {
		return err
	}
	f.parseFile(u, path, string(b), types)
	return nil
}

// parseFile lowers one file's statements, recording each type it
// declares in the unit's table.
func (*Scripted) parseFile(u *plugin.SourceUnit, filePath, content string, types map[string]*node.Struct) {
	gb := u.Graph()
	var (
		file        *node.File
		pkgPath     string
		bindings    = map[string][]string{}
		last        *node.Struct
		pending     []string
		annotations symbol.Annotations
		waiting     []directive.Raw
	)
	for i, line := range strings.Split(content, "\n") {
		at := position.Pos{File: filePath, Line: i + 1}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch {
		case fields[0] == "package" && len(fields) == 2:
			pkgPath = fields[1]
			pkg := gb.Package(pkgPath)
			pkg.Name = path.Base(pkgPath)
			file = &node.File{Path: filePath, Pos: at}
			pkg.Files = append(pkg.Files, file)
		case strings.HasPrefix(fields[0], "//") && file == nil:
			// A comment above the package line is the file's header,
			// which contains a generated file's frame, and lowers to
			// nothing.
		case file == nil:
			u.Errorf(ScriptedBadFile, at, "%s opens with %q, not a package line", filePath, fields[0])
			return
		case fields[0] == "import" && len(fields) >= 3:
			bindings[fields[1]] = append(bindings[fields[1]], fields[2:]...)
			for _, imported := range fields[2:] {
				file.Imports = append(file.Imports, &node.Import{Path: imported, Alias: fields[1], Pos: at})
			}
		case fields[0] == "type" && len(fields) >= 2:
			last = &node.Struct{
				Name: fields[1], Pos: at,
				Doc: pending, Annotations: annotations,
			}
			for _, raw := range waiting {
				gb.Attach(last, raw)
			}
			pending, annotations, waiting = nil, nil, nil
			for j, spelling := range fields[2:] {
				last.Fields = append(last.Fields, &node.Field{
					Name: "f" + string(rune('0'+j)),
					Pos:  at,
					Type: &node.TypeRef{Spelling: spelling, Pos: at},
				})
			}
			file.Decls = append(file.Decls, last)
			types[last.Name] = last
		case fields[0] == "on" && len(fields) == 2:
			// The methods after it fold onto a type another member
			// declared: each method's position names this file, and
			// the type's names the other.
			last = types[fields[1]]
			if last == nil {
				u.Errorf(ScriptedBadFile, at, "%s folds onto %s, which no earlier member declares", filePath, fields[1])
			}
		case fields[0] == "typeparam" && len(fields) == 2 && last != nil:
			last.TypeParams = append(last.TypeParams, &node.TypeParam{Name: fields[1], Pos: at})
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
		case fields[0] == "pkgnote" && len(fields) >= 2:
			// A directive on the package node itself, so the suite
			// can check that the splice re-homes the records of every
			// unit of a merged package.
			raw, err := directive.Parse(strings.Join(fields[1:], " "))
			if err != nil {
				u.Errorf(ScriptedBadFile, at, "%s has a package directive outside the grammar: %v", filePath, err)
				continue
			}
			raw.Pos = at
			gb.Attach(gb.Package(pkgPath), raw)
		case strings.HasPrefix(fields[0], "//"):
			// The kernel's own split decides what a comment contains,
			// so the reference language exercises it: documentation
			// and carriers attach to the next declaration, and a tool
			// directive lowers as an annotation.
			parts := u.Comment(line, at)
			pending = append(pending, parts.Docs...)
			annotations = append(annotations, parts.Annotations...)
			for _, c := range parts.Carriers {
				raw, err := directive.Parse(c.Payload)
				if err != nil {
					u.Errorf(ScriptedBadFile, c.Pos,
						"%s has a directive outside the grammar: %v", filePath, err)
					continue
				}
				raw.Pos = c.Pos
				raw.Negated = c.Negated()
				raw.DirectiveShaped = c.DirectiveShaped
				// A comment opens the declaration under it, so its
				// carrier waits for the type it documents.
				waiting = append(waiting, raw)
			}
		case strings.HasPrefix(fields[0], "+") && last != nil:
			raw, err := directive.Parse(strings.TrimPrefix(strings.TrimSpace(line), "+"))
			if err != nil {
				u.Errorf(ScriptedBadFile, at, "%s has a directive outside the grammar: %v", filePath, err)
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

// ScriptedDependent is the scripted language in the
// [plugin.Dependent] role. A need resolves to the scripted files of
// the directory its path names in [ScriptedStore], one unit per need.
// A need the store has no scripted file for yields no unit, and the
// round reports it placed nowhere, so a load without the store loads
// no dependency and reports every need.
type ScriptedDependent struct {
	*Scripted
}

// NewScriptedDependent returns the scripted language in the dependent
// role, under its usual claim.
func NewScriptedDependent() *ScriptedDependent {
	return &ScriptedDependent{Scripted: NewScripted()}
}

// Dependencies returns one unit per need whose directory the store
// lists, its members the directory's scripted files in name order, and
// reports every other need placed nowhere: the store absent, the
// directory absent, or a directory without a scripted file.
func (*ScriptedDependent) Dependencies(
	_ context.Context, round *plugin.DependencyRound, r plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	var out [][]plugin.SourceRef
	for _, need := range round.Needs {
		dir := plugin.StorePath(ScriptedStore, need.Path)
		entries, err := r.ReadDir(dir)
		switch {
		case errors.Is(err, plugin.ErrStoreAbsent):
			round.Unplace(need.Path, "the load provides no "+ScriptedStore+" store")
			continue
		case errors.Is(err, fs.ErrNotExist):
			round.Unplace(need.Path, "no directory is at "+dir)
			continue
		case err != nil:
			return nil, err
		}
		var members []plugin.SourceRef
		for _, e := range entries {
			if !e.IsDir() && path.Ext(e.Name()) == scriptedExt {
				members = append(members, plugin.SourceRef{Path: dir + "/" + e.Name()})
			}
		}
		if len(members) == 0 {
			round.Unplace(need.Path, dir+" has no scripted file")
			continue
		}
		out = append(out, members)
	}
	return out, nil
}

// ScriptedExporter is the scripted language in the [plugin.Exporter]
// role: a file publishes every name of each package its
// [ScriptedPublish] alias binds, the shape of TypeScript's export *.
type ScriptedExporter struct {
	*Scripted
}

// NewScriptedExporter returns the scripted language in the exporter
// role, under its usual claim.
func NewScriptedExporter() *ScriptedExporter {
	return &ScriptedExporter{Scripted: NewScripted()}
}

// Exports returns, as one tier, the name in each package the file's
// publishing alias binds, and nothing for a file that binds none.
func (*ScriptedExporter) Exports(scope plugin.ImportScope, name string) plugin.Candidates {
	bindings, _ := scope.Bindings.(map[string][]string)
	var tier []symbol.Identity
	for _, pkg := range bindings[ScriptedPublish] {
		tier = append(tier, symbol.Identity{Lang: ScriptedLang, Package: pkg, Name: name})
	}
	if len(tier) == 0 {
		return nil
	}
	return plugin.Candidates{tier}
}
